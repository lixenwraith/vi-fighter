# Agentic Implementation Prompts

## Phase 1: Migrate CleanerSystem to EventRouter

# Task: Migrate CleanerSystem to EventRouter

## Context
The EventRouter infrastructure is now in place (`engine/event_router.go`). CleanerSystem currently calls `ctx.ConsumeEvents()` directly in its `Update()` method. This must be migrated to use the EventRouter pattern.

## Goal
- CleanerSystem implements `EventHandler` interface
- CleanerSystem registers with EventRouter
- Event consumption moves from `Update()` to `HandleEvent()`
- `Update()` only handles physics/lifecycle, not event consumption

## Files to Modify

### 1. `systems/cleaner.go`

**ADD** interface implementation methods:

```go
// EventTypes returns the event types CleanerSystem handles
func (cs *CleanerSystem) EventTypes() []engine.EventType {
    return []engine.EventType{
        engine.EventCleanerRequest,
        engine.EventDirectionalCleanerRequest,
    }
}

// HandleEvent processes cleaner-related events from the router
func (cs *CleanerSystem) HandleEvent(world *engine.World, event engine.GameEvent) {
    // Check if we already spawned for this frame (deduplication)
    if cs.spawned[event.Frame] {
        return
    }

    switch event.Type {
    case engine.EventCleanerRequest:
        cs.spawnCleaners(world)
        cs.spawned[event.Frame] = true
        cs.hasSpawnedSession = true

    case engine.EventDirectionalCleanerRequest:
        if payload, ok := event.Payload.(*engine.DirectionalCleanerPayload); ok {
            cs.spawnDirectionalCleaners(world, payload.OriginX, payload.OriginY)
            cs.spawned[event.Frame] = true
            cs.hasSpawnedSession = true
        }
    }
}
```

**MODIFY** the `Update()` method:
- REMOVE the entire event consumption block (lines that call `cs.ctx.ConsumeEvents()` and the for loop processing events)
- KEEP the spawned map cleanup logic
- KEEP the cleaner physics/movement/collision logic
- KEEP the `EventCleanerFinished` push logic

The `Update()` method should start with the spawned map cleanup, then proceed to process active cleaners. It should NOT contain any `ConsumeEvents()` call.

**REMOVE** these lines from `Update()`:
```go
// DELETE THIS BLOCK:
events := cs.ctx.ConsumeEvents()
for _, event := range events {
    if event.Type == engine.EventCleanerRequest {
        // ...
    } else if event.Type == engine.EventDirectionalCleanerRequest {
        // ...
    }
}
```

### 2. `cmd/vi-fighter/main.go`

**ADD** registration of CleanerSystem with the EventRouter.

Find where `clockScheduler` is created (after `engine.NewClockScheduler()`).
Add this line BEFORE `clockScheduler.Start()`:

```go
clockScheduler.RegisterEventHandler(cleanerSystem)
```

## Verification
After this change:
- App compiles and runs
- Gold completion at max heat triggers horizontal cleaners (EventCleanerRequest)
- Enter key at heat>=10 triggers directional cleaners (EventDirectionalCleanerRequest)
- Nugget collection at max heat triggers directional cleaners
- Cleaner physics/animation unchanged

## Important Notes
- Do NOT modify the `spawnCleaners()` or `spawnDirectionalCleaners()` methods
- Do NOT modify the cleaner physics in `Update()`
- The `world` parameter in `HandleEvent()` gives access to the World for spawning
- The `spawned` map deduplication must work in `HandleEvent()`, not `Update()`

---

## Phase 2: Migrate EnergySystem to EventRouter

# Task: Migrate EnergySystem to EventRouter for Event Handling

## Context
EnergySystem needs to:
1. Implement `EventHandler` interface to receive `EventCharacterTyped` and `EventEnergyTransaction`
2. Process typing events that will be pushed by InputHandler (Phase 3)
3. Process energy transaction events for nugget jump costs

## Goal
- EnergySystem implements `EventHandler` interface
- EnergySystem registers with EventRouter
- `HandleCharacterTyping` logic accessible via `HandleEvent()`
- Energy transactions processed via events

## Files to Modify

### 1. `systems/energy.go`

**ADD** interface implementation methods after the existing methods:

