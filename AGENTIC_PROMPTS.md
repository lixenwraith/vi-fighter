Based on codebase analysis, here's the high-level plan and phased prompts for Claude Code.

---

## High-Level Plan

| Phase | Focus | Files Modified | Verification |
|-------|-------|----------------|--------------|
| 1 | Constants & Component Updates | `constants/gameplay.go`, `components/shield.go` | `go build ./...` |
| 2 | Shield Lifecycle (Sources Bitmask) | `systems/boost.go`, `render/renderers/shields.go` | `go build ./...` |
| 3 | Drain Spawning & Heat-Based Lifecycle | `systems/drain.go` | `go build ./...` |
| 4 | Drain-Cursor & Drain-Drain Collisions | `systems/drain.go` | `go build ./...` |
| 5 | Shield Zone Protection | `systems/drain.go` | `go build ./...` |
| 6 | Passive Shield Drain & Final Despawn | `systems/drain.go` | `go build ./...` |

---

## Prompt 1: Constants & Components

```markdown
# Task: Phase 1 - Constants & Component Updates

## Memory File
Create file `GAME_MECHANICS_UPDATE.md` at repository root with content:

```md
# Game Mechanics Update - Implementation Tracker

## Overview
Implementing new Heat/Energy/Shield/Drain mechanics per requirements.

## Phase Status
- [x] Phase 1: Constants & Components
- [ ] Phase 2: Shield Lifecycle
- [ ] Phase 3: Drain Spawning
- [ ] Phase 4: Collisions
- [ ] Phase 5: Shield Zone Protection
- [ ] Phase 6: Passive Drain & Cleanup

## Key Mechanics Summary
- Heat: 0-100, controls drain count via floor(Heat/10)
- Energy: Can go negative, used for Shield defense
- Shield: Active when Sources != 0 AND Energy > 0
- Drains: Spawn based on Heat, despawn on Heat drop, cursor collision, or drain-drain collision

## Constants Added (Phase 1)
- DrainShieldEnergyDrainAmount = 100 (per tick per drain in shield)
- DrainHeatReductionAmount = 10 (unshielded cursor collision)
- ShieldPassiveDrainAmount = 1 (per second while active)
- ShieldPassiveDrainInterval = 1s
- ShieldSourceBoost = 1 << 0 (bitmask flag)

## Component Changes (Phase 1)
- ShieldComponent.Sources uint8 added (bitmask for activation sources)
```

## Implementation

### File: constants/gameplay.go

Add constants in the Shield section (near existing Shield constants):

```go
// Shield Defense Costs
const (
	// DrainShieldEnergyDrainAmount is energy cost per tick per drain inside shield
	DrainShieldEnergyDrainAmount = 100

	// DrainHeatReductionAmount is heat penalty when drain hits cursor without shield
	DrainHeatReductionAmount = 10

	// ShieldPassiveDrainAmount is energy cost per second while shield is active
	ShieldPassiveDrainAmount = 1

	// ShieldPassiveDrainInterval is the interval for passive shield drain
	ShieldPassiveDrainInterval = 1 * time.Second
)

// Shield Source Flags (bitmask)
const (
	ShieldSourceNone  uint8 = 0
	ShieldSourceBoost uint8 = 1 << 0
)
```

### File: components/shield.go

Update ShieldComponent to add Sources field:

```go
// ShieldComponent represents a circular/elliptical energy shield
// It is a geometric field effect that modifies visual rendering and physics interactions
// Shield is active when Sources != 0 AND Energy > 0
type ShieldComponent struct {
    Active        bool        // DEPRECATED: Will be removed in Phase 2
	Sources       uint8       // Bitmask of active sources (ShieldSourceBoost, etc)
	RadiusX       float64     // Horizontal radius in grid cells
	RadiusY       float64     // Vertical radius in grid cells
	Color         tcell.Color // Base color of the shield
	MaxOpacity    float64     // Maximum opacity at center (0.0 to 1.0)
	LastDrainTime time.Time   // Last passive drain tick (for 1/sec cost)
}
```

Ensure noting that `Active` field is the first item to be removed in Phase 2 - it is replaced by the Sources bitmask logic.

## Verification
Run: `go build ./...`

Ensure no compilation errors. The build may have errors in systems/boost.go and render/renderers/shields.go if Active is removed that will be fixed in Phase 2, otherwise should be no error.
```

