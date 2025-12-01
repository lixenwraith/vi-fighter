# Requirements: Renderer Pipeline Refactor
[complex, full, direct]

## Core Objectives
1.  **Complete Decoupling:** The renderer must function as a self-contained compositing engine. `tcell` is demoted to a strict Input/Output adapter role. The render logic must not depend on `tcell` internal structures (like `Style` or `Decompose`) for state.
2.  **High-Fidelity Compositing:** The pipeline must support advanced blending operations (Alpha Blending, Additive Mixing) to handle visual overlaps (Shields over Grid, Flashes over Entities) correctly.
3.  **Internal State Management:** The render buffer must maintain the authoritative state of every pixel (RGB values) during the frame construction.
4.  **Write-Only Logic:** Renderers must not read from the buffer to make logic decisions. They emit draw commands; the buffer handles the mathematical integration (blending) of those commands into the existing state.

## Architectural Constraints
*   **Memory/CPU:** Allocating full-screen buffers (or multi-layers) is acceptable.
*   **Resolution:** Text-mode resolution (approx. 200x60) allows for expensive per-cell floating-point math if necessary, though integer math is preferred for consistency.
*   **Dependencies:** `tcell` imports allowed only for the final `Flush` (converting internal state to screen output) and basic type aliases if convenient (e.g., `tcell.Color` as a parameter), provided no `tcell` logic is used.

## Functional Requirements

