## Updated Plan

### Decisions Summary
| Item | Decision |
|------|----------|
| Font | Assume generated, placeholder file |
| Colors | `RgbSplashInsert` (light gray), `RgbSplashNormal` (ping orange base) |
| Tab/Enter | Excluded |
| Arrow/Home/End | Excluded (vi keys only) |
| Failed actions | No splash (existing feedback sufficient) |
| Store pattern | Add to `allStores` (matches Cursors pattern) |
| Priority | 150 (background layer, before entities) |

### Implementation Sequence
1. `constants/splash.go` — constants
2. `components/splash.go` — component struct
3. `assets/splash_font.go` — placeholder bitmap array
4. `engine/world.go` — add Splashes store
5. `engine/game_context.go` — add SplashEntity
6. `engine/splash.go` — trigger helpers
7. `systems/splash.go` — timeout system
8. `render/priority.go` — add PrioritySplash
9. `render/colors.go` — add splash colors
10. `render/renderers/splash.go` — renderer
11. `systems/energy.go` — Insert mode hooks
12. `modes/input.go` — Normal mode hooks
13. `cmd/vi-fighter/main.go` — registration

---

## Code

### 1. constants/splash.go [full]
```go
// FILE: constants/splash.go
package constants

import "time"

const (
	SplashCharWidth   = 16
	SplashCharHeight  = 12
	SplashCharSpacing = 1
	SplashMaxLength   = 8
	SplashDuration    = 1 * time.Second
)
```

### 2. components/splash.go [full]
```go
// FILE: components/splash.go
package components

import "github.com/lixenwraith/vi-fighter/terminal"

// SplashComponent holds state for the singleton splash effect
// Length == 0 indicates inactive state
type SplashComponent struct {
	Content   [8]rune      // Pre-allocated content buffer
	Length    int          // Active character count; 0 = inactive
	Color     terminal.RGB // Flat render color
	AnchorX   int          // Game-relative X (top-left of first char)
	AnchorY   int          // Game-relative Y (top-left of first char)
	StartNano int64        // GameTime.UnixNano() at activation
}
```

### 3. assets/splash_font.go [full]
```go
// FILE: assets/splash_font.go
package assets

// SplashFont maps printable ASCII (32-126) to 12-row bitmaps
// Each row is uint16; bit 15 = column 0 (leftmost), bit 0 = column 15 (rightmost)
// Index: SplashFont[rune - 32]
// Placeholder: actual bitmap data to be generated separately
var SplashFont = [95][12]uint16{
	// Index 0 = ' ' (space) - all zeros
	{0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000},
	// Remaining 94 characters (! through ~) initialized to zero
	// TODO: Generate actual bitmap data
}
```

### 4. engine/world.go [block]
```go
// World contains all entities and their components using compile-time typed stores
type World struct {
	mu           sync.RWMutex
	nextEntityID Entity

	// Global Resources
	Resources *ResourceStore

	// Component Stores (Public for direct system access)
	Positions     *PositionStore
	Characters    *Store[components.CharacterComponent]
	Sequences     *Store[components.SequenceComponent]
	GoldSequences *Store[components.GoldSequenceComponent]
	Decays        *Store[components.DecayComponent]
	Cleaners      *Store[components.CleanerComponent]
	Flashes       *Store[components.FlashComponent]
	Nuggets       *Store[components.NuggetComponent]
	Drains        *Store[components.DrainComponent]
	Materializers *Store[components.MaterializeComponent]
	Cursors       *Store[components.CursorComponent]
	Protections   *Store[components.ProtectionComponent]
	Shields       *Store[components.ShieldComponent]
	Splashes      *Store[components.SplashComponent]

	allStores []AnyStore // All stores for uniform lifecycle operations

	systems     []System
	updateMutex sync.Mutex // Prevents concurrent updates
}

// NewWorld creates a new ECS world with all component stores initialized
func NewWorld() *World {
	w := &World{
		nextEntityID:  1,
		Resources:     NewResourceStore(),
		systems:       make([]System, 0),
		Positions:     NewPositionStore(),
		Characters:    NewStore[components.CharacterComponent](),
		Sequences:     NewStore[components.SequenceComponent](),
		GoldSequences: NewStore[components.GoldSequenceComponent](),
		Decays:        NewStore[components.DecayComponent](),
		Cleaners:      NewStore[components.CleanerComponent](),
		Flashes:       NewStore[components.FlashComponent](),
		Nuggets:       NewStore[components.NuggetComponent](),
		Drains:        NewStore[components.DrainComponent](),
		Materializers: NewStore[components.MaterializeComponent](),
		Cursors:       NewStore[components.CursorComponent](),
		Protections:   NewStore[components.ProtectionComponent](),
		Shields:       NewStore[components.ShieldComponent](),
		Splashes:      NewStore[components.SplashComponent](),
	}

	// Register all stores for lifecycle operations
	w.allStores = []AnyStore{
		w.Positions,
		w.Characters,
		w.Sequences,
		w.GoldSequences,
		w.Decays,
		w.Cleaners,
		w.Flashes,
		w.Nuggets,
		w.Drains,
		w.Materializers,
		w.Cursors,
		w.Protections,
		w.Shields,
		w.Splashes,
	}

	// Set world reference for z-index lookups
	w.Positions.SetWorld(w)

	return w
}
```

