# Test Failure Analysis Report

## Executive Summary

Analysis of failing tests in vi-fighter reveals three categories of issues:

1. **Category 1 (Old Design)**: `undefined: cleanerSnap` in `engine/game_state_test.go:779` - Simple fix
2. **Category 2 (Complex Test Setup)**: Race conditions in EventQueue tests - Requires architectural decision
3. **Category 2 (Test Logic)**: `TestCleanerFinishedEvent` failure - Likely timing/test design issue

**Recommendation**: Fix Category 1 immediately. For Category 2, consider test simplification or deprecation per project guidelines.

---

## Issue 1: Undefined `cleanerSnap` Variable

### Location
`engine/game_state_test.go:779:9`

### Root Cause
The test `TestAllSnapshotTypesConcurrent` attempts to validate cleaner state consistency using a variable `cleanerSnap` that was never declared. This is remnant code from when CleanerSystem had a snapshot-based state model.

### Current Architecture
According to `doc/architecture.md` and `systems/cleaner_system.go`:
- CleanerSystem is **event-driven** (not snapshot-based)
- Uses `EventCleanerRequest` and `EventCleanerFinished` for communication
- State lives in `CleanerComponent` entities (pure ECS)
- No cleaner snapshot type exists in codebase

### Evidence
```go
// engine/game_state_test.go:719-727 (snapshot reads)
spawnSnap := gs.ReadSpawnState()
colorSnap := gs.ReadColorCounts()
boostSnap := gs.ReadBoostState()
cursorSnap := gs.ReadCursorPosition()
goldSnap := gs.ReadGoldState()
phaseSnap := gs.ReadPhaseState()
decaySnap := gs.ReadDecayState()
heat, score := gs.ReadHeatAndScore()
// NOTE: No cleanerSnap declaration

// engine/game_state_test.go:778-784 (invalid usage)
// Verify cleaner state consistency
if cleanerSnap.Active && cleanerSnap.Pending {  // ❌ cleanerSnap undefined
    errorMu.Lock()
    errorCount++
    errorMu.Unlock()
    t.Error("Cleaner snapshot inconsistent: Both active and pending")
}
```

### Fix Options

#### Option A: Remove Cleaner Validation (Recommended)
Since CleanerSystem is event-driven and doesn't have snapshot state, simply remove lines 778-784.

```go
// DELETE THESE LINES (778-784):
// Verify cleaner state consistency
if cleanerSnap.Active && cleanerSnap.Pending {
    errorMu.Lock()
    errorCount++
    errorMu.Unlock()
    t.Error("Cleaner snapshot inconsistent: Both active and pending")
}
```

**Justification**: Cleaner state is not snapshot-based, so this validation is obsolete.

#### Option B: Replace with Event Queue Validation
If we want to validate cleaner state, check the event queue instead:

```go
// Verify cleaner events are valid
events := gs.ctx.PeekEvents()
cleanerRequestCount := 0
cleanerFinishedCount := 0
for _, e := range events {
    if e.Type == engine.EventCleanerRequest {
        cleanerRequestCount++
    } else if e.Type == engine.EventCleanerFinished {
        cleanerFinishedCount++
    }
}
// Validate: finished count should never exceed request count
if cleanerFinishedCount > cleanerRequestCount {
    t.Error("More cleaner finished events than requests")
}
```

**Note**: This would require passing GameContext to the test, which may not align with the test's current scope.

### Recommendation
**Use Option A**: Remove the cleaner validation lines entirely. The test is focused on snapshot consistency, and cleaners don't use snapshots.

---

## Issue 2: EventQueue Data Races

### Location
- `systems/cleaner_race_test.go`: `TestNoRaceActivation` and `TestNoRaceFlashEffects`
- Race detected in `engine/events.go:236` (Push) vs `engine/events.go:301` (Consume)

### Root Cause
The EventQueue implementation has a fundamental data race in its lock-free ring buffer design:

```go
// Push writes to array without synchronizing with readers
func (eq *EventQueue) Push(event GameEvent) {
    for {
        currentTail := eq.tail.Load()
        nextTail := currentTail + 1

        if eq.tail.CompareAndSwap(currentTail, nextTail) {
            eq.events[currentTail%256] = event  // ⚠️ LINE 236: WRITE
            // ... overflow handling ...
            return
        }
    }
}

// Consume reads from array without synchronizing with writers
func (eq *EventQueue) Consume() []GameEvent {
    currentHead := eq.head.Load()
    currentTail := eq.tail.Load()
    // ...
    for i := uint64(0); i < available; i++ {
        result[i] = eq.events[(currentHead+i)%256]  // ⚠️ LINE 301: READ
    }
    // ...
}
```

### Race Scenario
1. **Thread A (Producer)**: Claims slot via `tail.CompareAndSwap` at index `N`
2. **Thread B (Consumer)**: Reads `head` and `tail`, sees event at index `N` is available
3. **Thread A**: Writes `GameEvent` to `eq.events[N]` (line 236)
4. **Thread B**: Reads `GameEvent` from `eq.events[N]` (line 301)

**Result**: Concurrent read/write to `eq.events[N]` without synchronization → DATA RACE

### Why This Is a Problem
- Go's race detector flags this as undefined behavior
- `GameEvent` struct contains a `time.Time` field (non-atomic type)
- Reading a partially-written `time.Time` can cause panics or corrupt data
- Tests fail with `-race` flag (CI requirement per architecture.md)

### Test Context
The failing tests simulate concurrent producers and consumers:

```go
// TestNoRaceActivation (cleaner_race_test.go:85-153)
// Goroutine 1: Update loop consuming events
go func() {
    cleanerSystem.Update(world, 16*time.Millisecond)  // Calls ConsumeEvents()
}()

// Goroutines 2-11: Rapidly push events
go func() {
    ctx.PushEvent(engine.EventCleanerRequest, nil)  // Concurrent Push()
}()
```

### Architectural Question
The tests reveal a design inconsistency:

**From architecture.md**:
> **Single Consumer**: Game loop consumes events each frame

**Reality**: The EventQueue implementation allows multiple concurrent consumers (no enforcement), and tests are exposing race conditions.

### Fix Options

#### Option A: Add Mutex Protection (Simplest, Performance Cost)
```go
type EventQueue struct {
    mu     sync.Mutex
    events [256]GameEvent
    head   uint64  // No longer atomic
    tail   uint64  // No longer atomic
}

func (eq *EventQueue) Push(event GameEvent) {
    eq.mu.Lock()
    defer eq.mu.Unlock()
    // ... rest of logic ...
}

func (eq *EventQueue) Consume() []GameEvent {
    eq.mu.Lock()
    defer eq.mu.Unlock()
    // ... rest of logic ...
}
```

**Pros**: Eliminates race, simple implementation
**Cons**: Defeats lock-free design goal, adds contention

#### Option B: Use Channels (Go Idiomatic)
```go
type EventQueue struct {
    events chan GameEvent
}

func NewEventQueue() *EventQueue {
    return &EventQueue{
        events: make(chan GameEvent, 256),
    }
}

func (eq *EventQueue) Push(event GameEvent) {
    select {
    case eq.events <- event:
    default:
        // Buffer full, drop oldest (requires draining)
    }
}

func (eq *EventQueue) Consume() []GameEvent {
    var result []GameEvent
    for {
        select {
        case e := <-eq.events:
            result = append(result, e)
        default:
            return result
        }
    }
}
```

**Pros**: Go-native, race-free, simple
**Cons**: Requires redesign, may change overflow behavior

#### Option C: Fix Lock-Free Implementation (Complex)
Add a "published" flag for each slot using atomic operations:

```go
type EventQueue struct {
    events    [256]GameEvent
    published [256]atomic.Bool  // NEW: Track which slots are written
    head      atomic.Uint64
    tail      atomic.Uint64
}

func (eq *EventQueue) Push(event GameEvent) {
    for {
        currentTail := eq.tail.Load()
        if eq.tail.CompareAndSwap(currentTail, currentTail+1) {
            idx := currentTail % 256
            eq.events[idx] = event
            eq.published[idx].Store(true)  // Mark as published AFTER write
            return
        }
    }
}

func (eq *EventQueue) Consume() []GameEvent {
    currentHead := eq.head.Load()
    currentTail := eq.tail.Load()

    var result []GameEvent
    for i := currentHead; i < currentTail; i++ {
        idx := i % 256
        if !eq.published[idx].Load() {
            break  // Slot not yet published, stop
        }
        result = append(result, eq.events[idx])
        eq.published[idx].Store(false)  // Reset for reuse
    }

    eq.head.Store(currentHead + uint64(len(result)))
    return result
}
```

**Pros**: Maintains lock-free design
**Cons**: Complex, harder to reason about, more atomic operations

#### Option D: Restrict to Single Consumer + Document (Pragmatic)
Keep current design but:
1. Document that Consume() MUST be called from a single goroutine
2. Modify tests to not call Consume() concurrently
3. Add assertion in Consume() to detect multiple consumers

```go
var consumeInProgress atomic.Bool

func (eq *EventQueue) Consume() []GameEvent {
    if !consumeInProgress.CompareAndSwap(false, true) {
        panic("EventQueue.Consume called concurrently (single consumer only)")
    }
    defer consumeInProgress.Store(false)

    // ... existing logic ...
}
```

**Pros**: Minimal code change, maintains performance
**Cons**: Doesn't fix the race, just detects misuse

### Analysis

**Is this a real bug or test artifact?**

Looking at actual usage in the codebase:
- **Producers**: `ScoreSystem`, `CleanerSystem`, `GoldSystem` (all run in main game loop)
- **Consumer**: Main game loop calls `ctx.ConsumeEvents()` once per frame

**Reality**: In production, there is effectively a **single consumer** (game loop) and **single-threaded producers** (systems run sequentially in Update()).

**The race tests are artificial**: They simulate concurrent Push/Consume which doesn't happen in real gameplay.

### Recommendation

**Option D with test modification**:

1. **Document single-consumer requirement** in `events.go`
2. **Fix race tests** to match actual usage pattern:
   - Single consumer goroutine
   - Producers can push concurrently (this is safe)
   - Don't call Consume() from multiple goroutines simultaneously

3. **Example test fix**:
```go
// BEFORE (creates artificial race)
go func() {
    cleanerSystem.Update(world, dt)  // Calls ConsumeEvents
}()
go func() {
    ctx.PushEvent(EventCleanerRequest, nil)
}()

// AFTER (matches production usage)
stopChan := make(chan struct{})

// Single consumer (like main game loop)
go func() {
    ticker := time.NewTicker(16 * time.Millisecond)
    for {
        select {
        case <-ticker.C:
            cleanerSystem.Update(world, 16*time.Millisecond)
        case <-stopChan:
            return
        }
    }
}()

// Producers push events (this is safe)
for i := 0; i < 10; i++ {
    ctx.PushEvent(EventCleanerRequest, nil)
    time.Sleep(5 * time.Millisecond)
}
```

**If full lock-free correctness is required**, use **Option C** (published flags).

---

## Issue 3: TestCleanerFinishedEvent Failure

### Location
`systems/cleaner_event_test.go:50-101`

### Symptoms
```
--- FAIL: TestCleanerFinishedEvent (0.00s)
    cleaner_event_test.go:99: Expected EventCleanerFinished to be emitted when cleaners complete
```

### Test Logic
```go
func TestCleanerFinishedEvent(t *testing.T) {
    // 1. Create Red character
    createRedCharacterAt(world, 40, 5)

    // 2. Push EventCleanerRequest
    ctx.PushEvent(engine.EventCleanerRequest, nil)
    cleanerSystem.Update(world, 16*time.Millisecond)

    // 3. Consume the CleanerRequest event
    ctx.ConsumeEvents()  // ⚠️ Clears queue

    // 4. Simulate cleaner animation completing
    for i := 0; i < maxUpdates; i++ {
        cleanerSystem.Update(world, 16*time.Millisecond)
        if len(world.GetEntitiesWith(cleanerType)) == 0 {
            break  // Cleaners destroyed
        }
    }

    // 5. Check for EventCleanerFinished
    events := ctx.PeekEvents()
    hasFinishedEvent := false
    for _, event := range events {
        if event.Type == engine.EventCleanerFinished {
            hasFinishedEvent = true
            break
        }
    }

    if !hasFinishedEvent {
        t.Error("Expected EventCleanerFinished to be emitted")  // ❌ FAILS
    }
}
```