```go
// EventTypes returns the event types EnergySystem handles
func (s *EnergySystem) EventTypes() []engine.EventType {
    return []engine.EventType{
        engine.EventCharacterTyped,
        engine.EventEnergyTransaction,
    }
}

// HandleEvent processes input-related events from the router
func (s *EnergySystem) HandleEvent(world *engine.World, event engine.GameEvent) {
    switch event.Type {
    case engine.EventCharacterTyped:
        if payload, ok := event.Payload.(*engine.CharacterTypedPayload); ok {
            s.HandleCharacterTyping(world, payload.X, payload.Y, payload.Char)
        }

    case engine.EventEnergyTransaction:
        if payload, ok := event.Payload.(*engine.EnergyTransactionPayload); ok {
            s.ctx.State.AddEnergy(payload.Amount)
        }
    }
}
```

**DO NOT MODIFY** the existing `HandleCharacterTyping()` method. It remains public for now and is called by `HandleEvent()`. It will become internal after InputHandler migration.

**DO NOT MODIFY** the existing `Update()` method. It handles time-based logic (error flash clearing, energy blink timeout).

### 2. `cmd/vi-fighter/main.go`

**ADD** registration of EnergySystem with the EventRouter.

Find where `clockScheduler.RegisterEventHandler(cleanerSystem)` was added in Phase 1.
Add this line immediately after:

```go
clockScheduler.RegisterEventHandler(energySystem)
```

## Verification
After this change:
- App compiles and runs
- Existing typing still works (InputHandler still calls `HandleCharacterTyping` directly)
- This phase prepares infrastructure; behavior unchanged until Phase 3

## Important Notes
- Keep `HandleCharacterTyping` as a public method for now
- The `HandleEvent` method is a wrapper that extracts payload and calls existing logic
- Energy transactions are additive (can be positive or negative)
- Do NOT change any existing logic in `HandleCharacterTyping` or `Update`

---

## Phase 3: Migrate NuggetSystem State to GameState

# Task: Hoist NuggetSystem Active Nugget State to GameState

## Context
NuggetSystem holds `activeNugget atomic.Uint64` locally. InputHandler currently calls `nuggetSystem.JumpToNugget()` to get nugget position. To decouple InputHandler, this state must be accessible via GameState.

## Goal
- NuggetSystem writes to `ctx.State.ActiveNuggetID` instead of local atomic
- Remove local `activeNugget` field from NuggetSystem
- Update all read/write operations to use GameState
- Keep `JumpToNugget()` method for now (will be removed in Phase 4)

## Files to Modify

### 1. `systems/nugget.go`

**REMOVE** the `activeNugget` field from the struct:
```go
// REMOVE this field from NuggetSystem struct:
activeNugget atomic.Uint64
```

**MODIFY** `spawnNugget()` method:
- Find the line: `s.activeNugget.Store(uint64(entity))`
- Replace with: `s.ctx.State.SetActiveNuggetID(uint64(entity))`

**MODIFY** `Update()` method:
- Find: `activeNuggetEntity := s.activeNugget.Load()`
- Replace with: `activeNuggetEntity := s.ctx.State.GetActiveNuggetID()`

- Find: `s.activeNugget.CompareAndSwap(activeNuggetEntity, 0)`
- Replace with: `s.ctx.State.ClearActiveNuggetID(activeNuggetEntity)`

**MODIFY** `GetActiveNugget()` method:
- Find: `return s.activeNugget.Load()`
- Replace with: `return s.ctx.State.GetActiveNuggetID()`

**MODIFY** `ClearActiveNugget()` method:
- Find: `s.activeNugget.Store(0)`
- Replace with: `s.ctx.State.SetActiveNuggetID(0)`

**MODIFY** `ClearActiveNuggetIfMatches()` method:
- Find: `return s.activeNugget.CompareAndSwap(uint64(entity), 0)`
- Replace with: `return s.ctx.State.ClearActiveNuggetID(uint64(entity))`

**MODIFY** `JumpToNugget()` method:
- Find: `activeNuggetEntity := s.activeNugget.Load()`
- Replace with: `activeNuggetEntity := s.ctx.State.GetActiveNuggetID()`

