# Bi-Directional Composite Entity System (BCES) — Technical Design Document

## Executive Summary

The Bi-Directional Composite Entity System replaces the legacy linear `SequenceComponent` architecture with a flexible, high-performance grouping mechanism for game entities. The system enables complex entity formations (circles, shields, bosses) while maintaining O(1) relationship resolution and cache-friendly iteration patterns.

---

## 1. Problem Statement

### 1.1 Legacy Architecture Limitations

The original `SequenceComponent` implementation suffered from several fundamental constraints:

```go
// Legacy: Each entity redundantly stores group metadata
type SequenceComponent struct {
    ID    int           // Duplicated across all members
    Index int           // Position in sequence
    Type  SequenceType  // Duplicated across all members
    Level SequenceLevel // Duplicated across all members
}
```

**Identified Issues:**

1. **Linear Constraint:** Sequences were restricted to horizontal arrangements. A 10-character Gold sequence occupied positions `(x, y)` through `(x+9, y)`. No support for vertical, diagonal, circular, or arbitrary formations.

2. **Data Redundancy:** For a 10-entity Gold sequence, `Type` and `Level` were stored 10 times. With future 50-entity Boss composites, this redundancy compounds.

3. **O(n) Group Queries:** Finding all members of a sequence required iterating the entire `SequenceStore`. No direct parent→child or child→parent relationship existed.

4. **Head Instability ("Shuffling Head" Problem):** If the leftmost entity of a sequence was destroyed, no mechanism existed to reassign leadership. The sequence became orphaned or required expensive re-election logic.

5. **No Lifecycle Controller:** Sequences lacked a persistent controller entity. Destruction of any member could corrupt group state.

### 1.2 Target Use Cases

The new architecture must support:

| Use Case | Entity Count | Formation | Typing Order | Special Behavior |
|----------|--------------|-----------|--------------|------------------|
| Gold Sequence | 10 | Horizontal line | Left-to-right strict | Timer, heat bonus |
| Bubble | 40-50 | Circle | Any order | Pop animation |
| Boss | 50-80 | Multi-layer | Core ordered, Shield any | Shield break → scatter |
| Spawned Block | 5-20 | Multi-line | Left-to-right per line | Color/level uniform |

---

## 2. Architectural Decisions

### 2.1 The Phantom Head Pattern

**Decision:** Every composite entity group is anchored by an invisible "Phantom Head" controller entity.

**Rationale:**

The Phantom Head solves the Shuffling Head problem by separating lifecycle control from visual/interactive entities. When a player destroys the leftmost visible character, the Phantom Head persists unchanged.

```
Before (Legacy):
  [A] - [B] - [C] - [D]    ← Destroy A, who is the new head?
   ^
   Head

After (BCES):
  (Phantom) - [A] - [B] - [C] - [D]    ← Destroy A, Phantom remains
      ^
      Head (invisible, protected)
```

**Properties of Phantom Head:**

| Property | Value | Rationale |
|----------|-------|-----------|
| `PositionComponent` | Yes | Defines anchor point for offset calculations |
| `CharacterComponent` | No | Invisible to renderer |
| `TypeableComponent` | No | Cannot be typed |
| `ProtectionComponent` | `ProtectAll` | Cannot be destroyed except by `CompositeSystem` |
| `CompositeHeaderComponent` | Yes | Stores group metadata and member list |

**Lifecycle Ownership:**

The `CompositeSystem` is the sole authority for Phantom Head destruction. It triggers destruction only when:
- All members are tombstoned (empty group)
- A behavior-specific completion condition is met (e.g., Gold timeout)
- An explicit `DestroyComposite()` call is made

### 2.2 Bi-Directional Linkage

**Decision:** Implement dual-direction entity relationships for O(1) resolution in both directions.

**Structure:**

```
                    ┌─────────────────────────────────────────┐
                    │        CompositeHeaderComponent         │
                    │  ┌─────────────────────────────────┐    │
                    │  │ Members: []MemberEntry          │    │
                    │  │   [0]: Entity=5, Offset=(0,0)   │───────► Entity 5
                    │  │   [1]: Entity=6, Offset=(1,0)   │───────► Entity 6
                    │  │   [2]: Entity=7, Offset=(2,0)   │───────► Entity 7
                    │  └─────────────────────────────────┘    │
                    └─────────────────────────────────────────┘
                                        ▲
                                        │ AnchorID
                    ┌───────────────────┼───────────────────┐
                    │                   │                   │
              ┌─────┴─────┐       ┌─────┴─────┐       ┌─────┴─────┐
              │ Entity 5  │       │ Entity 6  │       │ Entity 7  │
              │ Member    │       │ Member    │       │ Member    │
              │ Component │       │ Component │       │ Component │
              └───────────┘       └───────────┘       └───────────┘
```

