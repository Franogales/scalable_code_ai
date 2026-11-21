package ai

import (
	"fmt"

	"github.com/Franogales/scalable_code_ai/internal/domain"
)

// Registry maps AIProvider values to their concrete AIInstaller implementations.
type Registry struct {
	installers map[domain.AIProvider]domain.AIInstaller
}

// NewRegistry builds a Registry from the provided AI installers.
func NewRegistry(installers ...domain.AIInstaller) *Registry {
	m := make(map[domain.AIProvider]domain.AIInstaller, len(installers))
	for _, inst := range installers {
		m[inst.Provider()] = inst
	}
	return &Registry{installers: m}
}

// Get retrieves the AIInstaller for the given AIProvider.
func (r *Registry) Get(provider domain.AIProvider) (domain.AIInstaller, error) {
	inst, ok := r.installers[provider]
	if !ok {
		return nil, fmt.Errorf("no installer registered for AI provider %q", provider)
	}
	return inst, nil
}
