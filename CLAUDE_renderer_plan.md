# Renderer Pipeline Refactor: Final Migration Plan

## Design Decisions

| Issue | Resolution |
|-------|------------|
| Attribute storage | Use `tcell.AttrMask` directly until Phase 4 cleanup |
| Dual-mode buffer | **REJECTED** - Single source of truth only |
| Bridge lifetime | Temporary, deleted in Phase 4 |
| Color conversion | On-write via bridge, amortized cost acceptable |

---

## Phase 1: Foundation Types

### 1.1 `render/color.go` - RGB Type & Blend Operations

```go
// FILE: render/color.go
package render

// RGB stores explicit 8-bit color channels, decoupled from tcell
type RGB struct {
	R, G, B uint8
}

// Predefined colors
var (
	RGBBlack = RGB{0, 0, 0}
)

// Blend performs alpha blending: result = src*alpha + dst*(1-alpha)
func (dst RGB) Blend(src RGB, alpha float64) RGB {
	if alpha <= 0 {
		return dst
	}
	if alpha >= 1 {
		return src
	}
	inv := 1.0 - alpha
	return RGB{
		R: uint8(float64(src.R)*alpha + float64(dst.R)*inv),
		G: uint8(float64(src.G)*alpha + float64(dst.G)*inv),
		B: uint8(float64(src.B)*alpha + float64(dst.B)*inv),
	}
}

// Max returns per-channel maximum (non-destructive highlight)
func (dst RGB) Max(src RGB) RGB {
	return RGB{
		R: max(dst.R, src.R),
		G: max(dst.G, src.G),
		B: max(dst.B, src.B),
	}
}

// Add performs additive blend with clamping (light accumulation)
func (dst RGB) Add(src RGB) RGB {
	return RGB{
		R: uint8(min(int(dst.R)+int(src.R), 255)),
		G: uint8(min(int(dst.G)+int(src.G), 255)),
		B: uint8(min(int(dst.B)+int(src.B), 255)),
	}
}

// BlendMode defines compositing operations
type BlendMode uint8

const (
	BlendReplace BlendMode = iota // Dst = Src (opaque overwrite)
	BlendAlpha                    // Dst = Src*α + Dst*(1-α)
	BlendAdd                      // Dst = clamp(Dst + Src, 255)
	BlendMax                      // Dst = max(Dst, Src) per channel
)
```

### 1.2 `render/bridge.go` - Temporary tcell Conversion (Deleted in Phase 4)

```go
// FILE: render/bridge.go
package render

import "github.com/gdamore/tcell/v2"

// TcellToRGB converts tcell.Color to RGB
// Treats ColorDefault as the standard background color
func TcellToRGB(c tcell.Color) RGB {
	if c == tcell.ColorDefault {
		// Use RgbBackground values directly to avoid import cycle
		return RGB{26, 27, 38}
	}
	r, g, b := c.RGB()
	return RGB{uint8(r), uint8(g), uint8(b)}
}

// RGBToTcell converts RGB to tcell.Color
func RGBToTcell(rgb RGB) tcell.Color {
	return tcell.NewRGBColor(int32(rgb.R), int32(rgb.G), int32(rgb.B))
}
```

---

## Phase 2: Replace Buffer Internals

### 2.1 `render/cell.go` - New Cell Type

```go
// FILE: render/cell.go
package render

import "github.com/gdamore/tcell/v2"

// CompositorCell is the authoritative cell state
// Stores RGB colors directly, attributes preserved as tcell.AttrMask
type CompositorCell struct {
	Rune  rune
	Fg    RGB
	Bg    RGB
	Attrs tcell.AttrMask // Preserved exactly - includes bit 31 (AttrInvalid)
}

// Default background color (Tokyo Night)
var defaultBgRGB = RGB{26, 27, 38}

var emptyCell = CompositorCell{
	Rune:  ' ',
	Fg:    defaultBgRGB,
	Bg:    defaultBgRGB,
	Attrs: tcell.AttrNone,
}

// EmptyCell returns a copy of the empty cell sentinel
func EmptyCell() CompositorCell {
	return emptyCell
}
```

