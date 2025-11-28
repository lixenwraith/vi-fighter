## Prompt 0: Create Reference Document

```markdown
# Task: Create Renderer Migration Reference Document

Create `RENDER_MIGRATION.md` at the repository root with the following content exactly:

---

# Renderer Migration Reference

## Status Tracking

| Phase | Status | Notes |
|-------|--------|-------|
| Phase 0 | ✅ Complete | Foundation types in `render/` |
| Phase 1 | Pending | Hybrid integration |
| Phase 2 | Pending | Legacy renderer modification |
| Phase 3 | Pending | Incremental extraction |
| Final | Pending | Cleanup |

## Package Structure

```
render/
├── priority.go      # RenderPriority type and constants
├── context.go       # RenderContext struct
├── cell.go          # RenderCell, emptyCell, EmptyCell()
├── buffer.go        # RenderBuffer implementation
├── interface.go     # SystemRenderer, VisibilityToggle interfaces
├── orchestrator.go  # RenderOrchestrator
├── buffer_screen.go # BufferScreen shim
├── legacy_adapter.go # LegacyAdapter, LegacyRenderer interface
├── terminal_renderer.go # Legacy monolithic renderer (to be deprecated)
└── colors.go        # Color constants (unchanged)
```

## Core Type Signatures

```go
// render/priority.go
type RenderPriority int
const (
    PriorityBackground RenderPriority = 0
    PriorityGrid       RenderPriority = 100
    PriorityEntities   RenderPriority = 200
    PriorityEffects    RenderPriority = 300
    PriorityDrain      RenderPriority = 350
    PriorityUI         RenderPriority = 400
    PriorityOverlay    RenderPriority = 500
    PriorityDebug      RenderPriority = 1000
)

// render/context.go
type RenderContext struct {
    GameTime    time.Time
    FrameNumber int64
    DeltaTime   float64
    IsPaused    bool
    CursorX, CursorY int
    GameX, GameY, GameWidth, GameHeight int
    Width, Height int
}

// render/interface.go
type SystemRenderer interface {
    Render(ctx RenderContext, world *engine.World, buf *RenderBuffer)
}

type VisibilityToggle interface {
    IsVisible() bool
}

// render/buffer_screen.go
type BufferScreen struct { buf *RenderBuffer }
func NewBufferScreen(buf *RenderBuffer) *BufferScreen
func (bs *BufferScreen) SetContent(x, y int, primary rune, combining []rune, style tcell.Style)
func (bs *BufferScreen) GetContent(x, y int) (rune, []rune, tcell.Style, int)
func (bs *BufferScreen) Size() (int, int)

// render/legacy_adapter.go
type LegacyRenderer interface {
    RenderFrameToScreen(ctx *engine.GameContext, screen *BufferScreen)
}

type LegacyAdapter struct { ... }
func NewLegacyAdapter(renderer LegacyRenderer, gameCtx *engine.GameContext) *LegacyAdapter
func (l *LegacyAdapter) Render(ctx RenderContext, world *engine.World, buf *RenderBuffer)

// render/orchestrator.go
type RenderOrchestrator struct { ... }
func NewRenderOrchestrator(screen tcell.Screen, width, height int) *RenderOrchestrator
func (o *RenderOrchestrator) Register(r SystemRenderer, priority RenderPriority)
func (o *RenderOrchestrator) Resize(width, height int)
func (o *RenderOrchestrator) RenderFrame(ctx RenderContext, world *engine.World)
func (o *RenderOrchestrator) Buffer() *RenderBuffer
```

## Legacy Renderer Draw Functions (Migration Candidates)

| Function | Priority | Uses Decompose | Extraction Phase |
|----------|----------|----------------|------------------|
| `drawHeatMeter` | UI (400) | No | 3a |
| `drawLineNumbers` | UI (400) | No | 3a |
| `drawPingHighlights` | Grid (100) | No | 3b |
| `drawPingGrid` | Grid (100) | Yes | 3b |
| `drawShields` | Effects (300) | Yes | 3c |
| `drawCharacters` | Entities (200) | Yes | 3d |
| `drawDecay` | Effects (300) | Yes | 3e |
| `drawCleaners` | Effects (300) | No | 3e |
| `drawRemovalFlashes` | Effects (300) | Yes | 3e |
| `drawMaterializers` | Effects (300) | Yes | 3e |
| `drawDrain` | Drain (350) | Yes | 3f |
| `drawColumnIndicators` | UI (400) | No | 3a |
| `drawStatusBar` | UI (400) | No | 3a |
| `drawCursor` | UI (400) | Yes | 3g |
| `drawOverlay` | Overlay (500) | No | 3h |

## Current Main Loop Location

File: `main.go`, function: `runGameLoop`

Current render call:
```go
renderer.RenderFrame(ctx, decaySystem.IsAnimating(timeRes.GameTime), decaySystem.GetTimeUntilDecay(timeRes.GameTime))
```

## screenWriter Interface (Phase 2)

For legacy draw functions to work with both `tcell.Screen` and `BufferScreen`:

```go
type screenWriter interface {
    SetContent(x, y int, primary rune, combining []rune, style tcell.Style)
    GetContent(x, y int) (primary rune, combining []rune, style tcell.Style, width int)
    Size() (width, height int)
}
```

## Migration Rules

1. New renderers go in `render/renderers/` subdirectory
2. Each renderer is a single file named after its function (e.g., `heat_meter.go`)
3. Renderer structs embed required state, not GameContext pointer
4. Renderers receive `RenderContext` by value, `*engine.World` and `*RenderBuffer` by pointer
5. Use `buf.DecomposeAt()` instead of screen.GetContent + Decompose pattern
6. No direct tcell.Screen access in new renderers

---

Update the Phase status as each phase completes.
```

