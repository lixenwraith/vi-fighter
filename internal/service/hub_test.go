package service

import (
	"reflect"
	"testing"
)

type orderService struct {
	name         string
	dependencies []string
	initOrder    *[]string
	startOrder   *[]string
	stopOrder    *[]string
}

func (s *orderService) Name() string           { return s.name }
func (s *orderService) Dependencies() []string { return s.dependencies }
func (s *orderService) Init() error {
	*s.initOrder = append(*s.initOrder, s.name)
	return nil
}
func (s *orderService) Start() error {
	*s.startOrder = append(*s.startOrder, s.name)
	return nil
}
func (s *orderService) Stop() error {
	*s.stopOrder = append(*s.stopOrder, s.name)
	return nil
}

func TestHubLifecycleOrder(t *testing.T) {
	var initOrder, startOrder, stopOrder []string
	hub := NewHub()
	for _, spec := range []struct {
		name string
		deps []string
	}{
		{name: "b", deps: []string{"a"}},
		{name: "z"},
		{name: "a"},
	} {
		if err := hub.Register(&orderService{
			name: spec.name, dependencies: spec.deps,
			initOrder: &initOrder, startOrder: &startOrder, stopOrder: &stopOrder,
		}); err != nil {
			t.Fatal(err)
		}
	}

	if err := hub.InitAll(); err != nil {
		t.Fatal(err)
	}
	if err := hub.StartAll(); err != nil {
		t.Fatal(err)
	}
	hub.StopAll()

	wantForward := []string{"a", "z", "b"}
	if !reflect.DeepEqual(initOrder, wantForward) {
		t.Fatalf("init order = %v, want %v", initOrder, wantForward)
	}
	if !reflect.DeepEqual(startOrder, wantForward) {
		t.Fatalf("start order = %v, want %v", startOrder, wantForward)
	}
	wantStop := []string{"b", "z", "a"}
	if !reflect.DeepEqual(stopOrder, wantStop) {
		t.Fatalf("stop order = %v, want %v", stopOrder, wantStop)
	}
}