### 2.2 `render/buffer.go` - Compositor Implementation

```go
// FILE: render/buffer.go
package render

import "github.com/gdamore/tcell/v2"

// RenderBuffer is a compositor backed by RGB cells
// Single source of truth - all methods write to the same backing store
type RenderBuffer struct {
	cells  []CompositorCell
	width  int
	height int
}

// NewRenderBuffer creates a buffer with the specified dimensions
func NewRenderBuffer(width, height int) *RenderBuffer {
	size := width * height
	cells := make([]CompositorCell, size)
	for i := range cells {
		cells[i] = emptyCell
	}
	return &RenderBuffer{cells: cells, width: width, height: height}
}

// Resize adjusts buffer dimensions, reallocates only if capacity insufficient
func (b *RenderBuffer) Resize(width, height int) {
	size := width * height
	if cap(b.cells) < size {
		b.cells = make([]CompositorCell, size)
	} else {
		b.cells = b.cells[:size]
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
	b.cells[0] = emptyCell
	for filled := 1; filled < len(b.cells); filled *= 2 {
		copy(b.cells[filled:], b.cells[:filled])
	}
}

// Bounds returns buffer dimensions
func (b *RenderBuffer) Bounds() (width, height int) {
	return b.width, b.height
}

func (b *RenderBuffer) inBounds(x, y int) bool {
	return x >= 0 && x < b.width && y >= 0 && y < b.height
}

// =============================================================================
// COMPOSITOR API (New)
// =============================================================================

// SetPixel composites a pixel with specified blend mode
// mainRune: character to draw (0 preserves existing rune)
// fg, bg: foreground and background RGB colors
// mode: blending algorithm
// alpha: blend factor for BlendAlpha (0.0 = keep dst, 1.0 = use src)
// attrs: text attributes (preserved exactly)
func (b *RenderBuffer) SetPixel(x, y int, mainRune rune, fg, bg RGB, mode BlendMode, alpha float64, attrs tcell.AttrMask) {
	if !b.inBounds(x, y) {
		return
	}
	idx := y*b.width + x
	dst := &b.cells[idx]

	// Background blending
	switch mode {
	case BlendReplace:
		dst.Bg = bg
	case BlendAlpha:
		dst.Bg = dst.Bg.Blend(bg, alpha)
	case BlendAdd:
		dst.Bg = dst.Bg.Add(bg)
	case BlendMax:
		dst.Bg = dst.Bg.Max(bg)
	}

	// Foreground handling
	if mainRune != 0 {
		// New rune provided: set rune, fg, and attrs
		dst.Rune = mainRune
		dst.Fg = fg
		dst.Attrs = attrs
	} else {
		// No rune: blend effects only
		switch mode {
		case BlendAdd:
			dst.Fg = dst.Fg.Add(fg)
		case BlendMax:
			dst.Fg = dst.Fg.Max(fg)
		}
		// BlendReplace/BlendAlpha with rune=0: preserve existing fg/attrs
	}
}

// SetSolid is convenience for opaque BlendReplace
func (b *RenderBuffer) SetSolid(x, y int, mainRune rune, fg, bg RGB, attrs tcell.AttrMask) {
	b.SetPixel(x, y, mainRune, fg, bg, BlendReplace, 1.0, attrs)
}

// Get returns cell at (x,y), returns emptyCell on OOB
func (b *RenderBuffer) Get(x, y int) CompositorCell {
	if !b.inBounds(x, y) {
		return emptyCell
	}
	return b.cells[y*b.width+x]
}

// GetRGB returns fg and bg as RGB at (x,y)
func (b *RenderBuffer) GetRGB(x, y int) (fg, bg RGB, attrs tcell.AttrMask) {
	cell := b.Get(x, y)
	return cell.Fg, cell.Bg, cell.Attrs
}

// =============================================================================
// LEGACY BRIDGE API (Temporary - calls compositor internally)
// =============================================================================

// Set writes a rune and style (legacy API, converts tcell.Style to RGB)
func (b *RenderBuffer) Set(x, y int, r rune, style tcell.Style) {
	fgC, bgC, attrs := style.Decompose()
	b.SetPixel(x, y, r, TcellToRGB(fgC), TcellToRGB(bgC), BlendReplace, 1.0, attrs)
}

// SetRune writes a rune preserving existing colors and attrs
func (b *RenderBuffer) SetRune(x, y int, r rune) {
	if !b.inBounds(x, y) {
		return
	}
	b.cells[y*b.width+x].Rune = r
}

// SetString writes a string starting at (x, y), returns runes written
func (b *RenderBuffer) SetString(x, y int, s string, style tcell.Style) int {
	if y < 0 || y >= b.height {
		return 0
	}
	fgC, bgC, attrs := style.Decompose()
	fg, bg := TcellToRGB(fgC), TcellToRGB(bgC)
	written := 0
	for _, r := range s {
		if x >= b.width {
			break
		}
		if x >= 0 {
			b.SetPixel(x, y, r, fg, bg, BlendReplace, 1.0, attrs)
			written++
		}
		x++
	}
	return written
}

// SetMax writes with max-blend (legacy API for materializers/flashes)
func (b *RenderBuffer) SetMax(x, y int, r rune, style tcell.Style) {
	fgC, bgC, attrs := style.Decompose()
	b.SetPixel(x, y, r, TcellToRGB(fgC), TcellToRGB(bgC), BlendMax, 1.0, attrs)
}

// DecomposeAt returns fg, bg, attrs as tcell types (legacy bridge for reading)
func (b *RenderBuffer) DecomposeAt(x, y int) (fg, bg tcell.Color, attrs tcell.AttrMask) {
	cell := b.Get(x, y)
	return RGBToTcell(cell.Fg), RGBToTcell(cell.Bg), cell.Attrs
}

// =============================================================================
// OUTPUT
// =============================================================================

// Flush writes buffer contents to tcell.Screen
func (b *RenderBuffer) Flush(screen tcell.Screen) {
	for y := 0; y < b.height; y++ {
		row := y * b.width
		for x := 0; x < b.width; x++ {
			c := b.cells[row+x]
			style := tcell.StyleDefault.
				Foreground(RGBToTcell(c.Fg)).
				Background(RGBToTcell(c.Bg)).
				Attributes(c.Attrs)
			screen.SetContent(x, y, c.Rune, nil, style)
		}
	}
}
```

