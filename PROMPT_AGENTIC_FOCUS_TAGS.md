# Task: Auto-Tag Go Files for focus-catalog

## Objective

Add `// @focus:` tags to all `.go` files in the vi-fighter repository. Tags enable the `focus-catalog` tool to filter and select relevant files for LLM context generation.

## Tag Format

Insert **before** the `package` statement, as the first non-empty line or after existing top comments:
```go
// @focus: #group1 { tag1, tag2 } #group2 { tag3 }
package somepkg
```

**Rules:**
- One `// @focus:` line per file (combine all groups on one line)
- Groups start with `#`, tags inside `{ }`
- Tags are lowercase, use hyphens for multi-word: `game-state`, `z-index`
- A file can have multiple groups with multiple tags each
- Special: `#all` (no braces) marks files that should ALWAYS be included in any context

---

## Tag Taxonomy

### #core — ECS Infrastructure & Foundational Types

For files implementing the entity-component-system foundation, data structures, and core abstractions.

| Tag | Description |
|-----|-------------|
| `ecs` | Core ECS patterns, entity management, component interfaces |
| `entity` | Entity creation, destruction, ID management |
| `store` | Generic component storage, Store[T] implementation |
| `query` | ECS query building and execution |
| `spatial` | Spatial indexing, grid, position lookups |
| `world` | World struct, system registration, update loop |
| `resources` | Resource system, global shared state |
| `lifecycle` | Entity lifecycle, MarkedForDeath, CullSystem |
| `types` | Core type definitions (Entity, Point, modes) |
| `clock` | Timing, clocks, schedulers, tick management |
| `state` | GameState, snapshots, atomic state fields |

**Candidates:** `core/`, `engine/` (most files), `components/` (base interfaces)

---

### #game — Game Mechanics & Systems

For files implementing gameplay logic, game systems, and mechanics.

| Tag | Description |
|-----|-------------|
| `spawn` | Content spawning, sequence generation |
| `decay` | Sequence degradation, falling animation |
| `gold` | Gold sequence mechanics, 10-char bonus |
| `nugget` | Nugget spawning, collection, jump |
| `drain` | Hostile drain entities, cursor pursuit |
| `cleaner` | Cleaner system, row/directional cleanup |
| `shield` | Shield activation, energy protection |
| `energy` | Energy meter, character typing rewards |
| `heat` | Heat meter, boost threshold |
| `boost` | Boost activation, timer extension |
| `collision` | Collision detection, entity interactions |
| `phase` | Game phase state machine (Normal, Gold, Decay) |
| `flash` | Destruction flash effects |
| `protection` | Entity protection flags, immunity |
| `timer` | Duration-based timers, TimeKeeper |
| `splash` | Large visual feedback, countdown display |
| `cursor` | Cursor entity, movement, error flash |

**Candidates:** `systems/`, `components/` (gameplay components)

---

### #render — Rendering Pipeline & Visuals

For files handling display, visual effects, and terminal output composition.

| Tag | Description |
|-----|-------------|
| `buffer` | RenderBuffer, cell grid, compositing |
| `blend` | Blend modes, alpha compositing |
| `colors` | Color definitions, RGB utilities, palettes |
| `mask` | Stencil masks, selective rendering |
| `effects` | Visual effects (decay trails, cleaner trails) |
| `ui` | UI elements (meters, status bar, line numbers) |
| `post-process` | Post-processing (dim, grayout) |
| `cursor-render` | Cursor display, blink, error state |
| `splash-render` | Splash/countdown rendering, bitmap font |
| `shield-render` | Shield gradient, ellipse rendering |
| `ping` | Ping grid, crosshair overlay |
| `orchestrator` | Render orchestration, priority ordering |
| `context` | Render context, frame data |
| `characters` | Character entity rendering |

**Candidates:** `render/`, `render/renderers/`, `assets/`

---

### #input — Input Handling & Vi Emulation

For files handling keyboard input, vi motion/operator parsing, and mode management.

| Tag | Description |
|-----|-------------|
| `keys` | Key constants, terminal key mapping |
| `machine` | Input state machine, intent parsing |
| `intent` | Intent types, parsed actions |
| `motion` | Vi motions (hjkl, w, b, gg, etc.) |
| `operator` | Vi operators (d, x, D) |
| `char-motion` | Character motions (f, t, F, T) |
| `commands` | Colon commands (:new, :help, :debug) |
| `search` | Search mode, pattern matching, n/N |
| `modes` | Mode switching (Normal, Insert, Search, Command) |
| `router` | Intent routing, action dispatch |

**Candidates:** `input/`, `modes/`

---

### #events — Event System & Inter-System Communication

For files implementing the event-driven architecture.

| Tag | Description |
|-----|-------------|
| `queue` | Lock-free event queue, MPSC pattern |
| `router` | Generic event router, handler registration |
| `payloads` | Event payload structs |
| `types` | Event type definitions, EventType enum |
| `dispatch` | Event dispatch logic |

**Candidates:** `events/`

---

### #terminal — Terminal I/O & ANSI Control

For files handling raw terminal interaction.

