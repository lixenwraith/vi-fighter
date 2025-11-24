# Pure ECS Cursor Migration

## Overview
Migration from hybrid Atomic/ECS to Pure ECS model for cursor and spawn population tracking.

## Phase Status
- [x] Phase 1: Infrastructure & Protection
- [ ] Phase 2: Cursor Migration
- [ ] Phase 3: Spawn Census

## Architectural Decisions
| Decision | Rationale | Date |
|----------|-----------|------|
| ProtectionFlags bitmask | Extensible immunity system for future Shield/Orbit mechanics | 2025-11-24 |

## Implementation Notes

### Phase 1 Complete (2025-11-24)
- Created `components/cursor.go` with CursorComponent (ErrorFlashEnd, HeatDisplay)
- Created `components/protection.go` with ProtectionComponent and ProtectionFlags bitmask
- Updated `engine/ecs.go` to add Cursors and Protections stores to World struct
- Modified `World.DestroyEntity()` to check ProtectionFlags.ProtectAll before destruction
- All stores registered in allStores slice for proper lifecycle management
- All existing tests pass (engine/... and components/...)
- Project builds successfully

## Breaking Changes Log
(To be updated during Phase 2)
