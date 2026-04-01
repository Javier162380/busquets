package connectors

import (
	"fmt"
	"sync"
)

// Registry manages available connectors.
type Registry struct {
	mu         sync.RWMutex
	connectors map[string]Connector
}

// NewRegistry creates a new connector registry.
func NewRegistry() *Registry {
	return &Registry{
		connectors: make(map[string]Connector),
	}
}

// Register adds a connector to the registry.
func (r *Registry) Register(c Connector) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.connectors[c.Name()]; exists {
		return fmt.Errorf("connector %q already registered", c.Name())
	}
	r.connectors[c.Name()] = c
	return nil
}

// Get returns a connector by name.
func (r *Registry) Get(name string) (Connector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.connectors[name]
	return c, ok
}

// List returns all registered connector names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.connectors))
	for name := range r.connectors {
		names = append(names, name)
	}
	return names
}

// All returns all registered connectors.
func (r *Registry) All() []Connector {
	r.mu.RLock()
	defer r.mu.RUnlock()

	connectors := make([]Connector, 0, len(r.connectors))
	for _, c := range r.connectors {
		connectors = append(connectors, c)
	}
	return connectors
}
