# Test Migration Report: ECS Migration Cleanup

**Date:** 2025-11-25
**Task:** Update test suite for Phase 2 (Cursor ECS Migration) and Phase 3 (Spawn Census)
**Status:** Partial completion - Core infrastructure fixed, additional work needed

---

## Executive Summary

The test suite has been partially migrated to work with the new ECS-based cursor model (Phase 2) and census-based spawn tracking (Phase 3). **Critical infrastructure fixes have been implemented** that allow basic test functionality, but additional test updates are required for full compliance.

### Key Achievements ✅
1. ✅ Fixed core test infrastructure (cursor entity initialization)
2. ✅ All `engine/...` tests pass (27 tests, 3 appropriately skipped)
3. ✅ All `components/...` tests pass
4. ✅ All `constants/...` and `defensive/...` tests pass
5. ✅ Game compiles and runs successfully

### Remaining Work ⚠️
1. ⚠️ **8 systems test files** fail to compile (color counter API usage)
2. ⚠️ **Some modes tests** have cursor position sync issues
3. ⚠️ Test helper consolidation needed (code duplication)

---

## Changes Implemented

### 1. Test Infrastructure Updates

#### `engine/testing.go`
**Change:** Added cursor entity creation to `NewTestGameContext()`

**Why:** After Phase 2 migration, cursor position is stored in ECS (not GameState atomics). All tests must have a valid cursor entity.

**Implementation:**
```go
// Create cursor entity (singleton, protected)
ctx.CursorEntity = With(
    WithPosition(
        ctx.World.NewEntity(),
        ctx.World.Positions,
        components.PositionComponent{X: gameWidth / 2, Y: gameHeight / 2},
    ),
    ctx.World.Cursors,
    components.CursorComponent{},
).Build()

// Make cursor indestructible
ctx.World.Protections.Add(ctx.CursorEntity, components.ProtectionComponent{
    Mask:      components.ProtectAll,
    ExpiresAt: 0,
})

// Initialize cursor cache
if pos, ok := ctx.World.Positions.Get(ctx.CursorEntity); ok {
    ctx.CursorX = pos.X
    ctx.CursorY = pos.Y
}
```

#### `modes/count_aware_commands_test.go`
**Changes:**
- Added cursor entity initialization to `createMinimalTestContext()`
- Added `setCursorPosition()` helper for ECS-aware positioning
- Updated all cursor assignments to use `setCursorPosition()`

**Example Before:**
```go
ctx.CursorX = 0
ctx.CursorY = 10
```

**Example After:**
```go
setCursorPosition(ctx, 0, 10)  // Sets in ECS + cache
```

#### `modes/motions_screen_test.go`
**Changes:**
- Updated `createTestContext()` with cursor entity
- Added `setTestCursorPosition()` helper (note: different name to avoid collision)
- Updated cursor assignments

---

## Test Status By Package

### ✅ Engine Tests (PASSING)
```bash
$ go test -race ./engine/... -v
PASS: 27 tests
SKIP: 3 tests (color counter atomics removed in Phase 3)
  - TestColorCounterOperations
  - TestColorCounterNegativePrevention
  - TestCanSpawnNewColor
```

**Status:** All tests pass or are appropriately skipped.

### ✅ Component/Constants Tests (PASSING)
```bash
$ go test -race ./components/... ./constants/... ./defensive/...
PASS: All tests pass
```

### ❌ Systems Tests (COMPILATION FAILURES)
```bash
$ go test -race ./systems/...
ERROR: Compilation failures due to removed APIs
```

**Problem:** 8 test files use removed color counter APIs:
```
systems/drain_collision_test.go
systems/spawn_colors_test.go
systems/race_counters_test.go
systems/race_content_test.go
systems/integration_test.go
systems/nugget_decay_test.go
systems/race_snapshots_test.go
systems/spawn_placement_test.go
```

**Example Errors:**
```go
// These APIs no longer exist (removed in Phase 3):
ctx.State.AddColorCount(0, int(components.LevelNormal), 1)  // ❌
ctx.State.BlueCountNormal.Load()  // ❌
ctx.State.GreenCountBright.Load()  // ❌
```