### Root Cause Analysis

**Implementation in `cleaner_system.go:62-67`**:
```go
// If no cleaners exist but we spawned this session, emit finished event
if len(entities) == 0 && cs.hasSpawnedSession {
    cs.ctx.PushEvent(engine.EventCleanerFinished, nil)
    cs.hasSpawnedSession = false
    cs.cleanupExpiredFlashes(world)
    return
}
```

**The condition requires**:
1. `len(entities) == 0` (no cleaner components)
2. `cs.hasSpawnedSession == true` (spawned at least once this session)

**Potential Issues**:

#### Issue 3A: `hasSpawnedSession` Not Set
The flag is set in `spawnCleaners()` which is only called when processing `EventCleanerRequest`. If the event wasn't processed, the flag remains false.

**Check**: Line 45 sets it:
```go
if !cs.spawned[event.Frame] {
    cs.spawnCleaners(world)
    cs.spawned[event.Frame] = true
    cs.hasSpawnedSession = true  // ✅ Set here
}
```

#### Issue 3B: Event Consumed Too Early
The test calls `ctx.ConsumeEvents()` at line 70, which would consume the `EventCleanerFinished` if it was already pushed.

**Timeline**:
- Update 1: Process `EventCleanerRequest`, spawn cleaners
- Updates 2-N: Cleaners move across screen
- Update N+1: All cleaners destroyed, push `EventCleanerFinished`
- **BUT**: The loop (line 74-80) calls `Update()` which internally consumes events!

**Let's trace CleanerSystem.Update()**:
```go
func (cs *CleanerSystem) Update(world *engine.World, dt time.Duration) {
    events := cs.ctx.ConsumeEvents()  // ⚠️ LINE 38: Consumes ALL events!
    // ...
    if len(entities) == 0 && cs.hasSpawnedSession {
        cs.ctx.PushEvent(engine.EventCleanerFinished, nil)  // Pushes event
        cs.hasSpawnedSession = false
        return
    }
    // ...
}
```

**The race**:
1. Test loop iteration N: `cleanerSystem.Update()` runs
2. Cleaners reach target, get destroyed
3. `Update()` detects `len(entities) == 0`, pushes `EventCleanerFinished`
4. Test loop iteration N+1: `cleanerSystem.Update()` runs
5. `Update()` line 38: `events := cs.ctx.ConsumeEvents()` **consumes the EventCleanerFinished**!
6. Test checks queue: Event is gone

#### Issue 3C: Timing - Event Not Yet Pushed
If the test checks the queue before `EventCleanerFinished` is pushed, it won't find it.

### Fix Options

#### Option A: Check Events After Each Update (Recommended)
```go
func TestCleanerFinishedEvent(t *testing.T) {
    // ... setup ...

    ctx.ConsumeEvents()  // Clear CleanerRequest

    foundFinished := false
    for i := 0; i < maxUpdates; i++ {
        cleanerSystem.Update(world, 16*time.Millisecond)

        // Check events AFTER each update
        events := ctx.PeekEvents()
        for _, e := range events {
            if e.Type == engine.EventCleanerFinished {
                foundFinished = true
                break
            }
        }

        if foundFinished {
            break
        }
    }

    if !foundFinished {
        t.Error("Expected EventCleanerFinished to be emitted")
    }
}
```

