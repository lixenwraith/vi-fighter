# Drain Mechanic Verification

## Summary
The Drain mechanic score draining functionality has been verified to be **fully implemented and correct**.

## Requirements Verification

### 1. Check if drain position exactly matches current cursor position ✅
**Implementation:** `systems/drain_system.go:251`
```go
isOnCursor := (drain.X == cursor.X && drain.Y == cursor.Y)
```
The check performs an exact equality comparison for both X and Y coordinates.

### 2. Only drain score when drainX == cursorX AND drainY == cursorY ✅
**Implementation:** `systems/drain_system.go:260`
```go
if isOnCursor {
    // Draining logic only executes when condition is true
}
```
The draining code block is guarded by the `isOnCursor` condition.

### 3. Use separate timer for draining (drain.LastDrainTime) ✅
**Implementation:** `systems/drain_system.go:262,267`
```go
if now.Sub(drain.LastDrainTime) >= constants.DrainScoreDrainInterval {
    // ... drain score ...
    drain.LastDrainTime = now
}
```
The `drain.LastDrainTime` field is used independently from `drain.LastMoveTime`.

### 4. Drain exactly 10 score per DrainScoreDrainIntervalMs when on cursor ✅
**Implementation:** `systems/drain_system.go:264` + `constants/ui.go:89`
```go
s.ctx.State.AddScore(-constants.DrainScoreDrainAmount)  // -10 points
```
where `DrainScoreDrainAmount = 10` and `DrainScoreDrainIntervalMs = 1000`.

### 5. When cursor moves away, draining must stop immediately until drain catches up ✅
**Implementation:** `systems/drain_system.go:251`
The `isOnCursor` check is recalculated every frame:
- When cursor moves away: `isOnCursor` becomes `false`, draining stops
- When drain catches up: `isOnCursor` becomes `true`, draining resumes

## Test Coverage
All existing drain system tests pass:
- `TestDrainSystem_ScoreDrainWhenOnCursor` - Validates draining when on cursor
- `TestDrainSystem_NoDrainWhenNotOnCursor` - Validates no draining when off cursor
- `TestDrainSystem_IsOnCursorStateTracking` - Validates state tracking
- `TestDrainSystem_MultipleDrainTicks` - Validates multiple drain cycles
- `TestDrainSystem_NoDrainBeforeInterval` - Validates timing requirements
- `TestDrainSystem_ScoreDrainDespawnAtZero` - Validates lifecycle at zero score
- `TestDrainSystem_LastDrainTimeUpdated` - Validates timer updates

## Conclusion
The Drain mechanic implementation in `systems/drain_system.go` correctly implements all required functionality for score draining. No code changes are needed.

**Status:** ✅ VERIFIED CORRECT
**Date:** 2025-11-20
**Verified by:** Claude (Sonnet 4.5)
