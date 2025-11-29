# Final Technical Plan: Z-Index Integration & Gold Bootstrap Decoupling

---

## Part 1: Z-Index Engine Integration

### 1.1 Problem Restatement

Z-index serves **two distinct purposes**:

| Domain | Purpose | Current State |
|--------|---------|---------------|
| **Engine Logic** | Entity selection from multi-entity cells | Not implemented; workarounds exist |
| **Rendering** | Visual layering when entities overlap | Ad-hoc `if/else` in renderers |

The current `EnergySystem` workaround demonstrates the problem:
```go
// We cannot use GetEntityAt because the Cursor entity masks the character
// Query all entities with Position and Character components
entities := world.Query().With(world.Positions).With(world.Characters)...
```

### 1.2 Design Requirements

1. **Multi-Entity Cell Support:** `PositionStore` must track multiple entities per cell
2. **Z-Index Resolution:** Selection API must use z-index priority
3. **Backward Compatibility:** Existing `GetEntityAt()` behavior preserved
4. **Future Compatibility:** Support for multiple dynamic entities (orbiting decay, falling entities)

### 1.3 Architecture Changes

#### PositionStore Enhancement

```
Current Cell Structure:
┌─────────────────────┐
│ Cell                │
│   Count: int        │  ← Only tracks count
│   Entity: Entity    │  ← Single entity (last write wins)
└─────────────────────┘

Enhanced Cell Structure:
┌─────────────────────────────┐
│ Cell                        │
│   Entities: []Entity        │  ← All entities at this position
│   TopEntity: Entity         │  ← Cached highest z-index entity
│   TopZIndex: int            │  ← Cached z-index value
└─────────────────────────────┘
```

#### API Surface

| Method | Purpose | Z-Index Aware |
|--------|---------|---------------|
| `GetEntityAt(x, y)` | Returns highest z-index entity | Yes |
| `GetAllEntitiesAt(x, y)` | Returns all entities at position | No (returns slice) |
| `HasAny(x, y)` | Returns true if any entity present | No |
| `GetTopEntityFiltered(x, y, filter)` | Returns highest z-index matching filter | Yes |

#### Z-Index Hierarchy (Finalized)

```go
const (
    ZIndexBackground = 0    // Future: background effects
    ZIndexSpawnChar  = 100  // Spawned characters (interactable)
    ZIndexNugget     = 200  // Nuggets (interactable)
    ZIndexDecay      = 300  // Decay entities (non-interactable)
    ZIndexDrain      = 400  // Drain entity (non-interactable)
    ZIndexShield     = 500  // Shield effect (visual only)
    ZIndexCursor     = 1000 // Cursor (always on top)
)
```

### 1.4 Implementation Phases

**Phase 1:** Enhance `engine/z-index.go` with complete API
**Phase 2:** Modify `engine/position_store.go` for multi-entity tracking
**Phase 3:** Update consumers (renderers, energy system)

---

## Part 2: Gold Bootstrap Decoupling

### 2.1 Problem Restatement

Gold initialization is embedded in `GoldSystem.Update()`:
- `FirstUpdateTime` tracking
- `InitialSpawnComplete` flag
- Delay calculation inside system Update

This prevents:
- Clean game reset (`:new`)
- Debug state injection
- Future state advancement (A→B transitions)

### 2.2 Design Requirements

1. **Separation:** Bootstrap logic in scheduler, not systems
2. **Reset Support:** `:new` command triggers clean bootstrap
3. **State Advancement Compatible:** Design allows future `advanceState(from, to)` API
4. **Minimal Disruption:** Use existing `ClockScheduler` infrastructure

### 2.3 Architecture Changes

#### New Phase: `PhaseBootstrap`