### 1. The Compositing Buffer
Instead of a simple wrapper around `tcell.SetContent`, the `RenderBuffer` becomes a **Compositor**.
*   **Internal Storage:** Must store explicit `Red`, `Green`, `Blue` channels (uint8) for both Foreground and Background of every cell.
*   **Default State:** All cells initialize to "Black/Transparent".
*   **Z-Order/Priority:** Handled by the execution order of renderers (Painter's Algorithm). Lower priority systems draw first; higher priority systems blend on top.

### 2. Blending Primitives
The Compositor must expose methods that define *how* a new pixel interacts with the existing pixel at that coordinate.

*   **Op: Replace (Opaque):** `Dst = Src`. Used for solid entities (Characters, UI, Base Grid).
*   **Op: Alpha Blend (Dampening):** `Dst = Src * Alpha + Dst * (1 - Alpha)`. Used for transparent overlays (Shields, Trails). This ensures lower layers (Grid) remain visible but dampened under higher layers.
*   **Op: Additive/Max (Saturation):** `Dst = Max(Dst, Src)` or `Dst = Dst + Src`. Used for glowing effects (Materializer heads, Energy flashes) to prevent them from darkening the scene or looking "muddy".

### 3. Visual Correctness
*   **Shields:** Must render as transparent layers. If drawn over the Ping Grid, the Grid lines must remain visible but tinted/dimmed.
*   **Cleaners:** If opaque, they overwrite the background. If semi-transparent (trails), they blend.
*   **Flashes:** Must stack non-destructively (Additive) to allow multiple overlaps without z-fighting artifacts.

---

# Architecture & Implementation Pattern

## 1. Data Structures

### Color Primitive
Decoupled from `tcell.Color` to ensure we own the state.
```go
type RGB struct {
    R, G, B uint8
}
```

### Internal Cell State
The authoritative state of a single terminal cell during rendering.
```go
type CompositorCell struct {
    Rune       rune
    Fg         RGB
    Bg         RGB
    StyleAttrs int // Bitmask for Bold, Underline, etc.
    IsDirty    bool
}
```

### Blending Modes
Enumeration of supported compositing operations.
```go
type BlendMode int

const (
    BlendReplace BlendMode = iota // Solid overwrite
    BlendAlpha                    // Standard transparency (requires Alpha param)
    BlendAdd                      // Additive (Light simulation)
    BlendMax                      // Max channel retention (Preserve brightest)
)
```

## 2. Component Interfaces

### The Compositor (RenderBuffer)
This interface replaces the direct buffer access. It is passed to all Renderers.

```go
type Compositor interface {
    // Clear resets the buffer to the default background color
    Clear()

    // Resize adjusts the internal buffer dimensions
    Resize(width, height int)

    // SetPixel draws a single cell with specific blending logic
    // x, y: Coordinate
    // mainRune: The character to draw (0 to preserve existing)
    // fg: Foreground RGB
    // bg: Background RGB
    // mode: How to blend this pixel with what's already there
    // alpha: 0.0-1.0 (used only for BlendAlpha)
    SetPixel(x, y int, mainRune rune, fg, bg RGB, mode BlendMode, alpha float64)

    // SetPixelSolid is a convenience for BlendReplace (Alpha=1.0)
    SetPixelSolid(x, y int, mainRune rune, fg, bg RGB)

    // Batch drawing methods can be added for performance (DrawHorizontalLine, etc.)
}
```

### The Output Adapter
Responsible for converting the `Compositor` state to the specific terminal library (tcell).

```go
type ScreenAdapter interface {
    // Flush takes the internal state and pushes it to the real screen
    Flush(width, height int, cells []CompositorCell)
}
```

## 3. Implementation Logic

### Blending Math (Internal to Compositor)
When `SetPixel` is called, the Compositor reads the **current** state at `(x,y)` (The Destination) and applies the math with the **new** data (The Source).

**Logic for `BlendAlpha`:**
1.  **Background:**
    *   `NewBg.R = (Src.R * Alpha) + (Dst.R * (1 - Alpha))`
    *   `NewBg.G = (Src.G * Alpha) + (Dst.G * (1 - Alpha))`
    *   `NewBg.B = (Src.B * Alpha) + (Dst.B * (1 - Alpha))`
2.  **Foreground:**
    *   Typically, foreground (text) is drawn opaque if a rune is provided.
    *   If `mainRune` is provided: `NewFg = Src.Fg`.
    *   If `mainRune` is 0 (just tinting background): `NewFg` remains `Dst.Fg` (potentially tinted if we want complex lighting, but usually left alone).

**Logic for `BlendMax`:**
1.  **Background:**
    *   `NewBg.R = Max(Src.R, Dst.R)`
    *   ... (same for G, B)
2.  **Foreground:**
    *   `NewFg.R = Max(Src.R, Dst.R)` (Useful for overlapping colored text)

### Rendering Pipeline Step-by-Step

1.  **Initialize:** `Compositor.Clear()` sets all pixels to Black RGB.
2.  **Layer 0 (Background/Grid):**
    *   `PingGridRenderer` iterates. Calls `SetPixelSolid(..., Grey, Black)`.
    *   *Result:* Buffer has Grey text on Black background.
3.  **Layer 1 (Entities):**
    *   `CharacterRenderer` iterates. Calls `SetPixelSolid(..., CharColor, Black)`.
    *   *Result:* Entities overwrite Grid (standard opacity).
4.  **Layer 2 (Shields):**
    *   `ShieldRenderer` calculates shield color (e.g., Blue) and Alpha (0.5) based on radius.
    *   Calls `SetPixel(..., 0, ZeroColor, Blue, BlendAlpha, 0.5)`.
    *   *Result:* Buffer blends current Bg (could be Black or Grid Grey) with Blue. Grid lines turn "Blue-ish Grey".
5.  **Layer 3 (Effects):**
    *   `FlashRenderer` calls `SetPixel(..., FlashColor, FlashColor, BlendMax, 1.0)`.
    *   *Result:* Flash brightens whatever is below it without deleting it.
6.  **Output:**
    *   Game Loop calls `Adapter.Flush()`.
    *   Adapter iterates `Compositor` cells, converts `RGB` -> `tcell.Color`, calls `screen.SetContent`.

## 4. Migration Steps

1.  Create `render/internal/color.go` defining `RGB` and helpers.
2.  Create `render/internal/compositor.go` implementing the buffer and mixing logic.
3.  Update `RenderContext` (if needed) and `SystemRenderer` interface signature.
4.  Refactor all Renderers to construct `RGB` values and call `SetPixel`.
    *   *Note:* `tcell.Color` constants can be converted to `RGB` once at startup or via helper to keep code clean.
5.  Update `RenderOrchestrator` to own the `Compositor` and trigger the `Flush`.

This approach satisfies all requirements: strict decoupling, advanced blending support, and no "read-modify-write" logic inside the renderers themselves.

[[plan]]