| Tag | Description |
|-----|-------------|
| `ansi` | ANSI escape sequences, CSI codes |
| `raw-input` | Raw stdin parsing, key decoding |
| `output` | Buffered output, diff rendering |
| `resize` | Terminal resize handling |
| `cell` | Cell structure, attributes |
| `color-mode` | Color mode detection (256/TrueColor) |

**Candidates:** `terminal/`

---

### #audio — Audio System

For files handling sound generation and playback.

| Tag | Description |
|-----|-------------|
| `engine` | Audio engine, playback control |
| `synth` | Sound synthesis, waveform generation |
| `mixer` | Audio mixing, channel management |
| `cache` | Sound caching |
| `detect` | Backend detection |

**Candidates:** `audio/`

---

### #content — Content Management

For files handling game content loading.

| Tag | Description |
|-----|-------------|
| `manager` | Content file discovery, validation |
| `loader` | Block loading, line processing |

**Candidates:** `content/`

---

### #constants — Configuration Constants

For files defining game constants and configuration.

| Tag | Description |
|-----|-------------|
| `gameplay` | Gameplay tuning constants |
| `ui` | UI dimension constants |
| `audio` | Audio constants |
| `entities` | Entity limits, counts |

**Candidates:** `constants/`

---

### Special Tag: #all

Use `#all` (no braces) for files that should be included in **every** context selection. These are foundational files that prevent LLM hallucination:

- `core/entity.go` — Entity type definition
- `core/modes.go` — GameMode constants
- `core/point.go` — Point type
- `engine/world.go` — World struct (central ECS container)
- `events/types.go` — Event type enum (referenced everywhere)

**Format:** `// @focus: #all #core { types }`

---

## Tagging Guidelines

### Scope Selection

1. **Primary group**: What subsystem does this file belong to?
2. **Secondary groups**: What other subsystems does it interact with?
3. **Tags**: What specific features/concepts does it implement?

### Examples

**`systems/drain.go`** — Drain system implementation
```go
// @focus: #game { drain, collision, spawn } #core { ecs, spatial }
package systems
```

**`render/buffer.go`** — Render buffer
```go
// @focus: #render { buffer, blend, mask, colors }
package render
```

**`engine/clock_scheduler.go`** — Clock tick scheduler
```go
// @focus: #core { clock, ecs, lifecycle } #events { dispatch }
package engine
```

**`input/machine.go`** — Input state machine
```go
// @focus: #input { machine, intent, keys }
package input
```

**`events/queue.go`** — Lock-free event queue
```go
// @focus: #events { queue } #core { concurrency }
package events
```

**`terminal/output.go`** — Terminal output buffer
```go
// @focus: #terminal { output, ansi, cell }
package terminal
```

**`core/entity.go`** — Entity type (always needed)
```go
// @focus: #all #core { entity, types }
package core
```

### Cross-Cutting Tags

Some files span multiple concerns. Use multiple groups:
```go
// @focus: #game { energy, shield } #events { payloads } #render { ui }
```

### Component Files

Component files in `components/` should reference both `#core` (for ECS pattern) and `#game` (for the mechanic they support):
```go
// @focus: #core { ecs, types } #game { drain }
package components
```

---

## Execution Instructions

1. **Scan** all `.go` files in the repository (exclude `vendor/`, `testdata/`, `*_test.go`)

2. **Analyze** each file:
    - Read the package name
    - Identify what systems/features the file implements
    - Check imports to understand dependencies
    - Look at type/function names for feature hints

3. **Select tags**:
    - Choose 1-2 primary groups based on package location
    - Add relevant tags (2-5 tags per group typically)
    - Add cross-cutting groups if file bridges subsystems

4. **Insert** the `// @focus:` line:
    - Place after any build tags (`//go:build`)
    - Place after any license/copyright headers
    - Place immediately before `package` statement
    - Use a single line, combine all groups

5. **Verify**:
    - Every `.go` file should have exactly one `// @focus:` line
    - Tags should be lowercase with hyphens
    - `#all` files should be minimal (5-10 core files max)

---

## Files to Process

Process all `.go` files in these directories:
- `core/`
- `engine/`
- `components/`
- `systems/`
- `events/`
- `input/`
- `modes/`
- `render/` (including `render/renderers/`)
- `terminal/`
- `audio/`
- `content/`
- `constants/`
- `assets/`
- `cmd/vi-fighter/`

Skip:
- `cmd/focus-catalog/` (the tool itself)
- `cmd/blend-tester/`, `cmd/font-editor/`, `cmd/render-benchmark/` (dev tools)
- Any `*_test.go` files
- Any `vendor/` directory

---

## Output

For each file processed, show:
```
✓ systems/drain.go: #game { drain, collision, spawn } #core { ecs, spatial }
```

After completion, provide a summary:
```
Tagged 98 files
Groups used: #core (45), #game (32), #render (28), #input (12), #events (8), #terminal (8), #audio (7), #content (2), #constants (6)
#all files: 5
```

Save the comprehensive report at repo root, file `REPORT_FOCUS_TAGS.md`