**Resolution Paths:**

| Starting Point | Target | Method | Complexity |
|----------------|--------|--------|------------|
| Anchor → Members | Iterate `Members[]` slice | O(n) but cache-friendly |
| Member → Anchor | Read `MemberComponent.AnchorID` | O(1) |
| Member → Siblings | Anchor lookup, then iterate | O(1) + O(n) |

**Why Not Map-Based?**

A `map[core.Entity]MemberEntry` within the header would provide O(1) member lookup, but:
- Map iteration is non-deterministic (random order)
- Map hashing overhead is 50-100x slower than slice index for small N
- Maps defeat CPU cache prefetching; slices enable sequential memory access

For typical composite sizes (10-80 entities), contiguous slice iteration completes in <2μs.

### 2.3 Fixed-Point Sub-Pixel Movement (16.16 Arithmetic)

**Decision:** Movement velocity and accumulation use 32-bit integers with implicit 16-bit fractional precision.

**Rationale:**

Floating-point arithmetic introduces:
- Non-deterministic rounding across platforms
- Gradual drift accumulation over long sessions
- Slower execution on some architectures

Fixed-point provides:
- Bit-exact determinism (critical for potential network sync)
- Zero drift (fractional bits preserved exactly)
- Faster execution (bitwise operations vs. FPU)

**Representation:**

```
  Integer Part (16 bits)     Fractional Part (16 bits)
┌─────────────────────────┬─────────────────────────┐
│      Whole Cells        │    Sub-cell Position    │
└─────────────────────────┴─────────────────────────┘

1.0 velocity = 1 << 16 = 65536
0.5 velocity = 32768
0.25 velocity = 16384
```

**Integration Algorithm:**

```go
// Per tick:
header.AccX += header.VelX
header.AccY += header.VelY

// Extract integer movement
deltaX := int(header.AccX >> 16)  // Equivalent to / 65536
deltaY := int(header.AccY >> 16)

// Preserve fractional remainder
header.AccX &= 0xFFFF  // Equivalent to % 65536
header.AccY &= 0xFFFF

// Apply integer delta to PositionComponent
if deltaX != 0 || deltaY != 0 {
    anchorPos.X += deltaX
    anchorPos.Y += deltaY
}
```

**Example: Slow Drift**

A composite with `VelX = 3277` (0.05 cells/tick) at 20 ticks/second:
- Tick 1: AccX = 3277 → deltaX = 0 (3277 < 65536)
- Tick 20: AccX = 65540 → deltaX = 1, AccX = 4
- Result: Moves 1 cell every 20 ticks (1 cell/second)

### 2.4 Spatial Ordering Heuristics

**Decision:** Typing order is determined at query time via spatial heuristics, not stored indices.

**Rationale:**

Stored indices (`SequenceComponent.Index`) fail for:
- Circular formations (no "first" position)
- Dynamic formations (rotating shields)
- Partial destruction (index gaps)

Spatial heuristics provide:
- Formation-agnostic ordering
- Automatic adaptation to member death
- Behavior-specific customization

**Ordering Contract:**

```
Primary:   X ascending (left-to-right)
Secondary: Y ascending (top-to-bottom)
Tertiary:  EntityID ascending (deterministic tie-breaker)
```

**Behavior-Specific Overrides:**

| BehaviorID | Ordering Rule |
|------------|---------------|
| `BehaviorGold` | Strict left-to-right (X→Y→ID) |
| `BehaviorBubble` | Any member valid (no ordering) |
| `BehaviorBoss` | Core layer: X→Y→ID; Shield layer: any |

**Implementation:**

