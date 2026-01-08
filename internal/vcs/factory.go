package vcs

import (
	"fmt"
	"os"
	"strconv"
	"sync"
)

// Factory creates VCS instances based on detected type and preferences.
//
// The factory supports caching to avoid repeated detection for the same path,
// and allows configuration of preferences for colocated repositories.
type Factory struct {
	// preferredType specifies which VCS to prefer in colocated repos
	preferredType Type

	// fallbackType specifies which VCS to use if preferred is unavailable
	fallbackType Type

	// enableCache enables caching of VCS instances
	enableCache bool
}

// Global cache for VCS instances
var (
	vcsCache     sync.Map
	cacheMutex   sync.RWMutex
	cacheEnabled = true
)

// NewFactory creates a new VCS factory with the specified options.
//
// Default behavior:
//   - Caching enabled
//   - Prefer jj for colocated repos
//   - Fall back to git if preferred unavailable
func NewFactory(opts ...FactoryOption) *Factory {
	f := &Factory{
		preferredType: TypeJJ,
		fallbackType:  TypeGit,
		enableCache:   true,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// FactoryOption configures the factory
type FactoryOption func(*Factory)

// WithPreferredType sets the preferred VCS type for colocated repos
func WithPreferredType(t Type) FactoryOption {
	return func(f *Factory) {
		f.preferredType = t
	}
}

// WithFallbackType sets the fallback VCS type
func WithFallbackType(t Type) FactoryOption {
	return func(f *Factory) {
		f.fallbackType = t
	}
}

// WithCache enables or disables instance caching
func WithCache(enabled bool) FactoryOption {
	return func(f *Factory) {
		f.enableCache = enabled
	}
}

// Create creates a VCS instance for the given path.
//
// The factory will:
//  1. Check the cache for an existing instance (if caching enabled)
//  2. Detect the VCS type at the path
//  3. Create the appropriate implementation
//  4. Cache the instance (if caching enabled)
//
// For colocated repositories, the factory uses the preferred type
// if available, falling back as needed.
func (f *Factory) Create(path string) (VCS, error) {
	// Check cache first
	if f.enableCache && cacheEnabled {
		if cached, ok := vcsCache.Load(path); ok {
			return cached.(VCS), nil
		}
	}

	// Detect VCS type with availability check
	result, err := DetectWithAvailability(path)
	if err != nil {
		return nil, err
	}

	// Determine which implementation to use
	implType := f.determineImplementationType(result)

	// Create the implementation
	v, err := f.createImplementation(implType, result)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if f.enableCache && cacheEnabled {
		vcsCache.Store(path, v)
	}

	return v, nil
}

// determineImplementationType decides which VCS implementation to use
// based on detection results and factory preferences.
func (f *Factory) determineImplementationType(result *DetectionResult) Type {
	switch result.Type {
	case TypeGit:
		return TypeGit
	case TypeJJ:
		return TypeJJ
	case TypeColocate:
		// Use factory preference for colocated repos
		preferred := f.preferredType
		if preferred == "" {
			preferred = PreferredVCS()
		}

		// Check if preferred is available
		switch preferred {
		case TypeJJ:
			if result.HasJJ && IsJJAvailable() {
				return TypeJJ
			}
			// Fall back to git
			if result.HasGit && IsGitAvailable() {
				return TypeGit
			}
		case TypeGit:
			if result.HasGit && IsGitAvailable() {
				return TypeGit
			}
			// Fall back to jj
			if result.HasJJ && IsJJAvailable() {
				return TypeJJ
			}
		}

		// Last resort: try anything available
		if result.HasGit && IsGitAvailable() {
			return TypeGit
		}
		if result.HasJJ && IsJJAvailable() {
			return TypeJJ
		}

		return f.fallbackType
	default:
		return TypeGit
	}
}

// createImplementation creates the actual VCS implementation using the registry.
// Implementations must register themselves via Register() in their init() functions.
func (f *Factory) createImplementation(implType Type, result *DetectionResult) (VCS, error) {
	// Get constructor from registry
	constructor := getConstructor(implType)
	if constructor == nil {
		return nil, fmt.Errorf("no registered constructor for VCS type: %s (available: %v)", implType, RegisteredTypes())
	}

	// Create VCS instance using the registered constructor
	v, err := constructor(result.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s VCS instance: %w", implType, err)
	}

	return v, nil
}

// ===================
// Convenience Functions
// ===================

// Get returns a VCS instance for the current directory using default options.
// This is the most common entry point for beads commands.
func Get() (VCS, error) {
	return NewFactory().Create(".")
}

// GetForPath returns a VCS instance for the specified path.
func GetForPath(path string) (VCS, error) {
	return NewFactory().Create(path)
}

// GetWithPreference returns a VCS instance with a specific type preference.
// Useful when a command needs to force a particular VCS backend.
func GetWithPreference(preferred Type) (VCS, error) {
	return NewFactory(WithPreferredType(preferred)).Create(".")
}

// GetGit returns a git VCS instance, or error if git is not available.
func GetGit() (VCS, error) {
	result, err := Detect(".")
	if err != nil {
		return nil, err
	}
	if !result.HasGit {
		return nil, ErrNotInVCS
	}
	if !IsGitAvailable() {
		return nil, ErrVCSNotAvailable
	}
	return NewFactory(WithPreferredType(TypeGit)).Create(".")
}

// GetJJ returns a jj VCS instance, or error if jj is not available.
func GetJJ() (VCS, error) {
	result, err := Detect(".")
	if err != nil {
		return nil, err
	}
	if !result.HasJJ {
		return nil, ErrNotInVCS
	}
	if !IsJJAvailable() {
		return nil, ErrVCSNotAvailable
	}
	return NewFactory(WithPreferredType(TypeJJ)).Create(".")
}

// ===================
// Cache Management
// ===================

// ResetCache clears the VCS instance cache.
// This is primarily useful for testing, where the working directory
// may change between test cases.
func ResetCache() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	vcsCache = sync.Map{}
}

// DisableCache globally disables VCS instance caching.
// Useful for testing or when the repository state may change.
func DisableCache() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	cacheEnabled = false
}

