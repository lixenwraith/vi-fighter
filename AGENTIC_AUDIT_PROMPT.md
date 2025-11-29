## Task: Comprehensive Post-Implementation Audit

### Context
Two architectural changes have been implemented:
1. **Z-Index Engine Integration**: Multi-entity cell support in PositionStore with z-index based selection
2. **Gold Bootstrap Decoupling**: PhaseBootstrap phase handling moved to ClockScheduler

This audit verifies correctness, completeness, and absence of regressions.

### Audit Scope

#### Part A: Build & Static Analysis
```bash
# 1. Clean build
go build ./...

# 2. Vet for common issues
go vet ./...

# 3. Check for unused code (if staticcheck available)
staticcheck ./... 2>/dev/null || echo "staticcheck not installed, skipping"
```

All commands must pass without errors or warnings.

---

#### Part B: Dead Code Verification

Search the entire codebase for these terms. **None should exist**:

**Removed GameState fields/methods:**
- `FirstUpdateTime`
- `InitialSpawnComplete`
- `SetFirstUpdateTime`
- `GetFirstUpdateTime`
- `SetInitialSpawnComplete`
- `GetInitialSpawnComplete`

**Old patterns that should be replaced:**
- `// We cannot use GetEntityAt because` (comment indicating old workaround)
- Manual entity loops in `handleCharacterTyping` that query all positions
```bash
# Search commands
grep -rn "FirstUpdateTime" --include="*.go" .
grep -rn "InitialSpawnComplete" --include="*.go" .
grep -rn "SetFirstUpdateTime\|GetFirstUpdateTime" --include="*.go" .
grep -rn "SetInitialSpawnComplete\|GetInitialSpawnComplete" --include="*.go" .
grep -rn "cannot use GetEntityAt" --include="*.go" .
```

**Expected result**: No matches (or only in comments explaining the old approach was removed)

---

#### Part C: Z-Index Implementation Verification

**C1. Verify z-index.go exports:**
```go
// engine/z-index.go must contain:

// Constants (exact values)
const (
    ZIndexBackground = 0
    ZIndexSpawnChar  = 100
    ZIndexNugget     = 200
    ZIndexDecay      = 300
    ZIndexDrain      = 400
    ZIndexShield     = 500
    ZIndexCursor     = 1000
)

// Functions (signatures)
func GetZIndex(world *World, e Entity) int
func SelectTopEntity(entities []Entity, world *World) Entity
func SelectTopEntityFiltered(entities []Entity, world *World, filter func(Entity) bool) Entity
func IsInteractable(world *World, e Entity) bool
```

**C2. Verify PositionStore changes:**
```go
// engine/position_store.go must contain:

// Updated Cell struct
type Cell struct {
    Entities  []Entity
    TopEntity Entity
    TopZIndex int
}

// New/updated methods
func (ps *PositionStore) GetAllEntitiesAt(x, y int) []Entity
func (ps *PositionStore) GetTopEntityFiltered(x, y int, world *World, filter func(Entity) bool) Entity
func (ps *PositionStore) SetWorld(w *World)

// Internal field
type PositionStore struct {
    // ... existing fields
    world *World
}
```

**C3. Verify World wiring:**
```go
// engine/world.go NewWorld() must:
// 1. Create PositionStore (possibly with nil)
// 2. Call SetWorld() to establish reference
```

**C4. Verify consumer updates:**

File: `systems/energy.go` - `handleCharacterTyping` method
- Must use `world.Positions.GetTopEntityFiltered()` or `engine.IsInteractable()`
- Must NOT have manual Query loop to find entity at cursor position

File: `render/renderers/cursor.go` - `Render` method
- Must use `world.Positions.GetAllEntitiesAt()` or z-index selection
- Must NOT have hardcoded entity type priority checks

---

#### Part D: Bootstrap Decoupling Verification

**D1. Verify GamePhase enum:**
```go
// engine/clock_scheduler.go must have PhaseBootstrap as FIRST constant:
const (
    PhaseBootstrap GamePhase = iota  // Must be 0
    PhaseNormal                       // Must be 1
    PhaseGoldActive
    PhaseGoldComplete
    PhaseDecayWait
    PhaseDecayAnimation
)
```

**D2. Verify GameState changes:**
```go
// engine/game_state.go must:

// HAVE these fields:
GameStartTime time.Time
CurrentPhase  GamePhase  // Should initialize to PhaseBootstrap

// HAVE these methods:
func (gs *GameState) GetGameStartTime() time.Time
func (gs *GameState) ResetGameStart(now time.Time)
func (gs *GameState) TransitionPhase(to GamePhase, now time.Time) bool

// NOT HAVE these fields:
// FirstUpdateTime (DELETED)
// InitialSpawnComplete (DELETED)
```

**D3. Verify NewGameState initialization:**
```go
// NewGameState must set:
gs.GameStartTime = now
gs.CurrentPhase = PhaseBootstrap  // NOT PhaseNormal
```

**D4. Verify ClockScheduler.processTick:**
```go
// Must have PhaseBootstrap case:
case PhaseBootstrap:
    // Check delay elapsed
    // Transition to PhaseNormal
```

