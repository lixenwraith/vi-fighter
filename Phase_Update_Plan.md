## Phase Overview

The fix will be implemented in **5 sequential phases**, each leaving the game in a working state. Each phase includes validation steps and architecture.md updates.

### Phase Dependencies
```
Phase 1: Remove Goroutine (CRITICAL - blocks all others)
   ↓
Phase 2: Synchronous Cleaner Updates
   ↓
Phase 3: Frame-Coherent Snapshots
   ↓
Phase 4: State Machine Formalization
   ↓
Phase 5: Documentation & Validation
```

## Phase 1: Eliminate Autonomous Goroutine [CRITICAL]

**Goal**: Remove the CleanerSystem's independent goroutine to eliminate the primary race condition source.

**Scope**:
- Remove `updateLoop()` method and goroutine spawn
- Remove associated synchronization primitives (`wg`, `stopChan`)
- Ensure cleaners still animate (temporarily at frame rate)

**Architecture.md Updates Required**:
```markdown
## Concurrency Model
- ~~CleanerSystem: Concurrent update loop running at 60 FPS in separate goroutine~~
- CleanerSystem: Synchronous updates in main game loop (16ms tick)
```

### Claude Code Prompt - Phase 1:

```
# Task: Remove CleanerSystem Goroutine

## Context
The CleanerSystem in vi-fighter currently runs an autonomous goroutine (`updateLoop()`) that modifies ECS entities independently of the main game loop. This causes race conditions detected by `go build -race`.

## Requirements
1. **Remove the goroutine spawn** in `NewCleanerSystem()` - delete the line `go cs.updateLoop()`
2. **Remove the `updateLoop()` method** entirely from `systems/cleaner_system.go`
3. **Remove unused synchronization fields** from CleanerSystem struct:
   - `wg sync.WaitGroup`
   - `stopChan chan struct{}`
4. **Move update logic to Update() method** - for now, just call `cs.updateCleaners()` directly in Update() if `cs.isActive.Load()` is true
5. **Fix Shutdown() method** - remove WaitGroup wait and channel close
6. **Preserve all other functionality** - cleaners should still spawn, move, and destroy entities

## Validation
- Game should compile without errors
- Run `go test -race ./systems/...` - should show fewer or different race conditions
- Cleaners should still appear when Gold is completed at max heat
- Animation might be slightly different (16ms vs 60 FPS) - this is acceptable for now

## Files to Modify
- `systems/cleaner_system.go`

Do NOT modify any other files in this phase. Keep changes minimal and focused.
```

---

## Phase 2: Synchronous Cleaner Updates

**Goal**: Restructure cleaner updates to work correctly at 16ms frame rate without goroutines.

**Scope**:
- Refactor `updateCleaners()` to accept delta time properly
- Ensure smooth animation at 60 FPS equivalent using delta time
- Remove world reference storage

**Architecture.md Updates Required**:
```markdown
## Performance Guidelines
### Hot Path Optimizations
5. CleanerSystem updates synchronously with frame-accurate delta time
```

### Claude Code Prompt - Phase 2:

```
# Task: Synchronous Cleaner Updates

## Context
Phase 1 removed the goroutine. Now we need to make cleaner updates work properly in the synchronous Update() method with accurate delta time calculations.

## Requirements

1. **Remove world reference storage**:
   - Remove `world *engine.World` field from CleanerSystem struct
   - Remove all `cs.world` assignments and reads
   - Pass world explicitly where needed

2. **Fix `updateCleaners()` signature and logic**:
   ```go
   func (cs *CleanerSystem) updateCleaners(world *engine.World, dt time.Duration)
   ```
   - Accept world and dt parameters
   - Calculate deltaTime as `dt.Seconds()` not from timestamps
   - Remove lastUpdateTime tracking (use dt instead)

3. **Fix spawn request handling**:
   - Modify `cleanerSpawnRequest` struct to include rows to spawn
   - In `processSpawnRequest()`, scan for red rows immediately and spawn
   - Don't store world reference

4. **Update the Update() method**:
   ```go
   func (cs *CleanerSystem) Update(world *engine.World, dt time.Duration) {
       // Process spawn requests
       select {
       case <-cs.spawnChan:
           cs.processSpawnRequest(world)
       default:
       }
       
       // Update cleaners if active
       if cs.isActive.Load() {
           cs.updateCleaners(world, dt)
       }
       
       // Cleanup flashes
       cs.cleanupExpiredFlashes(world)
   }
   ```

5. **Fix TriggerCleaners()**:
   - Should just send empty request through channel
   - Actual row scanning happens in processSpawnRequest()

## Validation
- Run `go test -race ./systems/...` - should pass without races
- Test cleaner animation visually - should be smooth
- Verify cleaners still detect and destroy Red characters

## Files to Modify
- `systems/cleaner_system.go`

Keep all atomic operations and other synchronization as-is for now.

---

## Phase 3: Frame-Coherent Snapshots

**Goal**: Implement proper snapshot mechanism for renderer to read cleaner state without races.

**Scope**:
- Create immutable snapshot structure
- Separate update state from render state
- Ensure renderer never directly accesses mutable cleaner data

**Architecture.md Updates Required**:
```markdown
## Rendering System
### Thread Safety
- CleanerSystem provides immutable snapshots via `GetCleanerSnapshots()`
- Renderer never directly accesses CleanerComponent entities during render
```

### Claude Code Prompt - Phase 3:

```
# Task: Frame-Coherent Cleaner Snapshots

