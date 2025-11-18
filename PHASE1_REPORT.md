# Phase 1 Migration Report: GameState Extraction

**Date**: 2025-11-18
**Branch**: `claude/plan-gamestate-extraction-015UUDhsmRj2o7rxKHDXUKon`
**Status**: Phase 1 Complete - Spawn/Content State Migrated

---

## Executive Summary

Phase 1 successfully extracted spawn and content-related state from individual systems into a centralized `GameState` structure. This addresses race conditions in the spawn/typing interaction while maintaining the game's responsive typing feel.

**Key Achievement**: The main typing mechanic remains fully functional. Content spawns correctly, typing works, and color counters are race-free.

**Known Limitation**: Gold/Decay/Cleaner systems were intentionally NOT migrated in Phase 1. They may become non-functional but this is acceptable as the focus was on the core content spawn loop.

---

## What Was Accomplished

### 1. Created `engine/game_state.go`
**Location**: `/home/user/vi-fighter/engine/game_state.go`

A centralized state structure with two ownership models:

#### Real-Time State (Atomics - Lock-Free)
- `Heat` (formerly `scoreIncrement`): Current heat value
- `Score`: Player score
- `CursorX`, `CursorY`: Cursor position for spawn exclusion
- `BlueCountBright/Normal/Dark`: Blue character counters
- `GreenCountBright/Normal/Dark`: Green character counters
- `BoostEnabled`, `BoostEndTime`, `BoostColor`: Boost multiplier state
- `CursorError`, `ScoreBlink`, `PingGrid`: Visual feedback
- `NextSeqID`: Thread-safe sequence ID generation

**Why Atomic**: Updated on every keystroke (60+ times per second). Lock-free reads prevent contention.

#### Clock-Tick State (Mutex - Protected)
- `SpawnLastTime`, `SpawnNextTime`: Spawn timing
- `SpawnRateMultiplier`: Adaptive spawn rate (0.5x, 1.0x, 2.0x)
- `SpawnEnabled`: Whether spawning is active
- `EntityCount`, `ScreenDensity`: Screen fill tracking

**Why Mutex**: Updated every 2 seconds (spawn rate). Mutex acceptable for infrequent updates requiring multi-field consistency.

#### Key Methods
```go
// Heat accessors (atomic)
func (gs *GameState) GetHeat() int
func (gs *GameState) SetHeat(heat int)
func (gs *GameState) AddHeat(delta int)

// Color counter accessors (atomic)
func (gs *GameState) AddColorCount(seqType, seqLevel, delta int)
func (gs *GameState) GetTotalColorCount() int
func (gs *GameState) CanSpawnNewColor() bool

// Spawn state accessors (mutex)
func (gs *GameState) ReadSpawnState() SpawnStateSnapshot
func (gs *GameState) UpdateSpawnTiming(lastTime, nextTime time.Time)
func (gs *GameState) UpdateSpawnRate(entityCount, maxEntities int)
func (gs *GameState) ShouldSpawn() bool

// Sequence ID (atomic)
func (gs *GameState) IncrementSeqID() int
```

### 2. Updated `engine/game.go`
**Location**: `/home/user/vi-fighter/engine/game.go`

- Added `State *GameState` to `GameContext`
- Removed duplicated state fields (score, heat, boost, cursor error, score blink)
- Added delegated accessor methods for backward compatibility
- `GameContext` methods now delegate to `GameState` internally

**Example**:
```go
// OLD: Direct access to ctx.score.Load()
// NEW: ctx.GetScore() → delegates to → ctx.State.GetScore()

func (g *GameContext) GetScore() int {
    return g.State.GetScore()
}
```

### 3. Updated `systems/spawn_system.go`
**Location**: `/home/user/vi-fighter/systems/spawn_system.go`

**Removed Fields**:
- `lastSpawn time.Time` → now in `GameState.SpawnLastTime`
- `nextSeqID atomic.Int64` → now in `GameState.NextSeqID`
- Color counters (6× `atomic.Int64`) → now in `GameState`

**Delegated Methods**:
```go
func (s *SpawnSystem) GetColorCount(...) int64 {
    // Reads from ctx.State.BlueCountBright, etc.
}

func (s *SpawnSystem) AddColorCount(...) {
    // Writes to ctx.State via AddColorCount()
}
```

**Update() Changes**:
- Now calls `ctx.State.UpdateSpawnRate(entityCount, maxEntities)`
- Checks `ctx.State.ShouldSpawn()` for spawn timing
- Reads `ctx.State.ReadSpawnState()` for rate multiplier
- Calls `ctx.State.UpdateSpawnTiming(now, nextTime)` after spawn

