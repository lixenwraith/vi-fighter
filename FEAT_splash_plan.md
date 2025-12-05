# Splash Visual Feedback System — Complete Specification

## 1. Overview

The Splash system provides immediate, high-impact visual feedback for successful user actions. Large block-style characters render in the terminal background when the user types characters in Insert Mode or executes commands in Normal Mode. The effect occupies "dead space" opposite the cursor position, ensuring zero obstruction of gameplay.

---

## 2. Requirements

### 2.1 Functional Requirements

| ID | Requirement |
|----|-------------|
| FR1 | Display large block characters as background-only visual feedback |
| FR2 | Trigger on successful character typing in Insert Mode |
| FR3 | Trigger on successful motion/action execution in Normal Mode |
| FR4 | Position splash in quadrant diagonally opposite to cursor |
| FR5 | Support variable-length content up to 8 characters |
| FR6 | Freeze animation during game pause |
| FR7 | Replace active splash immediately on new trigger |

### 2.2 Non-Functional Requirements

| ID | Requirement |
|----|-------------|
| NFR1 | Zero dynamic allocation during trigger/render cycle |
| NFR2 | Singleton entity pattern — no entity creation/destruction |
| NFR3 | Sub-microsecond render time per frame |
| NFR4 | Compatible with 256-color and TrueColor terminals |

---

## 3. Visual Specifications

### 3.1 Character Rendering

| Property | Value |
|----------|-------|
| Character Height | 12 cells |
| Character Width | 16 cells |
| Inter-character Spacing | 1 cell |
| Font Style | Sans-serif block glyphs |
| Supported Characters | ASCII 32–126 (printable) |

### 3.2 Layering

| Layer | Priority | Description |
|-------|----------|-------------|
| Grid/Background | 100 | Base layer |
| **Splash** | **150** | Background effect layer |
| Entities | 200 | Characters, nuggets, cursor |
| Effects | 300 | Decay, cleaners, flashes |
| UI | 400 | Status bar, meters |

Splash renders to **background channel only**. Foreground content at splash coordinates remains unaffected.

### 3.3 Animation

| Phase | Timing | Behavior |
|-------|--------|----------|
| Spawn | t=0 | Instant appearance at 100% opacity |
| Hold + Fade | 0 < t < 1.0s | Linear fade: `opacity = 1.0 - (elapsed / duration)` |
| Despawn | t ≥ 1.0s | Deactivate (Length = 0) |
| Interrupt | Any | Immediate replacement, new splash starts fresh |

**Duration:** 1.0 second fixed.

### 3.4 Colors

Single flat color per splash instance. No level variants, no blending.

| Context | Color Source |
|---------|--------------|
| Insert Mode | Sequence type base color (Green, Blue, Red, Gold) |
| Normal Mode | UI/Command color (ping orange or similar neutral) |

Color constants (existing or new in `render/colors.go`):
- Insert Green: `RgbSequenceGreenNormal`
- Insert Blue: `RgbSequenceBlueNormal`
- Insert Red: `RgbSequenceRedNormal`
- Insert Gold: `RgbSequenceGold`
- Normal Mode: `RgbPingNormal` or dedicated `RgbSplashCommand`

---

## 4. Positioning Algorithm

### 4.1 Quadrant Selection

Screen divided into four quadrants. Splash appears in quadrant diagonally opposite to cursor.

```
Cursor Quadrant    Splash Quadrant
───────────────    ───────────────
Top-Left (0)       Bottom-Right (3)
Top-Right (1)      Bottom-Left (2)
Bottom-Left (2)    Top-Right (1)
Bottom-Right (3)   Top-Left (0)
```

Quadrant index calculation:
```
quadrant = (cursorX >= GameWidth/2 ? 1 : 0) | (cursorY >= GameHeight/2 ? 2 : 0)
opposite = quadrant ^ 0b11
```

### 4.2 Anchor Point Calculation

Anchor represents top-left corner of first character in game coordinates.

```
quadrantCenterX = (opposite & 1) ? GameWidth * 3/4 : GameWidth / 4
quadrantCenterY = (opposite & 2) ? GameHeight * 3/4 : GameHeight / 4

totalWidth = Length * 17 - 1    // (16 width + 1 spacing) per char, minus trailing
totalHeight = 12

anchorX = quadrantCenterX - totalWidth / 2
anchorY = quadrantCenterY - totalHeight / 2
```

### 4.3 Boundary Handling

| Edge | Behavior |
|------|----------|
| Left | Clamp to 0. Content shifts right. |
| Right | Allow overflow. Rightmost characters truncated during render. |
| Top | Clamp to 0. |
| Bottom | Allow overflow. Bottom rows truncated during render. |

```
if anchorX < 0 { anchorX = 0 }
if anchorY < 0 { anchorY = 0 }
```

### 4.4 Position Persistence