```go
func (s *TypingSystem) isLeftmostMember(entity core.Entity, header *CompositeHeaderComponent) bool {
    candidates := make([]candidate, 0, len(header.Members))
    
    for _, m := range header.Members {
        if m.Entity == 0 { continue }  // Skip tombstones
        pos, ok := s.world.Positions.Get(m.Entity)
        if !ok { continue }
        candidates = append(candidates, candidate{m.Entity, pos.X, pos.Y})
    }
    
    sort.Slice(candidates, func(i, j int) bool {
        if candidates[i].x != candidates[j].x { return candidates[i].x < candidates[j].x }
        if candidates[i].y != candidates[j].y { return candidates[i].y < candidates[j].y }
        return candidates[i].entity < candidates[j].entity
    })
    
    return len(candidates) > 0 && candidates[0].entity == entity
}
```

### 2.5 Tombstone Pattern for Member Death

**Decision:** Dead members are marked with `Entity = 0` (tombstone) rather than immediate slice removal.

**Rationale:**

Immediate removal via `append(slice[:i], slice[i+1:]...)` is O(n) and:
- Invalidates any held indices
- Causes memory copying on every death
- Breaks iteration if performed mid-loop

Tombstoning provides:
- O(1) death marking
- Safe mid-iteration modification
- Deferred compaction batching

**Lifecycle Flow:**

```
1. Member dies (Decay hit, Typing, etc.)
   └─► CompositeSystem detects via liveness check
       └─► Sets Members[i].Entity = 0
       └─► Sets header.Dirty = true

2. CompositeSystem.Update() [Priority 14]
   └─► If header.Dirty:
       └─► Compact: remove tombstones via swap-remove
       └─► Reset header.Dirty = false
```

**Compaction Algorithm:**

```go
func (s *CompositeSystem) compactMembers(header *CompositeHeaderComponent) {
    write := 0
    for read := 0; read < len(header.Members); read++ {
        if header.Members[read].Entity != 0 {
            if write != read {
                header.Members[write] = header.Members[read]
            }
            write++
        }
    }
    header.Members = header.Members[:write]
}
```

### 2.6 Liveness Validation (vs. Event-Driven Detection)

**Decision:** Member death is detected via liveness checks during sync iteration, not via event subscription.

**Rationale:**

Event-driven approach (`CompositeSystem` subscribes to `EventRequestDeath`):
- Creates O(E) event overhead where E = death events per tick
- During mass-clearing (Cleaner sweep of 200 entities), causes event queue saturation
- Requires cross-referencing every death against every composite

Liveness validation:
- Already iterates members for position sync
- Adds single `Has()` check per member (O(1))
- Naturally discovers deaths from any source

**Implementation:**

```go
func (s *CompositeSystem) syncMembers(header *CompositeHeaderComponent, anchorX, anchorY int) {
    for i := range header.Members {
        member := &header.Members[i]
        
        if member.Entity == 0 { continue }  // Already tombstoned
        
        // Liveness check: if entity no longer has position, it was destroyed
        if !s.world.Positions.Has(member.Entity) {
            member.Entity = 0  // Tombstone
            header.Dirty = true
            continue
        }
        
        // Position propagation...
    }
}
```

---

## 3. Data Structures

### 3.1 CompositeHeaderComponent

Resides on the Phantom Head entity. Single source of truth for group state.

```go
type CompositeHeaderComponent struct {
    GroupID    uint64             // Unique identifier for this composite
    BehaviorID BehaviorID         // Routing: Gold, Boss, Bubble, Shield
    
    Members []MemberEntry         // Contiguous slice of member references
    
    // Fixed-Point (16.16) sub-pixel movement
    VelX, VelY int32              // Velocity in fixed-point units per tick
    AccX, AccY int32              // Fractional accumulation
    
    // Hierarchy support (Phase 4)
    ParentAnchor core.Entity      // 0 if root composite
    
    // Lazy compaction trigger
    Dirty bool
}
```

**Field Details:**

| Field | Type | Purpose |
|-------|------|---------|
| `GroupID` | `uint64` | Monotonic ID from `GameState.NextID`. Used for event correlation and debugging. |
| `BehaviorID` | `BehaviorID` | Enum routing events to behavior-specific systems (Gold, Boss, etc.) |
| `Members` | `[]MemberEntry` | Pre-allocated slice. Initial capacity matches expected member count. |
| `VelX/VelY` | `int32` | 16.16 fixed-point velocity. `65536` = 1 cell/tick. |
| `AccX/AccY` | `int32` | Accumulated fractional movement. Integer part extracted each tick. |
| `ParentAnchor` | `core.Entity` | For nested composites (Boss with Shield sub-composite). 0 = root. |
| `Dirty` | `bool` | Set when tombstone created. Triggers compaction next tick. |

