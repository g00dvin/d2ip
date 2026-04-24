package sourcereg

import "fmt"

// FactoryFunc creates a Source from configuration.
type FactoryFunc func(id, prefix string, cfg map[string]any) (Source, error)

var factories = make(map[SourceType]FactoryFunc)

// RegisterFactory registers a provider factory for the given type.
// It is typically called from a provider package's init function.
func RegisterFactory(t SourceType, fn FactoryFunc) {
	factories[t] = fn
}

func createSource(cfg SourceConfig) (Source, error) {
	fn, ok := factories[cfg.Provider]
	if !ok {
		return nil, fmt.Errorf("sourcereg: unknown provider %q", cfg.Provider)
	}
	return fn(cfg.ID, cfg.Prefix, cfg.Config)
}