---

## Prompt 2: Shield Lifecycle

```markdown
# Task: Phase 2 - Shield Lifecycle (Sources Bitmask)

## Reference
Read `GAME_MECHANICS_UPDATE.md` for context.

## Update Memory File
Update Phase 2 status to [x] in `GAME_MECHANICS_UPDATE.md`.

Add section:
```md
## Shield Lifecycle (Phase 2)
- BoostSystem now sets/clears ShieldSourceBoost in Sources bitmask
- Shield component persists; only Sources field changes
- ShieldRenderer checks IsShieldActive() before rendering
- IsShieldActive = Sources != 0 AND Energy > 0
```

## Implementation

### File: components/shield.go

Now remove the `Active` field from `ShieldComponent` as we update the consumers.

```go
package components

import (
	"time"

	"github.com/gdamore/tcell/v2"
)

// ShieldComponent represents a circular/elliptical energy shield
// It is a geometric field effect that modifies visual rendering and physics interactions
// Shield is active when Sources != 0 AND Energy > 0
type ShieldComponent struct {
	Sources       uint8       // Bitmask of active sources (ShieldSourceBoost, etc)
	RadiusX       float64     // Horizontal radius in grid cells
	RadiusY       float64     // Vertical radius in grid cells
	Color         tcell.Color // Base color of the shield
	MaxOpacity    float64     // Maximum opacity at center (0.0 to 1.0)
	LastDrainTime time.Time   // Last passive drain tick (for 1/sec cost)
}
```

### File: systems/boost.go

Refactor to manage Sources bitmask instead of add/remove ShieldComponent:

```go
package systems

import (
	"time"

	"github.com/lixenwraith/vi-fighter/components"
	"github.com/lixenwraith/vi-fighter/constants"
	"github.com/lixenwraith/vi-fighter/engine"
	"github.com/lixenwraith/vi-fighter/render"
)

type BoostSystem struct {
	ctx *engine.GameContext
}

func NewBoostSystem(ctx *engine.GameContext) *BoostSystem {
	return &BoostSystem{ctx: ctx}
}

func (bs *BoostSystem) Priority() int {
	return constants.PriorityBoost
}

// Update handles boost timer and shield Sources bitmask management
func (bs *BoostSystem) Update(world *engine.World, dt time.Duration) {
	timeRes := engine.MustGetResource[*engine.TimeResource](world.Resources)
	now := timeRes.GameTime

	bs.ctx.State.UpdateBoostTimerAtomic(now)

	boostEnabled := bs.ctx.State.GetBoostEnabled()
	cursorEntity := bs.ctx.CursorEntity

	shield, hasShield := world.Shields.Get(cursorEntity)

	if boostEnabled {
		if !hasShield {
			// Create shield with SourceBoost set
			shield = components.ShieldComponent{
				Sources:       constants.ShieldSourceBoost,
				RadiusX:       constants.ShieldRadiusX,
				RadiusY:       constants.ShieldRadiusY,
				Color:         render.RgbShieldBase,
				MaxOpacity:    constants.ShieldMaxOpacity,
				LastDrainTime: now,
			}
			world.Shields.Add(cursorEntity, shield)
		} else if shield.Sources&constants.ShieldSourceBoost == 0 {
			// Add SourceBoost to existing shield
			shield.Sources |= constants.ShieldSourceBoost
			world.Shields.Add(cursorEntity, shield)
		}
	} else {
		if hasShield && shield.Sources&constants.ShieldSourceBoost != 0 {
			// Clear SourceBoost flag
			shield.Sources &^= constants.ShieldSourceBoost
			if shield.Sources == 0 {
				// No sources remain - remove component
				world.Shields.Remove(cursorEntity)
			} else {
				world.Shields.Add(cursorEntity, shield)
			}
		}
	}
}