**D5. Verify GoldSystem.Update simplified:**
```go
// systems/gold.go Update() must:
// - NOT reference FirstUpdateTime
// - NOT reference InitialSpawnComplete
// - Only spawn when Phase == PhaseNormal && !goldSnapshot.Active
```

**D6. Verify CanTransition includes PhaseBootstrap:**
```go
case PhaseBootstrap:
    return to == PhaseNormal
```

**D7. Verify :new command:**
```go
// modes/commands.go handleNewCommand must call:
ctx.State.ResetGameStart(ctx.PausableClock.Now())
```

---

#### Part E: Functional Verification Checklist

Run the game and verify each scenario:

**E1. Startup Sequence:**
- [ ] Game starts, no gold sequence visible for ~2 seconds
- [ ] After delay, gold sequence spawns
- [ ] No errors/panics in terminal

**E2. Normal Gameplay:**
- [ ] Cursor moves correctly (h/j/k/l)
- [ ] Typing on characters works (cursor over spawned text)
- [ ] Character under cursor displays correctly
- [ ] Energy increases when typing correct characters
- [ ] Heat meter updates

**E3. Entity Overlap Scenarios:**
- [ ] Cursor over character: character rune visible in cursor
- [ ] Cursor over empty space: space/block visible
- [ ] When Drain spawns: Drain visible, typing still works on characters
- [ ] Shield effect (if boost active): renders correctly

**E4. Gold Sequence:**
- [ ] Gold spawns after bootstrap delay
- [ ] Typing gold sequence works
- [ ] Gold completion triggers decay phase
- [ ] After decay, new gold spawns

**E5. Reset Command:**
- [ ] Type `:new` and press Enter
- [ ] Screen clears
- [ ] ~2 second delay before gold spawns (bootstrap phase)
- [ ] Energy/Heat reset to 0
- [ ] Game fully playable after reset

**E6. Debug Command:**
- [ ] Type `:debug` or `:d`
- [ ] Overlay appears
- [ ] No crashes
- [ ] ESC closes overlay

**E7. Edge Cases:**
- [ ] Rapid typing doesn't cause crashes
- [ ] Window resize works correctly
- [ ] Pause (command mode) and resume works
- [ ] Multiple `:new` commands in succession work

---

#### Part F: Code Quality Checks

**F1. No TODO comments referencing completed work:**
```bash
grep -rn "TODO.*decouple\|TODO.*bootstrap\|TODO.*z-index\|TODO.*integrate" --include="*.go" .
```
Remove any TODOs that reference the now-completed work.

**F2. Comments are accurate:**
- Review comments in modified files
- Remove outdated comments explaining old behavior
- Add comments explaining new behavior where non-obvious

**F3. Consistent error handling:**
- All new methods handle edge cases (nil, empty slices, out of bounds)
- No panics in normal operation paths

**F4. No debug prints left:**
```bash
grep -rn "fmt.Print\|log.Print" --include="*.go" . | grep -v "_test.go"
```
Review any matches - should only be intentional logging.

---

#### Part G: Audit Report Template

After completing all checks, provide a report:
```
## Audit Report: Z-Index & Bootstrap Implementation

### Build Status
- [ ] go build ./... : PASS/FAIL
- [ ] go vet ./... : PASS/FAIL

### Dead Code Removal
- [ ] All FirstUpdateTime references removed: YES/NO
- [ ] All InitialSpawnComplete references removed: YES/NO

### Z-Index Implementation
- [ ] z-index.go complete: YES/NO
- [ ] PositionStore multi-entity support: YES/NO
- [ ] World wiring correct: YES/NO
- [ ] EnergySystem updated: YES/NO
- [ ] CursorRenderer updated: YES/NO

### Bootstrap Implementation
- [ ] PhaseBootstrap added: YES/NO
- [ ] GameState fields updated: YES/NO
- [ ] ClockScheduler handles PhaseBootstrap: YES/NO
- [ ] GoldSystem simplified: YES/NO
- [ ] :new command updated: YES/NO

### Functional Tests
- [ ] Startup sequence: PASS/FAIL
- [ ] Normal gameplay: PASS/FAIL
- [ ] Entity overlaps: PASS/FAIL
- [ ] Gold sequence: PASS/FAIL
- [ ] Reset command: PASS/FAIL

### Issues Found
1. [List any issues discovered]

### Recommendations
1. [List any suggested improvements]
```

---

### Files Expected to be Modified

Verify these files were touched:

| File | Z-Index | Bootstrap |
|------|---------|-----------|
| `engine/z-index.go` | ✓ | |
| `engine/position_store.go` | ✓ | |
| `engine/world.go` | ✓ | |
| `engine/game_state.go` | | ✓ |
| `engine/clock_scheduler.go` | | ✓ |
| `systems/energy.go` | ✓ | |
| `systems/gold.go` | | ✓ |
| `render/renderers/cursor.go` | ✓ | |
| `modes/commands.go` | | ✓ |

### Success Criteria

Implementation is complete when:
1. All build commands pass
2. No dead code references remain
3. All functional tests pass
4. Audit report shows all items checked YES/PASS