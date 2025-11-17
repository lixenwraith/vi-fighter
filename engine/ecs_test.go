package engine

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

// MockComponent for testing
type MockComponent struct {
	Value int
}

// Test entity query caching
func TestQueryCaching(t *testing.T) {
	world := NewWorld()

	// Create entities with components
	mockType := reflect.TypeOf(MockComponent{})
	entity1 := world.CreateEntity()
	entity2 := world.CreateEntity()
	entity3 := world.CreateEntity()

	world.AddComponent(entity1, MockComponent{Value: 1})
	world.AddComponent(entity2, MockComponent{Value: 2})
	world.AddComponent(entity3, MockComponent{Value: 3})

	// First query - should populate cache
	result1 := world.GetEntitiesWith(mockType)
	if len(result1) != 3 {
		t.Errorf("Expected 3 entities, got %d", len(result1))
	}

	// Second query - should use cache (verify by checking it's fast)
	result2 := world.GetEntitiesWith(mockType)
	if len(result2) != 3 {
		t.Errorf("Expected 3 entities from cache, got %d", len(result2))
	}

	// Modify world - should invalidate cache
	entity4 := world.CreateEntity()
	world.AddComponent(entity4, MockComponent{Value: 4})

	// Next query should see the new entity
	result3 := world.GetEntitiesWith(mockType)
	if len(result3) != 4 {
		t.Errorf("Expected 4 entities after cache invalidation, got %d", len(result3))
	}
}

// Test cache invalidation on entity destruction
func TestQueryCacheInvalidationOnDestroy(t *testing.T) {
	world := NewWorld()
	mockType := reflect.TypeOf(MockComponent{})

	entity1 := world.CreateEntity()
	world.AddComponent(entity1, MockComponent{Value: 1})

	// Populate cache
	result1 := world.GetEntitiesWith(mockType)
	if len(result1) != 1 {
		t.Errorf("Expected 1 entity, got %d", len(result1))
	}

	// Destroy entity and verify cache is invalidated
	world.DestroyEntity(entity1)
	result2 := world.GetEntitiesWith(mockType)
	if len(result2) != 0 {
		t.Errorf("Expected 0 entities after destruction, got %d", len(result2))
	}
}

// Test component type index maintenance
func TestComponentTypeIndexMaintenance(t *testing.T) {
	world := NewWorld()
	mockType := reflect.TypeOf(MockComponent{})

	// Create entity and add component
	entity := world.CreateEntity()
	world.AddComponent(entity, MockComponent{Value: 42})

	// Verify it's in the type index
	entities := world.GetEntitiesWith(mockType)
	if len(entities) != 1 || entities[0] != entity {
		t.Errorf("Expected entity in type index")
	}

	// Remove component
	world.RemoveComponent(entity, mockType)

	// Verify it's removed from type index
	entities = world.GetEntitiesWith(mockType)
	if len(entities) != 0 {
		t.Errorf("Expected entity removed from type index, got %d entities", len(entities))
	}

	// Add it back
	world.AddComponent(entity, MockComponent{Value: 43})

	// Verify it's back in the index
	entities = world.GetEntitiesWith(mockType)
	if len(entities) != 1 {
		t.Errorf("Expected entity back in type index")
	}
}

// Test thread-safe concurrent access
func TestConcurrentSystemIteration(t *testing.T) {
	world := NewWorld()

	// Create a mock system
	mockSystem := &MockSystem{priority: 10}
	world.AddSystem(mockSystem)

	// Run updates concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			world.Update(16 * time.Millisecond)
		}()
	}

	wg.Wait()

	// Verify system was called (no crashes = success for thread safety)
	if mockSystem.updateCount == 0 {
		t.Error("System was never updated")
	}
}

// MockSystem for testing
type MockSystem struct {
	priority    int
	updateCount int
	mu          sync.Mutex
}

func (s *MockSystem) Update(world *World, dt time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateCount++
}

func (s *MockSystem) Priority() int {
	return s.priority
}

// Test spatial index cleanup
func TestSpatialIndexCleanup(t *testing.T) {
	world := NewWorld()

	entity := world.CreateEntity()
	x, y := 5, 10

	// Add to spatial index
	world.UpdateSpatialIndex(entity, x, y)

	// Verify it's there
	foundEntity := world.GetEntityAtPosition(x, y)
	if foundEntity != entity {
		t.Errorf("Expected entity at position (%d,%d), got %d", x, y, foundEntity)
	}

	// Remove from spatial index
	world.RemoveFromSpatialIndex(x, y)

	// Verify it's gone
	foundEntity = world.GetEntityAtPosition(x, y)
	if foundEntity != 0 {
		t.Errorf("Expected no entity at position after removal, got %d", foundEntity)
	}
}

// Test that DestroyEntity cleans up spatial index
func TestDestroyEntityCleansSpatialIndex(t *testing.T) {
	world := NewWorld()

	entity := world.CreateEntity()
	x, y := 3, 7
	world.UpdateSpatialIndex(entity, x, y)

	// Destroy entity
	world.DestroyEntity(entity)

	// Spatial index should be cleaned up
	foundEntity := world.GetEntityAtPosition(x, y)
	if foundEntity != 0 {
		t.Errorf("Expected spatial index cleaned up after entity destruction, got entity %d", foundEntity)
	}
}
