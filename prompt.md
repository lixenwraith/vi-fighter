# Vi-Fighter Restructuring Implementation

## Context
Refactor a 2000+ line Go terminal game into a modular, extensible architecture supporting future sound, external text sources, and multiplayer features.

## Phase 1: Core Abstractions [PRIORITY]
Create the following modules maintaining ALL existing functionality:

### core/buffer.go
```go
type Buffer struct {
    lines [][]Cell  // 2D grid of cells
    dirty map[Point]bool  // Changed cells for efficient rendering
}
type Cell struct {
    Rune  rune
    Style tcell.Style
    Entity *Entity  // Optional entity at this position
}
```
- Implement spatial indexing for O(1) character lookups
- Support viewport scrolling (prepare for files larger than screen)
- Track dirty regions for optimized rendering

### engine/ecs.go
```go
type Entity uint64
type Component interface{}
type System interface {
    Update(world *World, dt time.Duration)
}
type World struct {
    entities map[Entity]map[reflect.Type]Component
    systems  []System
}
```
- Implement component storage with type-safe getters
- Add spatial index for PositionComponent queries
- Support system priority ordering

### modes/command.go
Extract the 600+ line input handling into a command pattern:
```go
type Command interface {
    Execute(ctx *GameContext) error
    Undo() error  // For future undo support
}
type MotionCommand struct {
    direction Direction
    count     int
}
```
- Separate parsing from execution
- Support command composition (d + motion)
- Maintain existing vi motion semantics EXACTLY

## Phase 2: Refactor Existing Features
Migrate WITHOUT changing behavior:

1. **Character spawning** → `systems/spawn_system.go`
   - Preserve sequence generation logic
   - Maintain collision avoidance
   - Keep adaptive spawn rates

2. **Scoring** → `systems/score_system.go`  
   - Extract heat meter calculation
   - Preserve multiplier logic
   - Maintain trail bonus timing

3. **Rendering** → `render/terminal_renderer.go`
   - Extract color gradient functions
   - Componentize status bar, line numbers
   - Preserve pixel-perfect layout

## Phase 3: Integration Points
Add extension interfaces:

### source/source.go
```go
type TextSource interface {
    NextSequence() ([]rune, SequenceMetadata)
}
type SequenceMetadata struct {
    Difficulty Level
    Category   string  // "keyword", "function", "error"
    Points     int
}
```

### audio/engine.go (stub for future)
```go
type AudioEngine interface {
    PlayEffect(name string)
    SetBackgroundMusic(track string)
}
```

## Constraints
- MUST compile and run after each phase
- ZERO gameplay changes - identical user experience
- Preserve ALL vi motions exactly as implemented
- Keep tcell as renderer (no library changes)
- Maintain 60 FPS performance

## Testing Requirements
Create tests/migration_test.go validating:
- All 30+ vi commands work identically
- Score calculation unchanged
- Frame rate maintained
- Memory usage not increased >10%

## File Structure Target
```
vi-fighter/
├── cmd/
│   └── vi-fighter/
│       └── main.go (50 lines - setup only)
├── core/
│   ├── buffer.go
│   ├── viewport.go  
│   └── cursor.go
├── engine/
│   ├── ecs.go
│   ├── world.go
│   └── timing.go
├── systems/
│   ├── spawn_system.go
│   ├── score_system.go
│   ├── trail_system.go
│   └── render_system.go
├── modes/
│   ├── command.go
│   ├── normal.go
│   ├── insert.go
│   └── search.go
├── components/
│   ├── position.go
│   ├── character.go
│   ├── sequence.go
│   └── trail.go
└── render/
    ├── terminal_renderer.go
    ├── colors.go
    └── ui_components.go
```

## Success Criteria
The refactored code should:
1. Run identically to current version
2. Support hot-reload of text sources
3. Allow system enable/disable at runtime
4. Expose clean interfaces for sound/network additions
5. Reduce main.go to <100 lines

Begin with Phase 1 buffer implementation, ensuring the Cell grid properly represents the current character positioning system while adding spatial indexing for improved collision detection.