```
Phase State Machine (Updated):
                                    
    ┌──────────────┐                
    │PhaseBootstrap│ ← NEW: Initial state
    └──────┬───────┘                
           │ delay elapsed          
           ▼                        
    ┌──────────────┐                
    │ PhaseNormal  │◄───────────────┐
    └──────┬───────┘                │
           │ gold spawns            │
           ▼                        │
    ┌──────────────┐                │
    │PhaseGoldActive│               │
    └──────┬───────┘                │
           │ timeout/complete       │
           ▼                        │
    ┌───────────────┐               │
    │PhaseGoldComplete│             │
    └──────┬────────┘               │
           │                        │
           ▼                        │
    ┌──────────────┐                │
    │ PhaseDecayWait│               │
    └──────┬───────┘                │
           │ timer expires          │
           ▼                        │
    ┌─────────────────┐             │
    │PhaseDecayAnimation│───────────┘
    └─────────────────┘  animation ends
```

#### GameState Changes

```go
// New fields
GameStartTime time.Time  // Set on NewGameState() and reset on :new

// Removed fields  
FirstUpdateTime time.Time      // DELETE
InitialSpawnComplete bool      // DELETE
```

#### Phase Transition Logic

```go
// In ClockScheduler.processTick()
case PhaseBootstrap:
    if gameNow.Sub(gs.GameStartTime) >= constants.GoldInitialSpawnDelay {
        gs.TransitionPhase(PhaseNormal, gameNow)
    }
```

### 2.4 Implementation Phases

**Phase 1:** Add `PhaseBootstrap` to `GameState` and phase machine
**Phase 2:** Migrate bootstrap logic to `ClockScheduler`
**Phase 3:** Strip initialization logic from `GoldSystem`
**Phase 4:** Update `:new` command for proper reset

---

## Part 3: Execution Prompts for Claude Code

### Task 1: Z-Index Engine Integration

---

#### Prompt 1.1: Enhance z-index.go API

```
## Task: Enhance engine/z-index.go with Complete Z-Index API

### Context
The file `engine/z-index.go` contains stub code for entity z-index priority. It defines constants and two functions (`GetZIndex`, `SelectTopEntity`) that are currently unused. We need to enhance this to be the single source of truth for entity layering.

### Requirements

1. **Update Z-Index Constants** with the following hierarchy (higher = on top):
   - ZIndexBackground = 0
   - ZIndexSpawnChar = 100
   - ZIndexNugget = 200  
   - ZIndexDecay = 300
   - ZIndexDrain = 400
   - ZIndexShield = 500
   - ZIndexCursor = 1000

2. **Keep existing functions** but ensure they work correctly:
   - `GetZIndex(world *World, e Entity) int` - returns z-index based on component checks
   - `SelectTopEntity(entities []Entity, world *World) Entity` - returns highest z-index entity

3. **Add new function** for filtered selection:
```go
// SelectTopEntityFiltered returns the entity with highest z-index that passes the filter
// Returns 0 if no entities pass the filter or slice is empty
// Filter receives entity and returns true if entity should be considered
func SelectTopEntityFiltered(entities []Entity, world *World, filter func(Entity) bool) Entity
```

4. **Add helper function** for checking if entity is interactable (Characters, Nuggets, GoldSequence):
```go
// IsInteractable returns true if the entity is an interactable game element
// Interactable entities: Characters (with SequenceComponent), Nuggets
// Non-interactable: Cursor, Drain, Decay, Shield, Flash
func IsInteractable(world *World, e Entity) bool
```

### Implementation Notes

- `GetZIndex` should check component stores in priority order (highest first) for early exit
- Check order: Cursors → Shields → Drains → Decays → Nuggets → default (SpawnChar)
- `IsInteractable` returns true only if entity has CharacterComponent AND SequenceComponent, OR has NuggetComponent
- Do NOT change any other files in this step

### File to modify
- `engine/z-index.go`

### Verification
After changes, the file should compile with `go build ./engine/...`
```

---

#### Prompt 1.2: Enhance PositionStore for Multi-Entity Support