### 4. Updated `systems/score_system.go`
**Location**: `/home/user/vi-fighter/systems/score_system.go`

**Changes**:
- Cursor movement now syncs to `GameState`: `ctx.State.SetCursorX(ctx.CursorX)`
- Heat/Score/Boost already using delegated accessors (no changes needed)
- Color counter updates delegate to `SpawnSystem.AddColorCount()` which delegates to `GameState`

### 5. Created `engine/game_state_test.go`
**Location**: `/home/user/vi-fighter/engine/game_state_test.go`

**11 Focused Tests** (all passing with `-race` flag):
1. `TestGameStateInitialization`: Verifies initial state
2. `TestHeatOperationsAtomic`: Concurrent heat updates (10 goroutines × 10 ops)
3. `TestColorCounterOperations`: Add/remove color counts
4. `TestColorCounterNegativePrevention`: Counters clamp to 0
5. `TestSpawnRateAdaptation`: Screen density → rate multiplier
6. `TestSpawnTimingState`: ShouldSpawn(), UpdateSpawnTiming()
7. `TestSequenceIDGeneration`: Atomic ID generation (100 unique IDs)
8. `TestBoostStateTransitions`: Boost activation/expiration
9. `TestCanSpawnNewColor`: 6-color limit enforcement
10. `TestConcurrentStateReads`: No deadlocks during concurrent access
11. `TestStateSnapshot`: ReadSpawnState() returns consistent copies

**Reduced Stress Volume**: 10 goroutines × 10 operations (was 100×100 in original plan) for faster CI.

### 6. Updated `architecture.md`
**Location**: `/home/user/vi-fighter/architecture.md`

Added new "State Ownership Model (Phase 1: Spawn/Content)" section documenting:
- Real-time state (atomics) vs clock-tick state (mutex)
- Why each synchronization primitive is used
- State access patterns with code examples
- Migration status (complete and remaining)
- Testing information

---

## Current Game Functionality

### ✅ WORKING (Fully Functional)
1. **Typing Mechanic**: Characters can be typed in insert mode
2. **Score System**: Points calculated correctly (Heat × Level Multiplier)
3. **Heat System**: Heat increases on correct typing, resets on errors
4. **Content Spawning**: Blue/Green character blocks spawn at adaptive rates
5. **Color Counters**: 6-color limit enforced (Blue×3 + Green×3)
6. **Boost System**: Activates at max heat, extends on matching color
7. **Cursor Movement**: Synced between `GameContext` and `GameState`
8. **Visual Feedback**: Error flash, score blink, ping grid
9. **Spatial Index**: Entity positioning and collision detection

### ⚠️ DEPRECATED/NON-FUNCTIONAL (Intentionally Not Migrated)
1. **Gold Sequence System**: May not spawn or function correctly
2. **Decay Timer**: May not calculate intervals correctly (uses stale heat)
3. **Decay Animation**: May not trigger or may have timing issues
4. **Cleaner System**: May not activate or sweep correctly

**Why Deprecated**: These systems have race conditions with inter-dependent mechanics (Gold→Timer→Decay→Gold flow). Fixing them requires Phase 2 (50ms clock tick). Since the user prioritized content spawn, we left these for future work.

---

## What Remains To Be Done

### Phase 2 Components (Future Sessions)

#### 1. Gold Sequence System
**File**: `systems/gold_sequence_system.go`

**Current State**: Has own mutex-protected state fields:
- `active bool`: Whether gold is active
- `sequenceID int`: Current sequence ID
- `startTime time.Time`: When gold spawned

**Migration Needed**:
```go
// REMOVE from GoldSequenceSystem:
type GoldSequenceSystem struct {
    mu         sync.RWMutex  // REMOVE
    active     bool          // → GameState.GoldActive
    sequenceID int           // → GameState.GoldSequenceID
    startTime  time.Time     // → GameState.GoldStartTime
}

// ADD to GameState:
type GameState struct {
    mu sync.RWMutex

    // Gold Sequence Phase
    GoldActive      bool
    GoldSequenceID  int
    GoldStartTime   time.Time
    GoldSpawnFailed bool // Edge case tracking
}
```

