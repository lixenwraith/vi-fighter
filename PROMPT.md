# Prompt: Implement Fixed-Capacity Dense Grid Spatial Index for vi-fighter ECS

## Role
Principal Software Architect specializing in Go, ECS game engine design, and high-performance zero-allocation patterns.

## Project Context

**vi-fighter**: Terminal-based typing game using custom Generics-based ECS in Go 1.25+.

**Codebase Structure** (relevant files):
- `engine/position_store.go` - Current spatial index (REPLACE)
- `engine/world.go` - ECS World, entity lifecycle, system orchestration
- `engine/store.go` - Generic component stores
- `systems/decay.go` - FallingDecay entities, uses `GetEntityAt()` for collision
- `systems/drain.go` - Drain entity, uses `GetEntityAt()` for collision
- `systems/cleaner.go` - Cleaner entities, uses `GetEntityAt()` for collision
- `systems/spawn.go` - Character spawning, uses `PositionStore.Add()` and batch API
- `render/terminal_renderer.go` - Iterates entities for drawing
- `components/*.go` - Component definitions

**Platform**: FreeBSD 14.3 (prod), Arch Linux (dev). Go 1.25+, stdlib-first, tcell/v2, beep.

---

## The Bug: Spatial Index Ghosting

### Root Cause
Current `PositionStore` allows only ONE entity per cell. When cursor moves onto an occupied cell:
```go
// position_store.go - Add()
ps.spatialIndex[pos.Y][pos.X] = e  // UNCONDITIONAL OVERWRITE
```

1. Cursor overwrites spatial index entry
2. Original entity retains `PositionComponent` but loses spatial index presence
3. Entity becomes "orphaned" - exists in component stores, invisible to `GetEntityAt()`
4. Systems using `GetEntityAt()` (Decay, Drain, Cleaner) skip orphaned entities

### Reproduction
```go
func TestCursorGhostingCharacter(t *testing.T) {
    ps := NewPositionStore()
    
    charEntity := Entity(100)
    ps.Add(charEntity, PositionComponent{X: 5, Y: 5})
    
    cursorEntity := Entity(999)
    ps.Add(cursorEntity, PositionComponent{X: 0, Y: 0})
    
    // Cursor moves onto character
    ps.Add(cursorEntity, PositionComponent{X: 5, Y: 5})
    
    // Cursor moves away
    ps.Add(cursorEntity, PositionComponent{X: 6, Y: 5})
    
    // BUG: Character at (5,5) is now invisible to spatial queries
    found := ps.GetEntityAt(5, 5)  // Returns 0, should return charEntity
}
```

---

## Performance Requirements: "Super-Boost" Scenario

The solution must handle this future game state at 60fps with zero GC pressure:
```
- Cursor has transparent gradient shield (5-char radius, ~80 cells)
- Shield has magnetic effect on decay entities causing orbital "whirlwind"
- Orbital movement is chaotic: varying speeds, elliptical paths, some far/some close
- Decay entities continue interacting with characters during orbit
- 10 super-drains (3x3 entities each = 9 cells per drain = 90 cell writes)
- Super-drains move, collide, destroy each other, respawn
- Spawn animation: 4 cleaner-like lines converge to spawn location
- Concurrent: normal spawning, decay, cleaners, gold sequences, nuggets
- Taking nugget shoots cleaners in 4 cardinal directions
- Everything runs at 2x normal speed
```

**Rejected Approaches** (insufficient performance):
- `map[int]map[int][]Entity` - GC pressure from slice allocations
- `map[int]map[int]map[Entity]struct{}` - Hash overhead, poor cache locality
- Layer slots (1 entity/layer/cell) - Cannot handle multiple decay in same cell during whirlwind

---

## Chosen Solution: Fixed-Capacity Dense Grid

### Data Structure
```go
package engine

const (
    MaxEntitiesPerCell = 16  // Expandable constant
    DefaultGridWidth   = 200 // Max terminal width
    DefaultGridHeight  = 60  // Max terminal height
)

// Cell is a value type - contiguous in memory, no pointers
type Cell struct {
    Entities [MaxEntitiesPerCell]Entity
    Count    uint8
    // Padding for cache alignment if needed
}

type SpatialGrid struct {
    Width, Height int
    Cells         []Cell  // 1D array: index = y*Width + x
    // Memory: 200*60*(16*8+1) ≈ 1.5MB - fits in L3 cache
}
```

