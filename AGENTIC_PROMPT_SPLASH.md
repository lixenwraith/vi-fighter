## Execution Order

1. **Font Generation (First)** — Creates the bitmap asset required by the renderer
2. **Implementation (Second)** — Integrates all components; depends on font asset existing

---

## Directive Reference for Claude Code

These directives are meta-instructions placed in prompts to control Claude Code behavior:

| Directive | Meaning |
|-----------|---------|
| `[full]` | Output complete file content, not snippets |
| `[block]` | Output only modified functions/structs with context placeholders |
| `[diff]` | Output minimal change with surrounding context lines |
| `[verify]` | Run build/test commands after changes |
| `[[code]]` | Action directive: generate code for preceding requirements |

---

## Output 1: CLAUDE.md for Font Generation Phase

```markdown
// FILE: CLAUDE.md
# vi-fighter Development Guide for Claude Code

## PROJECT CONTEXT
vi-fighter is a terminal-based typing game in Go using a compile-time Generics-based ECS (Go 1.24+).

## ARCHITECTURE OVERVIEW

### Core Systems
- **ECS**: Generics-based `World` with `Store[T]` and `PositionStore` (spatial hash).
- **Game Loop**: Fixed 50ms tick (`ClockScheduler`) decoupled from rendering.
- **Render Pipeline**: `RenderOrchestrator` coordinates `SystemRenderer` implementations. Frame update 16ms.
- **Input**: `InputHandler` processes `tcell` events, managing state transitions between Modes.

### Resources
- **Context**: `GameContext` acts as the root state container.
- **Resources**: `TimeResource`, `ConfigResource`, `InputResource` stored in `World.Resources`.

### Render Architecture
- **Orchestrator**: `RenderOrchestrator` manages render pipeline lifecycle.
- **Buffer**: `RenderBuffer` is a dense grid for compositing; zero-alloc after init.
- **Renderers**: Individual `SystemRenderer` implementations in `render/renderers/`.
- **Priority**: `RenderPriority` constants determine render order (lower first).

## DEVELOPMENT NOTES

When implementing new features or modifying existing systems, always:
- Follow strict ECS principles (entities = IDs, components = data, systems = logic)
- Use the Resource System for global shared data access
- Maintain thread safety with atomics for real-time state, mutexes for clock-tick state
- Respect the render pipeline architecture and priority ordering
- Test with `go build .` after each significant change

## VERIFICATION
- `go build .` must succeed after completion
- Visual verification delegated to user

## ENVIRONMENT

vi-fighter uses **pure Go** with no CGO dependencies.

**Prerequisites:**
- Go 1.24 or later
- Terminal with color support (truecolor with 256-color mix/fallback)
- (Optional) System audio backend for sound effects

**Build:**
```bash
go build -o vi-fighter ./cmd/vi-fighter
```

## CURRENT TASK

**Phase: Splash Font Asset Generation**

### Objective
Generate procedural bitmap font data for the Splash visual feedback system.

### Requirements
- File: `assets/splash_font.go`
- Format: `var SplashFont = [95][12]uint16{...}`
- Coverage: ASCII 32-126 (space through tilde)
- Dimensions: 16 columns × 12 rows per character
- Bit order: MSB-first (bit 15 = column 0/leftmost)
- Style: Sans-serif block glyphs, readable at terminal scale

### Character Priority
1. **Critical**: A-Z, a-z, 0-9 (typing feedback)
2. **Important**: Common punctuation (.,;:'"!?-_)
3. **Standard**: Remaining printable ASCII

### Technical Constraints
- Each row is `uint16` where set bit = filled pixel
- Index calculation: `SplashFont[rune - 32]`
- No external font files; pure Go literal data
- Visually distinct glyphs; avoid ambiguity (0/O, 1/l/I)

### Design Guidelines
- Block style: thick strokes (2-3 pixels wide)
- Consistent baseline and cap height
- Adequate inter-character whitespace in glyph design
- Alphanumerics should be recognizable at 50% opacity

### Output
Single file `assets/splash_font.go` with complete bitmap data for all 95 characters.

### Verification
- `go build .` must pass
- Spot-check: 'A' bitmap should show recognizable letter shape
```

---