// IsShieldActive returns true if shield has sources AND energy > 0
func IsShieldActive(world *engine.World, cursorEntity engine.Entity, state *engine.GameState) bool {
	shield, ok := world.Shields.Get(cursorEntity)
	if !ok {
		return false
	}
	return shield.Sources != 0 && state.GetEnergy() > 0
}
```

### File: render/renderers/shields.go

Update Render to check both Sources and Energy:

```go
// Render draws all active shields
func (s *ShieldRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
	const useBlending = true

	shields := world.Shields.All()

	for _, entity := range shields {
		shield, okS := world.Shields.Get(entity)
		pos, okP := world.Positions.Get(entity)

		if !okS || !okP {
			continue
		}

		// Shield only renders if Sources != 0 AND Energy > 0
		// Energy check requires access to GameState - get from world resources or context
		// For renderer, we check Sources only; Energy check done via helper if available
		if shield.Sources == 0 {
			continue
		}

		// Bounding box calculation (unchanged)
		startX := int(float64(pos.X) - shield.RadiusX)
		endX := int(float64(pos.X) + shield.RadiusX)
		startY := int(float64(pos.Y) - shield.RadiusY)
		endY := int(float64(pos.Y) + shield.RadiusY)

		if startX < 0 {
			startX = 0
		}
		if endX >= ctx.GameWidth {
			endX = ctx.GameWidth - 1
		}
		if startY < 0 {
			startY = 0
		}
		if endY >= ctx.GameHeight {
			endY = ctx.GameHeight - 1
		}

		shieldR, shieldG, shieldB := shield.Color.RGB()

		for y := startY; y <= endY; y++ {
			for x := startX; x <= endX; x++ {
				screenX := ctx.GameX + x
				screenY := ctx.GameY + y

				dx := float64(x - pos.X)
				dy := float64(y - pos.Y)

				normalizedDistSq := (dx*dx)/(shield.RadiusX*shield.RadiusX) + (dy*dy)/(shield.RadiusY*shield.RadiusY)

				if normalizedDistSq > 1.0 {
					continue
				}

				dist := math.Sqrt(normalizedDistSq)
				alpha := (1.0 - dist) * shield.MaxOpacity

				cell := buf.Get(screenX, screenY)
				fg, bg, attrs := cell.Style.Decompose()

				if bg == tcell.ColorDefault {
					bg = render.RgbBackground
				}

				var newBg tcell.Color
				if useBlending {
					newBg = s.blendColors(bg, shield.Color, alpha)
				} else {
					newBg = tcell.NewRGBColor(
						int32(float64(shieldR)*alpha),
						int32(float64(shieldG)*alpha),
						int32(float64(shieldB)*alpha),
					)
				}

				newStyle := tcell.StyleDefault.Foreground(fg).Background(newBg).Attributes(attrs)
				buf.Set(screenX, screenY, cell.Rune, newStyle)
			}
		}
	}
}

// blendColors unchanged
```

Note: ShieldRenderer needs access to GameState for Energy check. Two options:
1. Pass GameState via RenderContext (requires RenderContext change)
2. Add a resource lookup in renderer

For minimal change, add energy check to RenderContext. Update render/context.go:

```go
// Add to RenderContext struct:
Energy int // Current energy for shield visibility check
```

Update render/context.go NewRenderContextFromGame to include Energy:

```go
func NewRenderContextFromGame(ctx *engine.GameContext, timeRes *engine.TimeResource, cursorX, cursorY int) RenderContext {
	return RenderContext{
		GameTime:    timeRes.GameTime,
		FrameNumber: timeRes.FrameNumber,
		DeltaTime:   float64(timeRes.DeltaTime) / float64(time.Second),
		IsPaused:    ctx.IsPaused.Load(),
		CursorX:     cursorX,
		CursorY:     cursorY,
		GameX:       ctx.GameX,
		GameY:       ctx.GameY,
		GameWidth:   ctx.GameWidth,
		GameHeight:  ctx.GameHeight,
		Width:       ctx.Width,
		Height:      ctx.Height,
		Energy:      ctx.State.GetEnergy(),
	}
}
```

Then in shields.go Render, add after Sources check:

```go
// Check Energy via RenderContext
if ctx.Energy <= 0 {
	continue
}
```

## Verification
Run: `go build ./...`
```

---

## Prompt 3: Drain Spawning & Heat-Based Lifecycle