### 3.2 MemberEntry

Stored within `CompositeHeaderComponent.Members[]`. Describes a single member's relationship to the anchor.

```go
type MemberEntry struct {
    Entity  core.Entity  // Member entity ID. 0 = tombstone.
    OffsetX int8         // X offset from Phantom Head position
    OffsetY int8         // Y offset from Phantom Head position
    Layer   uint8        // 0=Core, 1=Shield, 2=Effect
}
```

**Field Details:**

| Field | Type | Range | Purpose |
|-------|------|-------|---------|
| `Entity` | `core.Entity` | 0 or valid ID | 0 indicates tombstone (dead member) |
| `OffsetX` | `int8` | -128 to +127 | Relative X from anchor. Sufficient for any terminal width. |
| `OffsetY` | `int8` | -128 to +127 | Relative Y from anchor. Sufficient for any terminal height. |
| `Layer` | `uint8` | 0-255 | Interaction layer. 0=typeable core, 1=protective shield, 2+=visual effects |

**Layer Semantics:**

| Layer | Name | Typeable | Purpose |
|-------|------|----------|---------|
| 0 | `LayerCore` | Yes | Primary interactive entities |
| 1 | `LayerShield` | Configurable | Protective barrier, may absorb hits |
| 2 | `LayerEffect` | No | Visual-only overlays (splash, glow) |

### 3.3 MemberComponent

Resides on member entities. Provides O(1) anchor resolution.

```go
type MemberComponent struct {
    AnchorID core.Entity  // Phantom Head entity ID
}
```

**Usage Pattern:**

```go
// From any member entity, resolve the group:
if member, ok := memberStore.Get(hitEntity); ok {
    header, _ := headerStore.Get(member.AnchorID)
    // Access group metadata, iterate siblings, etc.
}
```

### 3.4 TypeableComponent

Replaces the interaction contract previously embedded in `CharacterComponent`.

```go
type TypeableComponent struct {
    Char  rune
    Type  TypeableType   // Green, Red, Blue, Gold, Nugget
    Level TypeableLevel  // Dark, Normal, Bright
}
```

**Separation of Concerns:**

| Component | Responsibility |
|-----------|----------------|
| `CharacterComponent` | Visual rendering (glyph, color, style) |
| `TypeableComponent` | Interaction logic (matching, scoring) |
| `MemberComponent` | Group membership |

An entity may have:
- `CharacterComponent` only: Rendered but not typeable (visual effect)
- `CharacterComponent` + `TypeableComponent`: Standalone typeable
- `CharacterComponent` + `TypeableComponent` + `MemberComponent`: Composite member

### 3.5 BehaviorID Registry

Enum defining composite behavior types for event routing.

```go
type BehaviorID uint8

const (
    BehaviorNone   BehaviorID = iota
    BehaviorGold              // Gold sequence mechanics
    BehaviorBubble            // Pop-any-order mechanics
    BehaviorBoss              // Multi-layer with shield break
    BehaviorShield            // Protective barrier sub-composite
)
```

---

## 4. System Interactions

### 4.1 System Priority Chain

```
Priority  System          Responsibility
────────  ──────          ──────────────
   6      ShieldSystem    Shield drain calculations
   8      HeatSystem      Heat mutations
  10      EnergySystem    Energy mutations, delete handling
  12      BoostSystem     Boost state management
  13      TypingSystem    Character validation, order checking ← NEW
  14      CompositeSystem Position sync, liveness validation  ← NEW
  15      SpawnSystem     Entity spawning
  18      NuggetSystem    Nugget lifecycle
  20      GoldSystem      Gold composite lifecycle          ← REFACTORED
  ...
```

**Critical Ordering:**

1. **TypingSystem (13) before CompositeSystem (14):** Typing events are processed and routed before position sync. Ensures member death is detected in the same tick.

2. **CompositeSystem (14) before SpawnSystem (15):** Member positions are synchronized before spawn collision checks. Prevents "ghost collisions" against stale positions.

### 4.2 Event Flow: Character Typing

