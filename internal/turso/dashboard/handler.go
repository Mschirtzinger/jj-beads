// Package dashboard provides event handling and message formatting for the dashboard.
package dashboard

import (
	"encoding/json"
	"log"
	"time"

	"github.com/steveyegge/beads/internal/turso/schema"
)

// Handler manages daemon event subscriptions and formats them as dashboard messages.
// It bridges between daemon events and the WebSocket server.
type Handler struct {
	server *Server
	logger *log.Logger

	// Statistics tracking
	stats      *StatsData
	statsReady bool
}

// NewHandler creates a new event handler connected to a dashboard server
func NewHandler(server *Server, logger *log.Logger) *Handler {
	if logger == nil {
		logger = log.Default()
	}

	return &Handler{
		server: server,
		logger: logger,
		stats: &StatsData{
			ByStatus: make(map[string]int),
		},
	}
}

// OnTaskCreated handles task creation events
func (h *Handler) OnTaskCreated(task *schema.TaskFile) {
	h.logger.Printf("Task created: %s (%s)", task.ID, task.Title)

	// Update statistics
	h.stats.Total++
	h.stats.ByStatus[task.Status]++
	if task.Status == "in_progress" {
		h.stats.InProgress++
	}

	// Format task update data
	data := TaskUpdateData{
		TaskID:   task.ID,
		Action:   "created",
		Status:   task.Status,
		Title:    task.Title,
		Priority: task.Priority,
		Assignee: task.AssignedAgent,
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		h.logger.Printf("Failed to marshal task data: %v", err)
		return
	}

	// Broadcast message
	msg := Message{
		Type:      MessageTypeTaskUpdate,
		Timestamp: time.Now(),
		Data:      dataJSON,
	}
	h.server.Broadcast(msg)

	// Also broadcast updated stats
	h.broadcastStats()
}

// OnTaskUpdated handles task update events
func (h *Handler) OnTaskUpdated(oldTask, newTask *schema.TaskFile) {
	h.logger.Printf("Task updated: %s (%s)", newTask.ID, newTask.Title)

	// Update statistics
	if oldTask.Status != newTask.Status {
		h.stats.ByStatus[oldTask.Status]--
		h.stats.ByStatus[newTask.Status]++

		if oldTask.Status == "in_progress" {
			h.stats.InProgress--
		}
		if newTask.Status == "in_progress" {
			h.stats.InProgress++
		}
	}

	// Format task update data
	data := TaskUpdateData{
		TaskID:   newTask.ID,
		Action:   "updated",
		Status:   newTask.Status,
		Title:    newTask.Title,
		Priority: newTask.Priority,
		Assignee: newTask.AssignedAgent,
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		h.logger.Printf("Failed to marshal task data: %v", err)
		return
	}

	// Broadcast message
	msg := Message{
		Type:      MessageTypeTaskUpdate,
		Timestamp: time.Now(),
		Data:      dataJSON,
	}
	h.server.Broadcast(msg)

	// Broadcast updated stats
	h.broadcastStats()
}

// OnTaskDeleted handles task deletion events
func (h *Handler) OnTaskDeleted(taskID string, oldTask *schema.TaskFile) {
	h.logger.Printf("Task deleted: %s", taskID)

	// Update statistics
	if oldTask != nil {
		h.stats.Total--
		h.stats.ByStatus[oldTask.Status]--
		if oldTask.Status == "in_progress" {
			h.stats.InProgress--
		}
	}

	// Format task update data
	data := TaskUpdateData{
		TaskID: taskID,
		Action: "deleted",
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		h.logger.Printf("Failed to marshal task data: %v", err)
		return
	}

	// Broadcast message
	msg := Message{
		Type:      MessageTypeTaskUpdate,
		Timestamp: time.Now(),
		Data:      dataJSON,
	}
	h.server.Broadcast(msg)

	// Broadcast updated stats
	h.broadcastStats()
}