**MODIFY** `GetSystemState()` method (debug):
- Find: `activeNuggetEntity := s.activeNugget.Load()`
- Replace with: `activeNuggetEntity := s.ctx.State.GetActiveNuggetID()`

## Verification
After this change:
- App compiles and runs
- Nuggets spawn and appear on screen
- Tab key jumps to nugget (uses existing JumpToNugget via InputHandler)
- Nugget collection works (EnergySystem clears via ClearActiveNuggetIfMatches)

## Important Notes
- Do NOT remove `JumpToNugget()` method yet (InputHandler still uses it)
- Do NOT change `EnergySystem.handleNuggetCollection()` - it already calls `ClearActiveNuggetIfMatches()`
- The mutex `s.mu` in NuggetSystem is still needed for spawn timing logic
- All 6 methods that reference activeNugget must be updated

---

## Phase 4: Decouple InputHandler from EnergySystem

# Task: Decouple InputHandler from EnergySystem (Typing Events)

## Context
InputHandler currently calls `h.energySystem.HandleCharacterTyping()` directly. This must be replaced with pushing `EventCharacterTyped` to the event queue.

## Goal
- InputHandler pushes `EventCharacterTyped` instead of direct method call
- Remove `energySystem` field from InputHandler
- Update constructor to not require EnergySystem parameter

## Files to Modify

### 1. `modes/input.go`

**MODIFY** the `InputHandler` struct:
```go
// CHANGE FROM:
type InputHandler struct {
    ctx          *engine.GameContext
    energySystem *systems.EnergySystem
    nuggetSystem *systems.NuggetSystem
}

// CHANGE TO:
type InputHandler struct {
    ctx          *engine.GameContext
    nuggetSystem *systems.NuggetSystem
}
```

**MODIFY** `NewInputHandler()` function:
```go
// CHANGE FROM:
func NewInputHandler(ctx *engine.GameContext, energySystem *systems.EnergySystem) *InputHandler {
    return &InputHandler{
        ctx:          ctx,
        energySystem: energySystem,
    }
}

// CHANGE TO:
func NewInputHandler(ctx *engine.GameContext) *InputHandler {
    return &InputHandler{
        ctx: ctx,
    }
}
```

**MODIFY** `handleInsertMode()` method - find the `tcell.KeyRune` case:

Find this code block:
```go
case tcell.KeyRune:
    // SPACE key: move right without typing, no heat contribution
    if ev.Rune() == ' ' {
        pos, ok := h.ctx.World.Positions.Get(h.ctx.CursorEntity)
        if ok && pos.X < h.ctx.GameWidth-1 {
            pos.X++
            h.ctx.World.Positions.Add(h.ctx.CursorEntity, pos)
        }
        return true
    }
    // Delegate character typing to energy system (reads from ECS)
    pos, _ := h.ctx.World.Positions.Get(h.ctx.CursorEntity)
    h.energySystem.HandleCharacterTyping(h.ctx.World, pos.X, pos.Y, ev.Rune())
```

Replace the last two lines (after space handling) with:
```go
    // Push typing event to queue (processed by EnergySystem via EventRouter)
    pos, _ := h.ctx.World.Positions.Get(h.ctx.CursorEntity)
    payload := &engine.CharacterTypedPayload{
        Char: ev.Rune(),
        X:    pos.X,
        Y:    pos.Y,
    }
    h.ctx.PushEvent(engine.EventCharacterTyped, payload, h.ctx.PausableClock.Now())
```

**ADD** import for engine package if not present:
```go
import (
    // ... existing imports ...
    "github.com/lixenwraith/vi-fighter/engine"
)
```

### 2. `cmd/vi-fighter/main.go`

**MODIFY** InputHandler creation:

Find:
```go
inputHandler := modes.NewInputHandler(ctx, energySystem)
```

Replace with:
```go
inputHandler := modes.NewInputHandler(ctx)
```

## Verification
After this change:
- App compiles and runs
- Typing in Insert mode works (characters consumed, cursor moves, scoring works)
- There may be ~50ms visual latency on cursor movement (acceptable)
- Error flash on wrong character works
- Heat/energy updates work

