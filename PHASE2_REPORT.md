# Phase 2 Migration Report: Clock Scheduler Infrastructure

**Date**: 2025-11-18
**Branch**: `claude/phase-2-clock-scheduler-01Vj6PMwW8CSaNNCkGm4uqAf`
**Status**: Phase 2 Complete - Clock Infrastructure Implemented

---

## Executive Summary

Phase 2 successfully implemented the clock scheduler infrastructure for hybrid real-time/state ownership architecture. This phase adds the foundation for phase-based game mechanics without changing existing game logic.

**Key Achievement**: Separation of real-time input handling (16ms frame ticker) from game logic phase transitions (50ms clock ticker). The game continues to function exactly as before, but now has infrastructure to support deterministic phase transitions in Phase 3.

**Important**: This is an infrastructure-only phase. Gold/Decay/Cleaner systems remain unchanged and may still have race conditions. Phase 3 will migrate their logic to the clock scheduler.

---

## What Was Accomplished

### 1. Created GamePhase State Machine (`engine/clock_scheduler.go`)

**Location**: `/home/user/vi-fighter/engine/clock_scheduler.go`

A phase enumeration to track the game's mechanic cycle:

```go
type GamePhase int

const (
    PhaseNormal         // Regular gameplay, spawning content
    PhaseGoldActive     // Gold sequence is active and can be typed
    PhaseDecayWait      // Waiting for decay timer to expire after gold
    PhaseDecayAnimation // Decay animation is running, characters degrading
)
```

**Why These Phases**:
- **PhaseNormal**: Default state - spawn content, no special mechanics active
- **PhaseGoldActive**: Gold sequence spawned (Phase 3: will track timeout)
- **PhaseDecayWait**: Timer counting down (Phase 3: will track heat-based interval)
- **PhaseDecayAnimation**: Falling entities decaying characters (Phase 3: will track progress)

**String() Method**: Each phase has human-readable name for debugging:
```go
phase.String() // Returns "Normal", "GoldActive", "DecayWait", "DecayAnimation"
```

### 2. Added Phase State to GameState (`engine/game_state.go`)

**Clock-Tick State Fields** (mutex protected):
```go
// Phase State (Phase 2: Infrastructure)
CurrentPhase   GamePhase  // Current game phase
PhaseStartTime time.Time  // When current phase started
```

**Accessor Methods**:
```go
// Read current phase (thread-safe with RLock)
func (gs *GameState) GetPhase() GamePhase

// Transition to new phase (resets start time atomically)
func (gs *GameState) SetPhase(phase GamePhase)

// Get when current phase started
func (gs *GameState) GetPhaseStartTime() time.Time

// Calculate how long phase has been active
func (gs *GameState) GetPhaseDuration() time.Duration

// Snapshot all phase state consistently
func (gs *GameState) ReadPhaseState() PhaseSnapshot
```

**Initialization**:
- Game starts in `PhaseNormal`
- `PhaseStartTime` set to initial time from `TimeProvider`

### 3. Implemented ClockScheduler (`engine/clock_scheduler.go`)

**Core Infrastructure**:
```go
type ClockScheduler struct {
    ctx          *GameContext
    timeProvider TimeProvider
    ticker       *time.Ticker    // 50ms ticker
    stopChan     chan struct{}   // Graceful shutdown
    tickCount    uint64          // Debug/metrics counter
}
```

**Key Features**:

**50ms Clock Tick**:
- Fixed 50ms interval (20 ticks per second)
- Runs in dedicated goroutine (non-blocking)
- Independent of frame rate (16ms frame ticker)

**Thread-Safe Start/Stop**:
```go
scheduler.Start()  // Begins ticking in goroutine
scheduler.Stop()   // Graceful shutdown (idempotent)
```

**Tick Counter**:
- Atomically increments on each tick
- Used for debugging and metrics
- Accessible via `GetTickCount()`