---

## Prompt 1: Phase 1 - Hybrid Integration

```markdown
# Task: Renderer Migration Phase 1 - Hybrid Integration

Read `RENDER_MIGRATION.md` at the repo root for type signatures and context.

## Objective

Wire `RenderOrchestrator` into the main game loop. The legacy `TerminalRenderer` continues to do all rendering through `LegacyAdapter`.

## Files to Modify

### 1. `main.go`

In `runGameLoop` function:

**Add orchestrator creation after renderer creation:**

```go
// After: renderer := render.NewTerminalRenderer(...)

// Create render orchestrator
orchestrator := render.NewRenderOrchestrator(
    screen,
    ctx.Width,
    ctx.Height,
)
```

**Replace the render call in the frame ticker case:**

Find:
```go
renderer.RenderFrame(ctx, decaySystem.IsAnimating(timeRes.GameTime), decaySystem.GetTimeUntilDecay(timeRes.GameTime))
```

Replace with:
```go
// Build render context
cursorPos, _ := ctx.World.Positions.Get(ctx.CursorEntity)
renderCtx := render.RenderContext{
    GameTime:    timeRes.GameTime,
    FrameNumber: timeRes.FrameNumber,
    DeltaTime:   timeRes.DeltaTime.Seconds(),
    IsPaused:    ctx.IsPaused.Load(),
    CursorX:     cursorPos.X,
    CursorY:     cursorPos.Y,
    GameX:       ctx.GameX,
    GameY:       ctx.GameY,
    GameWidth:   ctx.GameWidth,
    GameHeight:  ctx.GameHeight,
    Width:       ctx.Width,
    Height:      ctx.Height,
}

orchestrator.RenderFrame(renderCtx, ctx.World)
```

**Note:** The orchestrator has no renderers registered yet, so the screen will be blank. This is expected - Phase 2 will wire the legacy adapter.

**Add resize handling in the event case:**

Find the block that calls `renderer.UpdateDimensions(...)`.

Add after it:
```go
orchestrator.Resize(ctx.Width, ctx.Height)
```

### 2. `render/context.go`

Add helper method to build context from GameContext (reduces main.go coupling):

```go
// NewRenderContextFromGame creates a RenderContext from engine.GameContext and TimeResource.
func NewRenderContextFromGame(ctx *engine.GameContext, timeRes *engine.TimeResource, cursorX, cursorY int) RenderContext {
    return RenderContext{
        GameTime:    timeRes.GameTime,
        FrameNumber: timeRes.FrameNumber,
        DeltaTime:   timeRes.DeltaTime.Seconds(),
        IsPaused:    ctx.IsPaused.Load(),
        CursorX:     cursorX,
        CursorY:     cursorY,
        GameX:       ctx.GameX,
        GameY:       ctx.GameY,
        GameWidth:   ctx.GameWidth,
        GameHeight:  ctx.GameHeight,
        Width:       ctx.Width,
        Height:      ctx.Height,
    }
}
```

Add import for `"github.com/lixenwraith/vi-fighter/engine"`.

Then in main.go, the render context creation simplifies to:
```go
cursorPos, _ := ctx.World.Positions.Get(ctx.CursorEntity)
renderCtx := render.NewRenderContextFromGame(ctx, timeRes, cursorPos.X, cursorPos.Y)
orchestrator.RenderFrame(renderCtx, ctx.World)
```

## Verification

1. `go build .` succeeds
2. Game launches but screen is blank/black (expected - no renderers registered)
3. No panics on resize

## After Completion

Update `RENDER_MIGRATION.md`: Change Phase 1 status to `✅ Complete`.
```

---

## Prompt 2: Phase 2 - Legacy Renderer Modification

