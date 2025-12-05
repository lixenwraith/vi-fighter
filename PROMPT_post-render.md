# Implementation Task: Post-Processing Pipeline with Stencil Masking

## Objective

Implement a stencil-based post-processing system for the render pipeline. This enables global visual effects (dim, grayscale) applied selectively to categories of rendered content via bitmasks.

## Architecture Overview

```
Renderers (Grid, Entities, Effects, UI)
↓ SetWriteMask() per renderer
RenderBuffer (cells[] + masks[])
↓ After all renderers complete
Post-Processors (Grayout, Dim)
↓ MutateGrayscale/MutateDim by mask
FlushToTerminal
```

Key principle: Renderers tag cells with masks. Post-processors query masks to selectively mutate colors.

---

## Files to Create

### 1. `render/mask.go`

```go
// FILE: render/mask.go
package render

// Render masks categorize buffer cells for selective post-processing
// Masks are bitfields allowing combination via OR and exclusion via XOR
const (
	MaskNone   uint8 = 0
	MaskGrid   uint8 = 1 << 0 // Background grid, ping overlay
	MaskEntity uint8 = 1 << 1 // Characters, nuggets, spawned content
	MaskShield uint8 = 1 << 2 // Cursor shield effect
	MaskEffect uint8 = 1 << 3 // Decay, cleaners, flashes, materializers, drains
	MaskUI     uint8 = 1 << 4 // Heat meter, status bar, line numbers, cursor, overlay
	MaskAll    uint8 = 0xFF
)
```

### 2. `render/renderers/post_process.go`

```go
// FILE: render/renderers/post_process.go
package renderers

import (
	"time"

	"github.com/lixenwraith/vi-fighter/engine"
	"github.com/lixenwraith/vi-fighter/render"
)

// DimRenderer applies brightness reduction to masked cells
type DimRenderer struct {
	gameCtx    *engine.GameContext
	factor     float64
	targetMask uint8
}

// NewDimRenderer creates a dim post-processor
// factor: brightness multiplier (0.0-1.0), targetMask: cells to affect
func NewDimRenderer(ctx *engine.GameContext, factor float64, targetMask uint8) *DimRenderer {
	return &DimRenderer{
		gameCtx:    ctx,
		factor:     factor,
		targetMask: targetMask,
	}
}

// Render applies dimming when game is paused
func (r *DimRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
	if !ctx.IsPaused {
		return
	}
	buf.MutateDim(r.factor, r.targetMask)
}

// GrayoutRenderer applies desaturation effect based on game state
type GrayoutRenderer struct {
	gameCtx    *engine.GameContext
	duration   time.Duration
	targetMask uint8
}

// NewGrayoutRenderer creates a grayscale post-processor
// duration: fade-out time, targetMask: cells to affect
func NewGrayoutRenderer(ctx *engine.GameContext, duration time.Duration, targetMask uint8) *GrayoutRenderer {
	return &GrayoutRenderer{
		gameCtx:    ctx,
		duration:   duration,
		targetMask: targetMask,
	}
}

// Render applies grayscale with intensity from game state
func (r *GrayoutRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
	intensity := r.gameCtx.State.GetGrayoutIntensity(ctx.GameTime, r.duration)
	if intensity <= 0 {
		return
	}
	buf.MutateGrayscale(intensity, r.targetMask)
}
```

---

## Files to Modify

### 3. `render/rgb.go`

Add these two functions after existing blend functions:

```go
// Grayscale converts RGB to grayscale using Rec. 601 luma coefficients
// Formula: Y = R*0.299 + G*0.587 + B*0.114
// Integer math: (R*299 + G*587 + B*114) / 1000
func Grayscale(c RGB) RGB {
	gray := uint8((int(c.R)*299 + int(c.G)*587 + int(c.B)*114) / 1000)
	return RGB{R: gray, G: gray, B: gray}
}

// Lerp linearly interpolates between two colors
// t=0 returns a, t=1 returns b
func Lerp(a, b RGB, t float64) RGB {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	return RGB{
		R: uint8(float64(a.R) + t*float64(int(b.R)-int(a.R))),
		G: uint8(float64(a.G) + t*float64(int(b.G)-int(a.G))),
		B: uint8(float64(a.B) + t*float64(int(b.B)-int(a.B))),
	}
}
```

