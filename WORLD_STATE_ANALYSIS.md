# Vi-Fighter World State Management Analysis

## Executive Summary

Vi-Fighter uses a **dual-state architecture** where world data is stored in two separate places that can easily become desynchronized:

1. **ECS World** (engine/ecs.go) - The authoritative source for all entity data (characters, positions, drains, etc.)
2. **GameState** (engine/game_state.go) - Fast-path atomic values for real-time rendering and game mechanics (cursor position, drain position)

This document identifies where state is stored, how systems access it, and critical synchronization points.

---

## 1. WHERE CHARACTER/WORLD DATA IS STORED

### 1.1 Primary Character Storage: ECS World
**File:** `engine/ecs.go`  
**Data Structure:** `World` struct with:
- `entities`: `map[Entity]map[reflect.Type]Component` - All entity data
- `spatialIndex`: `map[int]map[int]Entity` - [y][x] → Entity for fast position lookups
- `componentsByType`: `map[reflect.Type][]Entity` - Reverse index for fast component-based queries

**Key Components:**
- `PositionComponent` (components/position.go): X, Y coordinates
- `CharacterComponent` (components/character.go): Rune + Style
- `SequenceComponent` (components/character.go): Sequence ID, Type (Blue/Green/Red/Gold), Level (Dark/Normal/Bright)
- `DrainComponent` (components/drain.go): Position + timing state
- `NuggetComponent`, `FallingDecayComponent`, etc.

**Mutation Methods:**
- `CreateEntity()` - Creates new entity
- `AddComponent(entity, component)` - Adds/updates component
- `SafeDestroyEntity(entity)` - Removes entity atomically
- `UpdateSpatialIndex(entity, x, y)` - Updates position cache
- `GetEntityAtPosition(x, y)` - Fast lookup by position

### 1.2 Secondary State Storage: GameState Atomics
**File:** `engine/game_state.go`  
**Purpose:** Fast-path atomic values for systems that read frequently without needing full component data

**Cursor Position** (PRIMARY SYNC POINT):
```go
CursorX atomic.Int32  // Current cursor X
CursorY atomic.Int32  // Current cursor Y
```
- Read by: Drain system, Spawn system
- Updated by: Input handler (all cursor movements), Search mode, Motion commands
- **Critical:** Must stay in sync with input handling code

**Drain Entity Position** (RENDERING/MECHANICS MISMATCH POINT):
```go
DrainActive atomic.Bool    // Is drain spawned?
DrainEntity atomic.Uint64  // Entity ID for quick lookup
DrainX atomic.Int32        // Cached X position
DrainY atomic.Int32        // Cached Y position
```
- Read by: Rendering system, Drain collision detection
- Updated by: DrainSystem after movement
- **Warning:** Renderer reads from GameState (atomic), Drain mechanic reads from World ECS

**Color Count Tracking** (tracks how many of each color exist):
```go
BlueCountBright, BlueCountNormal, BlueCountDark atomic.Int64
GreenCountBright, GreenCountNormal, GreenCountDark atomic.Int64
```

**Phase and Game State** (mutex-protected):
```go
mu sync.RWMutex
CurrentPhase GamePhase
SpawnState, GoldState, DecayState, CleanerState
```

---

## 2. HOW SEARCH, MOTION COMMANDS ACCESS WORLD STATE

### 2.1 Search Mode (modes/search.go)

**Process:**
1. `PerformSearch(ctx, searchText, forward)` - Main entry point
2. `buildCharacterGrid(ctx)` - **Reads from World ECS**
   ```go
   entities := ctx.World.GetEntitiesWith(posType, charType)
   // Builds map[Point]rune from PositionComponent + CharacterComponent
   ```
3. `searchForward()` or `searchBackward()` - Pattern matching on grid
4. **When match found:**
   ```go
   ctx.CursorX = x
   ctx.CursorY = y
   ctx.State.SetCursorX(x)      // ✅ Syncs to GameState
   ctx.State.SetCursorY(y)      // ✅ Syncs to GameState
   ```

**Bug Fixed (Commit 3d136c8):**
- Previously: Only updated `ctx.CursorX/Y`, forgot to sync `ctx.State`
- Result: Drain continued tracking old cursor position from GameState while visual cursor was at new position
- Fix: Added sync calls to all search result points

### 2.2 Motion Commands (modes/motions.go)