Anchor calculated **once** at trigger time. Position remains fixed for splash duration regardless of cursor movement.

---

## 5. Data Structures

### 5.1 Component

```go
// SplashComponent holds state for the singleton splash effect
type SplashComponent struct {
    Content   [8]rune   // Pre-allocated content buffer
    Length    int       // Active character count; 0 = inactive
    Color     RGB       // Flat render color
    AnchorX   int       // Game-relative X (top-left of first char)
    AnchorY   int       // Game-relative Y (top-left of first char)
    StartNano int64     // GameTime.UnixNano() at activation
}
```

**Design Notes:**
- `Length == 0` indicates inactive state; no separate boolean flag
- `[8]rune` capacity handles worst-case commands (`2d11fa` = 6 chars) plus margin
- `int64` timestamp avoids `time.Time` allocation in hot path
- `RGB` stored directly, not computed per frame

### 5.2 Font Asset

```go
// SplashFont maps printable ASCII to 12-row bitmaps
// Each row is uint16; bit N set means column N is filled (MSB = left)
var SplashFont = [95][12]uint16{
    // Index 0 = ' ' (space), Index 1 = '!', ... Index 94 = '~'
}
```

**Indexing:** `SplashFont[rune - 32]` for ASCII 32–126.

**Memory:** 95 characters × 12 rows × 2 bytes = 2,280 bytes static.

### 5.3 Constants

```go
const (
    SplashCharWidth   = 16
    SplashCharHeight  = 12
    SplashCharSpacing = 1
    SplashMaxLength   = 8
    SplashDuration    = 1 * time.Second
    PrioritySplash    = 150
)
```

---

## 6. System Behavior

### 6.1 SplashSystem

**Priority:** Low (runs after game logic, before rendering).

**Update Logic:**
1. Retrieve `SplashComponent` from singleton entity
2. If `Length == 0`, return (inactive)
3. Fetch `GameTime` from `TimeResource`
4. Calculate elapsed: `now.UnixNano() - StartNano`
5. If elapsed ≥ `SplashDuration.Nanoseconds()`, set `Length = 0`

No other state mutation. Opacity calculated during render.

### 6.2 Trigger Function

Signature: `TriggerSplash(ctx *GameContext, content []rune, color RGB)`

**Logic:**
1. Fetch cursor position from `CursorEntity`
2. Calculate opposite quadrant
3. Compute anchor with boundary clamping
4. Populate `SplashComponent`:
    - Copy up to 8 runes from `content` to `Content`
    - Set `Length` to actual count (capped at 8)
    - Set `Color`
    - Set `AnchorX`, `AnchorY`
    - Set `StartNano` to `ctx.PausableClock.Now().UnixNano()`
5. Write component to singleton entity

**Zero-Alloc:** No slice allocation. Caller passes slice backed by stack or pre-existing buffer. Function copies runes individually.

#### 6.2.1 Refinement: Split the trigger into two specialized, zero-alloc functions:
TriggerSplashChar(ctx *GameContext, char rune, color RGB)
TriggerSplashString(ctx *GameContext, text string, color RGB)
Implementation: Both functions directly populate the [8]rune array in the component without creating intermediate slices.
---

## 7. Rendering

### 7.1 SplashRenderer

**Priority:** 150 (between Grid and Entities).

**Render Logic:**
1. Retrieve `SplashComponent`; if `Length == 0`, return
2. Fetch `GameTime`, calculate elapsed nanoseconds
3. Compute opacity: `1.0 - float64(elapsed) / float64(SplashDuration.Nanoseconds())`
4. Clamp opacity to [0.0, 1.0]
5. Scale color: `scaledColor = Scale(component.Color, opacity)`
6. Set write mask: `MaskEffect` (affected by DimRenderer, GrayoutRenderer)
7. For each character index `i` in `0..Length-1`:
    - Calculate character origin: `charX = AnchorX + i * 17`
    - Lookup bitmap: `SplashFont[Content[i] - 32]`
    - For each row `r` in `0..11`:
        - Screen Y: `screenY = ctx.GameY + AnchorY + r`
        - If `screenY < ctx.GameY` or `screenY >= ctx.GameY + ctx.GameHeight`, skip row
        - For each column `c` in `0..15`:
            - If bit not set in `bitmap[r]`, skip
            - Screen X: `screenX = ctx.GameX + charX + c`
            - If `screenX < ctx.GameX` or `screenX >= ctx.GameX + ctx.GameWidth`, skip
            - Call `buf.SetBgOnly(screenX, screenY, scaledColor)`

### 7.2 Bitmap Bit Order

MSB-first: Bit 15 = column 0 (leftmost), Bit 0 = column 15 (rightmost).

```go
for c := 0; c < 16; c++ {
    if bitmap[r] & (1 << (15 - c)) != 0 {
        // pixel set at column c
    }
}
```

### 7.3 Write Mask