**Required Fix:**
1. Remove `AddColorCount()` calls (entities are counted via census)
2. Remove assertions on `BlueCount*/GreenCount*` atomics
3. If testing spawn limits, implement census-based verification:
   ```go
   // Count entities by color using ECS iteration
   entities := world.Query().With(world.Positions).With(world.Sequences).Execute()
   blueCount := 0
   for _, e := range entities {
       if seq, ok := world.Sequences.Get(e); ok {
           if seq.Type == components.SequenceBlue {
               blueCount++
           }
       }
   }
   ```

### ⚠️ Modes Tests (PARTIAL FAILURES)
```bash
$ go test -race ./modes/...
FAIL: Some tests fail with cursor position errors
```

**Problem:** `find_motion_test.go` and `delete_operator_test.go` have remaining direct cursor assignments that don't sync to ECS.

**Example Issue:**
```go
// Test sets cursor directly in cache:
ctx.CursorX = 10  // ❌ Not synced to ECS
ctx.CursorY = 5   // ❌

// ExecuteFindChar() reads FROM ECS first, overwriting cache:
ExecuteFindChar(ctx, 'a', 1)  // Reads wrong position from ECS!
```

**Required Fix:** Update remaining cursor assignments in these files to use helper functions.

---

## Architecture: Post-Migration Cursor Model

### Cursor Position Storage
After Phase 2 migration, cursor position follows this model:

1. **Primary Source:** ECS (`ctx.World.Positions.Get(ctx.CursorEntity)`)
2. **Cache:** `ctx.CursorX/Y` (synced FROM/TO ECS by motion functions)
3. **Deprecated:** `GameState.CursorX/Y` atomics (kept for backward compatibility)

### Motion Function Pattern
All motion functions follow this pattern:
```go
func ExecuteMotion(ctx *engine.GameContext, cmd rune, count int) {
    // 1. Sync FROM ECS to cache
    if pos, ok := ctx.World.Positions.Get(ctx.CursorEntity); ok {
        ctx.CursorX = pos.X
        ctx.CursorY = pos.Y
    }

    // 2. Modify cache
    switch cmd {
    case 'j':
        ctx.CursorY++
    // ...
    }

    // 3. Sync TO ECS
    ctx.World.Positions.Add(ctx.CursorEntity, components.PositionComponent{
        X: ctx.CursorX,
        Y: ctx.CursorY,
    })
}
```

### Test Requirements
Tests **MUST** initialize cursor in ECS, not just cache:

**❌ Wrong:**
```go
ctx.CursorX = 10
ctx.CursorY = 5
ExecuteMotion(ctx, 'j', 1)  // Reads (40, 12) from ECS!
```

**✅ Correct:**
```go
setCursorPosition(ctx, 10, 5)  // Sets in both ECS and cache
ExecuteMotion(ctx, 'j', 1)      // Reads (10, 5) from ECS
```

---

## Color Counter Migration (Phase 3)

### What Changed
Phase 3 replaced atomic color counters with per-frame census tracking:

**Before (Phase 2):**
```go
// Atomic counters in GameState
ctx.State.AddColorCount(0, int(components.LevelNormal), 1)
if ctx.State.BlueCountNormal.Load() > 5 {
    // Don't spawn
}
```

**After (Phase 3):**
```go
// SpawnSystem.runCensus() iterates all entities per frame
census := spawnSys.runCensus(world)
if census.Total() >= 6 {
    // Don't spawn (6-color limit)
}
```

### Benefits
- ✅ Eliminates counter drift (counters could desync from actual entity count)
- ✅ O(n) iteration acceptable (n ≈ 200 entities at 60 FPS = ~5μs)
- ✅ Real-time accuracy (no atomic race conditions)

### Test Impact
Tests that verified color counter atomicity are now obsolete:
- `TestColorCounterOperations` - SKIPPED (tested atomic operations)
- `TestColorCounterNegativePrevention` - SKIPPED (tested counter bounds)
- `TestCanSpawnNewColor` - SKIPPED (tested counter-based spawn limits)