**Phase 2 Behavior**:
```go
func (cs *ClockScheduler) tick() {
    cs.mu.Lock()
    cs.tickCount++
    cs.mu.Unlock()

    // Phase 2: Just tick infrastructure
    // Phase 3 will add:
    // - Check spawn timing via ctx.State.ShouldSpawn()
    // - Check gold timeout (if PhaseGoldActive)
    // - Check decay timer (if PhaseDecayWait)
    // - Update animation state (if PhaseDecayAnimation)
}
```

### 4. Integrated Clock into Main Game Loop (`cmd/vi-fighter/main.go`)

**Changes**:
```go
// Create and start clock scheduler (50ms tick for game logic)
clockScheduler := engine.NewClockScheduler(ctx)
clockScheduler.Start()
defer clockScheduler.Stop()

// Frame ticker continues as before (16ms for rendering)
ticker := time.NewTicker(16 * time.Millisecond) // ~60 FPS
defer ticker.Stop()
```

**Architecture**:
```
Main Goroutine (select loop):
├── Frame Ticker (16ms)
│   ├── Process input events
│   ├── Update ECS systems (spawn, score, decay, gold, cleaner)
│   ├── Update boost timer (atomic)
│   ├── Update ping grid timer (atomic)
│   └── Render frame
│
└── Clock Goroutine (50ms) - separate thread
    └── Tick counter increment (Phase 3: phase transitions)
```

**Benefits**:
- Frame rate remains ~60 FPS (responsive input)
- Clock logic runs 3× per frame (50ms vs 16ms)
- Separation allows Phase 3 to add blocking logic to clock without affecting input

### 5. Created Comprehensive Tests (`engine/clock_scheduler_test.go`)

**11 Test Functions** (all passing with `-race` flag):

**Phase State Tests**:
1. `TestGamePhaseString`: Verify String() method for all phases
2. `TestPhaseStateInitialization`: Initial phase is PhaseNormal
3. `TestPhaseTransitions`: State changes work correctly
4. `TestPhaseSnapshot`: ReadPhaseState() returns consistent snapshot
5. `TestConcurrentPhaseReads`: Thread-safe reads (20 goroutines × 100 ops)

**Clock Scheduler Tests**:
6. `TestClockSchedulerCreation`: Initialization and configuration
7. `TestClockSchedulerTicking`: Verifies 50ms tick rate (8-12 ticks in 550ms)
8. `TestClockSchedulerStopIdempotent`: Stop() can be called multiple times safely
9. `TestClockSchedulerConcurrentAccess`: Thread-safe tick counter reads
10. `TestPhaseAndClockIntegration`: Phase changes during clock ticking
11. `TestClockSchedulerMemoryLeak`: No goroutine leaks after repeated start/stop

**Race Detection**: All tests pass with `-race` flag (no data races).

**Test Helpers**:
- `MockScreen`: Minimal tcell.Screen implementation for testing
- `MockTimeProvider`: Deterministic time control (existing from Phase 1)

### 6. Updated Documentation

**architecture.md**:
- Added "Clock Scheduler (Phase 2: Infrastructure Complete)" section
- Documented GamePhase state machine
- Explained hybrid real-time/clock architecture
- Listed Phase 2 vs Phase 3 differences
- Added clock scheduler testing details

**PHASE2_REPORT.md** (this file):
- Complete record of Phase 2 implementation
- Migration guide for Phase 3
- Known issues and limitations

---

## Current Game Functionality

### ✅ WORKING (Unchanged from Phase 1)
1. **Typing Mechanic**: Characters typed in insert mode
2. **Score System**: Points calculated correctly
3. **Heat System**: Heat increases/resets appropriately
4. **Content Spawning**: Blue/Green blocks spawn at adaptive rates
5. **Color Counters**: 6-color limit enforced (race-free)
6. **Boost System**: Activates at max heat, extends on matching color
7. **Visual Feedback**: Error flash, score blink, ping grid
8. **Spatial Index**: Entity positioning and collision detection

### ⚠️ UNCHANGED (Still Have Race Conditions - Phase 3 Will Fix)
1. **Gold Sequence System**: Still using internal mutex, may have timing issues
2. **Decay Timer**: Still caches heat value (stale during gold completion)
3. **Decay Animation**: Still using internal state (not on clock tick)
4. **Cleaner System**: Still using atomic operations (works but not phase-coordinated)