### 4. `render/buffer.go`

**Full replacement required.** Key changes:
- Add `masks []uint8` field
- Add `currentMask uint8` field
- Add `SetWriteMask()` method
- Update all draw methods to write mask
- Add `MutateDim()` and `MutateGrayscale()` methods
- Update `Resize()` and `Clear()` to handle masks

```go
// FILE: render/buffer.go
package render

import (
	"github.com/lixenwraith/vi-fighter/terminal"
)

// RenderBuffer is a compositor backed by terminal.Cell array with dirty tracking
type RenderBuffer struct {
	cells       []terminal.Cell
	touched     []bool
	masks       []uint8
	currentMask uint8
	width       int
	height      int
}

// NewRenderBuffer creates a buffer with the specified dimensions
func NewRenderBuffer(width, height int) *RenderBuffer {
	size := width * height
	cells := make([]terminal.Cell, size)
	touched := make([]bool, size)
	masks := make([]uint8, size)
	for i := range cells {
		cells[i] = terminal.Cell{
			Rune:  0,
			Fg:    DefaultBgRGB,
			Bg:    RGBBlack,
			Attrs: terminal.AttrNone,
		}
	}
	return &RenderBuffer{
		cells:       cells,
		touched:     touched,
		masks:       masks,
		currentMask: MaskNone,
		width:       width,
		height:      height,
	}
}

// Resize adjusts buffer dimensions, reallocates only if capacity insufficient
func (b *RenderBuffer) Resize(width, height int) {
	size := width * height
	if cap(b.cells) < size {
		b.cells = make([]terminal.Cell, size)
		b.touched = make([]bool, size)
		b.masks = make([]uint8, size)
	} else {
		b.cells = b.cells[:size]
		b.touched = b.touched[:size]
		b.masks = b.masks[:size]
	}
	b.width = width
	b.height = height
	b.Clear()
}

// Clear resets all cells to empty using exponential copy
func (b *RenderBuffer) Clear() {
	if len(b.cells) == 0 {
		return
	}
	b.cells[0] = terminal.Cell{
		Rune:  0,
		Fg:    DefaultBgRGB,
		Bg:    RGBBlack,
		Attrs: terminal.AttrNone,
	}
	b.touched[0] = false
	b.masks[0] = MaskNone

	for filled := 1; filled < len(b.cells); filled *= 2 {
		copy(b.cells[filled:], b.cells[:filled])
	}
	for filled := 1; filled < len(b.touched); filled *= 2 {
		copy(b.touched[filled:], b.touched[:filled])
	}
	for filled := 1; filled < len(b.masks); filled *= 2 {
		copy(b.masks[filled:], b.masks[:filled])
	}

	b.currentMask = MaskNone
}

// SetWriteMask sets the mask for subsequent draw operations
func (b *RenderBuffer) SetWriteMask(mask uint8) {
	b.currentMask = mask
}

// inBounds returns true if coordinates are within buffer
func (b *RenderBuffer) inBounds(x, y int) bool {
	return x >= 0 && x < b.width && y >= 0 && y < b.height
}

// ===== COMPOSITOR API =====

// Set composites a cell with specified blend mode
func (b *RenderBuffer) Set(x, y int, mainRune rune, fg, bg RGB, mode BlendMode, alpha float64, attrs terminal.Attr) {
	if !b.inBounds(x, y) {
		return
	}
	idx := y*b.width + x
	dst := &b.cells[idx]

	op := uint8(mode) & 0x0F
	flags := uint8(mode) & 0xF0

	b.masks[idx] = b.currentMask

	if mainRune != 0 {
		dst.Rune = mainRune
		dst.Attrs = attrs
	}

	if flags&flagBg != 0 {
		switch op {
		case opReplace:
			dst.Bg = bg
		case opAlpha:
			dst.Bg = Blend(dst.Bg, bg, alpha)
		case opAdd:
			dst.Bg = Add(dst.Bg, bg, alpha)
		case opMax:
			dst.Bg = Max(dst.Bg, bg, alpha)
		case opSoftLight:
			dst.Bg = SoftLight(dst.Bg, bg, alpha)
		case opScreen:
			dst.Bg = Screen(dst.Bg, bg, alpha)
		case opOverlay:
			dst.Bg = Overlay(dst.Bg, bg, alpha)
		}
		b.touched[idx] = true
	}

	if flags&flagFg != 0 {
		switch op {
		case opReplace:
			dst.Fg = fg
		case opAlpha:
			dst.Fg = Blend(dst.Fg, fg, alpha)
		case opAdd:
			dst.Fg = Add(dst.Fg, fg, alpha)
		case opMax:
			dst.Fg = Max(dst.Fg, fg, alpha)
		case opSoftLight:
			dst.Fg = SoftLight(dst.Fg, fg, alpha)
		case opScreen:
			dst.Fg = Screen(dst.Fg, fg, alpha)
		case opOverlay:
			dst.Fg = Overlay(dst.Fg, fg, alpha)
		}
	}
}

// SetFgOnly writes rune, foreground, and attrs while preserving existing background
func (b *RenderBuffer) SetFgOnly(x, y int, r rune, fg RGB, attrs terminal.Attr) {
	if !b.inBounds(x, y) {
		return
	}
	idx := y*b.width + x
	dst := &b.cells[idx]

	dst.Rune = r
	dst.Fg = fg
	dst.Attrs = attrs
	b.masks[idx] = b.currentMask
}

// SetBgOnly updates background color while preserving existing rune/foreground
func (b *RenderBuffer) SetBgOnly(x, y int, bg RGB) {
	if !b.inBounds(x, y) {
		return
	}
	idx := y*b.width + x

	b.cells[idx].Bg = bg
	b.touched[idx] = true
	b.masks[idx] = b.currentMask
}

// SetWithBg writes a cell with explicit fg and bg colors (opaque replace)
func (b *RenderBuffer) SetWithBg(x, y int, r rune, fg, bg RGB) {
	if !b.inBounds(x, y) {
		return
	}
	idx := y*b.width + x
	dst := &b.cells[idx]

	dst.Rune = r
	dst.Fg = fg
	dst.Bg = bg
	dst.Attrs = terminal.AttrNone
	b.touched[idx] = true
	b.masks[idx] = b.currentMask
}

// ===== POST-PROCESSING =====

// MutateDim multiplies colors by factor for cells matching targetMask
// Respects Fg/Bg granularity: touched cells get both mutated, untouched get Fg only
func (b *RenderBuffer) MutateDim(factor float64, targetMask uint8) {
	if factor >= 1.0 {
		return
	}
	for i := range b.cells {
		if b.masks[i]&targetMask == 0 {
			continue
		}
		cell := &b.cells[i]
		cell.Fg = Scale(cell.Fg, factor)
		if b.touched[i] {
			cell.Bg = Scale(cell.Bg, factor)
		}
	}
}

// MutateGrayscale desaturates cells matching targetMask
// intensity: 0.0 = no change, 1.0 = full grayscale
// Respects Fg/Bg granularity: touched cells get both mutated, untouched get Fg only
func (b *RenderBuffer) MutateGrayscale(intensity float64, targetMask uint8) {
	if intensity <= 0.0 {
		return
	}
	fullGray := intensity >= 1.0

	for i := range b.cells {
		if b.masks[i]&targetMask == 0 {
			continue
		}
		cell := &b.cells[i]

		fgGray := Grayscale(cell.Fg)
		if fullGray {
			cell.Fg = fgGray
		} else {
			cell.Fg = Lerp(cell.Fg, fgGray, intensity)
		}

		if b.touched[i] {
			bgGray := Grayscale(cell.Bg)
			if fullGray {
				cell.Bg = bgGray
			} else {
				cell.Bg = Lerp(cell.Bg, bgGray, intensity)
			}
		}
	}
}

// ===== OUTPUT =====

// finalize sets default background to untouched cells before Flush
func (b *RenderBuffer) finalize() {
	for i := range b.cells {
		if !b.touched[i] {
			b.cells[i].Bg = RgbBackground
		}
	}
}

// FlushToTerminal writes render buffer to terminal
func (b *RenderBuffer) FlushToTerminal(term terminal.Terminal) {
	b.finalize()
	term.Flush(b.cells, b.width, b.height)
}
```

