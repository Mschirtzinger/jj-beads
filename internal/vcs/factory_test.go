package vcs

import (
	"testing"
)

// Use a test-only type to avoid conflicting with real registrations
const (
	testTypeA Type = "test-type-a"
	testTypeB Type = "test-type-b"
)

func TestFactoryWithMockRegistration(t *testing.T) {
	// Register test implementations (won't conflict with real git/jj)
	Register(testTypeA, newMockVCS(testTypeA))
	Register(testTypeB, newMockVCS(testTypeB))

	tests := []struct {
		name         string
		implType     Type
		repoRoot     string
		wantErr      bool
		expectedName Type
	}{
		{
			name:         "test type A implementation",
			implType:     testTypeA,
			repoRoot:     "/test/a/repo",
			wantErr:      false,
			expectedName: testTypeA,
		},
		{
			name:         "test type B implementation",
			implType:     testTypeB,
			repoRoot:     "/test/b/repo",
			wantErr:      false,
			expectedName: testTypeB,
		},
		{
			name:     "unregistered type",
			implType: "unknown",
			repoRoot: "/test/unknown/repo",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			result := &DetectionResult{
				Type:     tt.implType,
				RepoRoot: tt.repoRoot,
				VCSDir:   tt.repoRoot + "/.vcs",
			}

			v, err := factory.createImplementation(tt.implType, result)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if v == nil {
				t.Fatal("Expected VCS instance but got nil")
			}

			if v.Name() != tt.expectedName {
				t.Errorf("Expected VCS name '%s', got '%s'", tt.expectedName, v.Name())
			}

			// Verify repo root is correct
			root, err := v.RepoRoot()
			if err != nil {
				t.Errorf("Failed to get repo root: %v", err)
			}
			if root != tt.repoRoot {
				t.Errorf("Expected repo root '%s', got '%s'", tt.repoRoot, root)
			}
		})
	}
}

func TestFactoryErrorMessageForUnregistered(t *testing.T) {
	factory := NewFactory()
	result := &DetectionResult{
		Type:     "definitely-not-registered",
		RepoRoot: "/test/repo",
		VCSDir:   "/test/repo/.unknown",
	}

	v, err := factory.createImplementation("definitely-not-registered", result)

	if err == nil {
		t.Fatal("Expected error for unregistered type")
	}

	if v != nil {
		t.Error("Expected nil VCS instance on error")
	}

	// Error message should mention available types
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Expected non-empty error message")
	}

	// Should mention what types are available
	t.Logf("Error message: %s", errMsg)
}

func TestFactoryCache(t *testing.T) {
	// This test would require actual VCS detection which needs a real repo
	// For now, we just verify the cache mechanism exists and can be controlled

	// Test cache can be disabled
	DisableCache()
	defer EnableCache()

	// Test cache can be reset
	ResetCache()

	// These operations shouldn't panic
}

func TestFactoryOptions(t *testing.T) {
	tests := []struct {
		name              string
		opts              []FactoryOption
		expectedPreferred Type
		expectedFallback  Type
		expectedCache     bool
	}{
		{
			name:              "default options",
			opts:              nil,
			expectedPreferred: TypeJJ,
			expectedFallback:  TypeGit,
			expectedCache:     true,
		},
		{
			name:              "prefer git",
			opts:              []FactoryOption{WithPreferredType(TypeGit)},
			expectedPreferred: TypeGit,
			expectedFallback:  TypeGit,
			expectedCache:     true,
		},
		{
			name:              "custom fallback",
			opts:              []FactoryOption{WithFallbackType(TypeJJ)},
			expectedPreferred: TypeJJ,
			expectedFallback:  TypeJJ,
			expectedCache:     true,
		},
		{
			name:              "disable cache",
			opts:              []FactoryOption{WithCache(false)},
			expectedPreferred: TypeJJ,
			expectedFallback:  TypeGit,
			expectedCache:     false,
		},
		{
			name: "multiple options",
			opts: []FactoryOption{
				WithPreferredType(TypeGit),
				WithFallbackType(TypeJJ),
				WithCache(false),
			},
			expectedPreferred: TypeGit,
			expectedFallback:  TypeJJ,
			expectedCache:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory(tt.opts...)

			if factory.preferredType != tt.expectedPreferred {
				t.Errorf("Expected preferred type '%s', got '%s'",
					tt.expectedPreferred, factory.preferredType)
			}

			if factory.fallbackType != tt.expectedFallback {
				t.Errorf("Expected fallback type '%s', got '%s'",
					tt.expectedFallback, factory.fallbackType)
			}

			if factory.enableCache != tt.expectedCache {
				t.Errorf("Expected cache enabled=%v, got %v",
					tt.expectedCache, factory.enableCache)
			}
		})
	}
}

func TestDetermineImplementationType(t *testing.T) {
	factory := NewFactory()

	tests := []struct {
		name     string
		result   *DetectionResult
		expected Type
		skipNoJJ bool // Skip if jj not available
	}{
		{
			name: "git only",
			result: &DetectionResult{
				Type:   TypeGit,
				HasGit: true,
				HasJJ:  false,
			},
			expected: TypeGit,
			skipNoJJ: false,
		},
		{
			name: "jj only",
			result: &DetectionResult{
				Type:   TypeJJ,
				HasGit: false,
				HasJJ:  true,
			},
			expected: TypeJJ,
			skipNoJJ: true,
		},
		{
			name: "colocated - prefer jj",
			result: &DetectionResult{
				Type:      TypeColocate,
				HasGit:    true,
				HasJJ:     true,
				Colocated: true,
			},
			expected: TypeJJ, // Default preference
			skipNoJJ: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests requiring jj if jj not available
			if tt.skipNoJJ && !IsJJAvailable() {
				t.Skip("jj not available, skipping test")
			}

			implType := factory.determineImplementationType(tt.result)

			if implType != tt.expected {
				t.Errorf("Expected implementation type '%s', got '%s'",
					tt.expected, implType)
			}
		})
	}
}

func TestDetermineImplementationTypeWithPreference(t *testing.T) {
	tests := []struct {
		name      string
		preferred Type
		result    *DetectionResult
		expected  Type
		skipNoJJ  bool // Skip if jj not available
	}{
		{
			name:      "colocated prefer git",
			preferred: TypeGit,
			result: &DetectionResult{
				Type:      TypeColocate,
				HasGit:    true,
				HasJJ:     true,
				Colocated: true,
			},
			expected: TypeGit,
			skipNoJJ: false,
		},
		{
			name:      "colocated prefer jj",
			preferred: TypeJJ,
			result: &DetectionResult{
				Type:      TypeColocate,
				HasGit:    true,
				HasJJ:     true,
				Colocated: true,
			},
			expected: TypeJJ,
			skipNoJJ: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests requiring jj if jj not available
			if tt.skipNoJJ && !IsJJAvailable() {
				t.Skip("jj not available, skipping test")
			}

			factory := NewFactory(WithPreferredType(tt.preferred))
			implType := factory.determineImplementationType(tt.result)

			if implType != tt.expected {
				t.Errorf("Expected implementation type '%s', got '%s'",
					tt.expected, implType)
			}
		})
	}
}