**Why Unchanged**: Phase 2 is infrastructure-only. These systems will be migrated to use GamePhase and clock tick in Phase 3.

---

## Architecture Patterns

### Hybrid Real-Time/Clock Model

**Real-Time Layer** (16ms frame ticker):
- **Purpose**: Immediate user feedback
- **Systems**: Input handling, cursor movement, typing scoring
- **State Access**: Atomic reads/writes (lock-free)
- **Performance**: No blocking, instant responsiveness

**Game Logic Layer** (50ms clock ticker):
- **Purpose**: Phase transitions, spawn decisions
- **Systems**: Gold lifecycle, Decay timing, Cleaner triggers (Phase 3)
- **State Access**: Mutex-protected reads/writes (consistent snapshots)
- **Performance**: Blocking acceptable (not on hot path)

### State Ownership Boundaries

**Real-Time Owned** (atomic):
- Heat, Score, Cursor position
- Color counters
- Boost state
- Visual feedback flags

**Clock Owned** (mutex):
- Game phase (PhaseNormal, PhaseGoldActive, etc.)
- Spawn timing and rate
- Phase 3: Gold timeout, Decay interval, Animation state

**Separation Principle**:
- Real-time systems READ clock state, WRITE real-time state
- Clock systems READ all state, WRITE clock state
- Render reads both via snapshots (no blocking)

### Thread Model

```
Main Goroutine:
├── Input Event Goroutine → eventChan → Main Loop
├── Frame Ticker (16ms) → ECS Update + Render
└── Clock Scheduler Goroutine (50ms) → Phase Logic

Concurrent Systems:
└── Cleaner Update Goroutine (60 FPS) → Cleaner Animation
```

**Synchronization**:
- Main goroutine owns ECS world (no concurrent updates)
- Clock goroutine updates GameState (mutex protected)
- Cleaner goroutine uses atomic operations
- All threads read GameState safely (atomics or RLock)

---

## What Phase 3 Will Add

### 1. Gold Sequence State Migration

**Move to GameState**:
```go
// Phase 3: Add to GameState
type GameState struct {
    mu sync.RWMutex

    // Gold Phase State
    GoldActive      bool      // Whether gold is active
    GoldSequenceID  int       // Current sequence ID
    GoldStartTime   time.Time // When gold spawned
    GoldTimeoutTime time.Time // When gold will timeout (10s)
}
```

**Clock Tick Logic**:
```go
// In ClockScheduler.tick()
if ctx.State.GetPhase() == PhaseGoldActive {
    snapshot := ctx.State.ReadPhaseState()
    if time.Now().After(goldTimeoutTime) {
        // Gold timeout - transition to DecayWait
        goldSystem.TimeoutGoldSequence()
        ctx.State.SetPhase(PhaseDecayWait)
    }
}
```

### 2. Decay Timer State Migration

**Move to GameState**:
```go
// Phase 3: Add to GameState
type GameState struct {
    mu sync.RWMutex

    // Decay Phase State
    DecayTimerActive bool      // Timer started
    DecayNextTime    time.Time // When decay will trigger
    DecayHeatSnapshot int      // Heat at Gold completion (NOT cached!)
}
```

**Fix Race Condition**:
```go
// OLD (DecaySystem): Cached heat causes stale value
func (s *DecaySystem) StartDecayTimer() {
    heat := s.heatIncrement // STALE! Set in constructor
    interval := calculateInterval(heat)
    s.nextDecayTime = time.Now().Add(interval)
}

// NEW (Clock Tick): Heat snapshot taken atomically
func (cs *ClockScheduler) tick() {
    if ctx.State.GetPhase() == PhaseGoldActive {
        // Gold just completed, transition to DecayWait
        heat := ctx.State.GetHeat() // Read current heat atomically
        interval := calculateInterval(heat) // 60s - (50s * heatPercentage)
        ctx.State.SetDecayNextTime(time.Now().Add(interval))
        ctx.State.SetPhase(PhaseDecayWait)
    }

    if ctx.State.GetPhase() == PhaseDecayWait {
        if time.Now().After(ctx.State.GetDecayNextTime()) {
            // Timer expired, start decay animation
            decaySystem.StartAnimation()
            ctx.State.SetPhase(PhaseDecayAnimation)
        }
    }
}
```