## Important Notes
- The space key handling remains unchanged (direct cursor move, no event)
- Only printable character typing goes through the event queue
- EnergySystem.HandleCharacterTyping() is now called via HandleEvent(), not directly
- Keep the nuggetSystem field for now (Phase 5 removes it)

---

## Phase 5: Decouple InputHandler from NuggetSystem

# Task: Decouple InputHandler from NuggetSystem (Nugget Jump)

## Context
InputHandler currently calls `h.nuggetSystem.JumpToNugget()` for Tab key functionality. This must be replaced with direct state/world queries and event-based energy deduction.

## Goal
- InputHandler queries `GameState.GetActiveNuggetID()` directly
- InputHandler queries `World.Positions.Get()` to find nugget position
- InputHandler pushes `EventEnergyTransaction` for cost deduction
- Remove `nuggetSystem` field from InputHandler
- Remove `SetNuggetSystem()` method

## Files to Modify

### 1. `modes/input.go`

**MODIFY** the `InputHandler` struct:
```go
// CHANGE FROM:
type InputHandler struct {
    ctx          *engine.GameContext
    nuggetSystem *systems.NuggetSystem
}

// CHANGE TO:
type InputHandler struct {
    ctx *engine.GameContext
}
```

**REMOVE** the `SetNuggetSystem()` method entirely:
```go
// DELETE THIS ENTIRE METHOD:
func (h *InputHandler) SetNuggetSystem(nuggetSystem *systems.NuggetSystem) {
    h.nuggetSystem = nuggetSystem
}
```

**MODIFY** `handleInsertMode()` - find the `tcell.KeyTab` case:

Find this code block:
```go
case tcell.KeyTab:
    // Tab: Jump to nugget if energy >= 10
    if h.nuggetSystem != nil {
        energy := h.ctx.State.GetEnergy()
        if energy >= 10 {
            // Get nugget position
            x, y := h.nuggetSystem.JumpToNugget(h.ctx.World)
            if x >= 0 && y >= 0 {
                // Deduct 10 from energy
                h.ctx.State.AddEnergy(-10)
                // Update cursor position in ECS
                h.ctx.World.Positions.Add(h.ctx.CursorEntity, components.PositionComponent{
                    X: x,
                    Y: y,
                })

                // Play bell sound for nugget collection
                if h.ctx.AudioEngine != nil {
                    cmd := audio.AudioCommand{
                        Type:       audio.SoundBell,
                        Priority:   1,
                        Generation: uint64(h.ctx.State.GetFrameNumber()),
                        Timestamp:  h.ctx.PausableClock.Now(),
                    }
                    h.ctx.AudioEngine.SendState(cmd)
                }
            }
        }
    }
    return true
```

Replace with:
```go
case tcell.KeyTab:
    // Tab: Jump to nugget if energy >= 10
    energy := h.ctx.State.GetEnergy()
    if energy < 10 {
        return true
    }

    // Get active nugget from centralized state
    nuggetID := engine.Entity(h.ctx.State.GetActiveNuggetID())
    if nuggetID == 0 {
        return true
    }

    // Query nugget position from World
    nuggetPos, ok := h.ctx.World.Positions.Get(nuggetID)
    if !ok {
        return true
    }

    // Move cursor to nugget position
    h.ctx.World.Positions.Add(h.ctx.CursorEntity, components.PositionComponent{
        X: nuggetPos.X,
        Y: nuggetPos.Y,
    })

    // Deduct energy via event
    payload := &engine.EnergyTransactionPayload{
        Amount: -10,
        Source: "NuggetJump",
    }
    h.ctx.PushEvent(engine.EventEnergyTransaction, payload, h.ctx.PausableClock.Now())

    // Play bell sound
    if h.ctx.AudioEngine != nil {
        cmd := audio.AudioCommand{
            Type:       audio.SoundBell,
            Priority:   1,
            Generation: uint64(h.ctx.State.GetFrameNumber()),
            Timestamp:  h.ctx.PausableClock.Now(),
        }
        h.ctx.AudioEngine.SendState(cmd)
    }
    return true
```

**MODIFY** `handleNormalMode()` - find the `tcell.KeyTab` case and apply the same changes as above.

### 2. `cmd/vi-fighter/main.go`