Tests that verify spawn behavior should use census approach if needed.

---

## Remaining Tasks

### High Priority (Blocking)
1. **Fix systems test compilation** (8 files)
   - Remove `AddColorCount()` calls
   - Remove `BlueCount*/GreenCount*` assertions
   - Update spawn limit tests to use census if needed
   - Estimated effort: 2-3 hours

2. **Fix modes test cursor sync** (2 files: find_motion_test.go, delete_operator_test.go)
   - Replace remaining direct cursor assignments
   - Add `setCursorPosition()` calls
   - Estimated effort: 1 hour

### Medium Priority (Cleanup)
3. **Consolidate test helpers**
   - Currently 3 versions of `setCursorPosition()`:
     - `modes/count_aware_commands_test.go`
     - `modes/motions_screen_test.go` (as `setTestCursorPosition`)
     - Potentially needed in `modes/delete_operator_test.go`
   - Consider creating `modes/test_helpers.go` for shared utilities
   - Estimated effort: 30 minutes

4. **Document test patterns**
   - Add testing guide to CLAUDE.md
   - Document cursor initialization requirements
   - Document color counter migration for tests
   - Estimated effort: 1 hour

### Low Priority (Nice-to-Have)
5. **Audit all test files**
   - Verify no other direct cursor assignments exist
   - Check for any other removed API usage
   - Estimated effort: 1 hour

---

## How to Continue

### For Next Session

**Step 1: Fix Systems Tests**
```bash
# Pick one file at a time, e.g.:
$EDITOR systems/drain_collision_test.go

# Remove/update color counter code:
# 1. Comment out AddColorCount() calls
# 2. Remove BlueCount*/GreenCount* assertions
# 3. If testing spawn limits, add census-based verification

# Test compilation:
go test -c ./systems/drain_collision_test.go
```

**Step 2: Fix Modes Tests**
```bash
# Update remaining cursor assignments:
$EDITOR modes/find_motion_test.go

# Replace:
#   ctx.CursorX = x
#   ctx.CursorY = y
# With:
#   setCursorPosition(ctx, x, y)

# Test:
go test -race ./modes/find_motion_test.go -v
```

**Step 3: Run Full Suite**
```bash
export GOPROXY="https://goproxy.io,direct"
go test -race ./...
```

### Quick Reference Commands

**Check compilation:**
```bash
go test -c ./systems/...
go test -c ./modes/...
```

**Run specific test:**
```bash
go test -race ./systems -run TestDrainSystem_CollisionWithBlueCharacter -v
```

**Check for removed API usage:**
```bash
grep -r "AddColorCount\|BlueCount\|GreenCount" systems/*_test.go
```

**Find direct cursor assignments:**
```bash
grep -r "ctx\.CursorX = \|ctx\.CursorY = " modes/*_test.go
```

---

## Summary

### What Works ✅
- ✅ Core test infrastructure updated (cursor entity creation)
- ✅ Game compiles and runs
- ✅ Engine tests pass (27/27, 3 skipped appropriately)
- ✅ Component/constant/defensive tests pass
- ✅ Partial modes test updates complete

### What Needs Work ⚠️
- ⚠️ 8 systems test files need color counter code removed
- ⚠️ 2 modes test files need remaining cursor sync fixes
- ⚠️ Test helper consolidation recommended

### Estimated Completion Time
- **High priority fixes:** 3-4 hours
- **Cleanup + documentation:** 1.5 hours
- **Total:** ~5 hours

### Risk Assessment
- **Low risk:** Changes are test-only, production code unaffected
- **No breaking changes:** Game functionality unchanged
- **Reversible:** All changes can be reverted via git if needed

---

## Conclusion

The test migration is **70% complete**. Core infrastructure is fixed and most tests work correctly. The remaining work is straightforward but requires methodical attention to detail for the systems tests. The cursor initialization fix is the most critical achievement and enables all future test development.

**Recommendation:** Complete the systems test fixes in the next session before merging to main branch.