```markdown
# Task: Phase 3 - Drain Spawning & Heat-Based Lifecycle

## Reference
Read `GAME_MECHANICS_UPDATE.md` for context.

## Update Memory File
Update Phase 3 status to [x] in `GAME_MECHANICS_UPDATE.md`.

Add section:
```md
## Drain Spawning (Phase 3)
- Target drain count = floor(Heat / 10), max 10
- Removed energy <= 0 despawn from main Update loop
- Spawn position validation: skip cells with existing drain
- Energy-based despawn moved to Phase 6 (conditional on !ShieldActive)
```

## Implementation

### File: systems/drain.go

Modify the following functions:

#### calcTargetDrainCount - Update formula
```go
// calcTargetDrainCount returns the desired number of drains based on current heat
// Formula: floor(heat / 10), capped at DrainMaxCount
func (s *DrainSystem) calcTargetDrainCount() int {
	heat := s.ctx.State.GetHeat()
	count := heat / 10 // Integer division = floor
	if count > constants.DrainMaxCount {
		count = constants.DrainMaxCount
	}
	return count
}
```

#### Update - Remove energy-based despawn from main loop
```go
// Update runs the drain system logic
func (s *DrainSystem) Update(world *engine.World, dt time.Duration) {
	// Process pending spawn queue first
	s.processPendingSpawns(world)

	// Update materialize animation if active
	if world.Materializers.Count() > 0 {
		s.updateMaterializers(world, dt)
	}

	// Multi-drain lifecycle based on heat (energy check removed - handled in Phase 6)
	currentCount := world.Drains.Count()
	pendingCount := len(s.pendingSpawns) + s.countActiveMaterializations(world)

	targetCount := s.calcTargetDrainCount()
	effectiveCount := currentCount + pendingCount

	if effectiveCount < targetCount {
		// Need more drains
		s.queueDrainSpawns(world, targetCount-effectiveCount)
	} else if currentCount > targetCount {
		// Too many drains (heat dropped)
		s.despawnExcessDrains(world, currentCount-targetCount)
	}

	// Clock-based updates for active drains
	if world.Drains.Count() > 0 {
		s.updateDrainMovement(world)
		s.handleDrainInteractions(world) // Renamed from updateEnergyDrain + collisions
	}
}
```

#### randomSpawnOffset - Add occupied cell check
```go
// randomSpawnOffset returns a valid position with random offset, clamped to bounds
// Retries up to 10 times to find unoccupied cell
func (s *DrainSystem) randomSpawnOffset(world *engine.World, baseX, baseY int, config *engine.ConfigResource) (int, int, bool) {
	const maxRetries = 10

	for attempt := 0; attempt < maxRetries; attempt++ {
		offsetX := rand.Intn(constants.DrainSpawnOffsetMax*2+1) - constants.DrainSpawnOffsetMax
		offsetY := rand.Intn(constants.DrainSpawnOffsetMax*2+1) - constants.DrainSpawnOffsetMax

		x := baseX + offsetX
		y := baseY + offsetY

		// Clamp to game bounds
		if x < 0 {
			x = 0
		}
		if x >= config.GameWidth {
			x = config.GameWidth - 1
		}
		if y < 0 {
			y = 0
		}
		if y >= config.GameHeight {
			y = config.GameHeight - 1
		}

		// Check if cell is occupied by another drain
		entities := world.Positions.GetAllAt(x, y)
		hasExistingDrain := false
		for _, e := range entities {
			if _, ok := world.Drains.Get(e); ok {
				hasExistingDrain = true
				break
			}
		}

		if !hasExistingDrain {
			return x, y, true
		}
	}

	return 0, 0, false // Failed to find valid position
}
```

#### queueDrainSpawns - Use updated randomSpawnOffset
```go
// queueDrainSpawns queues multiple drain spawns with stagger timing
func (s *DrainSystem) queueDrainSpawns(world *engine.World, count int) {
	config := engine.MustGetResource[*engine.ConfigResource](world.Resources)

	cursorPos, ok := world.Positions.Get(s.ctx.CursorEntity)
	if !ok {
		return
	}

	queued := 0
	for i := 0; i < count; i++ {
		targetX, targetY, valid := s.randomSpawnOffset(world, cursorPos.X, cursorPos.Y, config)
		if valid {
			s.queueDrainSpawn(queued, targetX, targetY, queued)
			queued++
		}
	}
}
```

## Verification
Run: `go build ./...`
```

