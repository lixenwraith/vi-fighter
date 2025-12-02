# Terminal Adapter Implementation

## Context
Building a minimal terminal rendering adapter for a game engine. The existing render pipeline uses a write-only buffer model (`RenderBuffer`) that composites all cells before flushing. The current tcell dependency seems to have broken color detection - it respects TERM over COLORTERM, causing true color terminals to quantize to 256-color palette.

## Requirements

### Core Functionality
1. **Direct ANSI Output**: Bypass terminfo/termcap entirely for color output
2. **True Color Detection**: Check COLORTERM first (truecolor/24bit), fall back to TERM for 256-color
3. **256-Color Fallback**: Convert RGB to nearest xterm-256 palette color when true color unavailable
4. **Attribute Support**: Bold, Dim, Underline, Blink, Reverse, Italic
5. **Resize Handling**: Detect SIGWINCH, provide blocking channel for resize events
6. **Terminal Restoration**: Proper cleanup on exit (normal and panic)
7. **High-performance Implementation**: Use strings.Builder and pre-calculated byte slices with zero-alloc buffer (both truecolor and fallback 256 colors)

### Architecture Constraints
- Write-only model: No Get()/Decompose() for cells - renderer holds all state
- Single Flush() call per frame writes entire buffer
- Input handling remains separate (can keep tcell for input or implement raw stdin)
- Must work on: Linux (standard terminals), macOS (Terminal.app, iTerm2), tmux, screen

## Interface Design
```go
package terminal

// ColorMode indicates terminal color capability
type ColorMode uint8

const (
    ColorMode256   ColorMode = iota // xterm-256 palette
    ColorModeTrueColor              // 24-bit RGB
)

// Attr represents text attributes (can be OR'd together)
type Attr uint8

const (
    AttrNone      Attr = 0
    AttrBold      Attr = 1 << 0
    AttrDim       Attr = 1 << 1
    AttrItalic    Attr = 1 << 2
    AttrUnderline Attr = 1 << 3
    AttrBlink     Attr = 1 << 4
    AttrReverse   Attr = 1 << 5
)

// RGB color
type RGB struct {
    R, G, B uint8
}

// Cell represents a single terminal cell
type Cell struct {
    Rune  rune
    Fg    RGB
    Bg    RGB
    Attrs Attr
}

// ResizeEvent sent on terminal size change
type ResizeEvent struct {
    Width  int
    Height int
}

// Terminal provides low-level terminal access
type Terminal interface {
    // Init enters raw mode, alternate screen buffer, hides cursor
    Init() error
    
    // Fini restores terminal state (must be defer'd)
    Fini()
    
    // Size returns current terminal dimensions
    Size() (width, height int)
    
    // ResizeChan returns channel that receives resize events
    // Channel is buffered(1), drops old events if not consumed
    ResizeChan() <-chan ResizeEvent
    
    // ColorMode returns detected color capability
    ColorMode() ColorMode
    
    // Flush writes cell buffer to terminal
    // Cells are row-major: cells[y*width + x]
    // Optimizes output by:
    //   - Tracking dirty regions (
    //   - Coalescing color changes
    //   - Using cursor movement shortcuts
    Flush(cells []Cell, width, height int)
    
    // Clear fills screen with default background
    Clear()
    
    // SetCursorVisible shows/hides cursor
    SetCursorVisible(visible bool)
    
    // MoveCursor positions cursor (for text input scenarios)
    MoveCursor(x, y int)
    
    // Sync forces full redraw (after external program, resize)
    Sync()
}
```

## Implementation Details

### Color Detection Logic
```
1. Check COLORTERM env var:
   - "truecolor" or "24bit" → ColorModeTrueColor
   - Set but other value → continue to TERM check
   
2. Check TERM env var for known true color terminals:
   - Contains "truecolor" → ColorModeTrueColor
   - Contains "24bit" → ColorModeTrueColor
   - "xterm-direct" → ColorModeTrueColor
   
3. Check terminal emulator env vars:
   - WT_SESSION set (Windows Terminal) → ColorModeTrueColor
   - KITTY_WINDOW_ID set → ColorModeTrueColor
   - KONSOLE_VERSION set → ColorModeTrueColor
   - ITERM_SESSION_ID set → ColorModeTrueColor
   
4. Default: ColorMode256
```

