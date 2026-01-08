package vcs

import (
	"fmt"
	"sync"
)

// VCSConstructor creates a VCS instance for a given repo root.
// Implementations register themselves with the registry using Register().
type VCSConstructor func(repoRoot string) (VCS, error)

// registry maps VCS types to their constructors
var (
	registry      = make(map[Type]VCSConstructor)
	registryMutex sync.RWMutex
)

// Register registers a VCS implementation constructor.
// This is called from init() functions in implementation packages (git, jj).
//
// Example:
//
//	func init() {
//	    vcs.Register(vcs.TypeGit, New)
//	}
func Register(t Type, constructor VCSConstructor) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	if constructor == nil {
		panic(fmt.Sprintf("vcs: Register constructor is nil for type %s", t))
	}

	if _, exists := registry[t]; exists {
		panic(fmt.Sprintf("vcs: Register called twice for type %s", t))
	}

	registry[t] = constructor
}

// getConstructor retrieves the constructor for a VCS type.
// Returns nil if the type is not registered.
func getConstructor(t Type) VCSConstructor {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return registry[t]
}

// IsRegistered returns true if a constructor is registered for the given type.
func IsRegistered(t Type) bool {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	_, exists := registry[t]
	return exists
}

// RegisteredTypes returns all registered VCS types.
// Useful for testing and debugging.
func RegisteredTypes() []Type {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	types := make([]Type, 0, len(registry))
	for t := range registry {
		types = append(types, t)
	}
	return types
}

// UnregisterAll clears all registered constructors.
// This is primarily useful for testing.
func UnregisterAll() {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	registry = make(map[Type]VCSConstructor)
}
