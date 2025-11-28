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