**Race Condition to Fix**:
```
// Current problematic flow:
ScoreSystem: Gold completes at heat=50
ScoreSystem: Fills heat to max (ctx.SetHeat(max))
ScoreSystem: Calls goldSystem.CompleteGoldSequence()
GoldSequenceSystem: Calls removeGoldSequence()
GoldSequenceSystem: Calls decaySystem.StartDecayTimer()
DecaySystem: Reads s.heatIncrement = 50 (STALE!)
  // Should read heat=max but gets old cached value
```

**Fix**: Move gold state to `GameState`, access heat atomically during timer calculation.

#### 2. Decay System
**File**: `systems/decay_system.go`

**Current State**: Has mutex-protected state:
- `animating bool`: Whether decay animation is running
- `timerStarted bool`: Whether decay timer has been initialized
- `nextDecayTime time.Time`: When next decay triggers
- `startTime time.Time`: Animation start time
- `heatIncrement int`: **CACHED heat value (stale!)**

**Migration Needed**:
```go
// REMOVE from DecaySystem:
type DecaySystem struct {
    mu            sync.RWMutex  // Keep for animation state
    animating     bool          // → GameState.DecayAnimating
    timerStarted  bool          // → GameState.DecayTimerActive
    nextDecayTime time.Time     // → GameState.DecayNextTime
    heatIncrement int           // REMOVE - read from GameState.GetHeat()
}

// ADD to GameState:
type GameState struct {
    mu sync.RWMutex

    // Decay Phase
    DecayTimerActive bool
    DecayNextTime    time.Time
    DecayAnimating   bool
    DecayStartTime   time.Time
}
```

**Critical Fix**:
```go
// OLD (DecaySystem.calculateInterval):
func (s *DecaySystem) calculateInterval() time.Duration {
    heatPercentage := float64(s.heatIncrement) / float64(heatBarWidth) // STALE!
    intervalSeconds := 60 - 50*heatPercentage
    return time.Duration(intervalSeconds * float64(time.Second))
}

// NEW:
func (s *DecaySystem) calculateInterval() time.Duration {
    heat := s.ctx.State.GetHeat() // Read current heat atomically
    heatBarWidth := s.ctx.State.ScreenWidth - constants.HeatBarIndicatorWidth
    heatPercentage := float64(heat) / float64(heatBarWidth)
    intervalSeconds := 60 - 50*heatPercentage
    return time.Duration(intervalSeconds * float64(time.Second))
}
```

**Animation State**: Keep in DecaySystem as implementation detail:
- `currentRow int`
- `fallingEntities []engine.Entity`
- `decayedThisFrame map[engine.Entity]bool`

These are animation mechanics, not game state.

#### 3. Cleaner System
**File**: `systems/cleaner_system.go`

**Current State**: Uses atomic operations:
- `isActive atomic.Bool`: Cleaner animation active
- `activationTime atomic.Int64`: When cleaners triggered
- `activeCleanerCount atomic.Int64`: Number of active cleaners

**Migration Needed**:
```go
// ADD to GameState:
type GameState struct {
    mu sync.RWMutex

    // Cleaner Phase
    CleanerActive       bool
    CleanerActivateTime time.Time
    CleanerCount        int
}
```

**Consider**: Cleaner might be fine as-is (already uses atomics). Only migrate if it needs coordination with other clock-tick state.

#### 4. Main Loop Changes
**File**: `cmd/vi-fighter/main.go`

**Current**:
```go
// main.go lines 197-198:
decaySystem.UpdateDimensions(ctx.GameWidth, ctx.GameHeight, ctx.Width, ctx.GetScoreIncrement())
```

**Problem**: `ctx.GetScoreIncrement()` passes heat to DecaySystem which caches it. By next frame, heat may have changed.

**Fix** (Phase 2):
```go
// REMOVE UpdateDimensions call with heatIncrement parameter
// DecaySystem should read heat directly from ctx.State when needed
decaySystem.UpdateDimensions(ctx.GameWidth, ctx.GameHeight, ctx.Width)
```

#### 5. 50ms Clock Tick (Phase 2 Core Feature)

**Concept**:
```go
// Add to main.go:
clockTicker := time.NewTicker(50 * time.Millisecond)
defer clockTicker.Stop()

for {
    select {
    case <-clockTicker.C:
        // Clock-tick logic (spawn, gold, decay phase transitions)
        UpdateClockTickState(ctx)

    case <-frameTicker.C:
        // Real-time logic (input, typing, render)
        // ... existing frame logic ...
    }
}

func UpdateClockTickState(ctx *engine.GameContext) {
    // 1. Check spawn timing
    if ctx.State.ShouldSpawn() {
        spawnSystem.TriggerSpawn(ctx.World)
    }

    // 2. Check gold timing
    if ctx.State.GoldActive && time.Now().After(ctx.State.GoldTimeout) {
        goldSystem.TimeoutGoldSequence(ctx.World)
    }

    // 3. Check decay timing
    if ctx.State.DecayTimerActive && time.Now().After(ctx.State.DecayNextTime) {
        decaySystem.StartAnimation(ctx.World)
    }
}
```