```
## Task: Enhance engine/position_store.go for Multi-Entity Cell Support

### Context
The current `PositionStore` tracks only a single entity per cell. We need to support multiple entities at the same position for proper z-index based selection. This is required because multiple entities can occupy the same cell (e.g., Cursor on top of Character on top of Decay).

### Current Structure (in engine/position_store.go)
```go
type Cell struct {
    Count  int
    Entity Entity // Single entity - last write wins
}
```

### Required Changes

1. **Update Cell struct** to track multiple entities:
```go
type Cell struct {
    Entities  []Entity  // All entities at this position (small slice, typically 1-3)
    TopEntity Entity    // Cached: highest z-index entity
    TopZIndex int       // Cached: z-index of TopEntity
}
```

2. **Update Add method** to:
    - Append entity to Entities slice (avoid duplicates)
    - Recalculate TopEntity/TopZIndex using GetZIndex
    - Handle the case where entity already exists (position update)

3. **Update Remove method** to:
    - Remove entity from Entities slice
    - Recalculate TopEntity/TopZIndex if removed entity was TopEntity
    - Handle empty cell (reset TopEntity to 0, TopZIndex to -1)

4. **Update GetEntityAt** to return `TopEntity` (maintains backward compatibility)

5. **Add new method GetAllEntitiesAt**:
```go
// GetAllEntitiesAt returns all entities at the given position
// Returns nil if position is out of bounds or empty
// The returned slice should not be modified by caller
func (ps *PositionStore) GetAllEntitiesAt(x, y int) []Entity
```

6. **Add new method GetTopEntityFiltered**:
```go
// GetTopEntityFiltered returns the highest z-index entity at position that passes filter
// Returns 0 if no matching entity found
func (ps *PositionStore) GetTopEntityFiltered(x, y int, world *World, filter func(Entity) bool) Entity
```

7. **Update HasAny** to check `len(Entities) > 0` instead of `Count > 0`

8. **Update Clear** to reset all cell fields properly

### Implementation Notes

- The `world *World` parameter is needed for `GetZIndex` calls. Add it to PositionStore struct:
```go
type PositionStore struct {
    // ... existing fields
    world *World  // Reference for z-index lookups
}
```

- Update `NewPositionStore()` signature to accept `*World` parameter
- Small entity slices (1-3 entities typical) - preallocate capacity of 4

- When adding entity that already exists in cell, just update z-index cache if needed

- The z-index recalculation helper:
```go
func (ps *PositionStore) recalculateTop(cell *Cell) {
    if len(cell.Entities) == 0 {
        cell.TopEntity = 0
        cell.TopZIndex = -1
        return
    }
    cell.TopEntity = SelectTopEntity(cell.Entities, ps.world)
    cell.TopZIndex = GetZIndex(ps.world, cell.TopEntity)
}
```

### Files to modify
- `engine/position_store.go`

### Files that need signature updates (do NOT modify logic, only fix compilation)
- `engine/world.go` - Update `NewPositionStore()` call to pass world reference
    - Note: This creates a chicken-egg problem. Solution: Create PositionStore with nil, then set world after World is created:
```go
func NewWorld() *World {
    w := &World{
        // ...
        Positions: NewPositionStore(nil),  // Create with nil initially
        // ...
    }
    w.Positions.SetWorld(w)  // Set world reference after creation
    // ...
}
```
- Add `SetWorld(w *World)` method to PositionStore

### Verification
```bash
go build ./engine/...
go build ./...
```
```

---

#### Prompt 1.3: Update Consumers to Use New Z-Index API

```
## Task: Update Consumers to Use Z-Index Based Entity Selection

### Context
With the enhanced PositionStore and z-index API, we need to update code that queries entities at positions to use the new z-index aware methods.

### Changes Required

#### 1. systems/energy.go - handleCharacterTyping method

**Current code pattern (REMOVE):**
```go
// Find character at cursor position using Query
// We cannot use GetEntityAt because the Cursor entity masks the character in the spatial index
var entity engine.Entity
var char components.CharacterComponent
found := false