**General Motions (w, e, b, h, j, k, l, $, 0, etc.):**
- Implemented in `ExecuteMotion(ctx, cmd, count)`
- Updates `ctx.CursorX` and/or `ctx.CursorY`
- **At end of ExecuteMotion:**
  ```go
  if ctx.State != nil {
      ctx.State.SetCursorX(ctx.CursorX)  // ✅ Syncs
      ctx.State.SetCursorY(ctx.CursorY)  // ✅ Syncs
  }
  ```

**Find Commands (f, F, t, T) - POTENTIAL SYNC ISSUE:**
- `ExecuteFindChar(ctx, targetChar, count)` - Find forward
- `ExecuteFindCharBackward(ctx, targetChar, count)` - Find backward  
- `ExecuteTillChar(ctx, targetChar, count)` - Till forward
- `ExecuteTillCharBackward(ctx, targetChar, count)` - Till backward

**Problem:**
```go
func ExecuteFindChar(ctx *engine.GameContext, targetChar rune, count int) {
    // Searches ECS for character matches
    entities := ctx.World.GetEntitiesWith(posType, charType)
    for x := ctx.CursorX + 1; x < ctx.GameWidth; x++ {
        // ... find logic ...
        ctx.CursorX = x
        return  // ❌ NO SYNC TO ctx.State!
    }
    if lastMatchX != -1 {
        ctx.CursorX = lastMatchX  // ❌ NO SYNC TO ctx.State!
    }
}
```

**Call Stack:**
```
input.go:265 - ExecuteFindChar() called directly from input handler
input.go:272 - return true (no cursor sync happens)
```

**Risk:** If Drain system runs before next input event, it reads stale cursor position from GameState

### 2.3 Character Grid Access Pattern

Both Search and Motions use the same pattern:
1. Call `ctx.World.GetEntitiesWith(posType, charType)` - Gets all character entities
2. Iterate through entities, reading PositionComponent and CharacterComponent
3. Compare position with target or pattern
4. Update `ctx.CursorX/Y` based on result
5. **Critical:** Must sync to `ctx.State` after update

---

## 3. HOW DRAIN, CLEANERS, DECAY REMOVE CHARACTERS

### 3.1 Drain System (systems/drain_system.go)

**Lifecycle:**
1. **Spawn:** When `score > 0` and drain not active
   - Creates entity in World
   - Adds PositionComponent + DrainComponent
   - Updates spatial index
   - Syncs position to GameState atomics (DrainX, DrainY)

2. **Movement:** Clock-based (every `DrainMoveInterval`)
   ```go
   // Read cursor from GameState atomic
   cursor := s.ctx.State.ReadCursorPosition()
   
   // Update both World ECS and GameState
   drain.X = newX
   drain.Y = newY
   world.AddComponent(entity, drain)         // Update DrainComponent
   world.AddComponent(entity, pos)           // Update PositionComponent
   world.UpdateSpatialIndex(entity, newX, newY)
   s.ctx.State.SetDrainX(newX)               // ✅ Sync to atomic
   s.ctx.State.SetDrainY(newY)               // ✅ Sync to atomic
   ```

3. **Collision Detection:**
   ```go
   // Read drain position from GameState (atomic)
   drainX := s.ctx.State.GetDrainX()
   drainY := s.ctx.State.GetDrainY()
   
   // Query World ECS for entity at position
   entity := world.GetEntityAtPosition(drainX, drainY)
   
   // Remove entity if collision
   world.SafeDestroyEntity(entity)
   ```

4. **Despawn:** When `score <= 0`
   - Removes from World ECS
   - Clears GameState atomics

**Critical Sync Points:**
- **Cursor position used for targeting:** Reads from `GameState.ReadCursorPosition()`
- **Collision detection:** Uses `GameState` position as source-of-truth (via `GetEntityAtPosition()`)

### 3.2 Cleaner System (systems/cleaner_system.go)

**Process:**
1. Spawned when gold sequence completed at max heat
2. Sweeps across rows containing Red characters
3. **Character removal:**
   ```go
   world.SafeDestroyEntity(entity)  // Uses World ECS
   ```
4. Creates RemovalFlashComponent for visual feedback

**State:** Uses `cleanerDataMap` and atomic flags, not direct GameState

### 3.3 Decay System (systems/decay_system.go)

**Process:**
1. Activated when gold sequence completes
2. Removes oldest characters from each row  
3. **Character removal:**
   ```go
   world.SafeDestroyEntity(entity)  // Uses World ECS
   ```