### 5. engine/game_context.go [diff]
```go
// In GameContext struct, after CursorEntity:
	// Cursor entity (singleton)
	CursorEntity Entity

	// Splash entity (singleton)
	SplashEntity Entity
```

```go
// In NewGameContext(), after cursor protection setup:
	// ... existing cursor setup ...

	// Create splash entity (singleton, no position component)
	ctx.SplashEntity = ctx.World.CreateEntity()
	ctx.World.Splashes.Add(ctx.SplashEntity, components.SplashComponent{})

	// Initialize ping atomic values (still local to input handling)
	// ... existing code ...
```

### 6. engine/splash.go [full]
```go
// FILE: engine/splash.go
package engine

import (
	"github.com/lixenwraith/vi-fighter/components"
	"github.com/lixenwraith/vi-fighter/constants"
	"github.com/lixenwraith/vi-fighter/terminal"
)

// TriggerSplashChar activates splash with a single character
func TriggerSplashChar(ctx *GameContext, char rune, color terminal.RGB) {
	cursorPos, ok := ctx.World.Positions.Get(ctx.CursorEntity)
	if !ok {
		return
	}

	anchorX, anchorY := calculateSplashAnchor(ctx, cursorPos.X, cursorPos.Y, 1)

	splash := components.SplashComponent{
		Length:    1,
		Color:     color,
		AnchorX:   anchorX,
		AnchorY:   anchorY,
		StartNano: ctx.PausableClock.Now().UnixNano(),
	}
	splash.Content[0] = char

	ctx.World.Splashes.Add(ctx.SplashEntity, splash)
}

// TriggerSplashString activates splash with a string (up to 8 chars)
func TriggerSplashString(ctx *GameContext, text string, color terminal.RGB) {
	if len(text) == 0 {
		return
	}

	cursorPos, ok := ctx.World.Positions.Get(ctx.CursorEntity)
	if !ok {
		return
	}

	runes := []rune(text)
	length := len(runes)
	if length > constants.SplashMaxLength {
		length = constants.SplashMaxLength
	}

	anchorX, anchorY := calculateSplashAnchor(ctx, cursorPos.X, cursorPos.Y, length)

	splash := components.SplashComponent{
		Length:    length,
		Color:     color,
		AnchorX:   anchorX,
		AnchorY:   anchorY,
		StartNano: ctx.PausableClock.Now().UnixNano(),
	}

	for i := 0; i < length; i++ {
		splash.Content[i] = runes[i]
	}

	ctx.World.Splashes.Add(ctx.SplashEntity, splash)
}

// calculateSplashAnchor computes top-left anchor for splash positioning
func calculateSplashAnchor(ctx *GameContext, cursorX, cursorY, charCount int) (int, int) {
	// Determine cursor quadrant and select opposite
	quadrant := 0
	if cursorX >= ctx.GameWidth/2 {
		quadrant |= 1
	}
	if cursorY >= ctx.GameHeight/2 {
		quadrant |= 2
	}
	opposite := quadrant ^ 0b11

	// Calculate quadrant center
	var centerX, centerY int
	if opposite&1 != 0 {
		centerX = ctx.GameWidth * 3 / 4
	} else {
		centerX = ctx.GameWidth / 4
	}
	if opposite&2 != 0 {
		centerY = ctx.GameHeight * 3 / 4
	} else {
		centerY = ctx.GameHeight / 4
	}

	// Calculate total splash dimensions
	totalWidth := charCount*constants.SplashCharWidth + (charCount-1)*constants.SplashCharSpacing
	totalHeight := constants.SplashCharHeight

	// Center splash in quadrant
	anchorX := centerX - totalWidth/2
	anchorY := centerY - totalHeight/2

	// Clamp to left/top boundaries only
	if anchorX < 0 {
		anchorX = 0
	}
	if anchorY < 0 {
		anchorY = 0
	}

	return anchorX, anchorY
}
```