### ANSI Escape Sequences
```
True Color:
  FG: \x1b[38;2;R;G;Bm
  BG: \x1b[48;2;R;G;Bm

256 Color:
  FG: \x1b[38;5;Nm  (N = palette index 0-255)
  BG: \x1b[48;5;Nm

Attributes:
  Bold:      \x1b[1m
  Dim:       \x1b[2m
  Italic:    \x1b[3m
  Underline: \x1b[4m
  Blink:     \x1b[5m
  Reverse:   \x1b[7m
  Reset:     \x1b[0m

Cursor:
  Move:      \x1b[{row};{col}H  (1-indexed)
  Hide:      \x1b[?25l
  Show:      \x1b[?25h

Screen:
  Alternate: \x1b[?1049h  (enter)
             \x1b[?1049l  (exit)
  Clear:     \x1b[2J\x1b[H
```

### 256-Color Palette Conversion
```go
// RGB to xterm-256 palette index
// Palette structure:
//   0-15:    Standard colors (black, red, green, yellow, blue, magenta, cyan, white) + bright variants
//   16-231:  6x6x6 color cube: 16 + 36*r + 6*g + b (r,g,b ∈ {0,1,2,3,4,5})
//   232-255: Grayscale ramp (24 shades)

func RGBTo256(rgb RGB) uint8 {
    // Check grayscale first (when r≈g≈b)
    // Then find nearest in 6x6x6 cube
    // Color cube values: 0, 95, 135, 175, 215, 255
}
```

### Resize Handling (Unix)
```go
// Use golang.org/x/sys/unix for signal handling
// 1. Create signal channel for SIGWINCH
// 2. Goroutine waits on signal, calls ioctl TIOCGWINSZ
// 3. Sends ResizeEvent to buffered channel (drop if full)
```

### Output Buffering Strategy
```
1. Use bufio.Writer with 64-128KB buffer
2. Track "cursor position" virtually during write
3. Emit color codes only when fg/bg/attr changes from previous cell
4. For horizontal runs of same color, batch rune writes
5. Use \x1b[{n}C (cursor forward) to skip unchanged regions if gap > 4 chars
6. Call Flush() on underlying writer once per frame
```

### Panic Recovery
```go
// Fini() must be called even on panic
// Use either:
// 1. Wrapper that defers Fini() around main game loop
// 2. Register cleanup with runtime.SetFinalizer (unreliable)
// 3. Signal handler for SIGTERM/SIGINT that calls Fini()
```

## File Structure
```
terminal/
├── terminal.go      // Interface + constructor
├── ansi.go          // Escape sequence builders
├── color.go         // Color detection, RGB→256 conversion
├── resize_unix.go   // SIGWINCH handling (build tag: !windows)
├── resize_windows.go // Windows console resize (build tag: windows)
└── output.go        // Buffered writer, Flush optimization
```

## Testing Checklist
- [ ] True color detected when COLORTERM=truecolor, regardless of TERM
- [ ] 256-color fallback produces reasonable approximations
- [ ] Resize events delivered within 16ms of SIGWINCH
- [ ] Terminal restored after panic (test with deliberate panic)
- [ ] No flickering on full-screen redraw (60fps target)
- [ ] Works inside tmux/screen (which may downgrade TERM)
- [ ] Attributes render correctly (bold actually bold, not bright)

## Dependencies
- `golang.org/x/term` - Raw mode, terminal size
- `golang.org/x/sys/unix` - Signal handling, ioctl (Unix only)
- No terminfo/termcap parsing - direct ANSI only
- Evaluate Windows compatibility and handle with build directives if necessary

## Non-Goals
- Input handling (keep existing or separate concern)
- Mouse support
- Clipboard integration
- Unicode width calculation (caller responsibility)
- Cell-level dirty tracking (caller provides full buffer each frame)