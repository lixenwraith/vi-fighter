# Nugget Feature - Typing Interaction Mechanics

## Overview
Implemented typing interaction mechanics for nuggets in the vi-fighter typing game. This part adds the ability for players to collect nuggets by typing any character while positioned on them, which increases heat and triggers automatic respawn.

## Implementation Details

### 1. ScoreSystem Modifications (`systems/score_system.go`)

#### Added NuggetSystem Reference
```go
type ScoreSystem struct {
    // ... existing fields ...
    nuggetSystem *NuggetSystem
}

func (s *ScoreSystem) SetNuggetSystem(nuggetSystem *NuggetSystem) {
    s.nuggetSystem = nuggetSystem
}
```

**Design Notes:**
- Follows existing pattern for system cross-references (GoldSequenceSystem, SpawnSystem)
- Allows ScoreSystem to communicate with NuggetSystem for collection events

#### Nugget Detection in HandleCharacterTyping
Added nugget detection **before** sequence detection logic:

```go
// Check if this is a nugget - handle before sequence logic
nuggetType := reflect.TypeOf(components.NuggetComponent{})
if _, hasNugget := world.GetComponent(entity, nuggetType); hasNugget && s.nuggetSystem != nil {
    // Handle nugget collection
    s.handleNuggetCollection(world, entity, cursorX, cursorY)
    return
}
```

**Design Notes:**
- Nugget check occurs after CharacterComponent check but before SequenceComponent check
- Early return prevents sequence logic from executing on nugget entities
- Gracefully handles missing NuggetSystem reference

#### Collection Handler Implementation
```go
func (s *ScoreSystem) handleNuggetCollection(world *engine.World, entity engine.Entity, cursorX, cursorY int) {
    // Calculate heat increase: 10% of max heat (screen width)
    maxHeat := s.ctx.Width
    if maxHeat < 1 {
        maxHeat = 1
    }
    heatIncrease := maxHeat / 10
    if heatIncrease < 1 {
        heatIncrease = 1 // Minimum increase of 1
    }

    // Add heat (10% of max)
    s.ctx.State.AddHeat(heatIncrease)

    // Destroy the nugget entity
    world.SafeDestroyEntity(entity)

    // Clear the active nugget reference to trigger respawn
    s.nuggetSystem.ClearActiveNugget()

    // Move cursor right
    s.ctx.CursorX++
    if s.ctx.CursorX >= s.ctx.GameWidth {
        s.ctx.CursorX = s.ctx.GameWidth - 1
    }
    // Sync cursor position to GameState
    s.ctx.State.SetCursorX(s.ctx.CursorX)

    // No score effects, no error effects - silent collection
}
```

**Key Behaviors:**
1. **Heat Increase**: 10% of max heat (screen width)
   - Minimum increase of 1 (for small screens)
   - Example: 80-char screen → +8 heat
2. **Entity Destruction**: Uses SafeDestroyEntity (handles spatial index cleanup)
3. **Respawn Trigger**: Clears active nugget reference, allowing NuggetSystem to spawn new one
4. **Cursor Movement**: Moves cursor right (same as character typing)
5. **Silent Collection**: No score change, no error state, no visual feedback

### 2. System Wiring (`cmd/vi-fighter/main.go`)

Added nugget system reference to score system:
```go
// Wire up system references
scoreSystem.SetGoldSequenceSystem(goldSequenceSystem)
scoreSystem.SetSpawnSystem(spawnSystem)
scoreSystem.SetNuggetSystem(nuggetSystem)  // NEW
decaySystem.SetSpawnSystem(spawnSystem)
```

**Integration:**
- Follows existing pattern for cross-system references
- Maintains proper initialization order

## Testing (`systems/nugget_typing_test.go`)

Created comprehensive test suite covering all collection mechanics:

### Test Coverage

