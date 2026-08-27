package engine

import (
	"errors"
	"reflect"
	"testing"

	"github.com/lixenwraith/vi-fighter/internal/core"
)

type factoryTestSystem struct {
	name         string
	dependencies SystemDependencies
}

func (*factoryTestSystem) Init()                              {}
func (*factoryTestSystem) Priority() int                      { return 10 }
func (s *factoryTestSystem) Name() string                     { return s.name }
func (*factoryTestSystem) Domain() SystemDomain               { return SystemShared }
func (s *factoryTestSystem) Dependencies() SystemDependencies { return s.dependencies }
func (*factoryTestSystem) Update()                            {}

func TestBuildSystemsSeparatesConstructionAndTickOrder(t *testing.T) {
	var construction []string
	makeFactory := func(name string, dependencies SystemDependencies) SystemFactory {
		return SystemFactory{
			Name:         name,
			Domain:       SystemShared,
			Dependencies: dependencies,
			Build: func() System {
				construction = append(construction, name)
				return &factoryTestSystem{name: name, dependencies: dependencies}
			},
		}
	}

	factories := []SystemFactory{
		makeFactory("dependent", SystemDependencies{Required: []string{"base"}, Optional: []string{"absent"}}),
		makeFactory("base", SystemDependencies{}),
	}
	systems, err := BuildSystems(factories)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"base", "dependent"}; !reflect.DeepEqual(construction, want) {
		t.Fatalf("construction order = %v, want %v", construction, want)
	}

	w := NewWorld()
	for _, system := range systems {
		w.AddSystem(system)
	}
	got := []string{w.Systems()[0].Name(), w.Systems()[1].Name()}
	if want := []string{"dependent", "base"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tick tie order = %v, want %v", got, want)
	}
}

func TestBuildSystemsRejectsMissingRequiredDependency(t *testing.T) {
	dependencies := SystemDependencies{Required: []string{"absent"}}
	_, err := BuildSystems([]SystemFactory{{
		Name:         "dependent",
		Domain:       SystemShared,
		Dependencies: dependencies,
		Build: func() System {
			return &factoryTestSystem{name: "dependent", dependencies: dependencies}
		},
	}})
	var missing *core.MissingDependencyError
	if !errors.As(err, &missing) {
		t.Fatalf("error = %v, want MissingDependencyError", err)
	}
	if missing.Name != "dependent" || missing.Dependency != "absent" {
		t.Fatalf("missing dependency = %+v", missing)
	}
}