```markdown
# Task: Renderer Migration Phase 2 - Legacy Renderer Modification

Read `RENDER_MIGRATION.md` at the repo root for type signatures and context.

## Objective

Modify `TerminalRenderer` to implement `LegacyRenderer` interface. Wire it through `LegacyAdapter` so the game renders correctly again.

## Files to Modify

### 1. `render/terminal_renderer.go`

**Add screenWriter interface at package level (before TerminalRenderer struct):**

```go
// screenWriter abstracts tcell.Screen for buffer compatibility.
type screenWriter interface {
    SetContent(x, y int, primary rune, combining []rune, style tcell.Style)
    GetContent(x, y int) (primary rune, combining []rune, style tcell.Style, width int)
    Size() (width, height int)
}
```

**Modify TerminalRenderer struct:**

Add a new field:
```go
type TerminalRenderer struct {
    // ... existing fields ...
    
    // For legacy adapter compatibility
    decayAnimating     bool
    decayTimeRemaining float64
}
```

**Add new method `RenderFrameToScreen`:**

This method is nearly identical to existing `RenderFrame`, but:
- Takes `*BufferScreen` instead of using `r.screen`
- Uses a local `screenWriter` variable for all draw calls

```go
// RenderFrameToScreen renders to a BufferScreen for orchestrator integration.
// Implements LegacyRenderer interface.
func (r *TerminalRenderer) RenderFrameToScreen(ctx *engine.GameContext, screen *BufferScreen) {
    // Use interface to allow both tcell.Screen and BufferScreen
    var sw screenWriter = screen
    r.renderToWriter(ctx, sw)
}

// renderToWriter is the shared implementation for both render paths.
func (r *TerminalRenderer) renderToWriter(ctx *engine.GameContext, sw screenWriter) {
    // FPS Calculation
    r.frameCount++
    now := time.Now()
    if now.Sub(r.lastFpsUpdate) >= time.Second {
        r.currentFps = r.frameCount
        r.frameCount = 0
        r.lastFpsUpdate = now
    }

    // Increment frame counter
    ctx.IncrementFrameNumber()

    // Clear via setting each cell (BufferScreen has no Clear method)
    defaultStyle := tcell.StyleDefault.Background(RgbBackground)
    width, height := sw.Size()
    for y := 0; y < height; y++ {
        for x := 0; x < width; x++ {
            sw.SetContent(x, y, ' ', nil, defaultStyle)
        }
    }

    // Draw heat meter
    r.drawHeatMeterTo(ctx.State.GetHeat(), defaultStyle, sw)

    // Read cursor position
    cursorPos, ok := ctx.World.Positions.Get(ctx.CursorEntity)
    if !ok {
        panic(fmt.Errorf("cursor destroyed"))
    }

    // Draw line numbers
    r.drawLineNumbersTo(cursorPos.Y, ctx, defaultStyle, sw)

    // Get ping color
    pingColor := r.getPingColorTo(ctx.World, cursorPos.X, cursorPos.Y, ctx, sw)

    // Draw ping highlights and grid
    r.drawPingHighlightsTo(cursorPos.X, cursorPos.Y, ctx, pingColor, defaultStyle, sw)

    // Draw shields
    r.drawShieldsTo(ctx.World, sw)

    // Draw characters
    r.drawCharactersTo(ctx.World, cursorPos.X, cursorPos.Y, pingColor, defaultStyle, ctx, sw)

    // Draw decay
    if r.decayAnimating {
        r.drawDecayTo(ctx.World, defaultStyle, sw)
    }

    // Draw cleaners
    r.drawCleanersTo(ctx.World, defaultStyle, sw)

    // Draw removal flashes
    r.drawRemovalFlashesTo(ctx.World, ctx, defaultStyle, sw)

    // Draw materializers
    r.drawMaterializersTo(ctx.World, defaultStyle, sw)

    // Draw drain
    r.drawDrainTo(ctx.World, defaultStyle, sw)

    // Draw column indicators
    r.drawColumnIndicatorsTo(cursorPos.X, ctx, defaultStyle, sw)

    // Draw status bar
    r.drawStatusBarTo(ctx, defaultStyle, r.decayTimeRemaining, sw)

    // Draw cursor
    if !ctx.IsSearchMode() && !ctx.IsCommandMode() {
        r.drawCursorTo(cursorPos.X, cursorPos.Y, ctx, defaultStyle, sw)
    }

    // Draw overlay
    if ctx.IsOverlayMode() && ctx.OverlayActive {
        r.drawOverlayTo(ctx, defaultStyle, sw)
    }
}
```

**Modify existing RenderFrame to use renderToWriter:**

```go
// RenderFrame renders the entire game frame to tcell.Screen.
func (r *TerminalRenderer) RenderFrame(ctx *engine.GameContext, decayAnimating bool, decayTimeRemaining float64) {
    r.decayAnimating = decayAnimating
    r.decayTimeRemaining = decayTimeRemaining
    
    r.screen.Clear()
    r.renderToWriter(ctx, r.screen)
    r.screen.Show()
}
```

**Create `*To` variants of all draw functions:**

For each `drawX` function, create a `drawXTo` variant that takes `screenWriter` as final parameter instead of using `r.screen`.

Example pattern for `drawHeatMeter`:

```go
func (r *TerminalRenderer) drawHeatMeterTo(heat int, defaultStyle tcell.Style, sw screenWriter) {
    // Same logic as drawHeatMeter, but replace r.screen with sw
    // ...
    sw.SetContent(x, 0, '█', nil, style)
    // ...
}