### 7. systems/splash.go [full]
```go
// FILE: systems/splash.go
package systems

import (
	"time"

	"github.com/lixenwraith/vi-fighter/constants"
	"github.com/lixenwraith/vi-fighter/engine"
)

// SplashSystem handles splash timeout
type SplashSystem struct {
	ctx *engine.GameContext
}

// NewSplashSystem creates a new splash system
func NewSplashSystem(ctx *engine.GameContext) *SplashSystem {
	return &SplashSystem{ctx: ctx}
}

// Priority returns the system's priority (low, after game logic)
func (s *SplashSystem) Priority() int {
	return constants.PrioritySplash
}

// Update checks splash timeout and deactivates if expired
func (s *SplashSystem) Update(world *engine.World, dt time.Duration) {
	splash, ok := world.Splashes.Get(s.ctx.SplashEntity)
	if !ok || splash.Length == 0 {
		return
	}

	timeRes := engine.MustGetResource[*engine.TimeResource](world.Resources)
	elapsed := timeRes.GameTime.UnixNano() - splash.StartNano

	if elapsed >= constants.SplashDuration.Nanoseconds() {
		splash.Length = 0
		world.Splashes.Add(s.ctx.SplashEntity, splash)
	}
}
```

### 8. constants/priority.go [diff]
```go
// Add after existing system priorities:
	PrioritySplash = 800 // After game logic, before rendering
```

### 9. render/priority.go [diff]
```go
// Add after PriorityGrid:
	PrioritySplash RenderPriority = 150
```

### 10. render/colors.go [diff]
```go
// Add in var block after existing colors:
	// Splash colors
	RgbSplashInsert = RGB{200, 200, 200} // Light gray for insert mode
	RgbSplashNormal = RGB{153, 102, 0}   // Dark orange (ping base) for normal mode
```

### 11. render/renderers/splash.go [full]
```go
// FILE: render/renderers/splash.go
package renderers

import (
	"github.com/lixenwraith/vi-fighter/assets"
	"github.com/lixenwraith/vi-fighter/constants"
	"github.com/lixenwraith/vi-fighter/engine"
	"github.com/lixenwraith/vi-fighter/render"
)

// SplashRenderer draws large block characters as background effect
type SplashRenderer struct {
	ctx *engine.GameContext
}

// NewSplashRenderer creates a new splash renderer
func NewSplashRenderer(ctx *engine.GameContext) *SplashRenderer {
	return &SplashRenderer{ctx: ctx}
}

// Render draws the splash effect to background channel
func (s *SplashRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
	splash, ok := world.Splashes.Get(s.ctx.SplashEntity)
	if !ok || splash.Length == 0 {
		return
	}

	buf.SetWriteMask(render.MaskEffect)

	// Calculate opacity based on elapsed time
	elapsed := ctx.GameTime.UnixNano() - splash.StartNano
	duration := constants.SplashDuration.Nanoseconds()
	opacity := 1.0 - float64(elapsed)/float64(duration)
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 1 {
		opacity = 1
	}

	// Scale color by opacity
	scaledColor := render.Scale(render.RGB(splash.Color), opacity)

	// Render each character
	for i := 0; i < splash.Length; i++ {
		charX := splash.AnchorX + i*(constants.SplashCharWidth+constants.SplashCharSpacing)
		s.renderChar(ctx, buf, splash.Content[i], charX, splash.AnchorY, scaledColor)
	}
}

// renderChar renders a single splash character bitmap
func (s *SplashRenderer) renderChar(ctx render.RenderContext, buf *render.RenderBuffer, char rune, gameX, gameY int, color render.RGB) {
	// Bounds check character
	if char < 32 || char > 126 {
		return
	}

	bitmap := assets.SplashFont[char-32]

	for row := 0; row < constants.SplashCharHeight; row++ {
		screenY := ctx.GameY + gameY + row
		if screenY < ctx.GameY || screenY >= ctx.GameY+ctx.GameHeight {
			continue
		}

		rowBits := bitmap[row]
		for col := 0; col < constants.SplashCharWidth; col++ {
			// MSB-first: bit 15 = column 0
			if rowBits&(1<<(15-col)) == 0 {
				continue
			}

			screenX := ctx.GameX + gameX + col
			if screenX < ctx.GameX || screenX >= ctx.GameX+ctx.GameWidth {
				continue
			}

			buf.SetBgOnly(screenX, screenY, color)
		}
	}
}
```

