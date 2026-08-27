package engine

import (
	"fmt"
	"slices"

	"github.com/lixenwraith/vi-fighter/internal/core"
)

// SystemFactory declares construction metadata without initializing the system.
type SystemFactory struct {
	Name         string
	Domain       SystemDomain
	Dependencies SystemDependencies
	Build        func() System
}

// BuildSystems constructs factories in dependency order and returns manifest order.
func BuildSystems(factories []SystemFactory) ([]System, error) {
	byName := make(map[string]SystemFactory, len(factories))
	for _, factory := range factories {
		if factory.Name == "" {
			return nil, fmt.Errorf("system factory has an empty name")
		}
		if _, exists := byName[factory.Name]; exists {
			return nil, fmt.Errorf("duplicate system factory %q", factory.Name)
		}
		if factory.Build == nil {
			return nil, fmt.Errorf("system factory %q has no constructor", factory.Name)
		}
		byName[factory.Name] = factory
	}

	graph := make(map[string][]string, len(factories))
	for _, factory := range factories {
		edges := append([]string(nil), factory.Dependencies.Required...)
		for _, dependency := range factory.Dependencies.Optional {
			if _, exists := byName[dependency]; exists {
				edges = append(edges, dependency)
			}
		}
		graph[factory.Name] = edges
	}

	order, err := core.ResolveDependencyOrder(graph)
	if err != nil {
		return nil, fmt.Errorf("resolve system dependencies: %w", err)
	}

	built := make(map[string]System, len(factories))
	for _, name := range order {
		factory := byName[name]
		instance := factory.Build()
		if instance == nil {
			return nil, fmt.Errorf("system factory %q returned nil", name)
		}
		if instance.Name() != factory.Name {
			return nil, fmt.Errorf("system factory %q constructed system %q", factory.Name, instance.Name())
		}
		if instance.Domain() != factory.Domain {
			return nil, fmt.Errorf("system %q domain is %s, declaration is %s", name, instance.Domain(), factory.Domain)
		}
		if !sameDependencies(instance.Dependencies(), factory.Dependencies) {
			return nil, fmt.Errorf("system %q dependency declaration differs from its factory", name)
		}
		built[name] = instance
	}

	systems := make([]System, len(factories))
	for i, factory := range factories {
		systems[i] = built[factory.Name]
	}
	return systems, nil
}

func sameDependencies(a, b SystemDependencies) bool {
	return slices.Equal(a.Required, b.Required) && slices.Equal(a.Optional, b.Optional)
}