**All systems use `world.SafeDestroyEntity()` which:**
- Removes from spatial index
- Removes from component type indices
- Deletes from entities map
- Atomically ensures consistency

---

## 4. HOW RENDERING SYSTEM ACCESSES WORLD DATA

### 4.1 Terminal Renderer (render/terminal_renderer.go)

**Process:**
1. `RenderFrame()` called each frame
2. Increments frame counter atomically
3. Clears screen
4. Renders in order (ensures proper layering):

**Drawing Order:**
```
1. Heat meter
2. Line numbers  
3. Ping highlights (cursor row/column)
4. Characters - reads from World ECS
   world.GetEntitiesWith(posType, charType)
5. Falling decay - reads from World ECS
6. Cleaners - reads from cleanerSystem snapshots
7. Removal flashes - reads from World ECS
8. Drain entity - reads from GameState ATOMICS ❌
   ctx.State.GetDrainX()
   ctx.State.GetDrainY()
9. Cursor (if not in search mode)
```

### 4.2 Critical Observation: Dual Data Sources

**Characters rendered from:**
```go
// Terminal Renderer
posType := reflect.TypeOf(components.PositionComponent{})
charType := reflect.TypeOf(components.CharacterComponent{})
entities := world.GetEntitiesWith(posType, charType)
// Reads directly from World ECS
```

**Drain rendered from:**
```go
// Terminal Renderer
drainX := ctx.State.GetDrainX()  // Reads from GameState atomic
drainY := ctx.State.GetDrainY()  // Reads from GameState atomic
```

**Problem:**
- Drain position cached in GameState (atomic for speed)
- If GameState not synced with World ECS DrainComponent, visual position mismatches

---

## 5. SEPARATE CACHES/INDICES STORING CHARACTER POSITIONS

### 5.1 World Spatial Index
**Location:** `engine/ecs.go:27`
```go
spatialIndex map[int]map[int]Entity  // [y][x] -> Entity
```

**Purpose:** O(1) position lookups for collision detection
**Mutated by:** All entity position changes
**Consistency:** Maintained via `UpdateSpatialIndex()` and `SafeDestroyEntity()`

### 5.2 Component Type Index
**Location:** `engine/ecs.go:29`
```go
componentsByType map[reflect.Type][]Entity  // Type -> [Entities]
```

**Purpose:** Fast queries by component type
**Used for:** 
- `GetEntitiesWith(CharacterComponent, PositionComponent)`
- `GetEntitiesWith(DrainComponent)`
- etc.

### 5.3 Rendering Buffer (Not Currently Used)
**Location:** `core/buffer.go`
```go
Buffer struct {
    lines [][]Cell           // 2D grid
    spatial map[Point]uint64  // Position -> entity ID
}
```

**Status:** Has spatial index but not actively kept in sync
**Purpose:** Could optimize rendering if maintained

### 5.4 Local Cursor Position (GameContext)
**Location:** `engine/game.go:49-50`
```go
CursorX, CursorY int  // Local to input handling
```

**Purpose:** Input handler state
**Must sync to:** `ctx.State.CursorX/Y` (GameState atomics)

### 5.5 GameState Atomics (Secondary Cache)
**Location:** `engine/game_state.go:14-54`
```go
CursorX, CursorY atomic.Int32        // Cursor position cache
DrainX, DrainY atomic.Int32          // Drain position cache
DrainEntity atomic.Uint64            // Drain entity ID
```

**Purpose:** Fast access for systems that need real-time cursor/drain data
**Consistency Model:** Updated after changes to primary sources

---

## CRITICAL SYNCHRONIZATION POINTS

### 1. Cursor Position (MOST CRITICAL)

**Sources of updates:**
- Basic motions (h, j, k, l) ✅
- Word motions (w, e, b, W, E, B) ✅
- Line motions (0, $, ^) ✅
- Find commands (f, F, t, T) ❌ **POTENTIAL BUG**
- Search (/, n, N) ✅ (fixed in commit 3d136c8)
- Delete operator (d)
- Tab completion jumps

**Sync requirements:**
```go
// After ANY cursor update:
ctx.CursorX = newX
ctx.CursorY = newY
if ctx.State != nil {
    ctx.State.SetCursorX(newX)
    ctx.State.SetCursorY(newY)
}
```

### 2. Drain Entity Position