### 5. `engine/game_state.go`

**Add fields to `GameState` struct** (in the "REAL-TIME STATE" section with other atomics):

```go
	// Grayout visual effect state
	GrayoutActive    atomic.Bool
	GrayoutStartTime atomic.Int64 // UnixNano
```

**Add to `initState()` method** (with other atomic initializations):

```go
	gs.GrayoutActive.Store(false)
	gs.GrayoutStartTime.Store(0)
```

**Add accessor methods** (at end of file, new section):

```go
// ===== GRAYOUT EFFECT ACCESSORS (atomic) =====

// TriggerGrayout activates the grayscale visual effect
func (gs *GameState) TriggerGrayout(now time.Time) {
	gs.GrayoutStartTime.Store(now.UnixNano())
	gs.GrayoutActive.Store(true)
}

// GetGrayoutIntensity returns current effect intensity (0.0 to 1.0)
// Returns 0.0 if effect inactive or duration exceeded
func (gs *GameState) GetGrayoutIntensity(now time.Time, duration time.Duration) float64 {
	if !gs.GrayoutActive.Load() {
		return 0.0
	}

	startNano := gs.GrayoutStartTime.Load()
	if startNano == 0 {
		return 0.0
	}

	elapsed := now.Sub(time.Unix(0, startNano))
	if elapsed >= duration {
		gs.GrayoutActive.Store(false)
		return 0.0
	}

	return 1.0 - (float64(elapsed) / float64(duration))
}
```