// Keep original for backward compatibility during transition
func (r *TerminalRenderer) drawHeatMeter(heat int, defaultStyle tcell.Style) {
    r.drawHeatMeterTo(heat, defaultStyle, r.screen)
}
```

Apply this pattern to ALL draw functions:
- `drawHeatMeter` → `drawHeatMeterTo`
- `drawLineNumbers` → `drawLineNumbersTo`
- `drawPingHighlights` → `drawPingHighlightsTo`
- `drawPingGrid` → `drawPingGridTo`
- `drawShields` → `drawShieldsTo`
- `drawCharacters` → `drawCharactersTo`
- `drawDecay` → `drawDecayTo`
- `drawCleaners` → `drawCleanersTo`
- `drawRemovalFlashes` → `drawRemovalFlashesTo`
- `drawMaterializers` → `drawMaterializersTo`
- `drawDrain` → `drawDrainTo`
- `drawColumnIndicators` → `drawColumnIndicatorsTo`
- `drawStatusBar` → `drawStatusBarTo`
- `drawCursor` → `drawCursorTo`
- `drawOverlay` → `drawOverlayTo`
- `getPingColor` → `getPingColorTo`

**Important:** For functions that call other draw functions (like `drawPingHighlights` calling `drawPingGrid`), the `*To` variant must call the `*To` variant of the sub-function.

### 2. `main.go`

**Wire the legacy adapter:**

After creating the orchestrator:
```go
// Create legacy adapter and register with orchestrator
legacyAdapter := render.NewLegacyAdapter(renderer, ctx)
orchestrator.Register(legacyAdapter, render.PriorityBackground)
```

**Pass decay state to renderer before orchestrator call:**

```go
// Update legacy renderer state
renderer.SetDecayState(decaySystem.IsAnimating(timeRes.GameTime), decaySystem.GetTimeUntilDecay(timeRes.GameTime))

// Build render context and render
cursorPos, _ := ctx.World.Positions.Get(ctx.CursorEntity)
renderCtx := render.NewRenderContextFromGame(ctx, timeRes, cursorPos.X, cursorPos.Y)
orchestrator.RenderFrame(renderCtx, ctx.World)
```

### 3. `render/terminal_renderer.go` (additional method)

Add setter for decay state:
```go
// SetDecayState updates decay animation state for legacy rendering.
func (r *TerminalRenderer) SetDecayState(animating bool, timeRemaining float64) {
    r.decayAnimating = animating
    r.decayTimeRemaining = timeRemaining
}
```

## Verification

1. `go build .` succeeds
2. Game launches and renders correctly (identical to before migration)
3. All visual elements appear: heat meter, characters, cursor, overlays
4. Resize works correctly

## After Completion

Update `RENDER_MIGRATION.md`: Change Phase 2 status to `✅ Complete`.
```

---

## Prompt 3: Phase 3a - Extract UI Renderers

```markdown
# Task: Renderer Migration Phase 3a - Extract UI Renderers

Read `RENDER_MIGRATION.md` at the repo root for type signatures and context.

## Objective

Extract `drawHeatMeter`, `drawLineNumbers`, `drawColumnIndicators`, and `drawStatusBar` into standalone SystemRenderer implementations.

## Files to Create

### 1. `render/renderers/heat_meter.go`

```go
package renderers

import (
    "github.com/gdamore/tcell/v2"
    "github.com/lixenwraith/vi-fighter/constants"
    "github.com/lixenwraith/vi-fighter/engine"
    "github.com/lixenwraith/vi-fighter/render"
)

// HeatMeterRenderer draws the heat meter bar at the top of the screen.
type HeatMeterRenderer struct {
    state *engine.GameState
}

// NewHeatMeterRenderer creates a heat meter renderer.
func NewHeatMeterRenderer(state *engine.GameState) *HeatMeterRenderer {
    return &HeatMeterRenderer{state: state}
}

