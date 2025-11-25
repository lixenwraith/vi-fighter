# Pure ECS Cursor Migration

## Overview
Migration from hybrid Atomic/ECS to Pure ECS model for cursor and spawn population tracking.

## Phase Status
- [x] Phase 1: Infrastructure & Protection
- [x] Phase 2: Cursor Migration
- [x] Phase 3: Spawn Census

## Architectural Decisions
| Decision | Rationale | Date |
|----------|-----------|------|
| ProtectionFlags bitmask | Extensible immunity system for future Shield/Orbit mechanics | 2025-11-24 |
| Per-frame census over atomic counters | Eliminates counter drift, O(n) with n≈200 is negligible at 60FPS | 2025-11-25 |

## Implementation Notes

### Phase 1 Complete (2025-11-24)
- Created `components/cursor.go` with CursorComponent (ErrorFlashEnd, HeatDisplay)
- Created `components/protection.go` with ProtectionComponent and ProtectionFlags bitmask
- Updated `engine/ecs.go` to add Cursors and Protections stores to World struct
- Modified `World.DestroyEntity()` to check ProtectionFlags.ProtectAll before destruction
- All stores registered in allStores slice for proper lifecycle management
- All existing tests pass (engine/... and components/...)
- Project builds successfully

### Phase 2 Complete (2025-11-25)
- Migrated cursor position from GameState atomics to ECS PositionComponent
- Updated InputHandler to write directly to ECS (0-latency cursor movement)
- Modified all systems to read cursor position from ECS via ctx.CursorEntity
- Removed CursorX/CursorY atomic fields from GameState
- Project builds successfully with cursor operating purely in ECS

### Phase 3 Complete (2025-11-25)
- Implemented ColorCensus struct with Total() and ActiveColors() methods
- Added SpawnSystem.runCensus() method for per-frame entity iteration
- Added SpawnSystem.getAvailableColorsFromCensus() to replace atomic counter checks
- Updated SpawnSystem.Update() and spawnSequence() to use census
- Removed SpawnSystem.AddColorCount() method and all counter update calls
- Removed counter updates from ScoreSystem.HandleCharacterTyping()
- Removed counter updates from DecaySystem.applyDecayToCharacter()
- Removed counter updates from DrainSystem.handleCharacterCollision()
- Removed all color counter atomics from GameState:
  - BlueCountBright, BlueCountNormal, BlueDark
  - GreenCountBright, GreenCountNormal, GreenCountDark
- Removed GameState.AddColorCount(), ReadColorCounts(), GetTotalColorCount(), CanSpawnNewColor()
- Removed ColorCountSnapshot type
- 6-color spawn limit now enforced via real-time census without counter drift

## Breaking Changes Log

### Phase 3: Spawn Census (2025-11-25)
- **REMOVED**: All color counter atomic fields from GameState
  - `BlueCountBright`, `BlueCountNormal`, `BlueCountDark`
  - `GreenCountBright`, `GreenCountNormal`, `GreenCountDark`
- **REMOVED**: `GameState.AddColorCount(seqType, seqLevel int, delta int)`
- **REMOVED**: `GameState.ReadColorCounts() ColorCountSnapshot`
- **REMOVED**: `GameState.GetTotalColorCount() int`
- **REMOVED**: `GameState.CanSpawnNewColor() bool`
- **REMOVED**: `ColorCountSnapshot` type
- **REMOVED**: `SpawnSystem.AddColorCount()` method
- **CHANGED**: SpawnSystem now uses per-frame census instead of counters
- **CHANGED**: ScoreSystem no longer updates color counters on entity destruction
- **CHANGED**: DecaySystem no longer updates color counters on level/color transitions
- **CHANGED**: DrainSystem no longer updates color counters on entity destruction

### Phase 2: Cursor Migration (2025-11-25)
- **REMOVED**: `GameState.CursorX` and `GameState.CursorY` atomic fields
- **REMOVED**: `GameState.SetCursorX()` and `GameState.SetCursorY()` methods
- **REMOVED**: `GameState.GetCursorX()` and `GameState.GetCursorY()` methods
- **CHANGED**: Cursor position now stored in ECS via `ctx.CursorEntity`
- **CHANGED**: InputHandler writes directly to ECS PositionStore

## Testing Checklist

### Phase 3 Verification
- [x] Project compiles without errors
- [ ] Game starts and runs at 60FPS
- [ ] New sequences spawn when colors are cleared
- [ ] 6-color limit enforced (max 6 color/level combinations)
- [ ] Decay transitions work correctly
- [ ] No counter drift after extended play
- [ ] All unit tests pass