### Core Operations
```go
// Add: O(1) - No allocation
func (g *SpatialGrid) Add(e Entity, x, y int) {
    if x < 0 || x >= g.Width || y < 0 || y >= g.Height {
        return
    }
    cell := &g.Cells[y*g.Width+x]
    if cell.Count < MaxEntitiesPerCell {
        cell.Entities[cell.Count] = e
        cell.Count++
    }
    // Soft clip if full - preferable to heap allocation
}

// Remove: O(k) where k ≤ 16 - Swap-remove pattern
func (g *SpatialGrid) Remove(e Entity, x, y int) {
    if x < 0 || x >= g.Width || y < 0 || y >= g.Height {
        return
    }
    cell := &g.Cells[y*g.Width+x]
    for i := uint8(0); i < cell.Count; i++ {
        if cell.Entities[i] == e {
            cell.Count--
            cell.Entities[i] = cell.Entities[cell.Count]
            cell.Entities[cell.Count] = 0
            return
        }
    }
}

// GetAllAt: O(1) access + returns slice view (no copy)
func (g *SpatialGrid) GetAllAt(x, y int) []Entity {
    if x < 0 || x >= g.Width || y < 0 || y >= g.Height {
        return nil
    }
    cell := &g.Cells[y*g.Width+x]
    return cell.Entities[:cell.Count]
}

// HasAny: O(1) - Fast empty check
func (g *SpatialGrid) HasAny(x, y int) bool {
    if x < 0 || x >= g.Width || y < 0 || y >= g.Height {
        return false
    }
    return g.Cells[y*g.Width+x].Count > 0
}
```

### Z-Index Priority for Rendering
```go
// Priority constants (higher = rendered on top)
const (
    ZIndexSpawnChar = 0
    ZIndexNugget    = 100
    ZIndexDecay     = 200
    ZIndexShield    = 300
    ZIndexDrain     = 400
    ZIndexCleaner   = 500
    ZIndexCursor    = 1000
)

// Derive Z-index from entity's components
func GetZIndex(world *World, e Entity) int {
    if world.Cursors.Has(e)       { return ZIndexCursor }
    if world.Cleaners.Has(e)      { return ZIndexCleaner }
    if world.Drains.Has(e)        { return ZIndexDrain }
    if world.FallingDecays.Has(e) { return ZIndexDecay }
    if world.Nuggets.Has(e)       { return ZIndexNugget }
    return ZIndexSpawnChar
}

// GetTopEntityAt: Returns highest Z-index entity at position
func (g *SpatialGrid) GetTopEntityAt(x, y int, world *World) Entity {
    entities := g.GetAllAt(x, y)
    if len(entities) == 0 {
        return 0
    }
    
    var top Entity
    maxZ := -1
    for _, e := range entities {
        z := GetZIndex(world, e)
        if z > maxZ {
            maxZ = z
            top = e
        }
    }
    return top
}
```

---

## Integration Requirements

### 1. Replace PositionStore

Current `PositionStore` maintains dual storage:
- `components map[Entity]PositionComponent` - Entity → Position lookup
- `spatialIndex map[int]map[int]Entity` - Position → Entity lookup

New design:
- Keep `components` map for `Get(entity)` queries
- Replace `spatialIndex` with `SpatialGrid`
- `Add()` updates both
- `Remove()` updates both

### 2. Update Collision Detection in Systems

**DecaySystem** (`updateFallingEntities`):
```go
// BEFORE
targetEntity := world.Positions.GetEntityAt(col, row)
if targetEntity != 0 { ... }

// AFTER - Must handle multiple entities
entities := world.Positions.GetAllAt(col, row)
for _, targetEntity := range entities {
    // Skip protected, skip already-decayed-this-frame, etc.
}
```

**DrainSystem** (`handleCollisions`, `updateDrainMovement`):
```go
// BEFORE
collidingEntity := world.Positions.GetEntityAt(newX, newY)

// AFTER
entities := world.Positions.GetAllAt(newX, newY)
for _, collidingEntity := range entities {
    if collidingEntity != drainEntity && collidingEntity != cursorEntity {
        s.handleCollisionAtPosition(world, collidingEntity)
    }
}
```

**CleanerSystem** (`checkAndDestroyAtPosition`):
```go
// BEFORE
targetEntity := world.Positions.GetEntityAt(x, y)

// AFTER
entities := world.Positions.GetAllAt(x, y)
for _, targetEntity := range entities {
    if seqComp, ok := world.Sequences.Get(targetEntity); ok {
        if seqComp.Type == components.SequenceRed {
            cs.spawnRemovalFlash(world, targetEntity)
            world.DestroyEntity(targetEntity)
        }
    }
}
```

### 3. Update SpawnSystem Collision Check
```go
// BEFORE
if world.Positions.GetEntityAt(startCol+i, row) != 0 {
    hasOverlap = true
}

// AFTER - Any entity at position = overlap
if world.Positions.HasAny(startCol+i, row) {
    hasOverlap = true
}
```

### 4. Batch API Compatibility