// Render implements SystemRenderer.
func (h *HeatMeterRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
    heat := h.state.GetHeat()
    defaultStyle := tcell.StyleDefault.Background(render.RgbBackground)
    
    displayHeat := int(float64(heat) / float64(constants.MaxHeat) * 10.0)
    if displayHeat > 10 {
        displayHeat = 10
    }
    if displayHeat < 0 {
        displayHeat = 0
    }

    segmentWidth := float64(ctx.Width) / 10.0
    for segment := 0; segment < 10; segment++ {
        segmentStart := int(float64(segment) * segmentWidth)
        segmentEnd := int(float64(segment+1) * segmentWidth)

        isFilled := segment < displayHeat

        var style tcell.Style
        if isFilled {
            progress := float64(segment+1) / 10.0
            color := render.GetHeatMeterColor(progress)
            style = defaultStyle.Foreground(color)
        } else {
            style = defaultStyle.Foreground(render.RgbBlack)
        }

        for x := segmentStart; x < segmentEnd && x < ctx.Width; x++ {
            buf.Set(x, 0, '█', style)
        }
    }
}
```

### 2. `render/renderers/line_numbers.go`

```go
package renderers

import (
    "fmt"
    
    "github.com/gdamore/tcell/v2"
    "github.com/lixenwraith/vi-fighter/engine"
    "github.com/lixenwraith/vi-fighter/render"
)

// LineNumbersRenderer draws relative line numbers.
type LineNumbersRenderer struct {
    lineNumWidth int
    gameCtx      *engine.GameContext
}

// NewLineNumbersRenderer creates a line numbers renderer.
func NewLineNumbersRenderer(lineNumWidth int, gameCtx *engine.GameContext) *LineNumbersRenderer {
    return &LineNumbersRenderer{
        lineNumWidth: lineNumWidth,
        gameCtx:      gameCtx,
    }
}

// Render implements SystemRenderer.
func (l *LineNumbersRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
    defaultStyle := tcell.StyleDefault.Background(render.RgbBackground)
    lineNumStyle := defaultStyle.Foreground(render.RgbLineNumbers)

    for y := 0; y < ctx.GameHeight; y++ {
        relativeNum := y - ctx.CursorY
        if relativeNum < 0 {
            relativeNum = -relativeNum
        }
        lineNum := fmt.Sprintf("%*d", l.lineNumWidth, relativeNum)

        var numStyle tcell.Style
        if relativeNum == 0 {
            if l.gameCtx.IsSearchMode() || l.gameCtx.IsCommandMode() {
                numStyle = defaultStyle.Foreground(render.RgbLineNumbersSearch)
            } else {
                numStyle = defaultStyle.Foreground(render.RgbLineNumbersCursor)
            }
        } else {
            numStyle = lineNumStyle
        }

        screenY := ctx.GameY + y
        for i, ch := range lineNum {
            buf.Set(i, screenY, ch, numStyle)
        }
    }
}

// UpdateLineNumWidth updates the line number column width.
func (l *LineNumbersRenderer) UpdateLineNumWidth(width int) {
    l.lineNumWidth = width
}
```

### 3. `render/renderers/column_indicators.go`

```go
package renderers

import (
    "fmt"
    
    "github.com/gdamore/tcell/v2"
    "github.com/lixenwraith/vi-fighter/engine"
    "github.com/lixenwraith/vi-fighter/render"
)

// ColumnIndicatorsRenderer draws column position indicators.
type ColumnIndicatorsRenderer struct{}

// NewColumnIndicatorsRenderer creates a column indicators renderer.
func NewColumnIndicatorsRenderer() *ColumnIndicatorsRenderer {
    return &ColumnIndicatorsRenderer{}
}

// Render implements SystemRenderer.
func (c *ColumnIndicatorsRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
    defaultStyle := tcell.StyleDefault.Background(render.RgbBackground)
    indicatorStyle := defaultStyle.Foreground(render.RgbColumnIndicator)

    y := ctx.GameY + ctx.GameHeight

    for col := 0; col < ctx.GameWidth; col++ {
        relativeCol := col - ctx.CursorX
        screenX := ctx.GameX + col

        if relativeCol == 0 {
            buf.Set(screenX, y, '0', indicatorStyle.Foreground(render.RgbLineNumbersCursor))
        } else if relativeCol%10 == 0 {
            label := fmt.Sprintf("%d", relativeCol)
            for i, ch := range label {
                if screenX+i < ctx.GameX+ctx.GameWidth {
                    buf.Set(screenX+i, y, ch, indicatorStyle)
                }
            }
        } else if relativeCol%5 == 0 {
            buf.Set(screenX, y, '·', indicatorStyle)
        }
    }
}
```

### 4. `render/renderers/status_bar.go`

```go
package renderers

import (
    "fmt"
    
    "github.com/gdamore/tcell/v2"
    "github.com/lixenwraith/vi-fighter/constants"
    "github.com/lixenwraith/vi-fighter/engine"
    "github.com/lixenwraith/vi-fighter/render"
)