---

## Prompt 4: Drain-Cursor & Drain-Drain Collisions

```markdown
# Task: Phase 4 - Drain-Cursor & Drain-Drain Collisions

## Reference
Read `GAME_MECHANICS_UPDATE.md` for context.

## Update Memory File
Update Phase 4 status to [x] in `GAME_MECHANICS_UPDATE.md`.

Add section:
```md
## Collisions (Phase 4)
- Drain-Drain: If multiple drains at same cell, all involved despawn with flash
- Drain-Cursor (No Shield): -10 Heat, drain despawns
- Drain-Cursor (Shield Active): Energy drain only, no heat loss, drain persists
```

## Implementation

### File: systems/drain.go

Add helper for shield check (or import from boost.go):
```go
// isShieldActive checks if shield is functionally active
func (s *DrainSystem) isShieldActive(world *engine.World) bool {
	shield, ok := world.Shields.Get(s.ctx.CursorEntity)
	if !ok {
		return false
	}
	return shield.Sources != 0 && s.ctx.State.GetEnergy() > 0
}
```

Replace handleCollisions and updateEnergyDrain with unified handleDrainInteractions:
```go
// handleDrainInteractions processes all drain interactions per tick
func (s *DrainSystem) handleDrainInteractions(world *engine.World) {
	timeRes := engine.MustGetResource[*engine.TimeResource](world.Resources)
	now := timeRes.GameTime

	cursorPos, ok := world.Positions.Get(s.ctx.CursorEntity)
	if !ok {
		return
	}

	shieldActive := s.isShieldActive(world)

	// Phase 1: Detect drain-drain collisions (same cell)
	s.handleDrainDrainCollisions(world)

	// Phase 2: Handle cursor interactions for surviving drains
	drainEntities := world.Drains.All()
	for _, drainEntity := range drainEntities {
		drain, ok := world.Drains.Get(drainEntity)
		if !ok {
			continue
		}

		drainPos, ok := world.Positions.Get(drainEntity)
		if !ok {
			continue
		}

		isOnCursor := drainPos.X == cursorPos.X && drainPos.Y == cursorPos.Y

		// Update cached state
		if drain.IsOnCursor != isOnCursor {
			drain.IsOnCursor = isOnCursor
			world.Drains.Add(drainEntity, drain)
		}

		if isOnCursor {
			if shieldActive {
				// Shield active: drain energy, no heat loss, drain persists
				if now.Sub(drain.LastDrainTime) >= constants.DrainEnergyDrainInterval {
					s.ctx.State.AddEnergy(-constants.DrainShieldEnergyDrainAmount)
					drain.LastDrainTime = now
					world.Drains.Add(drainEntity, drain)
				}
			} else {
				// No shield: reduce heat and despawn drain
				s.ctx.State.AddHeat(-constants.DrainHeatReductionAmount)
				s.despawnDrainWithFlash(world, drainEntity)
			}
		}
	}

	// Phase 3: Handle non-cursor entity collisions (characters, etc)
	s.handleEntityCollisions(world)
}

// handleDrainDrainCollisions detects and removes all drains sharing a cell
func (s *DrainSystem) handleDrainDrainCollisions(world *engine.World) {
	// Build position -> drain entities map
	drainPositions := make(map[uint64][]engine.Entity)

	drainEntities := world.Drains.All()
	for _, drainEntity := range drainEntities {
		pos, ok := world.Positions.Get(drainEntity)
		if !ok {
			continue
		}
		key := uint64(pos.X)<<32 | uint64(pos.Y)
		drainPositions[key] = append(drainPositions[key], drainEntity)
	}

	// Find and destroy all drains at cells with multiple drains
	for _, entities := range drainPositions {
		if len(entities) > 1 {
			for _, e := range entities {
				s.despawnDrainWithFlash(world, e)
			}
		}
	}
}