### 12. systems/energy.go [block]
```go
// In handleCharacterTyping, after successful character destruction and cursor move:
// Find this section after "world.DestroyEntity(entity)" and cursor position update

func (s *EnergySystem) handleCharacterTyping(world *engine.World, cursorX, cursorY int, typedRune rune) {
	// ... existing code up to character match success ...

	if char.Rune == typedRune {
		// ... existing scoring, blink, boost logic ...

		// Safely destroy the character entity
		world.DestroyEntity(entity)

		// Trigger splash for successful typing
		engine.TriggerSplashChar(s.ctx, typedRune, s.getSplashColorForSequence(seq))

		// Move cursor right in ECS
		cursorPos, ok := world.Positions.Get(s.ctx.CursorEntity)
		if ok {
			if cursorPos.X < config.GameWidth-1 {
				cursorPos.X++
			}
			world.Positions.Add(s.ctx.CursorEntity, cursorPos)
		}
	} else {
		// ... existing error handling (no splash) ...
	}
}

// getSplashColorForSequence returns splash color based on sequence type
func (s *EnergySystem) getSplashColorForSequence(seq components.SequenceComponent) terminal.RGB {
	switch seq.Type {
	case components.SequenceGreen:
		return terminal.RGB(render.RgbSequenceGreenNormal)
	case components.SequenceBlue:
		return terminal.RGB(render.RgbSequenceBlueNormal)
	case components.SequenceRed:
		return terminal.RGB(render.RgbSequenceRedNormal)
	case components.SequenceGold:
		return terminal.RGB(render.RgbSequenceGold)
	default:
		return terminal.RGB(render.RgbSplashInsert)
	}
}
```

```go
// In handleGoldSequenceTyping, after successful gold character:
func (s *EnergySystem) handleGoldSequenceTyping(world *engine.World, entity engine.Entity, char components.CharacterComponent, seq components.SequenceComponent, typedRune rune) {
	// ... existing validation ...

	if char.Rune != typedRune {
		// ... existing error handling (no splash) ...
		return
	}

	// Correct character - remove entity and move cursor
	// ... existing energy blink logic ...

	// Safely destroy the character entity
	world.DestroyEntity(entity)

	// Trigger splash for gold character
	engine.TriggerSplashChar(s.ctx, typedRune, terminal.RGB(render.RgbSequenceGold))

	// Move cursor right
	// ... existing cursor move logic ...
}
```

```go
// In handleNuggetCollection, after successful collection:
func (s *EnergySystem) handleNuggetCollection(world *engine.World, entity engine.Entity, char components.CharacterComponent, typedRune rune) {
	// ... existing validation ...

	if char.Rune != typedRune {
		// ... existing error handling (no splash) ...
		return
	}

	// Correct character - collect nugget
	// ... existing heat logic ...

	// Destroy the nugget entity
	world.DestroyEntity(entity)

	// Trigger splash for nugget collection
	engine.TriggerSplashChar(s.ctx, typedRune, terminal.RGB(render.RgbNuggetOrange))

	// ... existing cursor move and cleanup ...
}
```

### 13. modes/input.go [block]
```go
// In handleNormalMode, after action execution:
func (h *InputHandler) handleNormalMode(ev terminal.Event) bool {
	// ... existing special key handling ...

	if ev.Key != terminal.KeyRune {
		return true
	}

	result := h.machine.Process(ev.Rune, h.bindings)

	if result.ModeChange != 0 {
		h.ctx.Mode = result.ModeChange
		if result.ModeChange == engine.ModeSearch {
			h.ctx.SearchText = ""
		} else if result.ModeChange == engine.ModeCommand {
			h.ctx.CommandText = ""
		}
	}

	if result.Action != nil {
		if result.CommandString != "" {
			h.ctx.LastCommand = result.CommandString
			// Trigger splash for command execution
			engine.TriggerSplashString(h.ctx, result.CommandString, h.ctx.Terminal.RGB(render.RgbSplashNormal))
		}
		h.ctx.World.RunSafe(func() {
			result.Action(h.ctx)
		})
	}

	return result.Continue
}
```

### 14. cmd/vi-fighter/main.go [diff]
```go
// After flashSystem creation:
	flashSystem := systems.NewFlashSystem(ctx)
	ctx.World.AddSystem(flashSystem)

	splashSystem := systems.NewSplashSystem(ctx)
	ctx.World.AddSystem(splashSystem)
```

```go
// After pingGridRenderer registration:
	pingGridRenderer := renderers.NewPingGridRenderer(ctx)
	orchestrator.Register(pingGridRenderer, render.PriorityGrid)

	splashRenderer := renderers.NewSplashRenderer(ctx)
	orchestrator.Register(splashRenderer, render.PrioritySplash)
```

---

## Import Additions Required

**systems/energy.go:**
```go
import (
	// ... existing imports ...
	"github.com/lixenwraith/vi-fighter/render"
	"github.com/lixenwraith/vi-fighter/terminal"
)
```

**modes/input.go:**
```go
import (
	// ... existing imports ...
	"github.com/lixenwraith/vi-fighter/render"
)
```