// StatusBarRenderer draws the status bar at the bottom.
type StatusBarRenderer struct {
    gameCtx            *engine.GameContext
    decayTimeRemaining *float64
}

// NewStatusBarRenderer creates a status bar renderer.
func NewStatusBarRenderer(gameCtx *engine.GameContext, decayTimeRemaining *float64) *StatusBarRenderer {
    return &StatusBarRenderer{
        gameCtx:            gameCtx,
        decayTimeRemaining: decayTimeRemaining,
    }
}

// Render implements SystemRenderer.
func (s *StatusBarRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
    defaultStyle := tcell.StyleDefault.Background(render.RgbBackground)
    y := ctx.Height - 1

    // Clear status bar line
    for x := 0; x < ctx.Width; x++ {
        buf.Set(x, y, ' ', defaultStyle)
    }

    // Mode indicator and content depend on current mode
    // ... (copy logic from drawStatusBarTo, adapting to use buf.Set and buf.SetString)
    
    // This is a simplified version - full implementation should match terminal_renderer.go
    modeText := "NORMAL"
    if s.gameCtx.IsSearchMode() {
        modeText = "SEARCH"
    } else if s.gameCtx.IsCommandMode() {
        modeText = "COMMAND"
    } else if s.gameCtx.IsInsertMode() {
        modeText = "INSERT"
    }
    
    modeStyle := defaultStyle.Foreground(render.RgbStatusMode).Background(render.RgbStatusModeBg)
    buf.SetString(0, y, " "+modeText+" ", modeStyle)

    // Energy display
    energy := s.gameCtx.State.GetEnergy()
    energyText := fmt.Sprintf(" E:%d ", energy)
    energyX := ctx.Width - len(energyText)
    buf.SetString(energyX, y, energyText, defaultStyle.Foreground(render.RgbEnergy))
}
```

Note: The StatusBarRenderer above is simplified. Copy the full logic from `drawStatusBarTo` in `terminal_renderer.go`, replacing `sw.SetContent` calls with `buf.Set` or `buf.SetString`.

## Files to Modify

### 1. `main.go`

After creating the legacy adapter, register the new renderers:

```go
// Create and register UI renderers
heatMeterRenderer := renderers.NewHeatMeterRenderer(ctx.State)
orchestrator.Register(heatMeterRenderer, render.PriorityUI)

lineNumbersRenderer := renderers.NewLineNumbersRenderer(ctx.LineNumWidth, ctx)
orchestrator.Register(lineNumbersRenderer, render.PriorityUI)

columnIndicatorsRenderer := renderers.NewColumnIndicatorsRenderer()
orchestrator.Register(columnIndicatorsRenderer, render.PriorityUI)

statusBarRenderer := renderers.NewStatusBarRenderer(ctx, &renderer.decayTimeRemaining)
orchestrator.Register(statusBarRenderer, render.PriorityUI)
```

Add import: `"github.com/lixenwraith/vi-fighter/render/renderers"`

### 2. `render/terminal_renderer.go`

In `renderToWriter`, comment out or remove the calls to the migrated functions:

```go
// Migrated to HeatMeterRenderer
// r.drawHeatMeterTo(ctx.State.GetHeat(), defaultStyle, sw)

// Migrated to LineNumbersRenderer  
// r.drawLineNumbersTo(cursorPos.Y, ctx, defaultStyle, sw)

// Migrated to ColumnIndicatorsRenderer
// r.drawColumnIndicatorsTo(cursorPos.X, ctx, defaultStyle, sw)

// Migrated to StatusBarRenderer
// r.drawStatusBarTo(ctx, defaultStyle, r.decayTimeRemaining, sw)
```

Do NOT delete the `drawXTo` functions yet - keep for reference during migration.

## Verification

1. `go build .` succeeds
2. Heat meter displays correctly at top
3. Line numbers display correctly on left
4. Column indicators display correctly below game area
5. Status bar displays correctly at bottom
6. All mode indicators (NORMAL, INSERT, SEARCH, COMMAND) work

## After Completion

Update `RENDER_MIGRATION.md`: Change Phase 3a status to `✅ Complete` in a new row or notes.
```

---

## Prompt 4: Phase 3b-h - Remaining Extractions

