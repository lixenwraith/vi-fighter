# Design Document: Triple-Stack Clock Architecture

## 1. Overview
The `vi-fighter` event system requires a high-resolution processing layer to eliminate logic-chain latency (cascading events) while maintaining the stability of the game tick and rendering frames. This design introduces a **Triple-Stack Clock Architecture**, decoupling reactive logic from world-state updates and rendering.

### 1.1 Architecture Hierarchy

| Layer | Frequency | Responsible Component | Primary Function |
| :--- | :--- | :--- | :--- |
| **Frame Clock** | 16.6ms (60Hz) | `RenderOrchestrator` | Buffer composition, post-processing, and ANSI I/O. |
| **Game Tick** | 50ms (20Hz) | `ClockScheduler` | Heavy ECS Systems (Spawn, AI, Physics, Hull Cleanup). |
| **Event Loop** | 1ms (1000Hz) | `ClockScheduler` (Sub-step) | Reactive logic settling (Typing → Composite → Death). |

---

## 2. The Event Sub-stepping Model

### 2.1 Purpose
Logic in `vi-fighter` is frequently cascaded. An input event (`EventCharacterTyped`) triggers a sequence of dependent events (`EventMemberTyped` → `EventRequestDeath` → `EventGoldComplete`). In a single-pass 50ms model, this chain incurs a 150ms delay. The 1ms Event Loop allows the system to resolve up to 50 levels of event depth within a single game tick, appearing instantaneous to the player.

### 2.2 Mechanism
The Event Loop is a high-frequency goroutine that continuously monitors the `EventQueue`. It operates as a "reactive pump" that settles system states before the next heavy Game Tick or Frame Render occurs.

---

## 3. Concurrency & Guard Patterns

### 3.1 Lock Contention Guard
Because the Event Loop runs at 1ms, it must not block if the `World` is currently occupied by a heavy 50ms Game Tick (e.g., complex spatial queries or batch spawning).

**Design Pattern: Non-Blocking Back-off**
1.  The Event Loop attempts to acquire the `World.UpdateMutex`.
2.  If the mutex is held (by the Game Tick or Render Snapshot), the Event Loop **backs off** immediately.
3.  The pending events remain in the `EventQueue` to be processed in the next 1ms slice.

### 3.2 Sequential Consistency
To prevent race conditions, the Event Loop, Game Tick, and Render Snapshots all synchronize on the `World.UpdateMutex`. Only one component may mutate or snapshot the ECS Stores at any given micro-moment.

---

## 4. Implementation Patterns

### 4.1 Event Loop Structure
The loop uses a `time.Ticker` for 1ms resolution. It utilizes a `TryLock` pattern (or a `RunSafe` wrapper) to process events.

```go
// Logic Pattern: Reactive Event Loop
func (cs *ClockScheduler) eventLoop() {
    ticker := time.NewTicker(1 * time.Millisecond)
    for {
        select {
        case <-cs.stopChan: return
        case <-ticker.C:
            if cs.isPaused.Load() { continue }
            
            // GUARD: Attempt to process without blocking reactive thread
            // If World is locked by 50ms Game Tick, this skips and retries in 1ms
            if cs.world.TryLock() {
                cs.dispatchAndProcessOnePass()
                cs.world.Unlock()
            }
        }
    }
}
```

### 4.2 Game Tick Integration
The 50ms logic tick remains the authority for heavy system execution. It no longer needs to loop events internally, as the Event Loop handles settling.

```go
// Logic Pattern: 50ms Game Tick
func (cs *ClockScheduler) processTick() {
    cs.world.Lock() // Authorities the World for heavy systems
    defer cs.world.Unlock()

    // Update simulation time
    cs.timeRes.Update(...)

    // Run FSM (State transitions may emit events for the 1ms loop to catch)
    cs.fsm.Update(cs.world, cs.tickInterval)

    // Run standard ECS systems (e.g., SpawnSystem, CullSystem)
    cs.world.UpdateLocked()
}
```

---

## 5. Summary of Benefits

1.  **Zero-Latency Feel:** Input reactions (UI blinks, character removal) resolve in $\le 1ms$.
2.  **Logic Settling:** Chained events (e.g., chain explosions) resolve within the same 50ms tick.
3.  **Pressure Management:** Event "storms" are distributed over multiple 1ms slices if they exceed the logic budget, preventing frame spikes.
4.  **Decoupled Complexity:** Systems do not need to know about each other; the high-frequency loop ensures the "glue" (events) is applied rapidly.