## Context
The renderer currently calls `GetCleanerSnapshots()` which reads from `cleanerDataMap` while it's being modified. We need proper separation between update and render data.

## Requirements

1. **Enhance the snapshot mechanism**:
   - Keep the existing `GetCleanerSnapshots()` method
   - Ensure it's only called once per frame (not multiple times)
   - The returned snapshots should be truly immutable

2. **Improve cleanerDataMap synchronization**:
   - The `stateMu` should protect ALL accesses to `cleanerDataMap`
   - In `updateCleanerPositions()`, acquire `stateMu.Lock()` when updating `cleanerDataMap`
   - Ensure trail position updates happen under lock

3. **Add render caching** in TerminalRenderer:
   ```go
   type TerminalRenderer struct {
       // ... existing fields ...
       cleanerSnapshots []render.CleanerSnapshot
       cleanerSnapshotFrame int64  // Frame counter for cache invalidation
   }
   ```

4. **Update Render() method** to cache snapshots:
   ```go
   func (r *TerminalRenderer) Render(ctx *engine.GameContext) {
       frameNum := ctx.GetFrameNumber() // Add this to GameContext if needed
       
       // Update cleaner snapshot cache once per frame
       if r.cleanerSystem != nil && r.cleanerSnapshotFrame != frameNum {
           r.cleanerSnapshots = r.cleanerSystem.GetCleanerSnapshots()
           r.cleanerSnapshotFrame = frameNum
       }
       
       // ... rest of render logic uses r.cleanerSnapshots ...
   }
   ```

5. **Fix detectAndDestroyRedCharacters()**:
   - This method should also properly lock when accessing `cleanerDataMap`
   - Ensure no concurrent modification of trail positions

## Validation
- Run `go test -race ./...` - should pass completely
- Run game with `go run -race ./cmd/vi-fighter` - no race warnings
- Cleaner animation should be smooth and correct

## Files to Modify
- `systems/cleaner_system.go` 
- `render/terminal_renderer.go`
- `engine/game_context.go` (add frame counter if needed)

Focus on proper locking granularity - don't hold locks longer than necessary.
```

---

## Phase 4: State Machine Formalization

**Goal**: Formalize game phase transitions to prevent state inconsistencies.

**Scope**:
- Enhance GamePhase enum with cleaner states
- Add explicit transition validation
- Ensure atomic phase changes

**Architecture.md Updates Required**:
```markdown
## State Management
### Game Phases
- PhaseNormal: Default state, spawn active
- PhaseGoldActive: Gold sequence on screen
- PhaseGoldComplete: Transition state after gold completion
- PhaseDecayWait: Waiting for decay timer
- PhaseDecayAnimation: Decay animation running  
- PhaseCleanerPending: Cleaner activation requested
- PhaseCleanerActive: Cleaners animating

### Phase Transitions
All phase transitions are atomic and validated through GameState methods.
```

### Claude Code Prompt - Phase 4:

```
# Task: Formalize State Machine

## Context
The game has implicit state transitions that can race. We need explicit, validated phase transitions.

## Requirements

1. **Add missing phase states** to `engine/game_state.go`:
   ```go
   const (
       PhaseNormal GamePhase = iota
       PhaseGoldActive
       PhaseGoldComplete    // NEW
       PhaseDecayWait
       PhaseDecayAnimation
       PhaseCleanerPending  // NEW  
       PhaseCleanerActive   // NEW
   )
   ```