## Output 2: Font Generation Prompt

```
# Splash Font Asset Generation

[full, verify]

Generate procedural bitmap font for the Splash visual feedback system.

## Target File
`assets/splash_font.go`

## Specification

```go
// SplashFont maps printable ASCII (32-126) to 12-row bitmaps
// Each row is uint16; bit 15 = column 0 (leftmost), bit 0 = column 15 (rightmost)
// Index: SplashFont[rune - 32]
var SplashFont = [95][12]uint16{
    // Index 0 = ' ' (space)
    // Index 1 = '!'
    // ...
    // Index 94 = '~'
}
```

## Requirements

**Dimensions:** 16 columns × 12 rows per glyph

**Bit Encoding:**
- Row stored as `uint16`
- Bit 15 (0x8000) = leftmost column (column 0)
- Bit 0 (0x0001) = rightmost column (column 15)
- Set bit = pixel filled

**Style:**
- Sans-serif block style
- Stroke width: 2-3 pixels for visibility
- Clear distinction between similar characters (0/O, 1/l/I, 5/S)
- Consistent baseline (rows 10-11 typically empty or descender space)
- Cap height around rows 1-9

**Coverage:** All 95 printable ASCII characters (space through tilde)

## Example Structure

```go
// 'A' example (index 33 after space and !)
// Visual representation (X = filled):
//   Row 0:  ......XXXX......  = 0x03C0
//   Row 1:  .....XX..XX.....  = 0x0660
//   Row 2:  ....XX....XX....  = 0x0C30
//   ...
```

## Process

1. Design each glyph on 16×12 grid
2. Convert rows to hex values (MSB = left)
3. Generate complete Go source file
4. Ensure `go build .` passes

## Output

Complete `assets/splash_font.go` with all 95 character bitmaps.

[[code]]
```

---

## Output 3: CLAUDE.md for Implementation Phase

```markdown
// FILE: CLAUDE.md
# vi-fighter Development Guide for Claude Code

## PROJECT CONTEXT
vi-fighter is a terminal-based typing game in Go using a compile-time Generics-based ECS (Go 1.24+).

## ARCHITECTURE OVERVIEW

### Core Systems
- **ECS**: Generics-based `World` with `Store[T]` and `PositionStore` (spatial hash).
- **Game Loop**: Fixed 50ms tick (`ClockScheduler`) decoupled from rendering.
- **Render Pipeline**: `RenderOrchestrator` coordinates `SystemRenderer` implementations. Frame update 16ms.
- **Input**: `InputHandler` processes `tcell` events, managing state transitions between Modes.

### Resources
- **Context**: `GameContext` acts as the root state container.
- **Resources**: `TimeResource`, `ConfigResource`, `InputResource` stored in `World.Resources`.

### Render Architecture
- **Orchestrator**: `RenderOrchestrator` manages render pipeline lifecycle.
- **Buffer**: `RenderBuffer` is a dense grid for compositing; zero-alloc after init.
- **Renderers**: Individual `SystemRenderer` implementations in `render/renderers/`.
- **Priority**: `RenderPriority` constants determine render order (lower first).

## DEVELOPMENT NOTES

When implementing new features or modifying existing systems, always:
- Follow strict ECS principles (entities = IDs, components = data, systems = logic)
- Use the Resource System for global shared data access
- Maintain thread safety with atomics for real-time state, mutexes for clock-tick state
- Respect the render pipeline architecture and priority ordering
- Test with `go build .` after each significant change

## VERIFICATION
- `go build .` must succeed after each phase
- Visual verification delegated to user

## ENVIRONMENT

vi-fighter uses **pure Go** with no CGO dependencies.

**Prerequisites:**
- Go 1.24 or later
- Terminal with color support (truecolor with 256-color mix/fallback)
- (Optional) System audio backend for sound effects

**Build:**
```bash
go build -o vi-fighter ./cmd/vi-fighter
```

## CURRENT TASK

**Phase: Splash Visual Feedback System Implementation**

### Objective
Implement the Splash system providing large block-character visual feedback for successful user actions.

### Reference
All code changes are specified in `SPLASH_code_changes.md` at repo root.

### Implementation Sequence

| Phase | Files | Action |
|-------|-------|--------|
| 1 | `constants/splash.go` | CREATE - timing and dimension constants |
| 2 | `components/splash.go` | CREATE - SplashComponent struct |
| 3 | `engine/world.go` | MODIFY - add Splashes store |
| 4 | `engine/game_context.go` | MODIFY - add SplashEntity field and init |
| 5 | `engine/splash.go` | CREATE - trigger helper functions |
| 6 | `systems/splash.go` | CREATE - timeout system |
| 7 | `constants/priority.go` | MODIFY - add PrioritySplash constant |
| 8 | `render/priority.go` | MODIFY - add render PrioritySplash |
| 9 | `render/colors.go` | MODIFY - add splash color constants |
| 10 | `render/renderers/splash.go` | CREATE - SplashRenderer |
| 11 | `systems/energy.go` | MODIFY - Insert mode trigger hooks |
| 12 | `modes/input.go` | MODIFY - Normal mode trigger hook |
| 13 | `cmd/vi-fighter/main.go` | MODIFY - system and renderer registration |

### Key Patterns

**Singleton Entity:** Splash uses same pattern as CursorEntity - created once in NewGameContext, never destroyed, state managed via component.

**Trigger Functions:**
- `TriggerSplashChar()` for single character (Insert mode typing)
- `TriggerSplashString()` for command strings (Normal mode)

**Positioning:** Quadrant-based placement opposite cursor position with left/top boundary clamping.

**Rendering:** Background-only effect using `SetBgOnly()` with `MaskEffect` write mask.

### Import Requirements

Files requiring new imports (handle automatically or verify):
- `systems/energy.go`: add `render`, `terminal` packages
- `modes/input.go`: add `render` package

### Verification
1. `go build .` must pass after all changes
2. Manual test: Type characters in Insert mode → large character appears in opposite quadrant
3. Manual test: Execute Normal mode command (e.g., `dd`) → command string appears as splash
```

