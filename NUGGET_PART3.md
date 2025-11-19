# Nugget Feature - Tab Jump Mechanic

## Overview
Implemented Tab key functionality to jump the cursor directly to the nugget position. This mechanic requires a score of at least 10 and deducts 10 from the score upon successful jump. Available in both Normal and Insert modes.

## Implementation Details

### 1. NuggetSystem.JumpToNugget() Method (`systems/nugget_system.go`)

Added a method to retrieve the position of the active nugget:

```go
// JumpToNugget returns the position of the active nugget, or (-1, -1) if no nugget exists
func (s *NuggetSystem) JumpToNugget(world *engine.World) (int, int) {
    // Get active nugget entity ID
    activeNuggetEntity := s.activeNugget.Load()
    if activeNuggetEntity == 0 {
        return -1, -1
    }

    // Get position component from entity
    posType := reflect.TypeOf(components.PositionComponent{})
    posComp, ok := world.GetComponent(engine.Entity(activeNuggetEntity), posType)
    if !ok {
        // No position component (shouldn't happen, but handle gracefully)
        return -1, -1
    }

    // Extract position
    pos := posComp.(components.PositionComponent)
    return pos.X, pos.Y
}
```

**Design Notes:**
- Uses atomic load to read active nugget reference (thread-safe)
- Returns (-1, -1) if no active nugget exists
- Returns (-1, -1) if position component is missing (graceful error handling)
- Does not check score (that's the InputHandler's responsibility)
- Does not modify any state (read-only operation)

### 2. InputHandler Modifications (`modes/input.go`)

#### Added NuggetSystem Reference
```go
type InputHandler struct {
    ctx           *engine.GameContext
    scoreSystem   *systems.ScoreSystem
    nuggetSystem  *systems.NuggetSystem  // NEW
}

// SetNuggetSystem sets the nugget system reference for Tab jump functionality
func (h *InputHandler) SetNuggetSystem(nuggetSystem *systems.NuggetSystem) {
    h.nuggetSystem = nuggetSystem
}
```

**Design Notes:**
- Follows existing pattern for cross-system references (scoreSystem)
- Optional setter allows for graceful handling if nugget system is missing

#### Tab Key Handler in Insert Mode
```go
case tcell.KeyTab:
    // Tab: Jump to nugget if score >= 10
    if h.nuggetSystem != nil {
        score := h.ctx.State.GetScore()
        if score >= 10 {
            // Get nugget position
            x, y := h.nuggetSystem.JumpToNugget(h.ctx.World)
            if x >= 0 && y >= 0 {
                // Deduct 10 from score
                h.ctx.State.AddScore(-10)
                // Update cursor position atomically
                h.ctx.CursorX = x
                h.ctx.CursorY = y
                h.ctx.State.SetCursorX(x)
                h.ctx.State.SetCursorY(y)
            }
        }
    }
    return true
```

**Key Behaviors:**
1. **Score Check**: Only jumps if score >= 10
2. **Position Validation**: Only updates cursor if valid position returned (x >= 0, y >= 0)
3. **Score Deduction**: Deducts exactly 10 points from score
4. **Atomic Update**: Updates both GameContext cursor and GameState cursor atomically
5. **Silent Failure**: No error feedback if score insufficient or no nugget (by design)

#### Tab Key Handler in Normal Mode
Same logic as Insert mode, with one addition:
```go
h.ctx.LastCommand = "" // Clear last command
```

**Design Notes:**
- Clears last command to maintain consistency with other navigation keys
- Identical behavior to Insert mode otherwise

### 3. System Wiring (`cmd/vi-fighter/main.go`)

Added nugget system reference to input handler:
```go
// Create input handler
inputHandler := modes.NewInputHandler(ctx, scoreSystem)
inputHandler.SetNuggetSystem(nuggetSystem)
```

**Integration:**
- Follows existing pattern for system wiring
- Called immediately after InputHandler creation
- Ensures nugget system is available before game loop starts

## Testing (`systems/nugget_jump_test.go`)

Created comprehensive test suite covering all jump mechanic scenarios:

### Test Coverage

#### 1. TestNuggetJumpWithSufficientScore
- Verifies jump succeeds when score >= 10
- Validates position returned correctly
- Confirms score deduction (15 → 5 after -10)