// Query all entities with Position and Character components
entities := world.Query().
    With(world.Positions).
    With(world.Characters).
    Execute()

// Loop to find entity at cursor position...
```

**Replace with:**
```go
// Find interactable entity at cursor position using z-index filtered lookup
entity := world.Positions.GetTopEntityFiltered(cursorX, cursorY, world, func(e engine.Entity) bool {
    return engine.IsInteractable(world, e)
})

if entity == 0 {
    return // No interactable entity at cursor
}

char, ok := world.Characters.Get(entity)
if !ok {
    return // Entity has no character component (shouldn't happen for interactable)
}
```

#### 2. render/renderers/cursor.go - Render method

The cursor renderer needs to determine what character to display under the cursor. Currently it has logic scattered through the render method.

**Find the section that determines `charAtCursor`** and update to use:
```go
// Get the interactable entity under cursor for display
// This excludes Cursor itself, Drain, Decay, Shield from the character lookup
entities := world.Positions.GetAllEntitiesAt(cursorPos.X, cursorPos.Y)
displayEntity := engine.SelectTopEntityFiltered(entities, world, func(e engine.Entity) bool {
    // Exclude cursor itself and non-character entities
    if e == c.gameCtx.CursorEntity {
        return false
    }
    // Only consider entities with characters
    return world.Characters.Has(e)
})

var charAtCursor rune = ' '
if displayEntity != 0 {
    if char, ok := world.Characters.Get(displayEntity); ok {
        charAtCursor = char.Rune
    }
}
```

#### 3. render/renderers/drain.go - Render method

The drain renderer currently reads background from buffer. This is correct behavior - drain should be transparent over whatever is below. **No changes needed**, but verify the existing logic is correct:
```go
// Get background from buffer (preserves what's rendered below)
_, bg, _ := buf.DecomposeAt(screenX, screenY)
```

### Files to modify
- `systems/energy.go`
- `render/renderers/cursor.go`

### DO NOT modify
- `render/renderers/drain.go` (verify only)
- Any other renderer files

### Verification
```bash
go build ./...
go test ./... 2>/dev/null || true  # Tests may not exist
```

Run the game and verify:
1. Typing on characters works (cursor over spawned text)
2. Cursor displays correct character when over entities
3. Drain renders correctly (transparent background)
```

---

### Task 2: Gold Bootstrap Decoupling

---

#### Prompt 2.1: Add PhaseBootstrap to GameState

```
## Task: Add PhaseBootstrap Phase to Game State Machine

### Context
We are decoupling the gold system initialization from `GoldSystem.Update()`. The bootstrap delay will be handled by the `ClockScheduler` using a new `PhaseBootstrap` phase.

### Requirements

#### 1. Update engine/clock_scheduler.go - Add PhaseBootstrap constant

Find the `GamePhase` type and constants:
```go
type GamePhase int

const (
    PhaseNormal GamePhase = iota
    PhaseGoldActive
    PhaseGoldComplete
    PhaseDecayWait
    PhaseDecayAnimation
)
```

**Change to:**
```go
type GamePhase int

const (
    PhaseBootstrap GamePhase = iota  // NEW: Initial state, waiting for game start delay
    PhaseNormal
    PhaseGoldActive
    PhaseGoldComplete
    PhaseDecayWait
    PhaseDecayAnimation
)
```

#### 2. Update the String() method for GamePhase

Add case for PhaseBootstrap:
```go
func (p GamePhase) String() string {
    switch p {
    case PhaseBootstrap:
        return "Bootstrap"
    case PhaseNormal:
        return "Normal"
    // ... rest unchanged
    }
}
```

#### 3. Update engine/game_state.go - Add GameStartTime field

Find the GameState struct and add:
```go
type GameState struct {
    // ... existing fields ...
    
    // Game Lifecycle state (clock-tick domain, mutex-protected)
    // ... existing lifecycle fields ...
    GameStartTime time.Time  // NEW: When game/round started (for bootstrap delay)
    
    // REMOVE these fields:
    // FirstUpdateTime time.Time      // DELETE
    // InitialSpawnComplete bool      // DELETE
}
```

#### 4. Update NewGameState function

In `NewGameState(maxEntities int, now time.Time)`:

**Add:**
```go
gs.GameStartTime = now
gs.CurrentPhase = PhaseBootstrap  // Start in Bootstrap phase
```

**Remove:**
```go
// DELETE: gs.FirstUpdateTime = time.Time{}
// DELETE: gs.InitialSpawnComplete = false
```

#### 5. Add accessor for GameStartTime

```go
// GetGameStartTime returns when the current game/round started
func (gs *GameState) GetGameStartTime() time.Time {
    gs.mu.RLock()
    defer gs.mu.RUnlock()
    return gs.GameStartTime
}