### 3. Decay Animation State Migration

**Move to GameState**:
```go
// Phase 3: Add to GameState
type GameState struct {
    mu sync.RWMutex

    // Decay Animation State
    DecayAnimating   bool      // Animation in progress
    DecayStartTime   time.Time // When animation started
}
```

**Clock Tick Logic**:
```go
// In ClockScheduler.tick()
if ctx.State.GetPhase() == PhaseDecayAnimation {
    if decaySystem.IsAnimationComplete() {
        // Animation done, spawn gold and return to Normal
        goldSystem.SpawnGoldSequence()
        ctx.State.SetPhase(PhaseGoldActive)
    }
}
```

### 4. Phase Transition Flow (Phase 3)

```
PhaseNormal (content spawning)
    ↓
    [Decay animation completes]
    ↓
PhaseGoldActive (10s timeout)
    ↓
    [Gold typed or timeout]
    ↓
    [Heat snapshot: calculate decay interval]
    ↓
PhaseDecayWait (10-60s based on heat)
    ↓
    [Timer expires]
    ↓
PhaseDecayAnimation (1.6-4.8s falling entities)
    ↓
    [Animation completes]
    ↓
PhaseNormal (cycle restarts)
```

---

## Testing Strategy

### Phase 2 Tests (Completed)

**Infrastructure Tests**:
- Clock ticking at 50ms intervals
- Phase state transitions
- Thread-safe concurrent access
- Graceful start/stop
- No memory leaks

**Integration Tests**:
- Phase changes during clock ticking
- Concurrent phase reads/writes
- MockTimeProvider for deterministic testing

### Phase 3 Tests (Future)

**Phase Transition Tests**:
- Gold completion → DecayWait transition
- Heat snapshot accuracy during transition
- Decay timer calculation with various heat levels
- DecayWait → DecayAnimation trigger timing
- DecayAnimation → PhaseNormal → GoldActive cycle

**Race Condition Tests**:
- Verify no stale heat values in decay timer
- Concurrent Gold completion and heat updates
- Phase transition atomicity

**Scenario Tests**:
1. Gold completed at heat=50 → Verify decay timer = 60 - 50*(50/74) ≈ 26.3s
2. Gold completed at heat=74 (max) → Verify decay timer = 10s
3. User types during gold, heat increases → Verify timer uses final heat
4. Gold timeout (not typed) → Decay timer starts correctly
5. Full cycle: PhaseNormal → GoldActive → DecayWait → DecayAnimation → back to Normal

---

## Known Issues and Limitations

### Issue 1: Gold/Decay/Cleaner Still Use Old Architecture
**Status**: Expected - Phase 2 is infrastructure-only
**Impact**: Race conditions from Phase 1 still present
**Workaround**: None (requires Phase 3 migration)
**Fix**: Phase 3 will migrate these systems to use GamePhase and clock tick

### Issue 2: Clock Tick Does Nothing (Yet)
**Status**: Expected - Phase 2 is infrastructure-only
**Impact**: 50ms ticker running but only increments counter
**Workaround**: Not needed (working as designed)
**Fix**: Phase 3 will add phase transition logic to clock tick

### Issue 3: Phase State Not Used (Yet)
**Status**: Expected - Phase 2 is infrastructure-only
**Impact**: GamePhase always PhaseNormal
**Workaround**: Not needed (working as designed)
**Fix**: Phase 3 will add phase transitions (Gold spawns → PhaseGoldActive, etc.)

---

## Performance Impact

### Memory Overhead
- **GameState**: +16 bytes (GamePhase enum + PhaseStartTime)
- **ClockScheduler**: ~100 bytes (ticker, channels, counters)
- **Total**: <150 bytes (negligible)

