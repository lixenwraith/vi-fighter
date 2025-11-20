package systems

import (
	"reflect"
	"sync/atomic"
	"time"

	"github.com/lixenwraith/vi-fighter/components"
	"github.com/lixenwraith/vi-fighter/engine"
)

// DrainSystem manages the drain mechanic that spawns when score > 0,
// moves toward the cursor, destroys entities in a 3x3 area, and drains score.
//
// Lifecycle:
// - Spawns when score transitions from 0 to > 0
// - Active while score > 0
// - Despawns when score reaches 0
//
// Behavior:
// - Moves toward cursor every 250ms using Manhattan distance pathfinding
// - Destroys entities in 3x3 area centered on drain position
// - Drains 10 score every 250ms when centered on cursor
// - Can destroy Content characters (Blue, Green, Red), Nuggets, Gold sequences, and Decay entities
type DrainSystem struct {
	ctx            *engine.GameContext
	drainEntity    atomic.Value // Stores engine.Entity (0 if inactive)
	lastSpawnCheck atomic.Int64 // Unix nano timestamp of last spawn check
}

// NewDrainSystem creates a new drain system
func NewDrainSystem(ctx *engine.GameContext) *DrainSystem {
	ds := &DrainSystem{
		ctx: ctx,
	}

	// Initialize drain entity to 0 (inactive)
	ds.drainEntity.Store(engine.Entity(0))
	ds.lastSpawnCheck.Store(0)

	return ds
}

// Priority returns the system's priority
// Run after spawn system (10) but before decay system (20)
func (ds *DrainSystem) Priority() int {
	return 15
}

// Update runs the drain system logic synchronously in the main game loop
// Handles spawn/despawn based on score, movement, destruction, and score draining
func (ds *DrainSystem) Update(world *engine.World, dt time.Duration) {
	// Part 1: Foundation - basic lifecycle only
	// Actual spawn/despawn/movement/destruction logic will be implemented in later parts

	// Check if drain should spawn or despawn based on score
	currentScore := ds.ctx.State.GetScore()
	currentDrain := ds.drainEntity.Load().(engine.Entity)

	if currentScore > 0 && currentDrain == 0 {
		// Should spawn drain (Part 2)
		// ds.spawnDrain(world)
	} else if currentScore <= 0 && currentDrain != 0 {
		// Should despawn drain (Part 2)
		// ds.despawnDrain(world)
	}

	// If drain is active, update movement and effects (Parts 3-7)
	if currentDrain != 0 {
		// Check if entity still exists
		drainType := reflect.TypeOf(components.DrainComponent{})
		if _, exists := world.GetComponent(currentDrain, drainType); !exists {
			// Entity was destroyed, reset state
			ds.drainEntity.Store(engine.Entity(0))
			return
		}

		// Movement logic (Part 3)
		// ds.moveDrain(world, currentDrain, dt)

		// Destruction logic (Parts 4, 5, 7)
		// ds.processDrainEffects(world, currentDrain)

		// Score draining logic (Part 6)
		// ds.drainScore(world, currentDrain)
	}
}

// spawnDrain creates a new drain entity at the cursor position (Part 2)
func (ds *DrainSystem) spawnDrain(world *engine.World) {
	// Implementation in Part 2
}

// despawnDrain removes the drain entity (Part 2)
func (ds *DrainSystem) despawnDrain(world *engine.World) {
	// Implementation in Part 2
}

// moveDrain calculates path and moves drain toward cursor (Part 3)
func (ds *DrainSystem) moveDrain(world *engine.World, drainEntity engine.Entity, dt time.Duration) {
	// Implementation in Part 3
}

// processDrainEffects handles entity destruction in 3x3 area (Parts 4, 5, 7)
func (ds *DrainSystem) processDrainEffects(world *engine.World, drainEntity engine.Entity) {
	// Implementation in Parts 4, 5, 7
}

// destroyEntitiesInRadius destroys entities in 3x3 area (Part 4)
func (ds *DrainSystem) destroyEntitiesInRadius(world *engine.World, centerX, centerY int) {
	// Implementation in Part 4
}

// handleGoldInteraction handles special gold sequence destruction (Part 5)
func (ds *DrainSystem) handleGoldInteraction(world *engine.World, centerX, centerY int) {
	// Implementation in Part 5
}

// drainScore reduces score when centered on cursor (Part 6)
func (ds *DrainSystem) drainScore(world *engine.World, drainEntity engine.Entity) {
	// Implementation in Part 6
}