// OnDepAdded handles dependency addition events
func (h *Handler) OnDepAdded(dep *schema.DepFile) {
	h.logger.Printf("Dependency added: %s --%s--> %s", dep.From, dep.Type, dep.To)

	// Format dependency update data
	data := DepUpdateData{
		From:   dep.From,
		To:     dep.To,
		Type:   dep.Type,
		Action: "added",
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		h.logger.Printf("Failed to marshal dep data: %v", err)
		return
	}

	// Broadcast message
	msg := Message{
		Type:      MessageTypeDepUpdate,
		Timestamp: time.Now(),
		Data:      dataJSON,
	}
	h.server.Broadcast(msg)
}

// OnDepRemoved handles dependency removal events
func (h *Handler) OnDepRemoved(from, typ, to string) {
	h.logger.Printf("Dependency removed: %s --%s--> %s", from, typ, to)

	// Format dependency update data
	data := DepUpdateData{
		From:   from,
		To:     to,
		Type:   typ,
		Action: "removed",
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		h.logger.Printf("Failed to marshal dep data: %v", err)
		return
	}

	// Broadcast message
	msg := Message{
		Type:      MessageTypeDepUpdate,
		Timestamp: time.Now(),
		Data:      dataJSON,
	}
	h.server.Broadcast(msg)
}

// OnSyncComplete handles full sync completion events
func (h *Handler) OnSyncComplete(tasksProcessed, depsProcessed int, duration time.Duration) {
	h.logger.Printf("Sync complete: %d tasks, %d deps in %v", tasksProcessed, depsProcessed, duration)

	// Format sync complete data
	data := SyncCompleteData{
		TasksProcessed: tasksProcessed,
		DepsProcessed:  depsProcessed,
		Duration:       duration,
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		h.logger.Printf("Failed to marshal sync data: %v", err)
		return
	}

	// Broadcast message
	msg := Message{
		Type:      MessageTypeSyncComplete,
		Timestamp: time.Now(),
		Data:      dataJSON,
	}
	h.server.Broadcast(msg)
}

// OnBlockedCacheRefresh handles blocked cache refresh events
func (h *Handler) OnBlockedCacheRefresh(blockedCount, readyCount int) {
	h.logger.Printf("Blocked cache refreshed: %d blocked, %d ready", blockedCount, readyCount)

	// Update statistics
	h.stats.Blocked = blockedCount
	h.stats.Ready = readyCount

	// Format blocked cache data
	data := BlockedCacheData{
		BlockedCount: blockedCount,
		ReadyCount:   readyCount,
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		h.logger.Printf("Failed to marshal blocked cache data: %v", err)
		return
	}

	// Broadcast message
	msg := Message{
		Type:      MessageTypeBlockedCache,
		Timestamp: time.Now(),
		Data:      dataJSON,
	}
	h.server.Broadcast(msg)

	// Also broadcast updated stats
	h.broadcastStats()
}

// broadcastStats sends current statistics to all clients
func (h *Handler) broadcastStats() {
	dataJSON, err := json.Marshal(h.stats)
	if err != nil {
		h.logger.Printf("Failed to marshal stats: %v", err)
		return
	}

	msg := Message{
		Type:      MessageTypeStats,
		Timestamp: time.Now(),
		Data:      dataJSON,
	}
	h.server.Broadcast(msg)
}

// UpdateStats manually updates statistics from a full task list
// This is useful for initialization or periodic refresh
func (h *Handler) UpdateStats(tasks []*schema.TaskFile, blockedCount, readyCount int) {
	h.stats.Total = len(tasks)
	h.stats.ByStatus = make(map[string]int)
	h.stats.InProgress = 0

	for _, task := range tasks {
		h.stats.ByStatus[task.Status]++
		if task.Status == "in_progress" {
			h.stats.InProgress++
		}
	}

	h.stats.Blocked = blockedCount
	h.stats.Ready = readyCount
	h.statsReady = true

	h.broadcastStats()
}

// GetStats returns the current statistics
func (h *Handler) GetStats() StatsData {
	return *h.stats
}
