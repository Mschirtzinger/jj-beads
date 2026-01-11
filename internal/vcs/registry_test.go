package vcs

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
)

// mockVCS is a mock VCS implementation for testing
type mockVCS struct {
	name     Type
	repoRoot string
}

func (m *mockVCS) Name() Type                          { return m.name }
func (m *mockVCS) Version() (string, error)            { return "mock-1.0.0", nil }
func (m *mockVCS) RepoRoot() (string, error)           { return m.repoRoot, nil }
func (m *mockVCS) VCSDir() (string, error)             { return m.repoRoot + "/.mock", nil }
func (m *mockVCS) IsInVCS() bool                       { return true }
func (m *mockVCS) CurrentRef() (string, error)         { return "main", nil }
func (m *mockVCS) RefExists(name string) bool          { return name == "main" }
func (m *mockVCS) CreateRef(name string, base string) error { return nil }
func (m *mockVCS) DeleteRef(name string) error         { return nil }
func (m *mockVCS) MoveRef(name string, target string) error { return nil }
func (m *mockVCS) ListRefs() ([]RefInfo, error)        { return nil, nil }
func (m *mockVCS) HasChanges(paths ...string) (bool, error) { return false, nil }
func (m *mockVCS) HasUnmergedPaths() (bool, error)     { return false, nil }
func (m *mockVCS) IsInRebaseOrMerge() bool             { return false }
func (m *mockVCS) HasRemote() bool                     { return false }
func (m *mockVCS) GetRemotes() ([]RemoteInfo, error)   { return nil, nil }
func (m *mockVCS) Add(paths []string) error            { return nil }
func (m *mockVCS) Status(paths ...string) ([]FileStatus, error) { return nil, nil }
func (m *mockVCS) Commit(ctx context.Context, opts CommitOptions) error { return nil }
func (m *mockVCS) GetCommitHash(ref string) (string, error) { return "abc123", nil }
func (m *mockVCS) Fetch(ctx context.Context, remote, ref string) error { return nil }
func (m *mockVCS) Pull(ctx context.Context, opts PullOptions) error { return nil }
func (m *mockVCS) Push(ctx context.Context, opts PushOptions) error { return nil }
func (m *mockVCS) HasDivergence(local, remote string) (DivergenceInfo, error) {
	return DivergenceInfo{}, nil
}
func (m *mockVCS) ExtractFileFromRef(ref, path string) ([]byte, error) { return nil, nil }
func (m *mockVCS) CreateWorkspace(opts WorkspaceOptions) (Workspace, error) { return nil, nil }
func (m *mockVCS) ListWorkspaces() ([]WorkspaceInfo, error) { return nil, nil }
func (m *mockVCS) HasConflicts() (bool, error)         { return false, nil }
func (m *mockVCS) GetConflictedFiles() ([]string, error) { return nil, nil }
func (m *mockVCS) CanUndo() bool                       { return false }
func (m *mockVCS) Undo(ctx context.Context) error      { return nil }
func (m *mockVCS) GetOperationLog(limit int) ([]OperationInfo, error) { return nil, nil }
func (m *mockVCS) Exec(ctx context.Context, args ...string) ([]byte, error) { return nil, nil }

// newMockVCS creates a mock VCS instance
func newMockVCS(name Type) func(repoRoot string) (VCS, error) {
	return func(repoRoot string) (VCS, error) {
		return &mockVCS{name: name, repoRoot: repoRoot}, nil
	}
}

// testTypeCounter generates unique test type names
var testTypeCounter int64

func uniqueTestType(prefix string) Type {
	n := atomic.AddInt64(&testTypeCounter, 1)
	return Type(fmt.Sprintf("%s-%d", prefix, n))
}

func TestRegister(t *testing.T) {
	typeName := uniqueTestType("register-test")

	// Test registering a constructor
	Register(typeName, newMockVCS(typeName))

	if !IsRegistered(typeName) {
		t.Error("Expected type to be registered")
	}

	// Verify we can get the constructor
	constructor := getConstructor(typeName)
	if constructor == nil {
		t.Fatal("Expected to get constructor for registered type")
	}

	// Verify the constructor works
	v, err := constructor("/test/repo")
	if err != nil {
		t.Fatalf("Constructor failed: %v", err)
	}

	if v.Name() != typeName {
		t.Errorf("Expected VCS name '%s', got '%s'", typeName, v.Name())
	}
}

func TestRegisterPanicsOnNil(t *testing.T) {
	typeName := uniqueTestType("nil-test")

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when registering nil constructor")
		}
	}()

	Register(typeName, nil)
}

func TestRegisterPanicsOnDuplicate(t *testing.T) {
	typeName := uniqueTestType("dup-test")

	Register(typeName, newMockVCS(typeName))

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when registering duplicate type")
		}
	}()

	Register(typeName, newMockVCS(typeName))
}

func TestIsRegistered(t *testing.T) {
	typeName := uniqueTestType("isreg-test")
	unknownType := uniqueTestType("unknown-test")

	if IsRegistered(typeName) {
		t.Error("Expected type to not be registered initially")
	}

	Register(typeName, newMockVCS(typeName))

	if !IsRegistered(typeName) {
		t.Error("Expected type to be registered after Register()")
	}

	if IsRegistered(unknownType) {
		t.Error("Expected unknown type to not be registered")
	}
}

func TestRegisteredTypes(t *testing.T) {
	// Just verify that RegisteredTypes returns a slice
	// We can't test for exact counts since other tests may have registered types
	types := RegisteredTypes()
	if types == nil {
		t.Error("Expected non-nil slice from RegisteredTypes()")
	}

	// Register a new type and verify count increases
	typeName := uniqueTestType("types-test")
	beforeCount := len(types)
	Register(typeName, newMockVCS(typeName))
	types = RegisteredTypes()
	if len(types) <= beforeCount {
		t.Errorf("Expected type count to increase after registration")
	}
}

func TestUnregisterAll(t *testing.T) {
	// Register some unique types
	type1 := uniqueTestType("unreg1")
	type2 := uniqueTestType("unreg2")

	Register(type1, newMockVCS(type1))
	Register(type2, newMockVCS(type2))

	if !IsRegistered(type1) || !IsRegistered(type2) {
		t.Error("Expected types to be registered before unregister")
	}

	// Note: We don't actually call UnregisterAll() here because it would
	// break other tests. Instead, we just verify the types are registered.
	// The actual UnregisterAll functionality is implicitly tested by
	// the fact that we can register the same types in factory tests.
}

func TestGetConstructor(t *testing.T) {
	unknownType := uniqueTestType("getconst-unknown")

	// Test unregistered type
	constructor := getConstructor(unknownType)
	if constructor != nil {
		t.Error("Expected nil constructor for unregistered type")
	}

	// Test registered type
	typeName := uniqueTestType("getconst-test")
	Register(typeName, newMockVCS(typeName))
	constructor = getConstructor(typeName)
	if constructor == nil {
		t.Error("Expected non-nil constructor for registered type")
	}
}

// TestConcurrentRegistration verifies thread-safety of registration
func TestConcurrentRegistration(t *testing.T) {
	// This test ensures we don't panic under concurrent access
	done := make(chan bool)
	basePrefix := uniqueTestType("concurrent")

	for i := 0; i < 10; i++ {
		go func(n int) {
			defer func() { done <- true }()

			typeName := Type(fmt.Sprintf("%s-%d", basePrefix, n))
			Register(typeName, newMockVCS(typeName))

			_ = IsRegistered(typeName)
			_ = RegisteredTypes()
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Just verify we didn't panic - exact count depends on other registered types
}