```
┌─────────────────────────────────────────────────────────────────────┐
│                         INPUT LAYER                                  │
│  User presses 'a' at cursor position (5, 3)                         │
└─────────────────────────────────────┬───────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      EventCharacterTyped                             │
│  Payload: {Char: 'a', X: 5, Y: 3}                                   │
└─────────────────────────────────────┬───────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│                       TypingSystem.HandleEvent                       │
│  1. Query entity at (5,3) via GetTopEntityFiltered                  │
│  2. Check MemberComponent presence                                   │
│     ├─ YES: Route to handleCompositeMemberTyping                    │
│     └─ NO:  Check TypeableComponent → handleTypeableTyping          │
│             or fallback to legacy SequenceComponent                  │
└─────────────────────────────────────┬───────────────────────────────┘
                                      │ (Composite path)
                                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│              handleCompositeMemberTyping                             │
│  1. Validate character match (TypeableComponent.Char == 'a')        │
│  2. Resolve anchor via MemberComponent.AnchorID                     │
│  3. Get CompositeHeaderComponent                                     │
│  4. Validate typing order via BehaviorID heuristic                  │
│     ├─ BehaviorGold: isLeftmostMember() must return true            │
│     └─ BehaviorBubble: always valid                                 │
│  5. Emit EventMemberTyped                                           │
└─────────────────────────────────────┬───────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        EventMemberTyped                              │
│  Payload: {AnchorID, MemberEntity, Char, RemainingCount}            │
└─────────────────────────────────────┬───────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│                  CompositeSystem.HandleEvent                         │
│  1. Lookup header by AnchorID                                       │
│  2. Switch on BehaviorID:                                           │
│     ├─ BehaviorGold: Emit EventGoldMemberTyped                      │
│     └─ Other: handleGenericMemberDeath                              │
└─────────────────────────────────────┬───────────────────────────────┘
                                      │ (Gold path)
                                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     EventGoldMemberTyped                             │
│  Payload: {AnchorID, MemberEntity, Char, RemainingCount}            │
└─────────────────────────────────────┬───────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    GoldSystem.handleMemberTyped                      │
│  1. Tombstone member in header                                      │
│  2. Emit EventRequestDeath for member                               │
│  3. If RemainingCount == 0:                                         │
│     ├─ Check heat for cleaner trigger                               │
│     ├─ Set heat to max                                              │
│     ├─ Emit EventGoldComplete                                       │
│     └─ Destroy composite                                            │
└─────────────────────────────────────────────────────────────────────┘
```

### 4.3 Event Flow: Composite Movement