// EnableCache re-enables VCS instance caching.
func EnableCache() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	cacheEnabled = true
}

// ===================
// Feature Flags
// ===================

// Features holds feature flag settings for the VCS abstraction layer.
// These enable gradual rollout and testing of new functionality.
type Features struct {
	// UseAbstraction enables the new VCS abstraction layer.
	// When false, the legacy git-specific code paths are used.
	UseAbstraction bool

	// EnableJJ enables jj support.
	// Requires UseAbstraction to also be true.
	EnableJJ bool

	// PreferJJ prefers jj over git in colocated repositories.
	PreferJJ bool

	// LogVCSOperations logs all VCS operations for debugging.
	LogVCSOperations bool
}

// GetFeatures returns the current feature flag settings.
// Flags are read from environment variables:
//   - BD_VCS_ABSTRACTION: Enable abstraction layer (default: false)
//   - BD_VCS_JJ: Enable jj support (default: false)
//   - BD_VCS_PREFER_JJ: Prefer jj in colocated repos (default: false)
//   - BD_VCS_LOG: Log VCS operations (default: false)
func GetFeatures() Features {
	return Features{
		UseAbstraction:   envBool("BD_VCS_ABSTRACTION", false),
		EnableJJ:         envBool("BD_VCS_JJ", false),
		PreferJJ:         envBool("BD_VCS_PREFER_JJ", false),
		LogVCSOperations: envBool("BD_VCS_LOG", false),
	}
}

// IsAbstractionEnabled returns true if the VCS abstraction layer should be used.
// This is the main switch for the new vs legacy code paths.
func IsAbstractionEnabled() bool {
	return GetFeatures().UseAbstraction
}

// IsJJEnabled returns true if jj support is enabled.
func IsJJEnabled() bool {
	f := GetFeatures()
	return f.UseAbstraction && f.EnableJJ
}

// envBool reads a boolean from an environment variable.
// Returns defaultVal if the variable is not set or cannot be parsed.
func envBool(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
		// Also accept "1", "yes", "on" as true
		switch v {
		case "1", "yes", "on", "YES", "ON":
			return true
		case "0", "no", "off", "NO", "OFF":
			return false
		}
	}
	return defaultVal
}

// ===================
// Legacy Compatibility
// ===================

// UseLegacyGit returns true if legacy git code paths should be used.
// This is the inverse of IsAbstractionEnabled() and is used during
// the migration period to gradually enable the new abstraction layer.
func UseLegacyGit() bool {
	return !IsAbstractionEnabled()
}

// GetLegacyFallback returns a function that can be used as a fallback
// when the VCS abstraction fails. This enables graceful degradation
// during the rollout period.
func GetLegacyFallback() func() bool {
	return func() bool {
		return UseLegacyGit()
	}
}
