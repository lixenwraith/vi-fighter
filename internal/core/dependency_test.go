package core

import (
	"errors"
	"reflect"
	"testing"
)

func TestResolveDependencyOrderDeterministic(t *testing.T) {
	graph := map[string][]string{
		"z": nil,
		"b": {"a"},
		"a": nil,
		"d": {"b", "z"},
		"c": {"a"},
	}
	want := []string{"a", "z", "b", "c", "d"}

	for range 100 {
		got, err := ResolveDependencyOrder(graph)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestResolveDependencyOrderMissing(t *testing.T) {
	_, err := ResolveDependencyOrder(map[string][]string{"b": {"missing"}})
	var missing *MissingDependencyError
	if !errors.As(err, &missing) {
		t.Fatalf("error = %v, want MissingDependencyError", err)
	}
	if missing.Name != "b" || missing.Dependency != "missing" {
		t.Fatalf("missing = %+v", missing)
	}
}

func TestResolveDependencyOrderCycle(t *testing.T) {
	_, err := ResolveDependencyOrder(map[string][]string{
		"a":    {"b"},
		"b":    {"c"},
		"c":    {"a"},
		"free": nil,
	})
	var cycle *DependencyCycleError
	if !errors.As(err, &cycle) {
		t.Fatalf("error = %v, want DependencyCycleError", err)
	}
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(cycle.Names, want) {
		t.Fatalf("cycle = %v, want %v", cycle.Names, want)
	}
}