```
┌─────────────────────────────────────────────────────────────────────┐
│                    CompositeSystem.Update()                          │
│                       [Priority 14]                                  │
└─────────────────────────────────────┬───────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│  For each anchor in headerStore.All():                              │
│                                                                      │
│  Phase 1: Fixed-Point Integration                                   │
│  ┌────────────────────────────────────────────────────────────┐     │
│  │  header.AccX += header.VelX                                │     │
│  │  header.AccY += header.VelY                                │     │
│  │  deltaX = header.AccX >> 16                                │     │
│  │  deltaY = header.AccY >> 16                                │     │
│  │  header.AccX &= 0xFFFF                                     │     │
│  │  header.AccY &= 0xFFFF                                     │     │
│  └────────────────────────────────────────────────────────────┘     │
│                                                                      │
│  Phase 2: Anchor Position Update                                    │
│  ┌────────────────────────────────────────────────────────────┐     │
│  │  if deltaX != 0 || deltaY != 0:                            │     │
│  │      anchorPos.X += deltaX                                 │     │
│  │      anchorPos.Y += deltaY                                 │     │
│  │      positions.Add(anchor, anchorPos)                      │     │
│  └────────────────────────────────────────────────────────────┘     │
│                                                                      │
│  Phase 3: Member Sync + Liveness                                    │
│  ┌────────────────────────────────────────────────────────────┐     │
│  │  for each member in header.Members:                        │     │
│  │      if member.Entity == 0: continue  // tombstone         │     │
│  │      if !positions.Has(member.Entity):                     │     │
│  │          member.Entity = 0  // died externally             │     │
│  │          header.Dirty = true                               │     │
│  │          continue                                          │     │
│  │      newX = anchorPos.X + member.OffsetX                   │     │
│  │      newY = anchorPos.Y + member.OffsetY                   │     │
│  │      positions.Move(member.Entity, {newX, newY})           │     │
│  └────────────────────────────────────────────────────────────┘     │
│                                                                      │
│  Phase 4: Lazy Compaction                                           │
│  ┌────────────────────────────────────────────────────────────┐     │
│  │  if header.Dirty:                                          │     │
│  │      compactMembers(&header)  // Remove tombstones         │     │
│  │      header.Dirty = false                                  │     │
│  └────────────────────────────────────────────────────────────┘     │
│                                                                      │
│  Phase 5: Empty Composite Handling                                  │
│  ┌────────────────────────────────────────────────────────────┐     │
│  │  if len(header.Members) == 0:                              │     │
│  │      handleEmptyComposite(anchor, &header)                 │     │
│  └────────────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 5. Migration Strategy

### 5.1 Coexistence Model

During migration, both systems operate simultaneously:

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Entity Classification                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Legacy Path (SequenceComponent):                                   │
│  ├─ SpawnSystem-created blocks (Green/Blue/Red)                     │
│  └─ Routes via EventLegacyCharacterTyped → EnergySystem             │
│                                                                      │
│  Composite Path (MemberComponent):                                  │
│  ├─ GoldSystem-created sequences (Phase 3)                          │
│  ├─ Future: NuggetSystem, BossSystem, BubbleSystem                  │
│  └─ Routes via EventMemberTyped → CompositeSystem → BehaviorSystem  │
│                                                                      │
│  Standalone Path (TypeableComponent only):                          │
│  ├─ Migrated Nuggets (Phase 3)                                      │
│  └─ Direct handling in TypingSystem                                 │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 5.2 Phase Progression

| Phase | Scope | Components Added | Systems Modified |
|-------|-------|------------------|------------------|
| 1 | Foundation | `TypeableComponent`, `MemberComponent`, `CompositeHeaderComponent` | — |
| 2 | CompositeSystem | — | New `CompositeSystem` |
| 3 | Typing + Gold | `TypingSystem` | `GoldSystem` refactored, `EnergySystem` reduced |
| 4 | Hierarchy + Scatter | — | `CompositeSystem` enhanced |

### 5.3 Detection Logic in TypingSystem

```go
func (s *TypingSystem) handleTyping(cursorX, cursorY int, typedRune rune) {
    entity := s.world.Positions.GetTopEntityFiltered(cursorX, cursorY, ...)
    
    // Priority 1: Composite member (new architecture)
    if member, ok := s.memberStore.Get(entity); ok {
        s.handleCompositeMemberTyping(entity, member.AnchorID, typedRune, ...)
        return
    }
    
    // Priority 2: Standalone TypeableComponent (migrated entities)
    if typeable, ok := s.typeableStore.Get(entity); ok {
        s.handleTypeableTyping(entity, typeable, typedRune, ...)
        return
    }
    
    // Priority 3: Legacy SequenceComponent (not yet migrated)
    if s.seqStore.Has(entity) {
        s.world.PushEvent(event.EventLegacyCharacterTyped, payload)
        return
    }
}
```

---

## 6. Implementation Details

### 6.1 Phantom Head Creation

```go
func (s *CompositeSystem) CreatePhantomHead(x, y int, groupID uint64, behaviorID BehaviorID) core.Entity {
    entity := s.world.CreateEntity()
    
    // Position at anchor point (invisible but spatially present)
    s.world.Positions.Add(entity, component.PositionComponent{X: x, Y: y})
    
    // Header with empty member slice
    s.headerStore.Add(entity, component.CompositeHeaderComponent{
        GroupID:    groupID,
        BehaviorID: behaviorID,
        Members:    make([]component.MemberEntry, 0, 16),  // Pre-allocate
    })
    
    // Immortal protection
    s.protStore.Add(entity, component.ProtectionComponent{
        Mask: component.ProtectAll,
    })
    
    return entity
}
```

### 6.2 Member Attachment

```go
func (s *CompositeSystem) AddMember(anchorID, memberEntity core.Entity, offsetX, offsetY int8, layer uint8) {
    header, ok := s.headerStore.Get(anchorID)
    if !ok { return }
    
    // Forward link: anchor → member
    header.Members = append(header.Members, component.MemberEntry{
        Entity:  memberEntity,
        OffsetX: offsetX,
        OffsetY: offsetY,
        Layer:   layer,
    })
    s.headerStore.Add(anchorID, header)
    
    // Backward link: member → anchor
    s.memberStore.Add(memberEntity, component.MemberComponent{
        AnchorID: anchorID,
    })
}
```

### 6.3 Gold Sequence Spawning (Phase 3)

```go
func (s *GoldSystem) spawnGold() bool {
    // 1. Generate sequence
    sequence := make([]rune, constant.GoldSequenceLength)
    for i := 0; i < constant.GoldSequenceLength; i++ {
        sequence[i] = constant.AlphanumericRunes[rand.Intn(len(constant.AlphanumericRunes))]
    }
    
    // 2. Find position
    x, y := s.findValidPosition(constant.GoldSequenceLength)
    if x < 0 { return false }
    
    // 3. Create Phantom Head
    groupID := uint64(s.res.State.State.IncrementID())
    anchorEntity := s.world.CreateEntity()
    s.world.Positions.Add(anchorEntity, component.PositionComponent{X: x, Y: y})
    s.protStore.Add(anchorEntity, component.ProtectionComponent{Mask: component.ProtectAll})
    
    // 4. Create member entities
    members := make([]component.MemberEntry, 0, constant.GoldSequenceLength)
    entities := make([]entityData, 0, constant.GoldSequenceLength)
    
    for i := 0; i < constant.GoldSequenceLength; i++ {
        entity := s.world.CreateEntity()
        entities = append(entities, entityData{
            entity: entity,
            pos:    component.PositionComponent{X: x + i, Y: y},
            offset: int8(i),
        })
    }
    
    // 5. Batch position commit (atomic validation)
    batch := s.world.Positions.BeginBatch()
    for _, ed := range entities {
        batch.Add(ed.entity, ed.pos)
    }
    if err := batch.Commit(); err != nil {
        // Cleanup on collision
        for _, ed := range entities {
            s.world.DestroyEntity(ed.entity)
        }
        s.world.DestroyEntity(anchorEntity)
        return false
    }
    
    // 6. Add components
    for i, ed := range entities {
        // Visual
        s.charStore.Add(ed.entity, component.CharacterComponent{
            Rune: sequence[i], SeqType: component.SequenceGold, SeqLevel: component.LevelBright,
        })
        // Interaction
        s.typeableStore.Add(ed.entity, component.TypeableComponent{
            Char: sequence[i], Type: component.TypeGold, Level: component.LevelBright,
        })
        // Membership
        s.memberStore.Add(ed.entity, component.MemberComponent{AnchorID: anchorEntity})
        // Protection
        s.protStore.Add(ed.entity, component.ProtectionComponent{
            Mask: component.ProtectFromDelete | component.ProtectFromDecay,
        })
        
        members = append(members, component.MemberEntry{
            Entity: ed.entity, OffsetX: ed.offset, OffsetY: 0, Layer: component.LayerCore,
        })
    }
    
    // 7. Create header
    s.headerStore.Add(anchorEntity, component.CompositeHeaderComponent{
        GroupID: groupID, BehaviorID: component.BehaviorGold, Members: members,
    })
    
    // 8. Emit event
    s.world.PushEvent(event.EventGoldSpawned, &event.GoldSpawnedPayload{...})
    
    return true
}
```

---

## 7. Testing Considerations

### 7.1 Unit Test Scenarios

| Scenario | Input | Expected Outcome |
|----------|-------|------------------|
| Leftmost member typed | Type 'a' at leftmost Gold char | Member destroyed, cursor moves right |
| Non-leftmost member typed | Type 'b' at middle Gold char | Error flash, no destruction |
| All members typed | Type full Gold sequence | `EventGoldComplete`, composite destroyed |
| Member killed by Decay | Decay passes through Gold char | Member tombstoned, Gold continues |
| Phantom Head direct attack | Attempt to destroy anchor | Blocked by `ProtectAll` |
| Composite timeout | Wait 10s after Gold spawn | `EventGoldTimeout`, all members destroyed |
| Mid-sequence resize | Terminal resize during Gold | Members culled, `EventGoldDestroyed` |

### 7.2 Integration Test Scenarios

| Scenario | Setup | Validation |
|----------|-------|------------|
| Legacy coexistence | Spawn Green block + Gold sequence | Both typeable, correct routing |
| Mass composite spawn | Spawn 10 Gold sequences rapidly | No ID collision, correct grouping |
| Cleaner vs composite | Cleaner sweeps through Gold | Members flash-destroyed, group ends |
| Boost + Gold | Max heat, complete Gold | Cleaner triggered, boost extended |

---

## 8. Performance Characteristics

### 8.1 Memory Layout

```
CompositeHeaderComponent (typical Gold):
├─ GroupID:    8 bytes
├─ BehaviorID: 1 byte
├─ Members:    24 bytes (slice header) + 10 × 4 bytes (entries) = 64 bytes
├─ VelX/VelY:  8 bytes
├─ AccX/AccY:  8 bytes
├─ ParentAnchor: 8 bytes
├─ Dirty:      1 byte
└─ Total:      ~98 bytes + padding ≈ 104 bytes