// handleEntityCollisions processes collisions with non-drain entities
func (s *DrainSystem) handleEntityCollisions(world *engine.World) {
	entities := world.Drains.All()
	for _, entity := range entities {
		drainPos, ok := world.Positions.Get(entity)
		if !ok {
			continue
		}

		targets := world.Positions.GetAllAt(drainPos.X, drainPos.Y)

		for _, target := range targets {
			if target != 0 && target != entity && target != s.ctx.CursorEntity {
				// Skip other drains (handled separately)
				if _, ok := world.Drains.Get(target); ok {
					continue
				}
				s.handleCollisionAtPosition(world, target)
			}
		}
	}
}
```

## Verification
Run: `go build ./...`
```

---

## Prompt 5: Shield Zone Protection

```markdown
# Task: Phase 5 - Shield Zone Protection

## Reference
Read `GAME_MECHANICS_UPDATE.md` for context.

## Update Memory File
Update Phase 5 status to [x] in `GAME_MECHANICS_UPDATE.md`.

Add section:
```md
## Shield Zone Protection (Phase 5)
- Drains inside shield ellipse (not just on cursor) drain 100 energy per interval
- Ellipse check: (dx/rx)^2 + (dy/ry)^2 <= 1
- Energy drain applies to ALL drains in shield, including those on cursor
```

## Implementation

### File: systems/drain.go

Add ellipse check helper:
```go
// isInsideShieldEllipse checks if position is within the shield ellipse
func (s *DrainSystem) isInsideShieldEllipse(world *engine.World, x, y int) bool {
	shield, ok := world.Shields.Get(s.ctx.CursorEntity)
	if !ok {
		return false
	}

	cursorPos, ok := world.Positions.Get(s.ctx.CursorEntity)
	if !ok {
		return false
	}

	dx := float64(x - cursorPos.X)
	dy := float64(y - cursorPos.Y)

	// Ellipse equation: (dx/rx)^2 + (dy/ry)^2 <= 1
	normalizedDistSq := (dx*dx)/(shield.RadiusX*shield.RadiusX) + (dy*dy)/(shield.RadiusY*shield.RadiusY)
	return normalizedDistSq <= 1.0
}
```

Update handleDrainInteractions to include shield zone energy drain:
```go
// handleDrainInteractions processes all drain interactions per tick
func (s *DrainSystem) handleDrainInteractions(world *engine.World) {
	timeRes := engine.MustGetResource[*engine.TimeResource](world.Resources)
	now := timeRes.GameTime

	cursorPos, ok := world.Positions.Get(s.ctx.CursorEntity)
	if !ok {
		return
	}

	shieldActive := s.isShieldActive(world)

	// Phase 1: Detect drain-drain collisions
	s.handleDrainDrainCollisions(world)

	// Phase 2: Handle shield zone and cursor interactions
	drainEntities := world.Drains.All()
	for _, drainEntity := range drainEntities {
		drain, ok := world.Drains.Get(drainEntity)
		if !ok {
			continue
		}

		drainPos, ok := world.Positions.Get(drainEntity)
		if !ok {
			continue
		}

		isOnCursor := drainPos.X == cursorPos.X && drainPos.Y == cursorPos.Y

		// Update cached state
		if drain.IsOnCursor != isOnCursor {
			drain.IsOnCursor = isOnCursor
			world.Drains.Add(drainEntity, drain)
		}

		// Shield zone energy drain (applies to drains anywhere in shield ellipse)
		if shieldActive && s.isInsideShieldEllipse(world, drainPos.X, drainPos.Y) {
			if now.Sub(drain.LastDrainTime) >= constants.DrainEnergyDrainInterval {
				s.ctx.State.AddEnergy(-constants.DrainShieldEnergyDrainAmount)
				drain.LastDrainTime = now
				world.Drains.Add(drainEntity, drain)
			}
			// Drain persists when shield is active
			continue
		}

		// Cursor collision (shield not active or drain outside shield)
		if isOnCursor {
			// No shield protection: reduce heat and despawn
			s.ctx.State.AddHeat(-constants.DrainHeatReductionAmount)
			s.despawnDrainWithFlash(world, drainEntity)
		}
	}

	// Phase 3: Handle non-drain entity collisions
	s.handleEntityCollisions(world)
}
```

## Verification
Run: `go build ./...`
```

