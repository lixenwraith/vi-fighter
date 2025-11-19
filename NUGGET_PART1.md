# Nugget Feature - Core Component & System Foundation

## Overview
Implemented the basic nugget infrastructure for the vi-fighter typing game. Nuggets are collectible entities that appear randomly on the game field. This part establishes the foundation without interaction mechanics.

## Implementation Details

### 1. NuggetComponent (`components/nugget_component.go`)
Created a simple data-only component following ECS principles:
```go
type NuggetComponent struct {
    ID        int       // Unique identifier for tracking
    SpawnTime time.Time // When this nugget was spawned
}
```

**Design Notes:**
- Pure data structure with no logic (ECS compliance)
- Unique ID for tracking and distinguishing between different nuggets
- SpawnTime allows for future time-based mechanics (expiration, animations)

### 2. Color Constants (`render/colors.go`)
Added two new color constants for nugget rendering:
```go
RgbNuggetOrange = tcell.NewRGBColor(255, 165, 0)  // Same as insert cursor
RgbNuggetDark   = tcell.NewRGBColor(101, 67, 33)  // Dark brown for contrast
```

**Design Notes:**
- Orange color matches the insert cursor for visual consistency
- Dark brown reserved for future use (potential contrast effects, collection animations)

### 3. NuggetSystem (`systems/nugget_system.go`)
Implemented the core nugget management system with the following features:

**Priority:** 18 (runs between SpawnSystem(15) and GoldSequenceSystem(20))

**State Management:**
- `activeNugget`: `atomic.Uint64` - Entity ID of the currently active nugget (0 if none)
- `nuggetID`: `atomic.Int32` - Atomic counter for generating unique nugget IDs
- `lastSpawnAttempt`: `time.Time` - Tracks last spawn attempt (protected by mutex)

**Spawn Logic:**
- Spawns one nugget every 5 seconds (configurable via `nuggetSpawnIntervalSeconds`)
- Only one nugget can be active at a time
- Automatic respawn after previous nugget is collected/destroyed

**Position Finding Algorithm:**
- Random position selection with collision detection
- Cursor exclusion zone: 5 horizontal, 3 vertical (same as SpawnSystem)
- Checks for overlaps with existing entities via spatial index
- Maximum 100 attempts to find valid position (`nuggetMaxAttempts`)
- Returns (-1, -1) if no valid position found

**Thread Safety:**
- Uses `atomic.Uint64` for lock-free active nugget tracking
- Uses `atomic.Int32` for thread-safe ID generation
- Mutex protection for spawn timing state
- Atomic reads of cursor position from GameState

**Entity Composition:**
Each nugget entity has three components:
1. `PositionComponent` - Spatial location (x, y)
2. `CharacterComponent` - Visual representation (orange '●' character)
3. `NuggetComponent` - Nugget-specific data (ID, spawn time)

### 4. Rendering (`render/terminal_renderer.go`)
No changes required - nuggets automatically render via existing `drawCharacters()` method since they have both `PositionComponent` and `CharacterComponent`.

**Visual Appearance:**
- Character: '●' (filled circle)
- Color: Orange (RGB 255, 165, 0)
- Background: Standard game background

### 5. System Registration (`cmd/vi-fighter/main.go`)
Registered NuggetSystem in the main game loop:
```go
nuggetSystem := systems.NewNuggetSystem(ctx)
ctx.World.AddSystem(nuggetSystem)
```

**Integration:**
- Added after SpawnSystem (priority 15)
- Before DecaySystem and GoldSequenceSystem (priority 20+)
- Follows standard system initialization pattern

## Testing
All tests pass with race detector:
```bash
go test -race ./...
```

**Test Results:**
- ✅ All packages build successfully
- ✅ No race conditions detected
- ✅ Integration with existing systems verified

## Architecture Compliance
This implementation strictly follows vi-fighter architecture principles:

1. **ECS Pattern:**
   - Components contain only data
   - Systems contain all logic
   - World is single source of truth

2. **State Ownership Model:**
   - Atomic operations for active nugget reference (lock-free reads)
   - Mutex protection for spawn timing (clock-tick state)
   - No local dimension caching - reads from GameContext

3. **Concurrency Model:**
   - Runs synchronously in main game loop
   - No autonomous goroutines
   - Atomic state for thread-safe checks
   - Follows existing system patterns (GoldSequenceSystem, SpawnSystem)

4. **Spatial Indexing:**
   - Updates spatial index on nugget spawn
   - Uses spatial index for collision detection
   - Automatic cleanup via SafeDestroyEntity

## Current Limitations (By Design)
This is **foundation-only** implementation:
- ✅ Nuggets spawn randomly
- ✅ Nuggets render correctly with orange color
- ✅ Collision detection prevents overlap
- ✅ Cursor exclusion zone prevents spawn near cursor
- ❌ No collection mechanics (future part)
- ❌ No interaction with player (future part)
- ❌ No respawn after collection (future part)
- ❌ No effects on game state (future part)

## Next Steps (Future Parts)
The following features are planned for subsequent implementations:
1. Collection mechanics when player moves cursor over nugget
2. Effects on game state (heat, score, or other mechanics)
3. Visual feedback on collection (flash, particles, etc.)
4. Respawn logic after collection
5. Advanced features (expiration, animations, etc.)

## Files Modified
- `components/nugget_component.go` - NEW
- `systems/nugget_system.go` - NEW
- `render/colors.go` - Modified (added color constants)
- `cmd/vi-fighter/main.go` - Modified (registered system)

## Verification
To test the implementation:
1. Build: `go build ./cmd/vi-fighter`
2. Run: `./vi-fighter`
3. Observe: Orange '●' character should appear randomly on screen every 5 seconds
4. Verify: Only one nugget visible at a time
5. Verify: Nugget never spawns near cursor
6. Verify: Nugget never overlaps with existing characters