### CPU Overhead
- **Clock Ticker**: 20 ticks/second in separate goroutine
- **Per Tick**: Lock counter mutex, increment, unlock (~50ns)
- **Total**: ~1 microsecond/second (0.0001% CPU)
- **Frame Ticker**: Unchanged, still ~60 FPS

### Goroutine Count
- **Added**: 1 goroutine (clock scheduler)
- **Existing**: 2 goroutines (input event, cleaner update)
- **Total**: 4 goroutines (acceptable)

### Mutex Contention
- **Before**: Minimal (spawn state only)
- **After**: +1 mutex (phase state, RLock for reads)
- **Impact**: Negligible (clock tick 3× slower than frame tick)

---

## File Manifest

### New Files (Phase 2)
- `engine/clock_scheduler.go` (150 lines) - Clock infrastructure
- `engine/clock_scheduler_test.go` (370 lines) - Comprehensive tests
- `PHASE2_REPORT.md` (this file) - Documentation

### Modified Files (Phase 2)
- `engine/game_state.go` - Added phase state fields and accessors (+70 lines)
- `cmd/vi-fighter/main.go` - Integrated clock scheduler (+4 lines)
- `architecture.md` - Added Phase 2 documentation section (+110 lines)

### Unchanged Files (Phase 2)
- All systems (`systems/*.go`) - No changes to game logic
- `components/*.go` - No component changes
- `modes/*.go` - No input handling changes
- `render/*.go` - No rendering changes

---

## Build and Test Commands

### Build
```bash
cd /home/user/vi-fighter
go build -o /tmp/vi-fighter ./cmd/vi-fighter
```

### Run All Engine Tests
```bash
# All engine tests with race detection
go test ./engine -v -race

# Phase 2 tests only
go test ./engine -v -run "TestGamePhase|TestPhase|TestClock" -race
```

### Run Specific Test Categories
```bash
# Phase state tests
go test ./engine -v -run "TestPhase" -race

# Clock scheduler tests
go test ./engine -v -run "TestClock" -race

# Concurrent access tests
go test ./engine -v -run "Concurrent" -race
```

### Verify No Regressions
```bash
# All tests (engine, modes, systems)
go test ./... -v -race

# Just engine (fast check)
go test ./engine -race
```

---

## Migration Checklist (Phase 2 Complete)

### ✅ Completed
- [x] Define GamePhase enum with String() method
- [x] Add CurrentPhase and PhaseStartTime to GameState
- [x] Implement phase accessor methods (Get/Set/Duration/Snapshot)
- [x] Create ClockScheduler struct with 50ms ticker
- [x] Implement scheduler Start()/Stop() with graceful shutdown
- [x] Integrate clock scheduler into main.go
- [x] Create comprehensive tests (11 test functions)
- [x] Verify all tests pass with `-race` flag
- [x] Update architecture.md documentation
- [x] Create PHASE2_REPORT.md
- [x] Build game successfully
- [x] Verify no regressions in existing functionality

### ⏳ Deferred to Phase 3
- [ ] Add Gold sequence state to GameState
- [ ] Add Decay timer state to GameState
- [ ] Add Decay animation state to GameState
- [ ] Implement phase transition logic in clock tick
- [ ] Migrate GoldSequenceSystem to use GameState
- [ ] Migrate DecaySystem to use GameState
- [ ] Remove cached heatIncrement from DecaySystem
- [ ] Fix race condition: Heat snapshot during Gold completion
- [ ] Add integration tests for Gold→Decay→Cleaner flow
- [ ] Update game.md if needed
- [ ] Remove UpdateDimensions(heatIncrement) calls

---

## Key Learnings

### 1. Infrastructure First, Logic Second
**Approach**: Phase 2 added clock infrastructure without changing game logic.
**Benefit**: Changes are testable and reviewable independently.
**Risk Mitigation**: If Phase 3 is delayed, game still works (no broken functionality).