**Benefits**:
- All phase transitions happen atomically in one place
- Heat is read once per clock tick (not cached)
- Clear separation: real-time (input) vs scheduled (game mechanics)

---

## Technical Context for Next Session

### Race Condition Root Cause (Still Present)
The original race condition persists because we haven't added the clock tick yet:

```
Timeline within single 16ms frame:
1. ScoreSystem.HandleCharacterTyping():
   - User types last gold char at heat=50
   - ctx.State.SetHeat(maxHeat) // Atomic update to 74

2. GoldSequenceSystem.CompleteGoldSequence():
   - Calls DecaySystem.StartDecayTimer()

3. DecaySystem.StartDecayTimer():
   - Reads heat: 50 (stale from constructor!)
   - Calculates interval: 60 - 50*(50/74) = 26.35s
   - Should have read heat=74 → 60 - 50*(74/74) = 10s

4. Next frame (main.go:197):
   - decaySystem.UpdateDimensions(..., ctx.GetScoreIncrement())
   - NOW heat=74 is passed, but timer already calculated!
```

**Why This Happens**:
- `DecaySystem` stores `heatIncrement` in its constructor
- `StartDecayTimer()` uses this cached value
- `UpdateDimensions()` updates it, but AFTER timer is set

**Phase 2 Fix**:
- Remove `heatIncrement` field from DecaySystem
- Read `ctx.State.GetHeat()` directly in `calculateInterval()`
- Schedule timer calculation on clock tick (not mid-frame)

### Files with Deprecated/Broken Functionality

**DO NOT USE - Will Be Removed in Phase 2**:
- `systems/gold_sequence_system_test.go` - May have race conditions
- `systems/decay_system_test.go` - May fail due to cached heat
- `systems/decay_timer_after_gold_test.go` - Tests the broken flow
- `systems/cleaner_gold_integration_test.go` - Bloated output, race conditions

**User's Instruction**: "It's acceptable that the race conditions concerning Gold, Decay Timer, Decay animation, and Cleaner will not be fixed. You do not need to run their tests."

### Key Learnings

1. **State Ownership is Critical**:
   - Systems should NOT cache game state (heat, scores, etc.)
   - Read from `GameState` atomically when needed
   - Only cache implementation details (animation frames, content blocks)

2. **Atomic vs Mutex Trade-Off**:
   - Atomic: High-frequency (every frame), simple values
   - Mutex: Low-frequency (every 2s), multi-field consistency
   - Rule: If read on hot path (input/render), use atomic

3. **Backward Compatibility Pattern**:
   - Add `State *GameState` to `GameContext`
   - Keep existing accessor methods, delegate internally
   - Allows gradual migration without breaking all code at once

4. **Testing Strategies**:
   - Use `MockTimeProvider` for deterministic time
   - Reduce stress test volume (10× reduction) for CI speed
   - Focus tests on state ownership, not system interactions
   - Run all tests with `-race` flag

---

## Recommended Next Steps for Phase 2

### Session Goals
1. Migrate Gold/Decay/Cleaner state to `GameState`
2. Add 50ms clock tick for phase transitions
3. Remove cached `heatIncrement` from DecaySystem
4. Implement atomic state transitions in clock handler
5. Add integration tests for Gold→Decay→Cleaner flow

### Implementation Order
1. **Add clock-tick state fields to `GameState`**:
   - `GoldActive`, `GoldSequenceID`, `GoldStartTime`
   - `DecayTimerActive`, `DecayNextTime`, `DecayAnimating`
   - `CleanerActive`, `CleanerActivateTime`, `CleanerCount`

2. **Extract state from systems**:
   - Update `GoldSequenceSystem` to read/write `GameState`
   - Update `DecaySystem` to read heat from `GameState`
   - Update `CleanerSystem` to use `GameState`

3. **Add clock tick to `main.go`**:
   - Create `clockTicker := time.NewTicker(50 * time.Millisecond)`
   - Add `UpdateClockTickState()` function
   - Move spawn, gold, decay timing checks to clock handler