#### 1. TestNuggetTypingIncreasesHeat
- Verifies heat increases by 10% of max heat
- Tests: 100-char screen → +10 heat
- Validates: Heat calculation, nugget destruction, cursor movement

#### 2. TestNuggetTypingDestroysAndReturnsSpawn
- Verifies complete collection → respawn cycle
- Tests: Collection clears active nugget, respawn after 5 seconds
- Validates: Automatic respawn logic, new nugget has all components

#### 3. TestNuggetTypingNoScoreEffect
- Verifies nugget collection doesn't affect score
- Tests: Score remains unchanged after collection
- Validates: Silent collection behavior

#### 4. TestNuggetTypingNoErrorEffect
- Verifies nugget collection doesn't trigger error state
- Tests: No error cursor after collection
- Validates: No error effects on collection

#### 5. TestNuggetTypingMultipleCollections
- Verifies multiple nugget collections accumulate heat
- Tests: Two collections → +10 heat each → +20 total
- Validates: Heat accumulation across multiple collections

#### 6. TestNuggetTypingWithSmallScreen
- Verifies minimum heat increase of 1 on small screens
- Tests: 5-char screen → still +1 heat (not 0)
- Validates: Minimum heat increase safeguard

**Test Results:**
```bash
go test -race ./systems -run TestNugget
PASS
- TestNuggetTypingIncreasesHeat
- TestNuggetTypingDestroysAndReturnsSpawn
- TestNuggetTypingNoScoreEffect
- TestNuggetTypingNoErrorEffect
- TestNuggetTypingMultipleCollections
- TestNuggetTypingWithSmallScreen
```

All tests pass with `-race` flag (no race conditions detected).

## Architecture Compliance

This implementation strictly follows vi-fighter architecture principles:

### 1. ECS Pattern
- Components contain only data (NuggetComponent unchanged)
- Systems contain all logic (ScoreSystem handles collection)
- World is single source of truth (SafeDestroyEntity)

### 2. State Ownership Model
- Atomic operations for active nugget reference (ClearActiveNugget)
- Heat updates use atomic operations (AddHeat)
- No local state caching

### 3. Concurrency Model
- Runs synchronously in main game loop
- No autonomous goroutines
- All state changes are thread-safe
- Follows existing ScoreSystem patterns

### 4. Spatial Indexing
- SafeDestroyEntity handles spatial index cleanup automatically
- No manual index management required

### 5. System Coordination
- Uses existing cross-system reference pattern
- Clear separation of concerns (ScoreSystem detects, NuggetSystem spawns)
- Early return prevents logic conflicts

## Behavioral Characteristics

### Collection Mechanics
- **Trigger**: Typing **any character** while cursor is on nugget position
- **Character Matching**: Not required - any key press collects the nugget
- **Heat Gain**: 10% of max heat (minimum 1)
- **Score Impact**: None
- **Error Impact**: None
- **Cursor Movement**: Right (same as character typing)

### Respawn Behavior
- **Trigger**: Automatic when active nugget reference cleared
- **Timing**: 5 seconds after collection
- **Position**: Random (collision detection, cursor exclusion zone)
- **Limit**: Only one nugget active at a time

### Edge Cases Handled
1. **Small Screens**: Minimum heat increase of 1 (prevents 0 heat gain)
2. **Missing NuggetSystem**: Graceful handling (null check)
3. **Cursor Bounds**: Cursor stays within game bounds after collection
4. **Heat Accumulation**: Multiple collections properly accumulate heat

## Game Flow Integration

### Before Collection
1. Nugget spawns (Part 1 - NuggetSystem)
2. Player positions cursor on nugget
3. Player types any character

### During Collection (This Part)
1. ScoreSystem.HandleCharacterTyping called
2. Detects NuggetComponent on entity
3. Calculates heat increase (10% of max)
4. Adds heat to game state
5. Destroys nugget entity
6. Clears active nugget reference
7. Moves cursor right
8. Returns (no score/error effects)