**REMOVE** the SetNuggetSystem call:
```go
// DELETE THIS LINE:
inputHandler.SetNuggetSystem(nuggetSystem)
```

## Verification
After this change:
- App compiles and runs
- Tab key in Insert mode jumps to nugget when energy >= 10
- Tab key in Normal mode jumps to nugget when energy >= 10
- Energy is deducted by 10 after jump
- Bell sound plays on successful jump
- Jump does nothing if no active nugget or insufficient energy

## Important Notes
- Energy deduction now happens via event (slight delay acceptable)
- Cursor movement is immediate (no delay)
- The bell sound still plays immediately (before energy event processed)
- Both Insert and Normal mode Tab handlers need identical changes

---

## Phase 6: Cleanup Dead Code

# Task: Remove Unused Methods and Dependencies

## Context
After Phases 1-5, several methods and dependencies are no longer used externally. Clean up the codebase.

## Goal
- Remove `EnergySystem.SetNuggetSystem()` (no longer called)
- Make `EnergySystem.HandleCharacterTyping()` private
- Remove `NuggetSystem.JumpToNugget()` (no longer called externally)
- Remove `NuggetSystem.GetActiveNugget()` (replaced by GameState method)
- Verify no dead imports

## Files to Modify

### 1. `systems/energy.go`

**REMOVE** `SetNuggetSystem()` method:
```go
// DELETE THIS ENTIRE METHOD:
func (s *EnergySystem) SetNuggetSystem(nuggetSystem *NuggetSystem) {
    s.nuggetSystem = nuggetSystem
}
```

**REMOVE** `nuggetSystem` field from struct:
```go
// In EnergySystem struct, DELETE this field:
nuggetSystem *NuggetSystem
```

**RENAME** `HandleCharacterTyping` to `handleCharacterTyping` (lowercase, private):
- Change function signature from `func (s *EnergySystem) HandleCharacterTyping(` to `func (s *EnergySystem) handleCharacterTyping(`

**UPDATE** `HandleEvent()` to call the renamed method:
- Change `s.HandleCharacterTyping(` to `s.handleCharacterTyping(`

**NOTE**: `handleNuggetCollection()` still calls `s.nuggetSystem.ClearActiveNuggetIfMatches()`. This must be changed to use GameState:

Find in `handleNuggetCollection()`:
```go
s.nuggetSystem.ClearActiveNuggetIfMatches(entity)
```

Replace with:
```go
s.ctx.State.ClearActiveNuggetID(uint64(entity))
```

### 2. `systems/nugget.go`

**REMOVE** `JumpToNugget()` method:
```go
// DELETE THIS ENTIRE METHOD (approximately 15 lines):
func (s *NuggetSystem) JumpToNugget(world *engine.World) (int, int) {
    // ...
}
```

**REMOVE** `GetActiveNugget()` method:
```go
// DELETE THIS ENTIRE METHOD:
func (s *NuggetSystem) GetActiveNugget() uint64 {
    return s.ctx.State.GetActiveNuggetID()
}
```

**KEEP** `ClearActiveNugget()` and `ClearActiveNuggetIfMatches()` - they may still be used internally or by other systems.

### 3. `cmd/vi-fighter/main.go`

**REMOVE** the nugget system wiring to energy system:
```go
// DELETE THIS LINE:
energySystem.SetNuggetSystem(nuggetSystem)
```

### 4. `systems/drain.go`

Check if DrainSystem uses nuggetSystem. If it calls methods that were removed, update to use GameState.

Find any calls to `s.nuggetSystem.ClearActiveNugget()` or similar and update to use `s.ctx.State.ClearActiveNuggetID(0)`.

## Verification
After this change:
- App compiles with no unused import warnings
- All existing functionality works
- No public methods exist that are only called internally

## Important Notes
- Run `go build ./...` to verify compilation
- Run `go vet ./...` to check for issues
- Some systems (DrainSystem, DecaySystem) may still hold nuggetSystem references for collision handling - evaluate if these can also be migrated

---

## Updated CLAUDE.md

# vi-fighter Development Guide for Claude Code

## PROJECT CONTEXT
vi-fighter is a terminal-based typing game in Go using a compile-time Generics-based ECS (Go 1.24+). The architecture combines real-time lock-free updates (atomics) for input/rendering with a discrete clock-tick system for game logic.