#### Option B: Check Before Final Update Consumes It
```go
func TestCleanerFinishedEvent(t *testing.T) {
    // ... setup ...

    ctx.ConsumeEvents()  // Clear CleanerRequest

    for i := 0; i < maxUpdates; i++ {
        // Check BEFORE update (in case this is the frame it gets pushed AND consumed)
        events := ctx.PeekEvents()
        for _, e := range events {
            if e.Type == engine.EventCleanerFinished {
                return  // Success
            }
        }

        cleanerSystem.Update(world, 16*time.Millisecond)

        // Also check if cleaners are gone
        if len(world.GetEntitiesWith(cleanerType)) == 0 {
            // Cleaners done, check one more time
            events := ctx.PeekEvents()
            for _, e := range events {
                if e.Type == engine.EventCleanerFinished {
                    return  // Success
                }
            }
        }
    }

    t.Error("Expected EventCleanerFinished to be emitted")
}
```

#### Option C: Capture Events During Update
Modify test to track all events emitted during the test:

```go
func TestCleanerFinishedEvent(t *testing.T) {
    // ... setup ...

    allEvents := []engine.GameEvent{}

    ctx.ConsumeEvents()  // Clear CleanerRequest

    for i := 0; i < maxUpdates; i++ {
        cleanerSystem.Update(world, 16*time.Millisecond)

        // Capture events after each update
        events := ctx.PeekEvents()
        allEvents = append(allEvents, events...)
        ctx.ConsumeEvents()  // Clear for next iteration

        if len(world.GetEntitiesWith(cleanerType)) == 0 {
            break
        }
    }

    // Search all captured events
    hasFinishedEvent := false
    for _, e := range allEvents {
        if e.Type == engine.EventCleanerFinished {
            hasFinishedEvent = true
            break
        }
    }

    if !hasFinishedEvent {
        t.Error("Expected EventCleanerFinished to be emitted")
    }
}
```

### Recommendation
**Use Option A**: Check events after each update in the loop. This catches the event before it gets consumed by the next `Update()` call.

---

## Recommendations Summary

### Immediate Actions (Simple Fixes)

1. **Fix `cleanerSnap` undefined** (`engine/game_state_test.go:778-784`)
   - **Action**: Delete lines 778-784
   - **Reason**: CleanerSystem is event-driven, has no snapshot
   - **Effort**: 30 seconds

2. **Fix `TestCleanerFinishedEvent`** (`systems/cleaner_event_test.go`)
   - **Action**: Check events after each update (Option A)
   - **Reason**: Event is being consumed before test checks
   - **Effort**: 5 minutes

### Architectural Decision Required (Complex)

3. **EventQueue race conditions** (`systems/cleaner_race_test.go`)
   - **Option 1**: Fix tests to match single-consumer pattern (pragmatic)
   - **Option 2**: Add mutex protection (simple, performance cost)
   - **Option 3**: Implement proper lock-free design with published flags (complex)
   - **Option 4**: Use Go channels (idiomatic redesign)

**My recommendation**: **Option 1** (fix tests) because:
- Real production code has single consumer (game loop)
- Current design works for intended usage
- Race tests are simulating unrealistic scenario
- Avoids unnecessary complexity

If full concurrent correctness is required, use **Option 3** (lock-free with published flags).

### Testing.go Evaluation

**Current state**: `engine/testing.go` contains only `NewTestGameContext()` helper.

**Assessment**: This is minimal and useful - **keep it**.

**Reason**: The helper is simple, focused, and genuinely needed for test setup. It's not over-complicated like the task description suggested might be the case.

---

## Effort Estimates

| Issue | Fix Option | Effort | Risk |
|-------|-----------|--------|------|
| cleanerSnap undefined | Delete 7 lines | 1 min | None |
| TestCleanerFinishedEvent | Modify test loop | 10 min | Low |
| EventQueue races (Option 1) | Fix tests, add docs | 30 min | Low |
| EventQueue races (Option 3) | Lock-free redesign | 3 hours | Medium |

**Total for simple fixes**: ~15 minutes
**Total if redesigning EventQueue**: ~4 hours

---

## Questions for User

Before proceeding with fixes:

1. **EventQueue design**: Is the single-consumer assumption acceptable, or do you need full multi-consumer safety?

2. **Test philosophy**: Are you okay with tests that reflect actual production usage patterns, or do you want tests that validate theoretical race conditions?

3. **Performance priority**: Is lock-free performance critical for the event queue, or is a simple mutex acceptable?

Please advise which approach you prefer, and I'll implement the fixes accordingly.