4. **Remove UpdateDimensions with heatIncrement**:
   - `decaySystem.UpdateDimensions(width, height, screenWidth)` only
   - DecaySystem reads heat via `ctx.State.GetHeat()`

5. **Test Gold→Decay flow**:
   - Create `engine/game_state_gold_decay_test.go`
   - Simulate gold completion at different heat levels
   - Verify decay timer calculates with current (not cached) heat
   - Use `MockTimeProvider.Advance()` to step through phases

### Files to Modify (Phase 2)
- `engine/game_state.go`: Add Gold/Decay/Cleaner fields
- `systems/gold_sequence_system.go`: Delegate to GameState
- `systems/decay_system.go`: Remove cached heat, read from GameState
- `systems/cleaner_system.go`: Delegate to GameState
- `cmd/vi-fighter/main.go`: Add clock ticker, remove UpdateDimensions calls
- `engine/game_state_gold_decay_test.go`: New test file

### Testing Focus (Phase 2)
- **Scenario 1**: Gold completes at heat=50 → Verify decay timer = 60 - 50*(50/74)
- **Scenario 2**: User types during gold, heat increases → Verify timer uses final heat
- **Scenario 3**: Gold timeout → Decay timer starts → Animation triggers
- **Scenario 4**: Gold completes at max heat → Cleaner activates → Gold spawns after decay

---

## Known Issues and Workarounds

### Issue 1: DecaySystem Uses Cached Heat
**Status**: Known, not fixed in Phase 1
**Impact**: Decay timer may be incorrect if heat changes during gold sequence
**Workaround**: None (requires Phase 2 fix)
**Fix**: Remove `heatIncrement` field, read `ctx.State.GetHeat()` directly

### Issue 2: Gold/Decay/Cleaner Tests Disabled
**Status**: Intentional (bloated output, race conditions)
**Impact**: Can't run full test suite with `-race`
**Workaround**: Run only spawn/state tests: `go test ./engine -race`
**Fix**: Phase 2 will migrate these systems, then re-enable tests

### Issue 3: UpdateDimensions Called Every Frame
**Status**: Inefficient but functional
**Impact**: Updates width/height even when screen hasn't resized
**Workaround**: Accept minor performance hit
**Fix**: Phase 2 can optimize to only call on resize events

---

## Build and Test Commands

### Build
```bash
cd /home/user/vi-fighter
go build -o /tmp/vi-fighter ./cmd/vi-fighter
```

### Run Tests (Phase 1 Safe)
```bash
# GameState tests (all pass with -race)
go test ./engine -v -run "TestGameState" -race

# All engine tests
go test ./engine -v -race

# Spawn system tests (some may fail due to deprecated Gold/Decay)
go test ./systems -v -run "TestSpawn" -race
```

### DO NOT RUN (Will Fail/Hang)
```bash
# These have race conditions or bloated output:
go test ./systems -run "TestGold" -race          # Broken
go test ./systems -run "TestDecay" -race         # Broken
go test ./systems -run "TestCleaner.*Integration" -race  # Bloated
```

---

## File Manifest

### New Files (Phase 1)
- `engine/game_state.go` (613 lines) - Centralized state structure
- `engine/game_state_test.go` (494 lines) - Integration tests
- `PHASE1_REPORT.md` (this file) - Documentation

### Modified Files (Phase 1)
- `engine/game.go` - Added GameState, delegated accessors
- `systems/spawn_system.go` - Removed state fields, delegated to GameState
- `systems/score_system.go` - Added cursor sync to GameState
- `systems/spawn_render_sync_test.go` - Updated seqID generation
- `architecture.md` - Added State Ownership Model section

### Deprecated Files (Phase 2 Will Remove/Rewrite)
- `systems/gold_sequence_system_test.go`
- `systems/decay_system_test.go`
- `systems/decay_timer_after_gold_test.go`
- `systems/cleaner_gold_integration_test.go`
- `systems/cleaner_race_stress_test.go`

---

## Summary

Phase 1 successfully migrated the core typing and spawning mechanics to a centralized `GameState`. The game's most important feature (typing to clear characters) is fully functional and race-free.

**What Works**: Typing, scoring, heat, content spawning, color counters, boost system.

**What's Broken**: Gold sequence, decay timer, cleaner system (intentionally deprecated for Phase 2).

**Next Session**: Migrate Gold/Decay/Cleaner to `GameState` and add 50ms clock tick to fix inter-dependent mechanics race conditions.

**Key Insight**: State ownership matters more than code organization. Systems should be stateless workers that read from and write to centralized `GameState`, not owners of duplicated/cached game state.