**Go Version:** 1.24+

## ARCHITECTURE OVERVIEW

### Event-Driven Communication
The game uses an **EventRouter** pattern for decoupled system communication:

```
InputHandler ──push──▶ EventQueue ──consume──▶ EventRouter ──dispatch──▶ Systems
                                                    │
                                                    ├──▶ EnergySystem (typing, transactions)
                                                    └──▶ CleanerSystem (spawn requests)
```

**Key Principle**: Systems do NOT call each other directly. They communicate via:
1. **Events** (async, via EventQueue/EventRouter)
2. **GameState** (shared atomics for cross-system state)
3. **World queries** (read component data)

### System Execution Order
```
1. ClockScheduler.processTick()
   ├── EventRouter.DispatchAll()     # Events processed FIRST
   │   ├── EnergySystem.HandleEvent()
   │   └── CleanerSystem.HandleEvent()
   └── World.Update()                # Systems run in priority order
       ├── BoostSystem      (5)
       ├── EnergySystem     (10)     # Time-based logic only
       ├── SpawnSystem      (15)
       ├── NuggetSystem     (18)
       ├── GoldSystem       (20)
       ├── CleanerSystem    (22)     # Physics only, no event consumption
       ├── DrainSystem      (25)
       ├── DecaySystem      (30)
       └── FlashSystem      (35)
```

### Event Types

| Event | Producer | Consumer | Payload |
|-------|----------|----------|---------|
| `EventCharacterTyped` | InputHandler | EnergySystem | `*CharacterTypedPayload{Char, X, Y}` |
| `EventEnergyTransaction` | InputHandler | EnergySystem | `*EnergyTransactionPayload{Amount, Source}` |
| `EventCleanerRequest` | EnergySystem | CleanerSystem | `nil` |
| `EventDirectionalCleanerRequest` | InputHandler, EnergySystem | CleanerSystem | `*DirectionalCleanerPayload{OriginX, OriginY}` |
| `EventCleanerFinished` | CleanerSystem | (observers) | `nil` |
| `EventGoldSpawned` | GoldSystem | (observers) | `nil` |
| `EventGoldComplete` | EnergySystem | (observers) | `nil` |

### Shared State (GameState)

| Field | Type | Purpose |
|-------|------|---------|
| `Energy` | `atomic.Int64` | Player score |
| `Heat` | `atomic.Int64` | Combo meter (0-100) |
| `ActiveNuggetID` | `atomic.Uint64` | Current nugget entity ID |
| `BoostEnabled` | `atomic.Bool` | Speed boost active |
| `FrameNumber` | `atomic.Int64` | Current frame |

---

## MODIFICATION PATTERNS

### Adding a New Event Type

1. **Define in `engine/events.go`**:
```go
const (
    // ... existing ...
    EventMyNewEvent
)

type MyNewEventPayload struct {
    Field1 int
    Field2 string
}

// Update String() method
```

2. **Push from producer**:
```go
payload := &engine.MyNewEventPayload{Field1: 42}
ctx.PushEvent(engine.EventMyNewEvent, payload, ctx.PausableClock.Now())
```

3. **Consume in system** (implement EventHandler):
```go
func (s *MySystem) EventTypes() []engine.EventType {
    return []engine.EventType{engine.EventMyNewEvent}
}

func (s *MySystem) HandleEvent(world *engine.World, event engine.GameEvent) {
    if payload, ok := event.Payload.(*engine.MyNewEventPayload); ok {
        // Process
    }
}
```

4. **Register with router** (in `main.go`):
```go
clockScheduler.RegisterEventHandler(mySystem)
```

### Reading Cross-System State

```go
// From InputHandler or any non-system code:
nuggetID := engine.Entity(h.ctx.State.GetActiveNuggetID())
if nuggetID != 0 {
    pos, ok := h.ctx.World.Positions.Get(nuggetID)
}

// From a System:
energy := s.ctx.State.GetEnergy()
heat := s.ctx.State.GetHeat()
```

### Pushing Events from InputHandler

