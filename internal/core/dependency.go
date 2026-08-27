package core

import (
	"fmt"
	"sort"
	"strings"
)

// MissingDependencyError reports an edge whose dependency is not registered.
type MissingDependencyError struct {
	Name       string
	Dependency string
}

func (e *MissingDependencyError) Error() string {
	return fmt.Sprintf("%s depends on missing dependency %s", e.Name, e.Dependency)
}

// DependencyCycleError reports the nodes left unresolved by a dependency cycle.
type DependencyCycleError struct {
	Names []string
}

func (e *DependencyCycleError) Error() string {
	return "dependency cycle detected: " + strings.Join(e.Names, ", ")
}

// ResolveDependencyOrder returns a deterministic topological order for a name-to-dependencies graph.
func ResolveDependencyOrder(graph map[string][]string) ([]string, error) {
	inDegree := make(map[string]int, len(graph))
	dependents := make(map[string][]string, len(graph))

	names := make([]string, 0, len(graph))
	for name := range graph {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		dependencies := append([]string(nil), graph[name]...)
		sort.Strings(dependencies)
		for _, dependency := range dependencies {
			if _, exists := graph[dependency]; !exists {
				return nil, &MissingDependencyError{Name: name, Dependency: dependency}
			}
			inDegree[name]++
			dependents[dependency] = append(dependents[dependency], name)
		}
	}

	queue := make([]string, 0, len(names))
	for _, name := range names {
		if inDegree[name] == 0 {
			queue = append(queue, name)
		}
	}

	result := make([]string, 0, len(names))
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		result = append(result, name)

		next := dependents[name]
		sort.Strings(next)
		for _, dependent := range next {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(result) != len(names) {
		unresolved := make([]string, 0, len(names)-len(result))
		for _, name := range names {
			if inDegree[name] > 0 {
				unresolved = append(unresolved, name)
			}
		}
		return nil, &DependencyCycleError{Names: unresolved}
	}

	return result, nil
}