#### 2. TestNuggetJumpWithInsufficientScore
- Verifies position is still returned (method doesn't check score)
- Confirms InputHandler logic would prevent jump when score < 10
- Validates score remains unchanged when insufficient

#### 3. TestNuggetJumpWithNoActiveNugget
- Verifies (-1, -1) returned when no nugget active
- Confirms cursor position unchanged
- Validates score unchanged

#### 4. TestNuggetJumpUpdatesPosition
- Verifies cursor position updated correctly (5, 5) → (75, 20)
- Confirms both GameContext and GameState cursors updated
- Validates score deduction (20 → 10)

#### 5. TestNuggetJumpMultipleTimes
- Verifies multiple jumps work correctly
- Tests collection → respawn → jump again cycle
- Validates score deduction accumulates (30 → 20 → 10)

#### 6. TestNuggetJumpWithNuggetAtEdge
- Tests jumping to nuggets at all screen edges
- Verifies correct position returned for:
  - Top-left (0, 0)
  - Top-right (99, 0)
  - Bottom-left (0, 29)
  - Bottom-right (99, 29)
  - Middle edges (top, bottom, left, right)

#### 7. TestJumpToNuggetMethodReturnsCorrectPosition
- Tests JumpToNugget method directly
- Verifies (-1, -1) with no nugget
- Confirms correct position (30, 12) with nugget
- Validates (-1, -1) after ClearActiveNugget

#### 8. TestJumpToNuggetWithMissingComponent
- Tests graceful handling of missing PositionComponent
- Verifies (-1, -1) returned (error condition)
- Edge case that shouldn't happen in practice

#### 9. TestJumpToNuggetAtomicCursorUpdate
- Verifies both GameContext and GameState cursors updated
- Confirms atomic cursor state synchronization
- Validates cursors remain in sync after jump

#### 10. TestJumpToNuggetEntityStillExists
- Verifies nugget entity remains after jump (not collected)
- Confirms active nugget reference unchanged
- Validates jump is non-destructive operation

**Test Results:**
```bash
go test -race ./systems -run TestNugget
PASS
- All 16 tests pass
- No race conditions detected
```

## Architecture Compliance

This implementation strictly follows vi-fighter architecture principles:

### 1. ECS Pattern
- NuggetComponent unchanged (data-only)
- JumpToNugget method in NuggetSystem (logic in system)
- World is single source of truth for entity/component data

### 2. State Ownership Model
- Atomic read of active nugget reference (lock-free)
- Score check uses atomic GetScore operation
- Cursor update uses atomic SetCursorX/SetCursorY operations
- No local state caching

### 3. Concurrency Model
- Runs synchronously in main game loop (no goroutines)
- All state reads/writes are thread-safe
- Atomic operations for all shared state access
- No race conditions (verified with -race flag)

### 4. Input Handling
- Tab key handled consistently in both Normal and Insert modes
- Follows existing pattern for special keys (arrow keys, etc.)
- Silent failure when conditions not met (no error feedback)

### 5. System Coordination
- Uses existing cross-system reference pattern
- Clear separation of concerns (NuggetSystem provides position, InputHandler handles logic)
- No circular dependencies

## Behavioral Characteristics

### Jump Mechanics
- **Trigger**: Tab key in Normal or Insert mode
- **Requirement**: Score >= 10
- **Cost**: -10 score
- **Effect**: Cursor jumps to nugget position
- **Silent Failure**: No feedback if score < 10 or no nugget

### Score Management
- **Check**: Score >= 10 required before jump
- **Deduction**: Exactly 10 points deducted
- **Atomic**: Score deduction is atomic operation
- **No Refund**: Deduction happens regardless of what happens after jump

### Cursor Position Update
- **Atomic**: Both GameContext and GameState cursors updated
- **Validation**: Only updates if valid position (x >= 0, y >= 0)
- **Bounds**: No bounds checking needed (nugget can't spawn out of bounds)
- **Synchronization**: GameContext.CursorX/Y and GameState cursor kept in sync

### Nugget State
- **Preserved**: Nugget entity remains after jump (not collected)
- **Active Reference**: Active nugget reference unchanged
- **Collection**: Player must still type on nugget to collect it

### Edge Cases Handled
1. **No Active Nugget**: Jump fails silently, no score deduction
2. **Insufficient Score**: Jump fails silently, no state change
3. **Missing PositionComponent**: Returns (-1, -1), graceful failure
4. **Screen Edges**: Works correctly for nuggets at any position

## Game Flow Integration

### Before Jump
1. Nugget spawns (NuggetSystem)
2. Player accumulates score >= 10
3. Player presses Tab key

### During Jump
1. InputHandler receives Tab key event
2. Checks score >= 10
3. Calls NuggetSystem.JumpToNugget(world)
4. NuggetSystem returns nugget position
5. If valid position:
   - Deducts 10 from score
   - Updates GameContext cursor
   - Updates GameState cursor atomically
6. Returns control to game loop

### After Jump
1. Cursor is now at nugget position
2. Nugget still exists (not collected)
3. Player can type any character to collect nugget (Part 2 mechanic)
4. Score is 10 points less than before

## Interaction with Existing Features

### Score System
- Jump mechanic integrates seamlessly with existing score
- Score deduction uses existing AddScore(-10) method
- No conflicts with score gain from typing

### Heat System
- Jump does not affect heat meter
- Heat remains unchanged after jump
- Heat gain from nugget collection (Part 2) still works

### Boost System
- Jump does not affect boost state
- Boost remains active/inactive as before
- No interaction between jump and boost

### Nugget Collection (Part 2)
- Jump positions cursor but doesn't collect
- Player must still type on nugget to collect
- Collection mechanics unchanged (Part 2)
- Jump → Collection workflow is smooth

### Gold Sequence System
- No interaction with gold sequences
- Both can be active simultaneously
- Tab jump works even during active gold sequence

### Spawn System
- No interaction with spawn mechanics
- Nuggets continue to spawn every 5 seconds
- Jump doesn't affect spawn timing

## Performance Characteristics

### Time Complexity
- Score check: O(1) - atomic read
- JumpToNugget: O(1) - atomic load + component lookup
- Position update: O(1) - atomic writes
- Total: O(1) constant time

### Memory Impact
- No additional allocations
- Reuses existing entity/component infrastructure
- No memory overhead

### Concurrency Safety
- All operations are thread-safe
- Atomic reads/writes for shared state
- No race conditions (verified with -race flag)
- No blocking operations

## Files Modified

### New Files
- `systems/nugget_jump_test.go` - Comprehensive test suite for jump mechanic

### Modified Files
- `systems/nugget_system.go` - Added JumpToNugget() method
- `modes/input.go` - Added NuggetSystem reference and Tab key handlers
- `cmd/vi-fighter/main.go` - Wired up NuggetSystem to InputHandler

## Verification

To test the implementation:
1. Build: `go build ./cmd/vi-fighter`
2. Run: `./vi-fighter`
3. Wait for orange '●' nugget to appear
4. Type characters to gain score >= 10
5. Press Tab key
6. Observe:
   - Cursor jumps to nugget position (if score >= 10)
   - Score decreases by 10
   - Nugget remains visible
   - No visual feedback (silent operation)
7. Type any character to collect nugget (Part 2)

## Known Limitations (By Design)

Current implementation:
- ✅ Tab jumps to nugget when score >= 10
- ✅ Score deducted by exactly 10 points
- ✅ Cursor position updated atomically
- ✅ Works in both Normal and Insert modes
- ✅ Nugget remains after jump (not collected)
- ❌ No visual feedback on successful jump (silent operation)
- ❌ No visual feedback when insufficient score (silent failure)
- ❌ No audio feedback (no audio system exists)
- ❌ No animation during jump (instant teleport)

## Future Enhancements (Potential)

The following features could be added in future parts:
1. Visual feedback on successful jump (flash, trail effect)
2. Visual feedback when insufficient score (error indicator)
3. Sound effect for jump (requires audio system)
4. Animation during jump (cursor movement interpolation)
5. Variable jump cost based on distance
6. Jump cooldown timer
7. Multiple jump targets (choose from multiple nuggets)
8. Jump history tracking

## Testing Strategy

All tests follow vi-fighter testing patterns:
- Use `tcell.NewSimulationScreen` for UI tests
- Use `engine.NewGameContext` for context creation
- Manual entity creation for controlled test scenarios
- Verify atomic cursor state synchronization
- Test edge cases (no nugget, insufficient score, screen edges)
- Test integration with existing systems (score, nugget collection)
- Verify race conditions with -race flag
- Ensure all tests are deterministic and repeatable

## Concurrency Guarantees

### Thread Safety
1. **Active Nugget Reference**: Atomic load (no locks)
2. **Score Check**: Atomic read (no locks)
3. **Position Lookup**: Component read (World handles locking)
4. **Score Deduction**: Atomic write (no locks)
5. **Cursor Update**: Atomic writes (no locks)

### Race Condition Prevention
- All shared state uses atomic operations
- No manual locking required
- All tests pass with -race flag
- No data races detected

### Memory Safety
- No dangling pointers
- All entity references validated before use
- Graceful handling of missing components
- No memory leaks

## Integration Testing

Full game cycle tested:
1. Nugget spawn → accumulate score → Tab jump → collect
2. Multiple jumps with score tracking
3. Jump at screen edges
4. Jump with no nugget (silent failure)
5. Jump with insufficient score (silent failure)

All integration scenarios pass with -race flag.

## Conclusion

Tab jump mechanic is now fully functional. Players can jump to nuggets by pressing Tab (cost: 10 score) in both Normal and Insert modes. The implementation is thread-safe, well-tested, and follows all architecture guidelines. The mechanic integrates seamlessly with existing nugget collection (Part 2) and score systems.

## Next Steps (If Continuing)

If implementing additional nugget features:
1. Visual feedback should use existing rendering pipeline
2. Audio feedback should integrate with (future) audio system
3. Animation should use existing time provider
4. Alternative jump methods should extend InputHandler
5. Jump modifiers should be separate commands (not part of Tab)
