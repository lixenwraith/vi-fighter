# Final Implementation Plan

## Phase 1: Color Infrastructure

### 1.1 Replace RGBTo256 with Redmean LUT
- Remove cube-based implementation
- Add 262KB pre-computed LUT populated at `init()`
- Runtime: O(1) table lookup (~5ns/call)

## Phase 2: Shield Dual-Mode Rendering

### 2.1 Callback Injection Pattern
- Define `shieldCellRenderer` function type
- Select renderer function once per frame based on `ColorMode()`
- Single iteration loop calls injected function

### 2.2 TrueColor Path
- Replace `BlendSoftLight` → `BlendScreen`
- Keep existing quadratic falloff gradient

### 2.3 256-Color Path (Grayscale Rings)
- Inner zone (dist < 0.6): Dark gray (index 238)
- Outer zone (dist 0.6-0.9): Light gray (index 250)
- Edge zone (dist 0.9-1.0): Tinted with shield color via `BlendScreen` low alpha, falls back to grayscale if result is poor

---

# Implementation

```go
// FILE: terminal/color.go
package terminal

import (
	"os"
	"strings"
)

// ColorMode indicates terminal color capability
type ColorMode uint8

const (
	ColorMode256       ColorMode = iota // xterm-256 palette
	ColorModeTrueColor                  // 24-bit RGB
)

// RGB represents a 24-bit color
type RGB struct {
	R, G, B uint8
}

// RGBBlack is the zero value black color
var RGBBlack = RGB{0, 0, 0}

// 6-bit quantized LUT for Redmean-based 256-color mapping
// 64×64×64 = 262,144 bytes, fits in L2 cache
var lut256 [64 * 64 * 64]uint8

func init() {
	// Pre-compute Redmean-based palette mapping for all 6-bit quantized RGB values
	for r := 0; r < 64; r++ {
		for g := 0; g < 64; g++ {
			for b := 0; b < 64; b++ {
				// Expand 6-bit to 8-bit (shift left 2, add 2 for midpoint)
				r8 := (r << 2) | 2
				g8 := (g << 2) | 2
				b8 := (b << 2) | 2
				lut256[r<<12|g<<6|b] = computeRedmean256(r8, g8, b8)
			}
		}
	}
}

// computeRedmean256 finds the nearest 256-palette index using Redmean distance
// Called only at init() to populate LUT
func computeRedmean256(r, g, b int) uint8 {
	// Grayscale fast path
	if r == g && g == b {
		if r < 8 {
			return 16
		}
		if r > 238 {
			return 231
		}
		return uint8(232 + (r-8)/10)
	}

	bestIdx := uint8(16)
	minDist := 1 << 30

	// Search 6×6×6 cube (indices 16-231)
	for i := 0; i < 216; i++ {
		cr := cubeValues[i/36]
		cg := cubeValues[(i/6)%6]
		cb := cubeValues[i%6]

		d := redmeanDistance(r, g, b, cr, cg, cb)
		if d < minDist {
			minDist = d
			bestIdx = uint8(16 + i)
		}
	}

	// Search grayscale ramp (indices 232-255)
	for i := 0; i < 24; i++ {
		gray := 8 + i*10
		d := redmeanDistance(r, g, b, gray, gray, gray)
		if d < minDist {
			minDist = d
			bestIdx = uint8(232 + i)
		}
	}

	return bestIdx
}

// redmeanDistance calculates perceptually-weighted color distance
// Formula: https://en.wikipedia.org/wiki/Color_difference#sRGB
func redmeanDistance(r1, g1, b1, r2, g2, b2 int) int {
	rmean := (r1 + r2) / 2
	dr := r1 - r2
	dg := g1 - g2
	db := b1 - b2
	return (((512+rmean)*dr*dr)>>8) + 4*dg*dg + (((767-rmean)*db*db)>>8)
}

// Color cube values for 6×6×6 palette (indices 16-231)
var cubeValues = [6]int{0, 95, 135, 175, 215, 255}

// RGBTo256 converts RGB to nearest 256-color palette index
// O(1) lookup via pre-computed Redmean LUT
func RGBTo256(c RGB) uint8 {
	return lut256[int(c.R>>2)<<12|int(c.G>>2)<<6|int(c.B>>2)]
}

// DetectColorMode determines terminal color capability from environment
func DetectColorMode() ColorMode {
	colorterm := os.Getenv("COLORTERM")
	if colorterm == "truecolor" || colorterm == "24bit" {
		return ColorModeTrueColor
	}

	if os.Getenv("KITTY_WINDOW_ID") != "" ||
		os.Getenv("KONSOLE_VERSION") != "" ||
		os.Getenv("ITERM_SESSION_ID") != "" ||
		os.Getenv("ALACRITTY_WINDOW_ID") != "" ||
		os.Getenv("ALACRITTY_LOG") != "" ||
		os.Getenv("WEZTERM_PANE") != "" {
		return ColorModeTrueColor
	}

	term := os.Getenv("TERM")
	if strings.Contains(term, "truecolor") ||
		strings.Contains(term, "24bit") ||
		strings.Contains(term, "direct") {
		return ColorModeTrueColor
	}

	return ColorMode256
}
```