2. **Add phase transition validation**:
   ```go
   // Add to GameState
   func (gs *GameState) CanTransition(from, to GamePhase) bool {
       validTransitions := map[GamePhase][]GamePhase{
           PhaseNormal:         {PhaseGoldActive, PhaseCleanerPending},
           PhaseGoldActive:     {PhaseGoldComplete},
           PhaseGoldComplete:   {PhaseDecayWait, PhaseCleanerPending},
           PhaseDecayWait:      {PhaseDecayAnimation},
           PhaseDecayAnimation: {PhaseNormal},
           PhaseCleanerPending: {PhaseCleanerActive},
           PhaseCleanerActive:  {PhaseNormal},
       }
       
       allowed := validTransitions[from]
       for _, phase := range allowed {
           if phase == to {
               return true
           }
       }
       return false
   }
   
   func (gs *GameState) TransitionPhase(to GamePhase) bool {
       gs.mu.Lock()
       defer gs.mu.Unlock()
       
       if !gs.CanTransition(gs.CurrentPhase, to) {
           return false
       }
       
       gs.CurrentPhase = to
       gs.PhaseStartTime = gs.TimeProvider.Now()
       return true
   }
   ```

3. **Update ClockScheduler** to use new phases:
   - Check for PhaseGoldComplete -> trigger decay or cleaner
   - Check for PhaseCleanerPending -> activate cleaners
   - Use TransitionPhase() for all phase changes

4. **Update state methods** to use phases:
   - `RequestCleaners()` should transition to PhaseCleanerPending
   - `ActivateCleaners()` should transition to PhaseCleanerActive
   - `DeactivateCleaners()` should transition to PhaseNormal

5. **Add phase consistency checks**:
   - Gold can only spawn in PhaseNormal
   - Decay can only start from PhaseGoldComplete
   - Cleaners can only activate from PhaseCleanerPending

## Validation
- Test rapid gold completions - no invalid transitions
- Test cleaner->gold->cleaner cycles
- Add unit test for CanTransition() method
- Run full game with race detector

## Files to Modify
- `engine/game_state.go`
- `engine/clock_scheduler.go`
- `systems/gold_sequence_system.go` (use new phases)
- `systems/decay_system.go` (use new phases)

Ensure backward compatibility - existing tests should still pass.
```

---

## Phase 5: Documentation & Validation

**Goal**: Update all documentation, add comprehensive tests, and validate the complete fix.

**Scope**:
- Update architecture.md completely
- Add race condition regression tests  
- Performance validation
- Code cleanup

**Architecture.md Updates Required**:
Complete rewrite of concurrency model section, state management, and testing strategy.

### Claude Code Prompt - Phase 5:

```
# Task: Documentation and Validation

## Context
All race conditions should be fixed. Now we need to update documentation and add tests to prevent regression.

## Requirements

1. **Update architecture.md**:
   - Remove all references to CleanerSystem goroutine
   - Document the synchronous update model
   - Add section on "Race Condition Prevention"
   - Update the "Testing Strategy" with new race tests
   - Document frame coherence strategy

2. **Add race condition tests** in `systems/cleaner_race_test.go`:
   ```go
   func TestNoRaceCleanerConcurrentRenderUpdate(t *testing.T) {
       // Spawn 100 goroutines: 50 updating, 50 rendering
       // Should complete without race detector warnings
   }
   
   func TestNoRaceRapidCleanerCycles(t *testing.T) {
       // Rapidly trigger/complete cleaner cycles
       // Verify no races or state corruption
   }
   ```

3. **Add deterministic test** in `systems/cleaner_deterministic_test.go`:
   ```go
   func TestDeterministicCleanerLifecycle(t *testing.T) {
       // Use MockTimeProvider
       // Verify exact frame-by-frame cleaner behavior
   }
   ```

4. **Performance benchmark** in `systems/cleaner_benchmark_test.go`:
   ```go
   func BenchmarkCleanerUpdateSync(b *testing.B) {
       // Measure performance of synchronous updates
       // Ensure < 1ms for 24 cleaners
   }
   ```

5. **Code cleanup**:
   - Remove any commented-out code
   - Remove unused imports
   - Ensure all comments reflect new architecture
   - Add comment headers explaining synchronous model

6. **Create RACE_FIX_SUMMARY.md**:
   - Document what was wrong
   - How it was fixed
   - Prevention strategies
   - Testing approach

## Validation
- `go test -race ./...` - MUST pass 100% 
- `go test -bench=. ./systems/...` - Performance unchanged or better
- Run game for 5 minutes with `-race` flag - no warnings
- Review with `go vet ./...` and `staticcheck ./...`

## Files to Create/Modify
- `architecture.md`
- `systems/cleaner_race_test.go` (create)
- `systems/cleaner_deterministic_test.go` (create) 
- `systems/cleaner_benchmark_test.go` (update)
- `RACE_FIX_SUMMARY.md` (create)

This phase ensures the fix is permanent and well-documented.
```