MemberEntry:
├─ Entity:  8 bytes
├─ OffsetX: 1 byte
├─ OffsetY: 1 byte
├─ Layer:   1 byte
└─ Total:   11 bytes + padding = 16 bytes (aligned)
```

### 8.2 Operation Costs

| Operation | Complexity | Typical Time |
|-----------|------------|--------------|
| Member → Anchor resolution | O(1) | <10ns |
| Anchor → All members iteration | O(n) | ~50ns per member |
| Position sync (n members) | O(n) | ~100ns per member |
| Typing order validation | O(n log n) | ~500ns for 10 members |
| Tombstone compaction | O(n) | ~200ns for 10 members |

### 8.3 Scalability

| Composite Count | Total Members | Sync Time per Tick |
|-----------------|---------------|-------------------|
| 1 (Gold) | 10 | ~1μs |
| 10 (Mixed) | 100 | ~10μs |
| 50 (Boss scenario) | 500 | ~50μs |
| 100 (Stress test) | 1000 | ~100μs |

Target frame budget: 16ms (60 FPS). Composite sync overhead: <1% even at 100 composites.

---

## 9. Future Extensions (Phase 4+)

### 9.1 Nested Composites

Boss with rotating shield:
```
Boss Phantom Head
├─ Core Members (10 entities, LayerCore)
└─ Shield Phantom Head (ParentAnchor = Boss)
    └─ Shield Members (20 entities, LayerShield)