---

## Output 4: Implementation Prompt

```
# Splash Visual Feedback System Implementation

[verify]

Implement the Splash visual feedback system using specifications in `SPLASH_code_changes.md`.

## Prerequisite
Font asset `assets/splash_font.go` must exist (generated in prior phase).

## Implementation Order

Execute in sequence, verifying build after each phase:

### Phase 1: Constants and Component
1. CREATE `constants/splash.go` [full]
2. CREATE `components/splash.go` [full]

### Phase 2: ECS Infrastructure
3. MODIFY `engine/world.go` [block] - add Splashes store to World struct and NewWorld()
4. MODIFY `engine/game_context.go` [diff] - add SplashEntity field and initialization
5. CREATE `engine/splash.go` [full] - trigger helper functions

### Phase 3: System
6. CREATE `systems/splash.go` [full]
7. MODIFY `constants/priority.go` [diff] - add PrioritySplash = 800

### Phase 4: Renderer
8. MODIFY `render/priority.go` [diff] - add PrioritySplash RenderPriority = 150
9. MODIFY `render/colors.go` [diff] - add RgbSplashInsert and RgbSplashNormal
10. CREATE `render/renderers/splash.go` [full]

### Phase 5: Integration Hooks
11. MODIFY `systems/energy.go` [block] - add splash triggers in:
    - `handleCharacterTyping()` after DestroyEntity on success
    - `handleGoldSequenceTyping()` after DestroyEntity on success
    - `handleNuggetCollection()` after DestroyEntity on success
    - ADD helper method `getSplashColorForSequence()`

12. MODIFY `modes/input.go` [block] - add splash trigger in `handleNormalMode()` when `result.CommandString != ""`

### Phase 6: Registration
13. MODIFY `cmd/vi-fighter/main.go` [diff] - register SplashSystem and SplashRenderer

## Code Reference
All code snippets are in `SPLASH_code_changes.md`. Apply exactly as specified.

## Critical Notes

- `terminal.RGB` and `render.RGB` are type aliases; cast as needed
- Splash color for sequences uses existing `render.RgbSequence*Normal` constants
- Normal mode splash uses `render.RgbSplashNormal`
- Insert mode success colors derive from sequence type via helper method

## Verification
After completion:
```bash
go build .
```

[[code]]
```