---

## Phase 3: Migrate Renderers

Renderers continue working unchanged via legacy bridge. Migration is optional but recommended for new compositor features.

### Migration Patterns

**Pattern A: Opaque Draw (Most Common)**
```go
// Before (legacy)
style := tcell.StyleDefault.Foreground(fg).Background(bg)
buf.Set(x, y, r, style)

// After (compositor)
buf.SetSolid(x, y, r, render.TcellToRGB(fg), render.TcellToRGB(bg), tcell.AttrNone)
```

**Pattern B: Preserve Existing Background (Decay, Drain, Characters)**
```go
// Before (anti-pattern: read-modify-write)
_, bg, _ := buf.DecomposeAt(x, y)
buf.Set(x, y, char, style.Background(bg))

// After (compositor: transparent background via alpha=0)
buf.SetPixel(x, y, char, fgRGB, render.RGBBlack, render.BlendAlpha, 0.0, attrs)
```

**Pattern C: Alpha Blending (Shields)**
```go
// Before (embedded blendColors method)
newBg := s.blendColors(bg, shieldColor, alpha)
buf.Set(x, y, cell.Rune, style.Background(newBg))

// After (compositor handles blend)
buf.SetPixel(x, y, 0, render.RGBBlack, shieldRGB, render.BlendAlpha, alpha, tcell.AttrNone)
```

**Pattern D: Max Blend (Materializers, Flashes)**
```go
// Before (legacy)
buf.SetMax(x, y, char, style)

// After (compositor)
buf.SetPixel(x, y, char, fgRGB, bgRGB, render.BlendMax, 1.0, attrs)
```