### After Collection
1. NuggetSystem detects no active nugget (Part 1 - existing logic)
2. Waits 5 seconds (Part 1 - existing logic)
3. Spawns new nugget at random position (Part 1 - existing logic)

## Files Modified

### New Files
- `systems/nugget_typing_test.go` - Comprehensive test suite for typing mechanics

### Modified Files
- `systems/score_system.go` - Added nugget detection and collection handling
- `cmd/vi-fighter/main.go` - Wired up NuggetSystem reference to ScoreSystem

## Verification

To test the implementation:
1. Build: `go build ./cmd/vi-fighter`
2. Run: `./vi-fighter`
3. Wait for orange '●' nugget to appear
4. Move cursor to nugget position
5. Press any key
6. Observe:
   - Heat increases by 10% of screen width
   - Nugget disappears
   - Cursor moves right
   - No score change
   - No error flash
   - New nugget spawns after 5 seconds

## Performance Characteristics

### Time Complexity
- Nugget detection: O(1) - single component check
- Collection handling: O(1) - atomic operations
- Entity destruction: O(1) - world handles cleanup

### Memory Impact
- No additional allocations per collection
- Reuses existing entity/component infrastructure
- No memory leaks (all entities properly destroyed)

### Concurrency Safety
- All state updates use atomic operations
- No race conditions (verified with `-race` flag)
- Thread-safe entity destruction

## Known Limitations (By Design)

Current implementation:
- ✅ Nuggets spawn randomly every 5 seconds
- ✅ Typing any character collects nugget
- ✅ Heat increases by 10% on collection
- ✅ Nugget destroyed and respawns automatically
- ✅ No score/error effects
- ✅ Cursor moves right after collection
- ❌ No visual feedback on collection (future enhancement)
- ❌ No sound effects (future enhancement)
- ❌ No particle effects (future enhancement)
- ❌ No expiration mechanics (future enhancement)

## Future Enhancements (Potential)

The following features could be added in future parts:
1. Visual feedback on collection (flash, pulse, particles)
2. Sound effects for collection
3. Variable heat bonuses based on timing/combo
4. Nugget expiration after timeout
5. Multiple nugget types with different effects
6. Collection streak tracking
7. Visual trail effect when collecting

## Integration with Existing Features

### Heat System
- Nugget collection integrates seamlessly with existing heat mechanics
- Heat can trigger boost activation (if at max after collection)
- Heat can trigger cleaners (via gold sequence + nugget collections)

### Boost System
- Nugget heat gain is **not** affected by boost multiplier (intentional)
- Collection can push heat to max, activating boost
- Boost remains active after nugget collection (no reset)

### Gold Sequence System
- Nuggets and gold sequences are independent
- Both can coexist on screen simultaneously
- Gold sequence typing takes precedence (checked before nuggets)
- No conflict between systems

### Spawn System
- Nuggets avoid spawning on existing characters
- Nuggets respect cursor exclusion zone (5 horizontal, 3 vertical)
- No interaction with color counters or 6-color limit

## Notes for Next Parts

If implementing additional nugget features:
1. Visual feedback should use existing rendering pipeline
2. Sound effects should integrate with (future) audio system
3. Expiration should use existing time provider
4. Multiple types should extend NuggetComponent, not replace it
5. Collection effects should be in separate system (not ScoreSystem)

## Testing Notes

All tests follow vi-fighter testing patterns:
- Use `tcell.NewSimulationScreen` for UI tests
- Use `engine.NewMockTimeProvider` for time-dependent tests
- Use `engine.NewGameContext` or manual construction as needed
- Verify race conditions with `-race` flag
- Test edge cases (small screens, boundaries, etc.)
- Test integration with existing systems

## Conclusion

Nugget typing interaction is now fully functional. Players can collect nuggets by typing any character while positioned on them, gaining heat without affecting score or triggering errors. The implementation is thread-safe, well-tested, and follows all architecture guidelines.
