# Task: Renderer Migration Post-Audit

## Context

The vi-fighter project has completed a renderer architecture migration from a monolithic `TerminalRenderer` (~1300 LOC) to a System Render Interface pattern with individual `SystemRenderer` implementations.

Read `RENDER_MIGRATION.md` at the repo root for architecture details and type signatures.

## Audit Objectives

1. **Code Quality Audit**
2. **Performance Verification**
3. **Dead Code Removal**
4. **Documentation Update**

---

## 1. Code Quality Audit

### 1.1 Interface Compliance

Verify all renderers in `render/renderers/` properly implement `SystemRenderer`:
```bash
# Find all renderer files
find render/renderers -name "*.go" -type f

# For each, verify Render method signature matches:
# Render(ctx render.RenderContext, world *engine.World, buf *render.RenderBuffer)
```

Check for any renderers implementing `VisibilityToggle` - ensure `IsVisible()` has correct logic.

### 1.2 Buffer Access Patterns

Audit each renderer for correct buffer usage:

**Correct patterns:**
- `buf.Set(x, y, r, style)` for writing
- `buf.Get(x, y)` returns `RenderCell` for reading
- `buf.DecomposeAt(x, y)` for style decomposition
- `buf.SetString(x, y, s, style)` for string output
- Bounds checking is silent (OOB returns empty/no-op)

**Anti-patterns to flag:**
- Direct `tcell.Screen` access
- Calls to deprecated `screen.GetContent()`
- Manual bounds checking before `buf.Set()` (unnecessary)
- Storing `*RenderBuffer` in renderer struct (should receive per-frame)

### 1.3 Dependency Audit

For each renderer, verify minimal dependencies:

**Acceptable:**
- `*engine.World` for component access
- `*engine.GameState` for game state
- `*engine.GameContext` only if mode checks required
- Primitive config values (dimensions, entity IDs)

**Flag for review:**
- Full `*engine.GameContext` when only subset needed
- Renderer holding references to other renderers
- Circular dependencies between renderers

### 1.4 Priority Registration

In `main.go`, verify registration order matches priorities:
```go
// Expected order (ascending priority):
// PriorityBackground (0)
// PriorityGrid (100)
// PriorityEntities (200)
// PriorityEffects (300)
// PriorityDrain (350)
// PriorityUI (400)
// PriorityOverlay (500)
```

Flag any out-of-order registrations or duplicate priorities without stable ordering guarantee.

---

## 2. Performance Verification

### 2.1 Allocation Analysis

Run escape analysis on render package:
```bash
go build -gcflags="-m -m" ./render/... 2>&1 | grep -E "(escapes|moved to heap)"
```

Flag any unexpected heap allocations in:
- `RenderBuffer.Clear()` - should use exponential copy, no allocs
- `RenderBuffer.Set()` - should be inline-able
- `RenderOrchestrator.RenderFrame()` - loop should not allocate

### 2.2 Hot Path Review

Identify hot paths (called every frame):
- `RenderOrchestrator.RenderFrame()`
- `RenderBuffer.Clear()`
- `RenderBuffer.Flush()`
- Each renderer's `Render()` method

For each, verify:
- No `fmt.Sprintf` in loops (use `strconv` or pre-format)
- No slice allocations per frame
- No map allocations per frame
- No interface conversions in tight loops

### 2.3 Benchmark Baseline

If time permits, create baseline benchmark:
```go
// render/buffer_test.go
func BenchmarkBufferClear(b *testing.B) {
    buf := NewRenderBuffer(200, 50)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        buf.Clear()
    }
}

func BenchmarkBufferFlush(b *testing.B) {
    // Requires mock screen or skip
}
```

---

## 3. Dead Code Removal

### 3.1 Legacy Renderer Cleanup

In `render/terminal_renderer.go`, identify and remove:

- [ ] All `drawX` methods (original versions)
- [ ] All `drawXTo` methods (migration intermediates)
- [ ] `screenWriter` interface
- [ ] `renderToWriter` method
- [ ] `RenderFrameToScreen` method
- [ ] `SetDecayState` method
- [ ] `decayAnimating`, `decayTimeRemaining` fields
- [ ] Gradient fields if moved to `EffectsRenderer`

If `TerminalRenderer` is empty after cleanup, delete the file.

### 3.2 Migration Scaffolding

Remove if no longer used:

- [ ] `render/legacy_adapter.go` - entire file
- [ ] `render/buffer_screen.go` - if no renderer uses `BufferScreen`
- [ ] `LegacyRenderer` interface references

### 3.3 Unused Imports

Run:
```bash
go mod tidy
goimports -w ./render/...
```

### 3.4 Commented Code

Search and remove migration comments:
```bash
grep -rn "// Migrated to" render/
grep -rn "// TODO.*migration" render/
grep -rn "// DEPRECATED" render/
```

---

## 4. Documentation Update

### 4.1 RENDER_MIGRATION.md

Update status to COMPLETE. Final content should include:
- Architecture diagram (ASCII)
- Render pipeline description
- Instructions for adding new renderers
- Remove phase tracking table (historical)

### 4.2 Code Comments

Ensure each file in `render/` has package-level doc comment:
```go
// Package render provides the rendering pipeline for vi-fighter.
// 
// Architecture:
// - RenderOrchestrator coordinates frame rendering
// - RenderBuffer provides compositing surface
// - SystemRenderer implementations in renderers/ subpackage
package render
```

### 4.3 Architecture Documentation

Update `doc/architecture.md` (if exists) with:
- New render pipeline description
- Component diagram showing orchestrator → renderers → buffer → screen flow

---

## Deliverables

1. **Audit Report**: List of issues found with severity (Critical/High/Medium/Low)
2. **Code Changes**: PRs or commits addressing identified issues
3. **Updated Documentation**: RENDER_MIGRATION.md in final state

## Verification Commands
```bash
# Build
go build .

# Vet
go vet ./...

# Static analysis (if available)
staticcheck ./render/...

# Run game and verify visually
./vi-fighter
```

## Success Criteria

- [ ] Zero compiler warnings
- [ ] `go vet` clean
- [ ] No dead code in render package
- [ ] All renderers follow consistent patterns
- [ ] Documentation reflects final architecture
- [ ] Game runs identically to pre-migration