```

Shield movement: `Shield.AccX += BossVel.X + ShieldRotation.X`

### 9.2 Scatter Transformation

Shield break event:
```go
func (s *CompositeSystem) scatterMembers(anchorID core.Entity, velocityBase, randomness int32) {
    header, _ := s.headerStore.Get(anchorID)
    
    for _, m := range header.Members {
        if m.Entity == 0 || m.Layer != LayerShield { continue }
        
        // Remove from composite
        s.memberStore.Remove(m.Entity)
        
        // Add decay behavior with random velocity
        s.decayStore.Add(m.Entity, component.DecayComponent{
            VelocityX: velocityBase + rand.Int31n(randomness),
            VelocityY: velocityBase + rand.Int31n(randomness),
        })
    }
    
    // Clear shield members from header
    header.Members = filterByLayer(header.Members, LayerCore)
}
```

### 9.3 Destructive Spawn (ClearFootprint)

```go
func (batch *PositionBatch) CommitWithClear() error {
    // Phase 1: Collect entities to clear
    var toClear []core.Entity
    for _, add := range batch.additions {
        existing := batch.store.GetAllAt(add.pos.X, add.pos.Y)
        toClear = append(toClear, existing...)
    }
    
    // Phase 2: Request death for existing entities
    if len(toClear) > 0 {
        batch.world.PushEvent(event.EventRequestDeath, &event.DeathRequestPayload{
            Entities: toClear, EffectEvent: event.EventFlashRequest,
        })
    }
    
    // Phase 3: Standard commit
    return batch.Commit()
}
```

---

## 10. Glossary

| Term | Definition |
|------|------------|
| **Phantom Head** | Invisible controller entity anchoring a composite group |
| **Tombstone** | Member entry with `Entity = 0`, marking a dead slot |
| **Liveness Check** | Validation that a member entity still exists in the world |
| **16.16 Fixed-Point** | Integer representation with 16 bits integer, 16 bits fraction |
| **Spatial Heuristic** | Algorithm determining typing order based on position |
| **BehaviorID** | Enum routing events to behavior-specific systems |
| **LayerCore** | Member layer 0, primary typeable entities |
| **LayerShield** | Member layer 1, protective non-typeable barrier |