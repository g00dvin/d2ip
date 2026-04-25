package sourcereg

import (
	"fmt"
	"sync"
)

// FactoryFunc creates a Source from configuration.
type FactoryFunc func(id, prefix string, cfg map[string]any) (Source, error)

var (
	factoriesMu sync.RWMutex
	factories   = make(map[SourceType]FactoryFunc)
)

// RegisterFactory registers a provider factory for the given type.
// It is typically called from a provider package's init function.
func RegisterFactory(t SourceType, fn FactoryFunc) {
	factoriesMu.Lock()
	defer factoriesMu.Unlock()
	factories[t] = fn
}

// GetFactory returns the registered factory for the given type, or nil.
func GetFactory(t SourceType) FactoryFunc {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()
	return factories[t]
}

func createSource(cfg SourceConfig) (Source, error) {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()
	fn, ok := factories[cfg.Provider]
	if !ok {
		return nil, fmt.Errorf("sourcereg: unknown provider %q", cfg.Provider)
	}
	return fn(cfg.ID, cfg.Prefix, cfg.Config)
}