### 2. Separate Goroutines for Separate Concerns
**Pattern**: Frame ticker (16ms) and clock ticker (50ms) run independently.
**Benefit**: Real-time input never blocks on game logic.
**Trade-off**: Requires thread-safe state access (atomics/mutex).

### 3. Mutex Is Fine for Low-Frequency State
**Pattern**: Phase state uses `sync.RWMutex`, not atomics.
**Justification**: Phase transitions are infrequent (~10s intervals).
**Benefit**: Can protect multi-field updates (phase + start time) atomically.

### 4. Test the Infrastructure, Not the Logic (Yet)
**Approach**: Phase 2 tests focus on tick counting, not phase transitions.
**Benefit**: Tests pass immediately (no complex game logic).
**Phase 3**: Will add tests for actual phase transition logic.

### 5. Documentation as Code Review
**Practice**: Write detailed reports (PHASE1_REPORT.md, PHASE2_REPORT.md).
**Benefit**: Future developers understand why decisions were made.
**Trade-off**: Takes time, but pays off during debugging and Phase 3.

---

## Recommended Next Steps for Phase 3

### Session Goals
1. Add Gold/Decay/Cleaner state fields to GameState
2. Implement phase transition logic in ClockScheduler.tick()
3. Migrate GoldSequenceSystem to read/write GameState
4. Migrate DecaySystem to read heat from GameState (no caching)
5. Add integration tests for full Gold→Decay→Cleaner cycle
6. Remove UpdateDimensions(heatIncrement) calls

### Implementation Order
1. **Add state fields** to GameState:
   ```go
   // Gold state
   GoldActive      bool
   GoldSequenceID  int
   GoldStartTime   time.Time
   GoldTimeoutTime time.Time

   // Decay state
   DecayTimerActive bool
   DecayNextTime    time.Time
   DecayAnimating   bool

   // Cleaner state (optional - already atomic)
   CleanerActive   bool
   CleanerStartTime time.Time
   ```

2. **Implement clock tick phase logic**:
   - Check gold timeout (PhaseGoldActive)
   - Check decay timer (PhaseDecayWait)
   - Check animation complete (PhaseDecayAnimation)
   - Trigger phase transitions

3. **Migrate system calls**:
   - GoldSequenceSystem: Use ctx.State for active state
   - DecaySystem: Read heat via ctx.State.GetHeat()
   - CleanerSystem: Read trigger state from GameState

4. **Add integration tests**:
   - Simulate full cycle with MockTimeProvider
   - Verify heat snapshot accuracy
   - Test concurrent typing during phase transitions

5. **Clean up deprecated code**:
   - Remove heatIncrement field from DecaySystem
   - Remove internal mutex from GoldSequenceSystem
   - Remove UpdateDimensions(heatIncrement) calls

### Testing Focus (Phase 3)
- **Scenario 1**: Gold completes at heat=50 → Verify decay timer ≈ 26.3s
- **Scenario 2**: Gold completes at heat=74 (max) → Verify decay timer = 10s
- **Scenario 3**: User types during gold, heat 50→60 → Verify timer uses 60
- **Scenario 4**: Gold timeout (10s) → Decay timer starts correctly
- **Scenario 5**: Full cycle completes → Gold spawns again

---

## Summary

Phase 2 successfully implemented the clock scheduler infrastructure for vi-fighter's hybrid real-time/state ownership architecture.

**What Works**: All existing gameplay (typing, scoring, spawning, boost, visual feedback) continues to function exactly as before.

**What's New**: 50ms clock ticker running in background, GamePhase state tracking, comprehensive tests for scheduler and phase state.

**What's Next**: Phase 3 will migrate Gold/Decay/Cleaner logic to use the clock tick and GamePhase state, fixing the race conditions identified in Phase 1.

**Key Insight**: Separating infrastructure (Phase 2) from logic migration (Phase 3) reduces risk and makes each phase independently testable and reviewable.

**Build Status**: ✅ Compiles successfully
**Test Status**: ✅ All tests pass with `-race` flag (no data races)
**Game Status**: ✅ Plays correctly (no regressions)
**Ready for Phase 3**: ✅ Yes