// ResetGameStart resets the game start time and returns to bootstrap phase
// Used by :new command for clean game reset
func (gs *GameState) ResetGameStart(now time.Time) {
    gs.mu.Lock()
    defer gs.mu.Unlock()
    gs.GameStartTime = now
    gs.CurrentPhase = PhaseBootstrap
    gs.PhaseStartTime = now
}
```

#### 6. Remove old accessor methods

**DELETE these methods entirely:**
- `GetFirstUpdateTime()`
- `SetFirstUpdateTime()`
- `GetInitialSpawnComplete()`
- `SetInitialSpawnComplete()`

#### 7. Update CanTransition validation

Find `CanTransition(from, to GamePhase)` and update the valid transitions:

```go
func (gs *GameState) CanTransition(from, to GamePhase) bool {
    switch from {
    case PhaseBootstrap:
        return to == PhaseNormal  // Bootstrap can only go to Normal
    case PhaseNormal:
        return to == PhaseGoldActive
    case PhaseGoldActive:
        return to == PhaseGoldComplete
    case PhaseGoldComplete:
        return to == PhaseDecayWait
    case PhaseDecayWait:
        return to == PhaseDecayAnimation
    case PhaseDecayAnimation:
        return to == PhaseNormal
    default:
        return false
    }
}
```

### Files to modify
- `engine/clock_scheduler.go` (GamePhase const and String())
- `engine/game_state.go` (GameState struct, NewGameState, accessors)

### Verification
```bash
go build ./engine/...
```

Note: This will cause compilation errors in files that reference the deleted methods. Those will be fixed in subsequent prompts.
```

---

#### Prompt 2.2: Update ClockScheduler for Bootstrap Phase Handling