Current `PositionBatch` validates collisions before commit. Update to use `HasAny()`:
```go
func (pb *PositionBatch) Commit() error {
    // Check collisions using HasAny instead of single-entity check
    for _, add := range pb.additions {
        if pb.store.grid.HasAny(add.pos.X, add.pos.Y) {
            // Check if it's the same entity (update case)
            // vs different entity (collision)
        }
    }
    // ...
}
```

### 5. Renderer Updates

Current renderer iterates component stores. For cursor overlay logic:
```go
// render/terminal_renderer.go - drawCursorCell()
// Currently does manual iteration to find "masked" entities
// Can now use GetAllAt() and Z-index sorting
```

### 6. FallingDecay / Cleaner Consideration

These currently use private position fields (`Column`, `YPosition`, `PreciseX`, etc.) and are NOT in PositionStore.

**Decision needed**:
- Option A: Keep separate, add to SpatialGrid only for collision detection
- Option B: Migrate to PositionComponent + SpatialGrid fully

Recommendation: Option A for now - add to grid, keep own position fields for physics.

---

## API Surface
```go
// SpatialGrid methods
func NewSpatialGrid(width, height int) *SpatialGrid
func (g *SpatialGrid) Add(e Entity, x, y int)
func (g *SpatialGrid) Remove(e Entity, x, y int)
func (g *SpatialGrid) Move(e Entity, oldX, oldY, newX, newY int)  // Convenience
func (g *SpatialGrid) GetAllAt(x, y int) []Entity                 // Slice view, no alloc
func (g *SpatialGrid) HasAny(x, y int) bool
func (g *SpatialGrid) GetTopEntityAt(x, y int, world *World) Entity
func (g *SpatialGrid) Clear()                                     // Reset all cells
func (g *SpatialGrid) Resize(newWidth, newHeight int)             // Terminal resize

// Updated PositionStore (wraps SpatialGrid + component map)
func (ps *PositionStore) Add(e Entity, pos PositionComponent)     // Updates both
func (ps *PositionStore) Remove(e Entity)                         // Updates both
func (ps *PositionStore) Get(e Entity) (PositionComponent, bool)  // Component lookup
func (ps *PositionStore) Has(e Entity) bool
func (ps *PositionStore) GetAllAt(x, y int) []Entity              // Delegates to grid
func (ps *PositionStore) HasAny(x, y int) bool                    // Delegates to grid
func (ps *PositionStore) GetEntityAt(x, y int) Entity             // DEPRECATED or GetTopEntityAt
```

---

## Testing Requirements

1. **Unit Tests** (`engine/spatial_grid_test.go`):
    - Multi-entity add/remove at same cell
    - Capacity limit (17th entity soft-clipped)
    - Move operation correctness
    - Bounds checking
    - Concurrent access safety (if applicable)

2. **Regression Test** (ghosting bug):
    - Cursor moves onto occupied cell
    - Cursor moves away
    - Original entity remains queryable

3. **Integration Tests**:
    - Decay hits multiple stacked entities
    - Drain collision with overlapping entities
    - Cleaner sweeps through stacked entities

4. **Benchmark**:
    - 1000 entities, rapid add/remove cycles
    - Compare GC pauses vs old implementation

---

## Deliverables

1. `engine/spatial_grid.go` - New SpatialGrid implementation
2. `engine/position_store.go` - Updated to use SpatialGrid
3. `engine/z_index.go` - Z-index constants and helper
4. System updates: `decay.go`, `drain.go`, `cleaner.go`, `spawn.go`
5. Renderer updates if needed
6. Tests for all new functionality
7. Migration notes for any breaking API changes

---

## Constraints

- Zero heap allocations in hot paths (Add/Remove/GetAllAt)
- No external dependencies
- Maintain backward compatibility where possible (`GetEntityAt` can delegate to `GetTopEntityAt`)
- Thread-safety: Current code uses per-store mutexes; maintain same pattern

---

## Consideration

- Move GetTopEntityAt logic out of SpatialGrid. Instead, create a standalone helper or a method on PositionStore.
- The SpatialGrid is a low-level container and should not know about the World or Component Stores. Mixing high-level logic (Z-Index resolution) into the low-level storage struct creates tight coupling.

```go
// REMOVE THIS METHOD FROM SPATIAL GRID
- func (g *SpatialGrid) GetTopEntityAt(x, y int, world *World) Entity { ... }

// engine/z_index.go

// ADD THIS STANDALONE HELPER INSTEAD
// Selects the highest priority entity from a raw slice
+ func SelectTopEntity(entities []Entity, world *World) Entity {
+    if len(entities) == 0 {
+        return 0
+    }
+    var top Entity
+    maxZ := -1
+    for _, e := range entities {
+        z := GetZIndex(world, e)
+        if z > maxZ {
+            maxZ = z
+            top = e
+        }
+    }
+    return top
+ }
```