Use `MaskEffect`. Splash is a game-area visual effect and should be affected by:
- `DimRenderer` during certain states
- `GrayoutRenderer` during pause/death

No dedicated mask required.

---

## 8. Integration Points

### 8.1 Insert Mode (EnergySystem)

**Location:** `systems/energy.go`, `handleCharacterTyping()`, after successful character destruction.

**Trigger Point:** After `world.DestroyEntity(entity)`, before cursor position update.

**Data:**
- Content: Single rune (`[]rune{typedRune}`)
- Color: Based on `seq.Type`:
    - `SequenceGreen` → `RgbSequenceGreenNormal`
    - `SequenceBlue` → `RgbSequenceBlueNormal`
    - `SequenceRed` → `RgbSequenceRedNormal`
    - `SequenceGold` → `RgbSequenceGold`

### 8.2 Normal Mode (InputHandler)

**Location:** `modes/input.go`, `handleNormalMode()`, after action execution.

**Trigger Point:** After `result.Action(h.ctx)` call, when `result.CommandString != ""`.

**Data:**
- Content: `[]rune(result.CommandString)`
- Color: `RgbPingNormal` or dedicated command color

**Secondary Triggers:**
- `handleNormalModeSpecialKeys()`: Arrow keys, Home, End, Tab, Enter
    - May skip splash for basic navigation (h/j/k/l equivalents via arrows)
    - Tab (nugget jump) and Enter (directional cleaner) warrant splash

### 8.3 Gold Sequence (EnergySystem)

**Location:** `systems/energy.go`, `handleGoldSequenceTyping()`.

**Trigger Point:** After successful gold character typed.

**Data:**
- Content: Single rune (typed character)
- Color: `RgbSequenceGold`

---

## 9. Initialization

### 9.1 World Registration

Add to `engine/world.go` `NewWorld()`:
```go
Splashes: NewStore[components.SplashComponent](),
```

Add to `allStores` slice for lifecycle management.

### 9.2 Context Initialization

Add to `engine/game_context.go` `GameContext` struct:
```go
SplashEntity Entity
```

Add to `NewGameContext()`, after cursor entity creation:
```go
ctx.SplashEntity = ctx.World.CreateEntity()
ctx.World.Splashes.Add(ctx.SplashEntity, components.SplashComponent{})
```

### 9.3 System Registration

Add to `main.go`, after other systems:
```go
splashSystem := systems.NewSplashSystem(ctx)
ctx.World.AddSystem(splashSystem)
```

### 9.4 Renderer Registration

Add to `main.go`, after grid renderer:
```go
splashRenderer := renderers.NewSplashRenderer(ctx)
orchestrator.Register(splashRenderer, render.PrioritySplash)
```

---

## 10. File Structure

```
vi-fighter/
├── assets/
│   └── splash_font.go          # Font bitmaps [95][12]uint16
├── components/
│   └── splash.go               # SplashComponent struct
├── constants/
│   └── splash.go               # Splash-related constants
├── engine/
│   ├── game_context.go         # +SplashEntity field
│   ├── splash.go               # TriggerSplash() helper
│   └── world.go                # +Splashes store
├── render/
│   ├── priority.go             # +PrioritySplash = 150
│   └── renderers/
│       └── splash.go           # SplashRenderer
├── systems/
│   └── splash.go               # SplashSystem
└── modes/
    └── input.go                # Trigger hook (Normal Mode)
```

---

## 11. Implementation Sequence

| Phase | Task | Files |
|-------|------|-------|
| 1 | Define constants | `constants/splash.go` |
| 2 | Define component | `components/splash.go` |
| 3 | Add store to World | `engine/world.go` |
| 4 | Add entity to Context | `engine/game_context.go` |
| 5 | Create font asset | `assets/splash_font.go` |
| 6 | Implement trigger helper | `engine/splash.go` |
| 7 | Implement system | `systems/splash.go` |
| 8 | Add priority constant | `render/priority.go` |
| 9 | Implement renderer | `render/renderers/splash.go` |
| 10 | Hook EnergySystem | `systems/energy.go` |
| 11 | Hook InputHandler | `modes/input.go` |
| 12 | Register system + renderer | `cmd/vi-fighter/main.go` |

---

## 12. Testing Strategy

| Test Type | Coverage |
|-----------|----------|
| Unit | Quadrant calculation, boundary clamping, font indexing |
| Unit | Opacity calculation at various elapsed times |
| Integration | Trigger from Insert Mode keystroke |
| Integration | Trigger from Normal Mode command |
| Visual | Multi-character command rendering |
| Visual | Edge-case positioning (cursor at corners) |
| Visual | Pause freeze behavior |
| Performance | Render time measurement (target < 100µs) |

---

## 13. Future Considerations

- **TrueColor Enhancement:** Color gradients or glow effects for TrueColor terminals
- **Per-Mode Renderers:** Separate visual styles for Insert vs Normal feedback