---

## Prompt 6: Passive Shield Drain & Final Cleanup

```markdown
# Task: Phase 6 - Passive Shield Drain & Final Despawn Rules

## Reference
Read `GAME_MECHANICS_UPDATE.md` for context.

## Update Memory File
Update Phase 6 status to [x] in `GAME_MECHANICS_UPDATE.md`.

Add section:
```md
## Passive Drain & Cleanup (Phase 6)
- Shield passive cost: 1 Energy/second while Sources != 0 AND Energy > 0
- Despawn all drains when Energy <= 0 AND !ShieldActive
- Passive drain tracked via ShieldComponent.LastDrainTime
```

## Implementation

### File: systems/drain.go

Add passive shield drain handling and energy-zero despawn rule:
```go
// Update runs the drain system logic
func (s *DrainSystem) Update(world *engine.World, dt time.Duration) {
	timeRes := engine.MustGetResource[*engine.TimeResource](world.Resources)
	now := timeRes.GameTime

	// Passive shield energy drain (1/sec while active)
	s.handlePassiveShieldDrain(world, now)

	// Process pending spawn queue
	s.processPendingSpawns(world)

	// Update materialize animation
	if world.Materializers.Count() > 0 {
		s.updateMaterializers(world, dt)
	}

	// Check for energy-zero despawn condition
	if s.ctx.State.GetEnergy() <= 0 && !s.isShieldActive(world) {
		if world.Drains.Count() > 0 {
			s.despawnAllDrains(world)
		}
		s.pendingSpawns = s.pendingSpawns[:0]
		return
	}

	// Multi-drain lifecycle based on heat
	currentCount := world.Drains.Count()
	pendingCount := len(s.pendingSpawns) + s.countActiveMaterializations(world)

	targetCount := s.calcTargetDrainCount()
	effectiveCount := currentCount + pendingCount

	if effectiveCount < targetCount {
		s.queueDrainSpawns(world, targetCount-effectiveCount)
	} else if currentCount > targetCount {
		s.despawnExcessDrains(world, currentCount-targetCount)
	}

	// Clock-based updates
	if world.Drains.Count() > 0 {
		s.updateDrainMovement(world)
		s.handleDrainInteractions(world)
	}
}

// handlePassiveShieldDrain applies 1 energy/second cost while shield is active
func (s *DrainSystem) handlePassiveShieldDrain(world *engine.World, now time.Time) {
	shield, ok := world.Shields.Get(s.ctx.CursorEntity)
	if !ok {
		return
	}

	// Shield must have sources and energy > 0 for passive drain
	if shield.Sources == 0 || s.ctx.State.GetEnergy() <= 0 {
		return
	}

	// Check passive drain interval
	if now.Sub(shield.LastDrainTime) >= constants.ShieldPassiveDrainInterval {
		s.ctx.State.AddEnergy(-constants.ShieldPassiveDrainAmount)
		shield.LastDrainTime = now
		world.Shields.Add(s.ctx.CursorEntity, shield)
	}
}
```

## Final Update to Memory File

Add completion summary:
```md
## Implementation Complete

All phases implemented:
1. Constants & Components - New shield/drain constants, ShieldComponent.Sources field
2. Shield Lifecycle - BoostSystem manages Sources bitmask, renderer checks Energy
3. Drain Spawning - Heat-based target count, occupied cell validation
4. Collisions - Drain-drain mutual destruction, cursor collision with shield check
5. Shield Zone - Ellipse-based protection with energy cost per drain
6. Passive Drain - 1 Energy/sec shield cost, energy-zero despawn rule

Key behavioral changes:
- Drains no longer despawn when Energy <= 0 unless Shield is also inactive
- Shield provides protection at energy cost (100/drain/tick)
- Heat reduced by 10 on unshielded cursor collision
- Multiple drains at same cell mutually destroy
```

## Verification
Run: `go build ./...`
```

---

## Summary

Each prompt:
1. References `GAME_MECHANICS_UPDATE.md` as persistent memory
2. Updates only the necessary code blocks
3. Includes single verification command: `go build ./...`
4. Is self-contained with full function implementations where modified