```markdown
# Task: Renderer Migration Phase 3b-h - Complete Extraction

Read `RENDER_MIGRATION.md` at the repo root for type signatures and context.

## Objective

Extract all remaining draw functions into standalone SystemRenderer implementations. This is a large phase - work methodically through each group.

## Extraction Groups

### Phase 3b: Grid Renderers (PriorityGrid = 100)

Create `render/renderers/ping_grid.go`:
- Extract `drawPingHighlightsTo` and `drawPingGridTo`
- Combine into `PingGridRenderer`
- Needs: cursorX, cursorY, gameCtx for ping timer, world for shields

### Phase 3c: Shield Renderer (PriorityEffects = 300)

Create `render/renderers/shields.go`:
- Extract `drawShieldsTo` and `blendColors`
- `ShieldRenderer` struct
- Needs: world (for Shield components and cursor position)

### Phase 3d: Characters Renderer (PriorityEntities = 200)

Create `render/renderers/characters.go`:
- Extract `drawCharactersTo`
- `CharactersRenderer` struct
- Needs: world, gameCtx, ping color calculation
- Uses `buf.DecomposeAt()` for background preservation

### Phase 3e: Effects Renderers (PriorityEffects = 300)

Create `render/renderers/effects.go`:
- Extract `drawDecayTo`, `drawCleanersTo`, `drawRemovalFlashesTo`, `drawMaterializersTo`
- Can be single `EffectsRenderer` or separate renderers
- Needs: world, gameCtx, gradients

For gradients, either:
1. Pass gradient slices to renderer constructor, or
2. Move gradient building logic to renderer

### Phase 3f: Drain Renderer (PriorityDrain = 350)

Create `render/renderers/drain.go`:
- Extract `drawDrainTo`
- `DrainRenderer` struct
- Uses `buf.DecomposeAt()` for background preservation

### Phase 3g: Cursor Renderer (PriorityUI = 400)

Create `render/renderers/cursor.go`:
- Extract `drawCursorTo`
- `CursorRenderer` struct
- Complex logic with error flash, search mode, character display
- Needs: world, gameCtx

### Phase 3h: Overlay Renderer (PriorityOverlay = 500)

Create `render/renderers/overlay.go`:
- Extract `drawOverlayTo`
- `OverlayRenderer` struct with `VisibilityToggle` implementation
- `IsVisible()` returns `gameCtx.IsOverlayMode() && gameCtx.OverlayActive`

## Pattern for Each Renderer

```go
package renderers

import (
    "github.com/gdamore/tcell/v2"
    "github.com/lixenwraith/vi-fighter/engine"
    "github.com/lixenwraith/vi-fighter/render"
)

type XxxRenderer struct {
    // Required state - NOT full GameContext
    gameCtx *engine.GameContext // Only if truly needed for mode checks
    // Or specific fields like:
    // state *engine.GameState
    // cursorEntity engine.Entity
}

func NewXxxRenderer(...) *XxxRenderer {
    return &XxxRenderer{...}
}

func (x *XxxRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
    defaultStyle := tcell.StyleDefault.Background(render.RgbBackground)
    
    // Translate from drawXxxTo logic:
    // - sw.SetContent(x, y, r, nil, style) → buf.Set(x, y, r, style)
    // - sw.GetContent(x, y) → cell := buf.Get(x, y); then cell.Rune, cell.Style
    // - existingStyle.Decompose() → buf.DecomposeAt(x, y)
}

// Optional: implement VisibilityToggle
func (x *XxxRenderer) IsVisible() bool {
    return true // or condition
}
```

## main.go Registration Order

Register in priority order (lowest first):

```go
// Grid (100)
pingGridRenderer := renderers.NewPingGridRenderer(ctx)
orchestrator.Register(pingGridRenderer, render.PriorityGrid)

// Entities (200)
charactersRenderer := renderers.NewCharactersRenderer(ctx)
orchestrator.Register(charactersRenderer, render.PriorityEntities)

// Effects (300)
shieldRenderer := renderers.NewShieldRenderer()
orchestrator.Register(shieldRenderer, render.PriorityEffects)

effectsRenderer := renderers.NewEffectsRenderer(ctx, cleanerGradient, materializeGradient)
orchestrator.Register(effectsRenderer, render.PriorityEffects)

// Drain (350)
drainRenderer := renderers.NewDrainRenderer()
orchestrator.Register(drainRenderer, render.PriorityDrain)

// UI (400) - already registered in Phase 3a
// heatMeterRenderer, lineNumbersRenderer, columnIndicatorsRenderer, statusBarRenderer
cursorRenderer := renderers.NewCursorRenderer(ctx)
orchestrator.Register(cursorRenderer, render.PriorityUI)

// Overlay (500)
overlayRenderer := renderers.NewOverlayRenderer(ctx)
orchestrator.Register(overlayRenderer, render.PriorityOverlay)
```

## Handling Gradients

The cleaner and materialize gradients are built in TerminalRenderer constructor. Options:

**Option A (Recommended):** Move gradient building to effects renderer:
```go
type EffectsRenderer struct {
    gameCtx             *engine.GameContext
    cleanerGradient     []tcell.Color
    materializeGradient []tcell.Color
}