### Migration Order

Any order works since legacy API bridges to compositor. Suggested order by complexity:

| Priority | Renderer | Blend Pattern | Notes |
|----------|----------|---------------|-------|
| 1 | `PingGridRenderer` | Replace | Simplest, good validation |
| 2 | `LineNumbersRenderer` | Replace | UI, no blending |
| 3 | `ColumnIndicatorsRenderer` | Replace | UI, no blending |
| 4 | `StatusBarRenderer` | Replace | UI, no blending |
| 5 | `HeatMeterRenderer` | Replace | UI, no blending |
| 6 | `DrainRenderer` | Alpha (bg preserve) | Uses DecomposeAt |
| 7 | `CharactersRenderer` | Alpha (bg preserve) | Uses DecomposeAt, attrs |
| 8 | `EffectsRenderer` | Alpha + Max | Most complex, multiple patterns |
| 9 | `ShieldRenderer` | Alpha | Delete embedded blendColors |
| 10 | `CursorRenderer` | Replace | Special z-order logic |
| 11 | `OverlayRenderer` | Replace | Highest z, opaque |

---

## Phase 4: Cleanup

### 4.1 Convert `render/colors.go` to RGB

```go
// Before
RgbBackground = tcell.NewRGBColor(26, 27, 38)

// After
RgbBackground = RGB{26, 27, 38}

// Keep tcell helpers for components that store tcell.Style (e.g., Character component)
func (rgb RGB) ToTcell() tcell.Color {
    return tcell.NewRGBColor(int32(rgb.R), int32(rgb.G), int32(rgb.B))
}
```

### 4.2 Delete `render/bridge.go`

All conversion now happens in `colors.go` via `RGB.ToTcell()` method.

### 4.3 Remove Legacy Buffer Methods

Delete from `render/buffer.go`:
- `Set(x, y int, r rune, style tcell.Style)`
- `SetMax(x, y int, r rune, style tcell.Style)`
- `SetString(x, y int, s string, style tcell.Style) int`
- `DecomposeAt(x, y int) (fg, bg tcell.Color, attrs tcell.AttrMask)`

### 4.4 Optional: Update `SystemRenderer` Interface

```go
// Current (unchanged during migration)
type SystemRenderer interface {
    Render(ctx RenderContext, world *engine.World, buf *RenderBuffer)
}

// Optional future change (if full decoupling desired)
type Compositor interface {
    SetPixel(x, y int, mainRune rune, fg, bg RGB, mode BlendMode, alpha float64, attrs tcell.AttrMask)
    SetSolid(x, y int, mainRune rune, fg, bg RGB, attrs tcell.AttrMask)
    Get(x, y int) CompositorCell
    // ...
}

type SystemRenderer interface {
    Render(ctx RenderContext, world *engine.World, comp Compositor)
}
```

---

## File Summary

| Phase | Action | File | Description |
|-------|--------|------|-------------|
| 1 | Create | `render/color.go` | RGB type, blend methods, BlendMode enum |
| 1 | Create | `render/bridge.go` | TcellToRGB, RGBToTcell (temporary) |
| 2 | Rewrite | `render/cell.go` | CompositorCell with RGB + tcell.AttrMask |
| 2 | Rewrite | `render/buffer.go` | Compositor + legacy bridge methods |
| 3 | Modify | `render/renderers/*.go` | Migrate to SetPixel API |
| 4 | Modify | `render/colors.go` | Convert tcell.Color vars to RGB |
| 4 | Delete | `render/bridge.go` | Remove after all conversions inline |
| 4 | Modify | `render/buffer.go` | Remove legacy methods |

---

## Verification Checkpoints

| Phase | Verification |
|-------|--------------|
| 1 | Unit tests: RGB.Blend, RGB.Max, RGB.Add |
| 2 | All existing tests pass unchanged |
| 2 | Visual: no rendering differences |
| 3 | Per-renderer: visual regression check |
| 4 | Compile: no tcell imports in render/ except buffer.go Flush |

---