### 6. `systems/cleaner.go`

**Modify `spawnCleaners()` function.** Replace the beginning of the function:

```go
// spawnCleaners generates cleaner entities using generic stores
func (cs *CleanerSystem) spawnCleaners(world *engine.World) {
	config := engine.MustGetResource[*engine.ConfigResource](world.Resources)
	timeRes := engine.MustGetResource[*engine.TimeResource](world.Resources)

	redRows := cs.scanRedCharacterRows(world)

	// Phantom trigger: no targets to clean
	if len(redRows) == 0 {
		cs.ctx.State.TriggerGrayout(timeRes.GameTime)
		cs.ctx.PushEvent(engine.EventCleanerFinished, nil, timeRes.GameTime)
		return
	}

	if cs.ctx.AudioEngine != nil {
		cs.ctx.AudioEngine.SendRealTime(audio.AudioCommand{
			Type:       audio.SoundWhoosh,
			Priority:   1,
			Generation: uint64(cs.ctx.State.GetFrameNumber()),
			Timestamp:  timeRes.GameTime,
		})
	}

	// ... rest of existing spawning logic unchanged ...
```

### 7. Renderer Updates

**Pattern for all renderers**: Add `buf.SetWriteMask(render.MaskXXX)` as the FIRST line inside the `Render()` method.

#### `render/renderers/grid.go` (PingGridRenderer)
```go
func (p *PingGridRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
	buf.SetWriteMask(render.MaskGrid)
	// ... existing code unchanged ...
}
```

#### `render/renderers/characters.go` (CharactersRenderer)
```go
func (c *CharactersRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
	buf.SetWriteMask(render.MaskEntity)

	// Query entities with both position and character
	entities := world.Query().
		With(world.Positions).
		With(world.Characters).
		Execute()

	for _, entity := range entities {
		pos, okP := world.Positions.Get(entity)
		char, okC := world.Characters.Get(entity)
		if !okP || !okC {
			continue
		}

		screenX := ctx.GameX + pos.X
		screenY := ctx.GameY + pos.Y

		if screenX < ctx.GameX || screenX >= ctx.Width || screenY < ctx.GameY || screenY >= ctx.GameY+ctx.GameHeight {
			continue
		}

		fg := resolveCharacterColor(char)
		attrs := resolveTextStyle(char.Style)

		// REMOVED: Pause dimming logic - now handled by DimRenderer post-processor

		buf.SetFgOnly(screenX, screenY, char.Rune, fg, attrs)
	}
}
```

#### `render/renderers/shields.go` (ShieldRenderer)
```go
func (s *ShieldRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
	buf.SetWriteMask(render.MaskShield)
	// ... existing code unchanged ...
}
```

