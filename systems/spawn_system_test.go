package systems

import (
	"math"
	"testing"

	"github.com/lixenwraith/vi-fighter/engine"
)

// Test spawn exclusion zone uses OR logic
func TestSpawnExclusionZone(t *testing.T) {
	world := engine.NewWorld()
	cursorX, cursorY := 50, 50
	gameWidth, gameHeight := 100, 100

	spawnSystem := NewSpawnSystem(gameWidth, gameHeight, cursorX, cursorY)

	// Test many spawn attempts
	for i := 0; i < 100; i++ {
		x, y := spawnSystem.findValidPosition(world, 5)

		if x < 0 || y < 0 {
			// No valid position found is acceptable
			continue
		}

		// Verify spawn is NOT in exclusion zone
		// Exclusion zone uses OR: either X is far OR Y is far (or both)
		// So spawn should be excluded if (X is close) OR (Y is close)
		// Valid spawn means: (X is far) OR (Y is far) must be FALSE for exclusion
		// Which means: NOT((X is far) OR (Y is far)) = (X is close) AND (Y is close) should be FALSE

		// In other words: at least one of X or Y should be far from cursor
		xFar := math.Abs(float64(x-cursorX)) > 5
		yFar := math.Abs(float64(y-cursorY)) > 3

		if !xFar && !yFar {
			t.Errorf("Spawn at (%d,%d) is too close to cursor (%d,%d) in both X and Y - exclusion zone failed",
				x, y, cursorX, cursorY)
		}

		// At least one should be far
		if !(xFar || yFar) {
			t.Errorf("Spawn at (%d,%d) violates exclusion zone from cursor (%d,%d)", x, y, cursorX, cursorY)
		}
	}
}

// Test that spawns respect existing entities
func TestSpawnNoOverlap(t *testing.T) {
	world := engine.NewWorld()
	cursorX, cursorY := 10, 10
	gameWidth, gameHeight := 80, 40

	spawnSystem := NewSpawnSystem(gameWidth, gameHeight, cursorX, cursorY)

	// Place an entity
	entity := world.CreateEntity()
	world.UpdateSpatialIndex(entity, 50, 20)

	// Try to spawn - should not overlap with existing entity
	// This is harder to test definitively, but we can verify the logic exists
	x, y := spawnSystem.findValidPosition(world, 1)
	if x == 50 && y == 20 {
		t.Error("Spawn system allowed overlap with existing entity")
	}
}

// Test spawn respects game boundaries
func TestSpawnWithinBoundaries(t *testing.T) {
	world := engine.NewWorld()
	gameWidth, gameHeight := 40, 20
	cursorX, cursorY := 20, 10

	spawnSystem := NewSpawnSystem(gameWidth, gameHeight, cursorX, cursorY)

	seqLength := 10
	x, y := spawnSystem.findValidPosition(world, seqLength)

	if x >= 0 && y >= 0 {
		// If a position was found, verify it's within bounds
		if x+seqLength > gameWidth {
			t.Errorf("Spawn at x=%d with length=%d exceeds game width=%d", x, seqLength, gameWidth)
		}
		if y >= gameHeight {
			t.Errorf("Spawn at y=%d exceeds game height=%d", y, gameHeight)
		}
	}
}