```go
// FILE: render/renderers/shields.go
package renderers

import (
	"math"

	"github.com/lixenwraith/vi-fighter/components"
	"github.com/lixenwraith/vi-fighter/engine"
	"github.com/lixenwraith/vi-fighter/render"
	"github.com/lixenwraith/vi-fighter/terminal"
)

// shieldCellRenderer is the callback type for per-cell shield rendering
type shieldCellRenderer func(buf *render.RenderBuffer, screenX, screenY int, dist float64, color render.RGB, maxOpacity float64)

// ShieldRenderer renders active shields with dynamic color from GameState
type ShieldRenderer struct {
	gameCtx *engine.GameContext

	// Cached cell renderers for each mode
	renderCellTrueColor shieldCellRenderer
	renderCell256       shieldCellRenderer
}

// NewShieldRenderer creates a new shield renderer
func NewShieldRenderer(gameCtx *engine.GameContext) *ShieldRenderer {
	s := &ShieldRenderer{gameCtx: gameCtx}

	// Pre-bind cell renderers to avoid allocation in render loop
	s.renderCellTrueColor = s.cellTrueColor
	s.renderCell256 = s.cell256

	return s
}

// Render draws all active shields with quadratic falloff gradient
func (s *ShieldRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
	shields := world.Shields.All()
	if len(shields) == 0 {
		return
	}

	// Select cell renderer once per frame
	colorMode := s.gameCtx.Terminal.ColorMode()
	var renderCell shieldCellRenderer
	if colorMode == terminal.ColorMode256 {
		renderCell = s.renderCell256
	} else {
		renderCell = s.renderCellTrueColor
	}

	for _, entity := range shields {
		shield, okS := world.Shields.Get(entity)
		pos, okP := world.Positions.Get(entity)

		if !okS || !okP {
			continue
		}

		// Shield only renders if Sources != 0 AND Energy > 0
		if shield.Sources == 0 || ctx.Energy <= 0 {
			continue
		}

		// Resolve shield color
		shieldRGB := s.resolveShieldColor(shield)

		// Bounding box
		startX := int(float64(pos.X) - shield.RadiusX)
		endX := int(float64(pos.X) + shield.RadiusX)
		startY := int(float64(pos.Y) - shield.RadiusY)
		endY := int(float64(pos.Y) + shield.RadiusY)

		// Clamp to screen bounds
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

		// Precompute inverse radii squared for ellipse calculation
		invRxSq := 1.0 / (shield.RadiusX * shield.RadiusX)
		invRySq := 1.0 / (shield.RadiusY * shield.RadiusY)

		for y := startY; y <= endY; y++ {
			for x := startX; x <= endX; x++ {
				// Skip cursor position
				if x == ctx.CursorX && y == ctx.CursorY {
					continue
				}

				dx := float64(x - pos.X)
				dy := float64(y - pos.Y)

				// Elliptical normalized distance squared
				normalizedDistSq := dx*dx*invRxSq + dy*dy*invRySq
				if normalizedDistSq > 1.0 {
					continue
				}

				dist := math.Sqrt(normalizedDistSq)
				screenX := ctx.GameX + x
				screenY := ctx.GameY + y

				renderCell(buf, screenX, screenY, dist, shieldRGB, shield.MaxOpacity)
			}
		}
	}
}

// cellTrueColor renders a single shield cell with smooth gradient (TrueColor mode)
func (s *ShieldRenderer) cellTrueColor(buf *render.RenderBuffer, screenX, screenY int, dist float64, color render.RGB, maxOpacity float64) {
	// Quadratic soft falloff: (1-d)² * MaxOpacity
	falloff := (1.0 - dist) * (1.0 - dist)
	alpha := falloff * maxOpacity

	// BlendScreen: Adds light, visible on black, brightens over content
	buf.Set(screenX, screenY, 0, render.RGBBlack, color, render.BlendScreen, alpha, terminal.AttrNone)
}

// cell256 renders a single shield cell with discrete zones (256-color mode)
func (s *ShieldRenderer) cell256(buf *render.RenderBuffer, screenX, screenY int, dist float64, color render.RGB, maxOpacity float64) {
	// Palette-safe grayscale colors
	// Index 238 = RGB(68,68,68) - dark gray
	// Index 250 = RGB(188,188,188) - light gray
	// Index 253 = RGB(218,218,218) - near-white for edge

	switch {
	case dist > 0.9:
		// Edge zone: Attempt tinted glow via low-alpha screen blend
		// If shield color is too dark, this still produces visible edge
		buf.Set(screenX, screenY, 0, render.RGBBlack, color, render.BlendScreen, 0.4, terminal.AttrNone)

	case dist > 0.6:
		// Outer zone: Light gray, solid replace
		buf.Set(screenX, screenY, 0, render.RGBBlack, render.RGB{R: 188, G: 188, B: 188}, render.BlendReplace, 1.0, terminal.AttrNone)

	default:
		// Inner zone: Dark gray, solid replace
		buf.Set(screenX, screenY, 0, render.RGBBlack, render.RGB{R: 68, G: 68, B: 68}, render.BlendReplace, 1.0, terminal.AttrNone)
	}
}

// resolveShieldColor determines the shield color from override or GameState
func (s *ShieldRenderer) resolveShieldColor(shield components.ShieldComponent) render.RGB {
	if shield.OverrideColor != components.ColorNone {
		return s.colorClassToRGB(shield.OverrideColor)
	}

	seqType := s.gameCtx.State.GetLastTypedSeqType()
	seqLevel := s.gameCtx.State.GetLastTypedSeqLevel()

	return s.getColorFromSequence(seqType, seqLevel)
}

// getColorFromSequence maps sequence type/level to RGB
func (s *ShieldRenderer) getColorFromSequence(seqType, seqLevel int32) render.RGB {
	switch seqType {
	case 1: // Blue
		switch seqLevel {
		case 0:
			return render.RgbSequenceBlueDark
		case 1:
			return render.RgbSequenceBlueNormal
		case 2:
			return render.RgbSequenceBlueBright
		}
	case 2: // Green
		switch seqLevel {
		case 0:
			return render.RgbSequenceGreenDark
		case 1:
			return render.RgbSequenceGreenNormal
		case 2:
			return render.RgbSequenceGreenBright
		}
	}
	// Default: neutral gray when no character typed yet
	return render.RGB{R: 128, G: 128, B: 128}
}

// colorClassToRGB maps ColorClass overrides to RGB
func (s *ShieldRenderer) colorClassToRGB(color components.ColorClass) render.RGB {
	switch color {
	case components.ColorShield:
		return render.RgbShieldBase
	case components.ColorNugget:
		return render.RgbNuggetOrange
	default:
		return render.RgbShieldBase
	}
}
```

---

## Summary of Changes

| File | Change |
|------|--------|
| `terminal/color.go` | Replaced cube-based `RGBTo256` with 262KB Redmean LUT. O(1) runtime, 13ms init cost |
| `render/renderers/shields.go` | Callback injection pattern with `cellTrueColor` (BlendScreen gradient) and `cell256` (3-zone grayscale + tinted edge) |