#### `render/renderers/effects.go` (EffectsRenderer)
```go
func (e *EffectsRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
	buf.SetWriteMask(render.MaskEffect)
	// ... existing code unchanged ...
}
```

#### `render/renderers/drain.go` (DrainRenderer)
```go
func (d *DrainRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
	buf.SetWriteMask(render.MaskEffect)
	// ... existing code unchanged ...
}
```

#### `render/renderers/heat_meter.go` (HeatMeterRenderer)
```go
func (h *HeatMeterRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
	buf.SetWriteMask(render.MaskUI)
	// ... existing code unchanged ...
}
```

#### `render/renderers/line_numbers.go` (LineNumbersRenderer)
```go
func (l *LineNumbersRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
	buf.SetWriteMask(render.MaskUI)
	// ... existing code unchanged ...
}
```

#### `render/renderers/column_indicators.go` (ColumnIndicatorsRenderer)
```go
func (c *ColumnIndicatorsRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
	buf.SetWriteMask(render.MaskUI)
	// ... existing code unchanged ...
}
```

#### `render/renderers/status_bar.go` (StatusBarRenderer)
```go
func (s *StatusBarRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
	buf.SetWriteMask(render.MaskUI)
	// ... existing code unchanged ...
}
```

#### `render/renderers/cursor.go` (CursorRenderer)
```go
func (c *CursorRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
	buf.SetWriteMask(render.MaskUI)
	// ... existing code unchanged ...
}
```

#### `render/renderers/overlay.go` (OverlayRenderer)
```go
func (o *OverlayRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
	buf.SetWriteMask(render.MaskUI)
	// ... existing code unchanged ...
}
```

### 8. `cmd/vi-fighter/main.go`

**Add import** (if not present):
```go
	"time"
```

**Add post-processor registrations** after DrainRenderer and before UI renderers:

```go
	// Drain (350)
	drainRenderer := renderers.NewDrainRenderer()
	orchestrator.Register(drainRenderer, render.PriorityDrain)

	// Post-Processing (390-395)
	grayoutRenderer := renderers.NewGrayoutRenderer(ctx, 1*time.Second, render.MaskEntity)
	orchestrator.Register(grayoutRenderer, render.PriorityUI-10)

	dimRenderer := renderers.NewDimRenderer(ctx, 0.5, render.MaskAll^render.MaskUI)
	orchestrator.Register(dimRenderer, render.PriorityUI-5)

	// UI (400)
	heatMeterRenderer := renderers.NewHeatMeterRenderer(ctx.State)
	// ... existing code ...
```

---

## Verification

After implementation:

1. **Build**: `go build ./cmd/vi-fighter`
2. **Run**: `./vi-fighter`
3. **Test Dim**: Press `:` to enter command mode (pause). All game content except UI should dim to 50%.
4. **Test Grayout**: Ensure no red characters exist, then trigger cleaner phase (game mechanic). Entities should flash grayscale and fade over 1 second.

---

## Summary of Changes

| File | Action |
|------|--------|
| `render/mask.go` | CREATE |
| `render/renderers/post_process.go` | CREATE |
| `render/rgb.go` | ADD 2 functions |
| `render/buffer.go` | REPLACE (full rewrite) |
| `engine/game_state.go` | ADD fields + methods |
| `systems/cleaner.go` | MODIFY spawnCleaners |
| `render/renderers/characters.go` | MODIFY + REMOVE pause dim |
| `render/renderers/grid.go` | MODIFY (add SetWriteMask) |
| `render/renderers/shields.go` | MODIFY (add SetWriteMask) |
| `render/renderers/effects.go` | MODIFY (add SetWriteMask) |
| `render/renderers/drain.go` | MODIFY (add SetWriteMask) |
| `render/renderers/heat_meter.go` | MODIFY (add SetWriteMask) |
| `render/renderers/line_numbers.go` | MODIFY (add SetWriteMask) |
| `render/renderers/column_indicators.go` | MODIFY (add SetWriteMask) |
| `render/renderers/status_bar.go` | MODIFY (add SetWriteMask) |
| `render/renderers/cursor.go` | MODIFY (add SetWriteMask) |
| `render/renderers/overlay.go` | MODIFY (add SetWriteMask) |
| `cmd/vi-fighter/main.go` | MODIFY (add registrations) |