func NewEffectsRenderer(gameCtx *engine.GameContext) *EffectsRenderer {
    e := &EffectsRenderer{gameCtx: gameCtx}
    e.buildCleanerGradient()
    e.buildMaterializeGradient()
    return e
}
```

**Option B:** Export gradient slices from TerminalRenderer and pass to effects renderer.

## Verification Per Group

After each group, verify:
1. `go build .` succeeds
2. Visual element renders correctly
3. No duplicate rendering (comment out legacy call)

## Final State

After all extractions:
- `render/renderers/` contains 8+ files
- `LegacyAdapter` in orchestrator does nothing (all calls commented out in `renderToWriter`)
- Game renders identically to pre-migration

## After Completion

Update `RENDER_MIGRATION.md`: Add rows for each sub-phase completion.
```

---

## Prompt 5: Final Phase - Cleanup

```markdown
# Task: Renderer Migration Final Phase - Cleanup

Read `RENDER_MIGRATION.md` at the repo root for type signatures and context.

## Prerequisites

All Phase 3 sub-phases complete. All draw functions migrated to `render/renderers/`.

## Objective

Remove legacy code, dead code paths, and migration scaffolding.

## Files to Modify

### 1. `render/terminal_renderer.go`

**Delete the following:**
- `renderToWriter` method (now empty or fully commented)
- All `drawXTo` methods
- All original `drawX` methods
- `screenWriter` interface
- `RenderFrameToScreen` method
- `SetDecayState` method
- `decayAnimating`, `decayTimeRemaining` fields
- Gradient fields and build methods (if moved to EffectsRenderer)

**Keep:**
- `TerminalRenderer` struct with minimal fields needed for dimensions
- `NewTerminalRenderer` constructor
- `UpdateDimensions` method
- `RenderFrame` method (now delegates to orchestrator if still used, or remove entirely)

If TerminalRenderer is no longer needed at all, the file can be deleted entirely. The orchestrator and individual renderers replace it.

### 2. `render/legacy_adapter.go`

Delete the entire file - no longer needed.

### 3. `render/buffer_screen.go`

Keep if any renderer still uses it. Otherwise, delete.

Check: grep for `BufferScreen` usage in `render/renderers/*.go`. If none, delete.

### 4. `main.go`

**Remove:**
- `renderer := render.NewTerminalRenderer(...)` if TerminalRenderer deleted
- `legacyAdapter := render.NewLegacyAdapter(...)` 
- `orchestrator.Register(legacyAdapter, ...)` 
- `renderer.SetDecayState(...)` if method deleted

**Update orchestrator creation** if it was using TerminalRenderer's screen:
```go
orchestrator := render.NewRenderOrchestrator(screen, ctx.Width, ctx.Height)
```

**Ensure all renderers are properly registered** with correct dependencies.

### 5. `render/orchestrator.go`

If `Buffer()` method is no longer called externally, consider removing it.

### 6. Code Quality Pass

**Search and remove:**
- Commented-out legacy calls
- Unused imports
- Dead code paths
- TODO comments related to migration

**Verify no panics:**
- All renderers handle missing components gracefully
- Bounds checking on all buffer access

### 7. `RENDER_MIGRATION.md`

Update to final state:

```markdown
# Renderer Migration Reference

## Status: COMPLETE

All phases complete. Legacy monolithic renderer removed.

## Final Architecture

```
render/
├── priority.go       # RenderPriority constants
├── context.go        # RenderContext struct
├── cell.go           # RenderCell type
├── buffer.go         # RenderBuffer implementation
├── interface.go      # SystemRenderer, VisibilityToggle
├── orchestrator.go   # RenderOrchestrator
├── colors.go         # Color constants
└── renderers/
├── heat_meter.go
├── line_numbers.go
├── column_indicators.go
├── status_bar.go
├── ping_grid.go
├── shields.go
├── characters.go
├── effects.go
├── drain.go
├── cursor.go
└── overlay.go
```

## Render Pipeline

1. `RenderOrchestrator.RenderFrame()` clears buffer
2. Renderers execute in priority order (low → high)
3. Buffer flushes to tcell.Screen
4. Screen.Show() presents frame

## Adding New Visual Elements

1. Create struct implementing `SystemRenderer`
2. Choose appropriate `RenderPriority`
3. Register with orchestrator in main.go
4. Optionally implement `VisibilityToggle` for conditional rendering
```

## Verification

1. `go build .` succeeds with no warnings
2. `go vet ./...` clean
3. Game runs identically to pre-migration
4. All visual elements render correctly
5. Resize works
6. Pause/overlay modes work
7. No performance regression (visual smoothness)

## After Completion

Commit with message: "Complete renderer migration: remove legacy code"
```

---

These prompts are self-contained and reference the `RENDER_MIGRATION.md` document for consistency across sessions. Each phase builds on the previous and can be executed independently by Claude Code.