**Sources of updates:**
1. Spawn - `DrainSystem.spawnDrain()` ✅
2. Movement - `DrainSystem.updateDrainMovement()` ✅
3. Despawn - `DrainSystem.despawnDrain()` ✅

**Dual updates required:**
```go
// Update World ECS
world.AddComponent(entity, drainComponent)
world.AddComponent(entity, positionComponent)
world.UpdateSpatialIndex(entity, x, y)

// Sync to GameState atomics
ctx.State.SetDrainX(x)
ctx.State.SetDrainY(y)
```

### 3. Character Removal

**All removal paths use `world.SafeDestroyEntity()`:**
- Drain collisions ✅
- Cleaner sweeping ✅
- Decay animation ✅
- Delete operator (x command) ✅

---

## IDENTIFIED ISSUES

### Issue #1: Find Commands Don't Sync Cursor to GameState
**Severity:** MEDIUM (affects gameplay when using f/F/t/T with Drain)
**Location:** `modes/motions.go` lines 220, 231, 275, 286, 330, 342, 388, 400

**Example:**
```go
// ExecuteFindChar (line 220)
ctx.CursorX = x
return  // ❌ Missing ctx.State.SetCursorX(x)

// ExecuteFindChar (line 231)
ctx.CursorX = lastMatchX
// ❌ Missing ctx.State.SetCursorX(lastMatchX)
```

**Impact:** After using f/F/t/T command, if Drain system runs before next input, it reads stale cursor position from GameState

**Fix:** Add cursor sync calls in find command functions similar to Search mode (commit 3d136c8)

### Issue #2: RepeatFindChar Also Missing Sync
**Severity:** MEDIUM
**Location:** `modes/motions.go:405-455`

**Impact:** Using `;` or `,` to repeat finds also doesn't sync cursor

**Fix:** Same as Issue #1

### Issue #3: Rendering Uses Two Different Data Sources for Entity Positions

**Characters from:** World ECS directly
**Drain from:** GameState atomics

**Risk:** If GameState not kept in sync perfectly, drain visual position diverges from actual entity position

---

## RECOMMENDED SYNCHRONIZATION PATTERN

### Template for All Cursor-Moving Commands

```go
func MyCommand(ctx *engine.GameContext) {
    // ... calculate new cursor position ...
    
    newX := calculateX(ctx)
    newY := calculateY(ctx)
    
    // Update local cursor
    ctx.CursorX = newX
    ctx.CursorY = newY
    
    // CRITICAL: Sync to GameState for systems that read cursor
    if ctx.State != nil {
        ctx.State.SetCursorX(newX)
        ctx.State.SetCursorY(newY)
    }
}
```

### For Multi-Component Updates

```go
// When updating drain position:
// 1. Update World ECS
world.AddComponent(entity, updatedComponent)
world.UpdateSpatialIndex(entity, x, y)

// 2. Update GameState atomics immediately after
ctx.State.SetDrainX(x)
ctx.State.SetDrainY(y)
```

---

## SUMMARY TABLE

| Data Structure | Purpose | Primary Update | Secondary Sync | Consistency |
|---|---|---|---|---|
| World.entities | All entity data | AddComponent | - | Mutex protected |
| World.spatialIndex | Position → Entity lookup | UpdateSpatialIndex | - | Part of World.mu |
| GameState.CursorX/Y | Cursor position for systems | Input handlers | ALL cursor commands | Atomic |
| GameState.DrainX/Y | Drain position for rendering | DrainSystem | Each movement | Atomic |
| GameState.Drain* flags | Drain lifecycle | DrainSystem | Spawn/despawn | Atomic |
| GameContext.CursorX/Y | Local input state | Input handlers | GameState | Not protected |
| GameState.ColorCounts | Character counts | Spawn/Destroy | Add/remove operations | Atomic |

---

## AUDIT CHECKLIST

Use this checklist when making changes to world state:

- [ ] Read operations use consistent source (World ECS OR GameState)
- [ ] Write operations update ALL necessary locations:
  - [ ] World ECS entity
  - [ ] World spatial index (if position changed)
  - [ ] GameState atomics (if cursor/drain position changed)
- [ ] Cursor position syncs after EVERY change
- [ ] Drain position syncs after EVERY movement
- [ ] SafeDestroyEntity used for all entity removals
- [ ] No direct spatial index mutations (use API)
- [ ] Tests verify both local and GameState state

