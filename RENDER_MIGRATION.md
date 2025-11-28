# Renderer Migration Reference

## Status Tracking

| Phase | Status | Notes |
|-------|--------|-------|
| Phase 0 | ✅ Complete | Foundation types in `render/` |
| Phase 1 | ✅ Complete | Hybrid integration |
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
