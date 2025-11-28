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

1. **Create Renderer**: Define a struct in `render/renderers/` implementing `SystemRenderer`
   ```go
   type MyRenderer struct {
       // Minimal dependencies (prefer specific types over *GameContext)
   }

   func NewMyRenderer(deps...) *MyRenderer {
       return &MyRenderer{...}
   }

   func (m *MyRenderer) Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer) {
       // Use buf.Set(x, y, rune, style) for writing
       // Use buf.Get(x, y) for reading existing content
       // Use buf.DecomposeAt(x, y) to extract style components
   }
   ```

2. **Choose Priority**: Select from `render/priority.go`:
   - `PriorityBackground` (0) - Base layer
   - `PriorityGrid` (100) - Grid highlights
   - `PriorityEntities` (200) - Game entities
   - `PriorityEffects` (300) - Visual effects, shields
   - `PriorityDrain` (350) - Drain overlay
   - `PriorityUI` (400) - UI elements
   - `PriorityOverlay` (500) - Modal overlays
   - `PriorityDebug` (1000) - Debug info

3. **Register**: In `cmd/vi-fighter/main.go`:
   ```go
   myRenderer := renderers.NewMyRenderer(deps...)
   orchestrator.Register(myRenderer, render.PriorityUI)
   ```

4. **Optional Visibility**: Implement `VisibilityToggle` for conditional rendering:
   ```go
   func (m *MyRenderer) IsVisible() bool {
       return m.shouldRender
   }
   ```

## Best Practices

- **Dependencies**: Minimize renderer dependencies. Prefer specific types over full `*GameContext`
- **Buffer Access**: Always use `buf.Set()`, never direct screen access
- **Bounds Checking**: Buffer methods handle OOB silently, no manual checks needed
- **Performance**: Avoid `fmt.Sprintf` in tight loops; use `strconv` or pre-format
- **Layering**: Preserve backgrounds with `buf.DecomposeAt()` when needed
- **Style Composition**: Build styles from constants in `render/colors.go`