```go
// Typing event
payload := &engine.CharacterTypedPayload{Char: r, X: x, Y: y}
h.ctx.PushEvent(engine.EventCharacterTyped, payload, h.ctx.PausableClock.Now())

// Energy cost
payload := &engine.EnergyTransactionPayload{Amount: -10, Source: "Jump"}
h.ctx.PushEvent(engine.EventEnergyTransaction, payload, h.ctx.PausableClock.Now())

// Directional cleaners
payload := &engine.DirectionalCleanerPayload{OriginX: x, OriginY: y}
h.ctx.PushEvent(engine.EventDirectionalCleanerRequest, payload, h.ctx.PausableClock.Now())
```

---

## DECOUPLING RULES

### InputHandler Independence
- **NEVER** hold System references (no `*EnergySystem`, `*NuggetSystem`, etc.)
- **NEVER** call System methods directly
- **DO** read from `GameState` for shared state
- **DO** query `World` for entity/component data
- **DO** push events for actions that affect game state

### System Independence
- **NEVER** call other System methods directly (except via DI for tightly coupled logic like GoldSystem↔EnergySystem)
- **DO** implement `EventHandler` interface for event consumption
- **DO** push events to trigger other systems
- **DO** read/write `GameState` for shared atomics

### Event vs Direct Call Decision

| Scenario | Use Event | Use Direct |
|----------|-----------|------------|
| Cross-system trigger | ✅ | ❌ |
| Atomic state change | ❌ | ✅ (`GameState`) |
| Input → System | ✅ | ❌ |
| System → System (same tick) | ✅ | ❌ |
| Reading component data | ❌ | ✅ (`World.Query()`) |

---

## CURRENT TASK CONTEXT

### Event Router Migration (Completed)
- CleanerSystem uses EventRouter for spawn events
- EnergySystem uses EventRouter for typing/transaction events
- InputHandler pushes events, no system dependencies

### Pending Cleanup
- Evaluate DrainSystem/DecaySystem nuggetSystem references
- Consider future event types for gold sequence completion

---

## TESTING CHECKLIST

### Event Flow
- [ ] Typing in Insert mode triggers `EventCharacterTyped`
- [ ] Tab jump triggers `EventEnergyTransaction`
- [ ] Enter at heat≥10 triggers `EventDirectionalCleanerRequest`
- [ ] Gold completion at max heat triggers `EventCleanerRequest`
- [ ] Nugget at max heat triggers `EventDirectionalCleanerRequest`

### State Consistency
- [ ] `GameState.ActiveNuggetID` updates on nugget spawn/clear
- [ ] Energy deduction visible after ~50ms (event processing delay)
- [ ] Heat updates immediately on typing

---

## ENVIRONMENT
```bash
export GOPROXY="https://goproxy.io,direct"
apt-get install -y libasound2-dev
go mod tidy
go build ./...
```

## FILE STRUCTURE

```
engine/
├── ecs.go              # Entity, System interface
├── events.go           # EventType, payloads, EventQueue
├── event_router.go     # EventHandler interface, EventRouter
├── game_context.go     # GameContext (World, State, queues)
├── game_state.go       # GameState (atomics, snapshots)
├── clock_scheduler.go  # ClockScheduler (tick loop, event dispatch)
└── ...

systems/
├── energy.go           # EnergySystem (typing logic, EventHandler)
├── cleaner.go          # CleanerSystem (spawn/physics, EventHandler)
├── nugget.go           # NuggetSystem (spawn logic)
└── ...

modes/
├── input.go            # InputHandler (event producer, no system deps)
└── ...
```
```

---

## Execution Summary

| Phase | Focus | Key Changes |
|-------|-------|-------------|
| 1 | CleanerSystem → EventRouter | Add `EventHandler` impl, remove `ConsumeEvents()` from `Update()`, register with router |
| 2 | EnergySystem → EventRouter | Add `EventHandler` impl for typing/transaction events, register with router |
| 3 | NuggetSystem state hoist | Replace local `activeNugget` atomic with `GameState.ActiveNuggetID` |
| 4 | InputHandler typing decoupling | Push `EventCharacterTyped` instead of direct `HandleCharacterTyping()` call |
| 5 | InputHandler nugget decoupling | Query `GameState`/`World` directly, push `EventEnergyTransaction` for cost |
| 6 | Cleanup | Remove dead methods, make internal methods private |