```
## Task: Add Bootstrap Phase Logic to ClockScheduler

### Context
The `ClockScheduler.processTick()` method handles phase transitions. We need to add handling for the new `PhaseBootstrap` phase.

### Requirements

#### 1. Find processTick method in engine/clock_scheduler.go

Locate the switch statement that handles phases:
```go
switch phaseSnapshot.Phase {
case PhaseGoldActive:
    // ... existing code
case PhaseGoldComplete:
    // ... existing code
// ... etc
}
```

#### 2. Add PhaseBootstrap case at the BEGINNING of the switch

```go
switch phaseSnapshot.Phase {
case PhaseBootstrap:
    // Check if bootstrap delay has elapsed
    bootstrapDelay := constants.GoldInitialSpawnDelay
    if gameNow.Sub(cs.ctx.State.GetGameStartTime()) >= bootstrapDelay {
        // Transition to Normal phase - gold system will handle spawning
        cs.ctx.State.TransitionPhase(PhaseNormal, gameNow)
    }

case PhaseGoldActive:
    // ... existing code unchanged
```

#### 3. Verify TransitionPhase method exists

Check that `GameState` has a `TransitionPhase` method. If not, add it:

```go
// TransitionPhase attempts to transition to a new phase
// Returns true if transition was valid and completed
func (gs *GameState) TransitionPhase(to GamePhase, now time.Time) bool {
    gs.mu.Lock()
    defer gs.mu.Unlock()
    
    if !gs.CanTransition(gs.CurrentPhase, to) {
        return false
    }
    
    gs.CurrentPhase = to
    gs.PhaseStartTime = now
    return true
}
```

### Files to modify
- `engine/clock_scheduler.go`
- `engine/game_state.go` (if TransitionPhase doesn't exist)

### Verification
```bash
go build ./engine/...
```
```

---

#### Prompt 2.3: Strip Initialization Logic from GoldSystem

```
## Task: Remove Bootstrap Logic from GoldSystem

### Context
The `GoldSystem` currently manages its own startup delay using `FirstUpdateTime` and `InitialSpawnComplete`. This logic has been moved to the `ClockScheduler`. We need to remove it from `GoldSystem`.

### Requirements

#### 1. Update systems/gold.go - Update() method

**Find and REMOVE this entire block:**
```go
// Initialize FirstUpdateTime on first call (using GameState)
s.ctx.State.SetFirstUpdateTime(now)
firstUpdateTime := s.ctx.State.GetFirstUpdateTime()

// Read state snapshots from GameState for consistent reads
goldSnapshot := s.ctx.State.ReadGoldState(now)
phaseSnapshot := s.ctx.State.ReadPhaseState(now)
initialSpawnComplete := s.ctx.State.GetInitialSpawnComplete()

// Spawn gold sequence at game start with delay
if !goldSnapshot.Active && !initialSpawnComplete && now.Sub(firstUpdateTime) >= constants.GoldInitialSpawnDelay {
    // Spawn initial gold sequence after delay
    // If spawn fails, system will remain in PhaseNormal and can retry on next update
    if s.spawnGold(world) {
        // Mark initial spawn as complete (whether it succeeded or not)
        // TODO: decouple by refactoring into a proper bootstrap process of GameState instead of here
        s.ctx.State.SetInitialSpawnComplete()
    }
}

// Detect transition from decay animation to normal phase (decay just ended)
// Phase transitions: PhaseDecayAnimation -> PhaseNormal (handled by DecaySystem.StopDecayAnimation)
// When we detect PhaseNormal and no active gold, spawn new gold
if !goldSnapshot.Active && phaseSnapshot.Phase == engine.PhaseNormal && initialSpawnComplete {
    // Decay ended and returned to normal phase - spawn gold sequence
    // If spawn fails, system will remain in PhaseNormal and can retry on next update
    s.spawnGold(world)
}
```

**Replace with simplified logic:**
```go
// Fetch resources
timeRes := engine.MustGetResource[*engine.TimeResource](world.Resources)
now := timeRes.GameTime

// Read state snapshots
goldSnapshot := s.ctx.State.ReadGoldState(now)
phaseSnapshot := s.ctx.State.ReadPhaseState(now)

// Gold spawning: Only in PhaseNormal when no gold is active
// Bootstrap delay is handled by ClockScheduler (PhaseBootstrap -> PhaseNormal transition)
if phaseSnapshot.Phase == engine.PhaseNormal && !goldSnapshot.Active {
    s.spawnGold(world)
}
```

#### 2. Verify no other references to removed methods

Search the file for:
- `SetFirstUpdateTime` - should not exist
- `GetFirstUpdateTime` - should not exist
- `SetInitialSpawnComplete` - should not exist
- `GetInitialSpawnComplete` - should not exist
- `InitialSpawnComplete` - should not exist
- `FirstUpdateTime` - should not exist

Remove any remaining references.

### Files to modify
- `systems/gold.go`

### Verification
```bash
go build ./systems/...
go build ./...
```
```

---

#### Prompt 2.4: Update :new Command for Proper Reset

```
## Task: Update :new Command to Reset Game State Properly

### Context
The `:new` command in `modes/commands.go` resets the game. It needs to use the new `ResetGameStart()` method to properly reset to `PhaseBootstrap`.

### Requirements

#### 1. Update modes/commands.go - handleNewCommand function

**Find the current implementation:**
```go
func handleNewCommand(ctx *engine.GameContext) bool {
    // Reset energy and heat
    ctx.State.SetEnergy(0)
    ctx.State.SetHeat(0)

    // Reset runtime metrics (GT, APM)
    ctx.State.ResetRuntimeStats()
    
    // ... rest of the function
}
```

**Add game start reset AFTER ResetRuntimeStats:**
```go
func handleNewCommand(ctx *engine.GameContext) bool {
    // Reset energy and heat
    ctx.State.SetEnergy(0)
    ctx.State.SetHeat(0)

    // Reset runtime metrics (GT, APM)
    ctx.State.ResetRuntimeStats()

    // Reset game start time and return to bootstrap phase
    // This triggers the initial spawn delay sequence
    ctx.State.ResetGameStart(ctx.PausableClock.Now())

    // Despawn drain entities before clearing world
    // ... rest unchanged
}
```

#### 2. Verify gold state is also reset

The existing code clears the world, which destroys gold entities. Verify that the gold state in GameState is also reset. If `DeactivateGoldSequence` or similar is not called, add:

```go
// Reset gold sequence state (in case one was active)
// The world clear will destroy entities, but GameState needs explicit reset
goldSnapshot := ctx.State.ReadGoldState(ctx.PausableClock.Now())
if goldSnapshot.Active {
    ctx.State.DeactivateGoldSequence(ctx.PausableClock.Now())
}
```

This should be added BEFORE `clearAllEntities(ctx.World)`.

### Files to modify
- `modes/commands.go`

### Verification
```bash
go build ./modes/...
go build ./...
```

Run the game and verify:
1. Start game, wait for gold to spawn
2. Type `:new` and press Enter
3. Verify ~2 second delay before gold spawns again
4. Verify energy/heat reset to 0
```

---

#### Prompt 2.5: Final Verification and Cleanup

```
## Task: Final Verification and Dead Code Cleanup

### Context
After all changes, verify the build succeeds and clean up any remaining dead code.

### Requirements

#### 1. Full build verification
```bash
go build ./...
```

Fix any compilation errors.

#### 2. Search for dead code references

Search entire codebase for these terms and remove any remaining references:
- `FirstUpdateTime`
- `InitialSpawnComplete`
- `SetFirstUpdateTime`
- `GetFirstUpdateTime`
- `SetInitialSpawnComplete`
- `GetInitialSpawnComplete`

#### 3. Verify phase transitions work correctly

Run the game and verify:
1. Game starts in Bootstrap phase (no gold for ~2 seconds)
2. After delay, gold sequence spawns
3. Complete gold sequence → decay phase → normal → gold spawns again
4. `:new` command resets to bootstrap phase with delay

#### 4. Verify debug command shows correct phase

Run `:debug` command and verify:
- Phase is shown correctly (Bootstrap → Normal → GoldActive → etc.)

If debug command doesn't show phase, this is expected (enhancement for later).

### Files that should have been modified (verify)
- `engine/z-index.go`
- `engine/position_store.go`
- `engine/world.go`
- `engine/game_state.go`
- `engine/clock_scheduler.go`
- `systems/energy.go`
- `systems/gold.go`
- `render/renderers/cursor.go`
- `modes/commands.go`

### Verification
```bash
go build ./...
go vet ./...
```
```

---

## Summary

| Task | Prompts | Scope |
|------|---------|-------|
| Z-Index Integration | 1.1, 1.2, 1.3 | Engine core + consumers |
| Gold Bootstrap | 2.1, 2.2, 2.3, 2.4, 2.5 | GameState + Scheduler + GoldSystem |

Execute in order: 1.1 → 1.2 → 1.3 → 2.1 → 2.2 → 2.3 → 2.4 → 2.5

Each prompt is self-contained and builds on previous changes. Compilation should succeed after each prompt (except 2.1 which intentionally breaks until 2.3 completes).