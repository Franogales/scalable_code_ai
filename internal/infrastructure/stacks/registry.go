package stacks

import (
	"fmt"

	"github.com/Franogales/scalable_code_ai/internal/domain"
)

// Registry maps StackKey values to their concrete StackInstaller implementations.
type Registry struct {
	installers map[domain.StackKey]domain.StackInstaller
}

// NewRegistry builds a Registry from the provided installers.
func NewRegistry(installers ...domain.StackInstaller) *Registry {
	m := make(map[domain.StackKey]domain.StackInstaller, len(installers))
	for _, inst := range installers {
		m[inst.Key()] = inst
	}
	return &Registry{installers: m}
}

// Get retrieves the StackInstaller for the given StackKey.
func (r *Registry) Get(key domain.StackKey) (domain.StackInstaller, error) {
	inst, ok := r.installers[key]
	if !ok {
		return nil, fmt.Errorf("no installer registered for stack %q", key.String())
	}
	return inst, nil
}
