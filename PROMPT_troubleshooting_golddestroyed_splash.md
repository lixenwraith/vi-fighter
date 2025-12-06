# Troubleshooting event handling in vi-fighter
[complex, full, colab, clarify]

**Strict** Rule: No code fix to be generated as the response until `code` action directive is used. Pure, deep analysis including flow trace is required.

## Context:
- A new splash system was added to the game recently. It is functional.
- Splash is (or should be) pur ECS, triggered by 2 event types and nothing else [verify]:
1. via input NORMAL or INSERT modes, when a successful command is entered of when a character is correctly typed (trigger through energy system) EventCharacterTyped > EventSplashRequest > spawn splash ( SplashModeTransient ) [verify]. related files: engine/events.go, engine/event_router.go, systems/energy.go, systems/splash.go, components/splash.go.
2. when gold is spawned, a countdown splash is triggered via EventGoldSpawned > EventSplashRequest > spawn splash ( SplashModePersistent ), that starts counting down from 9 to 0 (gold max uptime timer). gold is either timeout despawned (EventGoldTimeout), completed by typing 10 random characters of it correctly (EventGoldComplete), or destoryed by drain (EventGoldDestroyed) [verify]. related files: systems/gold.go, components/gold.go, systems/drain.go and all previously mentioned files.

### Issue:
- When purely handled by events, when gold is destroyed, the EventGoldDestroyed does not stop countdown splash, leaving it counting down instead of disappearing, even after gold is gone. In trying to find out if game transitions to next phase while this situation happens in rare occation a panic occurred due to nil referencing (unclear where, the message/stack trace was truncated). EmergencyReset() mechanic of main.go > terminal/terminal.go for panic recovery to sane terminal state did not engage.
- If gold is destroyed before countdown finishes, it reaches `0`, and leaves a black `0` standing there until next gold countdown starts that due to unique splash entity per type, cleans up the old stale and stuck splash.
- As a workaround, it was attempted to directly check if there is no gold system active, clean up the splash. It may be the cause for panic when drain had destroyed the gold and check was initiated on a non-existing entity due to not having enough guardrails to handle the rare race situation (it happened only once in 10s of tries).
- A preliminary analysis by Claude (within ROOT_CAUSE_ANALYSIS_REVISION_ONE xml tags) is provided below that may or may not be a valid approach. Multiple iterations has been tried with Claude, but did not fix the issue.

## Task:
- Perform a complete analysis of splash system concerning triggers of gold. Try to find structural issues, race potentials, or implementation gaps.
- Though there are workarounds to handle this, if the issue is related to bad event design, it should be fixed at the root, e.g. being fragile if component that triggers it is destroyed and certain defensive mechanisms should be in place.
- Provide a full trace report of all types of splash triggers with flow diagrams.

**ABSOLUTELY** NO code implementation. Snippets to demonstrate ideas or show problematic parts are ok.

[[eval]]

<ROOT_CAUSE_ANALYSIS_REVISION_ONE>

Looking at the event dispatch confirmation `if ev.Type == EventGoldDestroyed {`, the failure must occur during handler execution. Let me trace the potential failure points.

---

## Failure Point Analysis

### Suspect 1: Type Assertion Silently Fails

```go
case engine.EventGoldComplete, engine.EventGoldTimeout, engine.EventGoldDestroyed:
    if payload, ok := event.Payload.(*engine.GoldCompletionPayload); ok {
        s.handleGoldFinish(world, payload.SequenceID)
    }
    // Silent fail if ok == false - no else branch
```

If the type assertion fails, nothing happens. Add logging:
```go
if payload, ok := event.Payload.(*engine.GoldCompletionPayload); ok {
    s.handleGoldFinish(world, payload.SequenceID)
} else {
    fmt.Printf("DEBUG: EventGoldDestroyed type assertion failed, got %T\n", event.Payload)
}
```

### Suspect 2: Slice Mutation During Iteration

```go
func (s *SplashSystem) cleanupSplashesByMode(world *engine.World, mode components.SplashMode) {
    entities := world.Splashes.All()  // Returns slice view?
    for _, entity := range entities {
        // ...
        world.DestroyEntity(entity)  // Modifies store while iterating
    }
}
```

If `All()` returns a view into the store's internal slice, `DestroyEntity()` may corrupt iteration. This matches patterns in other systems that collect `toDestroy` first:

```go
// Correct pattern (used in Update)
var toDestroy []engine.Entity
for _, entity := range entities {
    // ...
    toDestroy = append(toDestroy, entity)
}
for _, e := range toDestroy {
    world.DestroyEntity(e)
}
```

### Suspect 3: Event Dispatch vs System Priority Race

```
Tick N:
  DispatchAll()         ← processes events from tick N-1
  GoldSystem.Update()   ← priority 20, might spawn new gold
  DrainSystem.Update()  ← priority 25, destroys gold, pushes event
  SplashSystem.Update() ← timer exists, event queued but not dispatched

Tick N+1:
  DispatchAll()         ← EventGoldDestroyed processed NOW
  GoldSystem.Update()   ← if phase transitioned fast, could spawn Gold B
  SplashSystem.Update() ← timer for Gold B exists (different ID)
```

If a new gold spawns before the handler runs, `handleGoldSpawn()` already calls `cleanupSplashesByMode()` for old timer, but creates new timer. The delayed EventGoldDestroyed then tries to destroy by ID - which no longer matches.

---

## Root Cause

The ID-matching approach is fundamentally flawed for single-instance mechanics:

```go
// Original - fragile
if splash.SequenceID == sequenceID { ... }
```

Between event push and dispatch (1+ ticks), the GameState can cycle through:
1. Gold A destroyed → EventGoldDestroyed(A) pushed
2. Phase transitions: GoldComplete → DecayWait → Normal
3. Gold B spawns → timer recreated with new ID
4. EventGoldDestroyed(A) dispatched → no timer with ID A exists

---

## Robust Solution

Combine immediate state check with defensive cleanup:

```go
// FILE: systems/splash.go

func (s *SplashSystem) HandleEvent(world *engine.World, event engine.GameEvent) {
    switch event.Type {
    case engine.EventSplashRequest:
        if payload, ok := event.Payload.(*engine.SplashRequestPayload); ok {
            s.handleSplashRequest(world, payload, event.Timestamp)
        }

    case engine.EventGoldSpawned:
        if payload, ok := event.Payload.(*engine.GoldSpawnedPayload); ok {
            s.handleGoldSpawn(world, payload, event.Timestamp)
        }

    case engine.EventGoldComplete, engine.EventGoldTimeout, engine.EventGoldDestroyed:
        // Unconditional cleanup - don't trust sequenceID matching
        s.cleanupSplashesByMode(world, components.SplashModePersistent)
    }
}

// cleanupSplashesByMode - fixed iteration safety
func (s *SplashSystem) cleanupSplashesByMode(world *engine.World, mode components.SplashMode) {
    var toDestroy []engine.Entity
    
    entities := world.Splashes.All()
    for _, entity := range entities {
        splash, ok := world.Splashes.Get(entity)
        if !ok {
            continue
        }
        if splash.Mode == mode {
            toDestroy = append(toDestroy, entity)
        }
    }
    
    for _, e := range toDestroy {
        world.DestroyEntity(e)
    }
}
```

Key changes:
1. Remove payload type assertion requirement for gold termination events
2. Fix slice mutation during iteration
3. Unconditional cleanup regardless of sequenceID

The orphan detection in `Update()` remains as belt-and-suspenders defense.

</ROOT_CAUSE_ANALYSIS_REVISION_ONE>

---

<Gemini_RESPONSE>

[complex, clarify]

## Analysis of Splash System & Gold Trigger Failure

### 1. Root Cause Trace: The "Stuck Timer"
The persistence of the gold timer after `EventGoldDestroyed` is primarily due to a missing case handler in the `SplashSystem`, compounded by a specific condition where the backup "orphan detection" logic fails.

#### A. The Event Handling Gap (Primary Defect)
In `systems/splash.go`, the `HandleEvent` method explicitly subscribes to `EventGoldDestroyed` in `EventTypes()`, but the switch statement inside `HandleEvent` **fails to include a case for it**.

**Current Flow (Broken):**
1. **Trigger:** `DrainSystem` collides with Gold Entity.
2. **Event Push:** `DrainSystem` pushes `EventGoldDestroyed`.
3. **Dispatch:** `ClockScheduler` dispatches event to `SplashSystem`.
4. **Handling:** `SplashSystem.HandleEvent` switch receives the event.
    - Case `EventSplashRequest`: No match.
    - Case `EventGoldSpawned`: No match.
    - Case `EventGoldComplete, EventGoldTimeout`: No match.
    - **Result:** Event falls through default (implicit) and is **ignored**.
5. **Outcome:** The cleanup logic associated with the event never runs. The timer entity persists.

#### B. The Orphan Detection Failure (Secondary Defect)
`SplashSystem.Update` contains "orphan detection" logic designed to destroy persistent splashes if the corresponding game state is inactive.

```go
// Existing Logic
goldSnapshot := s.ctx.State.ReadGoldState(timeRes.GameTime)
if !goldSnapshot.Active || splash.SequenceID != goldSnapshot.SequenceID {
    toDestroy = append(toDestroy, entity) // Should clean up
}
```

For the timer to remain stuck (as reported), `goldSnapshot.Active` MUST be `true` and `SequenceID` MUST match. This implies that while the Drain destroyed the entities, it **failed to update the GameState**.

**Scenario for Failed State Update:**
1. **Zombie Entity:** A gold entity from a *previous* sequence (ID 1) was missed during cleanup (rare bug or race).
2. **New Sequence:** A new gold sequence (ID 2) spawns and is Active.
3. **Collision:** Drain moves and hits the *Zombie Entity* (ID 1).
4. **Drain Check:** `DrainSystem` compares entity ID (1) vs GameState ID (2).
   ```go
   // systems/drain.go
   if !goldSnapshot.Active || goldSnapshot.SequenceID != sequenceID {
       world.DestroyEntity(entity) // Destroys zombie
       return // RETURNS EARLY - Does not call DeactivateGoldSequence!
   }
   ```
5. **Result:** The zombie is destroyed, but the *Active* gold sequence (ID 2) remains Active. The timer for ID 2 (which matches GameState) persists correctly, but visually the player might think "Gold was hit" (if the zombie was near the active one).
   *Correction:* If the Drain hit the *actual* active gold entity, `DeactivateGoldSequence` is called, `Active` becomes `false`, and orphan detection *should* work.
   *Likelihood:* The user reported "gold is destroyed... leaves it counting down". If the *entire* gold sequence is gone, `Deactivate` must have been called. If the timer persists, `SplashSystem.Update` is not running or `ReadGoldState` is stale. Given the "panic" report, it's highly likely the panic prevents the update loop from cleaning up.

### 2. Root Cause Trace: The "Panic & Crash"
The failure of `EmergencyReset` confirms the panic occurs in a background goroutine.

#### Critical Architecture Vulnerability
The `ClockScheduler` runs the game logic in a dedicated goroutine:
```go
// engine/clock_scheduler.go
func (cs *ClockScheduler) schedulerLoop() {
    // ...
    // NO recover() block here!
    // ...
    cs.processTick()
}
```
`processTick` calls `eventRouter.DispatchAll`, which invokes `SplashSystem.HandleEvent`.

**The Crash Flow:**
1. **Panic Origin:** A panic occurs anywhere within `processTick` (e.g., inside a system update or event handler).
2. **Propagation:** The panic bubbles up the `schedulerLoop` goroutine stack.
3. **Crash:** Since `schedulerLoop` has no `recover()`, the Go runtime terminates the **entire program immediately**.
4. **Bypass:** The `defer func()` in `main.go` (running on the main goroutine) is **never notified** and cannot run `EmergencyReset`. The terminal is left in raw mode/alternate screen.

#### Potential Panic Source (Nil Reference)
User mentioned "workaround attempted to directly check if there is no gold system active".
If code similar to `s.goldSystem.IsActive()` was added to `SplashSystem`:
1. `SplashSystem` struct does not have a `goldSystem` field initialized in the provided context.
2. If added manually without injection (wiring in `main.go`), it remains `nil`.
3. Accessing `nil.IsActive()` causes panic.

### 3. Implementation Plan

The solution requires three targeted fixes:
1.  **Event Handling:** Explicitly handle `EventGoldDestroyed` in `SplashSystem`.
2.  **Panic Safety:** Wrap `schedulerLoop` in a `defer recover()` block to catch logic crashes and allow `main` to handle cleanup (or handle it locally).
3.  **Iteration Safety:** Ensure `cleanupSplashesByMode` uses the safe "collect then destroy" pattern (which it currently does not appear to use in the snippet provided in analysis, though `Update` does).

Wait for the `[[code]]` directive to generate the fixes.

[[catalog]]
systems/splash.go
engine/clock_scheduler.go
engine/event_router.go
engine/events.go
systems/drain.go
main.go

</Gemini_RESPONSE>

---

<User_PROMPT>

## Feedback:
[complex, direct]
- agreed on goroutine panic propagation strategy.
- add the missed case handling. It was a rookie miss.
- for panic issue, the reasoning seems not to be conclusive. **WAIT**  on the fix for this item till next prompt that will be 100% focused.

**INDICATE** where the fix is applied in the code by putting `// ADDED: `, `// CHANGED: `, or `// FIXED: ` comment.

[[code]]

</User_PROMPT>

---

<Gemini_RESPONSE>

// FILE: systems/splash.go

// ... existing code ...

// HandleEvent processes events to create or destroy splash entities
func (s *SplashSystem) HandleEvent(world *engine.World, event engine.GameEvent) {
switch event.Type {
case engine.EventSplashRequest:
if payload, ok := event.Payload.(*engine.SplashRequestPayload); ok {
s.handleSplashRequest(world, payload, event.Timestamp)
}

	case engine.EventGoldSpawned:
		if payload, ok := event.Payload.(*engine.GoldSpawnedPayload); ok {
			s.handleGoldSpawn(world, payload, event.Timestamp)
		}

	// FIXED: Added EventGoldDestroyed case to ensure timer is cleaned up when gold is destroyed by mechanics like Drain
	case engine.EventGoldComplete, engine.EventGoldTimeout, engine.EventGoldDestroyed:
		if payload, ok := event.Payload.(*engine.GoldCompletionPayload); ok {
			s.handleGoldFinish(world, payload.SequenceID)
		}
	}
}

// ... existing code ...

</Gemini_RESPOSNE>

---

<User_PROMPT>

## Task:
- now provide panic propagation, not only for clock scheduler, but for any goroutine that's not caught. The goal is never to have broken terminal. panic should still occur, but engage EmergencyReset() and show stack trace in a sane terminal.
- make sure to check other goroutine and propagate their panic properly if exists.

**WAIT** on the fix for the cause of panic, let's handle the symptom for now, then we get to the root cause analysis. Even if the splash issue is fixed, the same broken state may occur at a different time and we should have a robust recovery mechanism to return terminal to sane state and show proper stack trace for troubleshooting.

[complex, direct, block]
[[code]]

</User_PROMPT>

---

<Gemini_RESPONSE>

// FILE: cmd/vi-fighter/main.go

// ... existing code ...
// Main game loop
frameTicker := time.NewTicker(constants.FrameUpdateInterval)
defer frameTicker.Stop()

	eventChan := make(chan terminal.Event, 256)
	go func() {
		// ADDED: Panic recovery for input polling goroutine to ensure terminal cleanup
		defer func() {
			if r := recover(); r != nil {
				terminal.EmergencyReset(os.Stdout)
				fmt.Fprintf(os.Stderr, "\n\x1b[31mEVENT POLLER CRASHED: %v\x1b[0m\n", r)
				fmt.Fprintf(os.Stderr, "Stack Trace:\n%s\n", debug.Stack())
				os.Exit(1)
			}
		}()

		for {
			eventChan <- term.PollEvent()
		}
	}()

	// Track last update state for rendering
	var updatePending bool
// ... existing code ...

// FILE: engine/clock_scheduler.go

// ... existing code ...
// schedulerLoop runs the main scheduling loop with pause awareness
// Implements adaptive sleeping that respects pause state to avoid busy-waiting
func (cs *ClockScheduler) schedulerLoop() {
defer cs.wg.Done()

	// ADDED: Panic recovery for game logic goroutine
	defer func() {
		if r := recover(); r != nil {
			// Restore terminal to sane state using the terminal package directly
			// Assumes terminal, os, fmt, debug imports are handled by IDE/User
			terminal.EmergencyReset(os.Stdout)
			fmt.Fprintf(os.Stderr, "\n\x1b[31mGAME LOOP CRASHED: %v\x1b[0m\n", r)
			fmt.Fprintf(os.Stderr, "Stack Trace:\n%s\n", debug.Stack())
			os.Exit(1)
		}
	}()

	// Initialize next tick deadline
	cs.mu.Lock()
	cs.nextTickDeadline = cs.timeProvider.Now().Add(cs.tickInterval)
// ... existing code ...

// FILE: systems/spawn.go

// ... existing code ...
// preFetchNextContent loads next content batch in background
func (s *SpawnSystem) preFetchNextContent() {
// ADDED: Panic recovery for content loader goroutine
defer func() {
if r := recover(); r != nil {
terminal.EmergencyReset(os.Stdout)
fmt.Fprintf(os.Stderr, "\n\x1b[31mCONTENT LOADER CRASHED: %v\x1b[0m\n", r)
fmt.Fprintf(os.Stderr, "Stack Trace:\n%s\n", debug.Stack())
os.Exit(1)
}
}()

	// Get new content from ContentManager
	lines, _, err := s.contentManager.SelectRandomBlockWithValidation()
	if err != nil || len(lines) == 0 {
// ... existing code ...

// FILE: terminal/input.go

// ... existing code ...
// readLoop is the main input reading goroutine
func (r *inputReader) readLoop() {
defer close(r.doneCh)

	// ADDED: Panic recovery for raw input reader
	defer func() {
		if r := recover(); r != nil {
			EmergencyReset(os.Stdout)
			fmt.Fprintf(os.Stderr, "\n\x1b[31mINPUT READER CRASHED: %v\x1b[0m\n", r)
			fmt.Fprintf(os.Stderr, "Stack Trace:\n%s\n", debug.Stack())
			os.Exit(1)
		}
	}()

	buf := make([]byte, 256)

	for {
// ... existing code ...

// FILE: terminal/resize_unix.go

// ... existing code ...
// watchLoop monitors for resize signals
func (r *resizeHandler) watchLoop() {
defer close(r.doneCh)

	// ADDED: Panic recovery for resize signal handler
	defer func() {
		if r := recover(); r != nil {
			EmergencyReset(os.Stdout)
			fmt.Fprintf(os.Stderr, "\n\x1b[31mRESIZE HANDLER CRASHED: %v\x1b[0m\n", r)
			fmt.Fprintf(os.Stderr, "Stack Trace:\n%s\n", debug.Stack())
			os.Exit(1)
		}
	}()

	for {
		select {
		case <-r.stopCh:
// ... existing code ...

</Gemini_RESPONSE>

---

<User_PROMPT>

## Feedback:
- changes implemented, codebase in context updated. old codebase removed to reduce noise, refer to the latest code.

- With mostly sane state of panic now, below is the proper stack trace caused by drain/gold interaction.
```
GAME LOOP CRASHED: panic called with nil argument
Stack Trace:
goroutine 16 [running]:
runtime/debug.Stack()
/usr/lib/go/src/runtime/debug/stack.go:26 +0x68
github.com/lixenwraith/vi-fighter/engine.(*ClockScheduler).schedulerLoop.func1()
/home/lixen/git/lixenwraith/vi-fighter/engine/clock_scheduler.go:173 +0xba
panic({0x0?, 0x0?})
/usr/lib/go/src/runtime/panic.go:783 +0x132
github.com/lixenwraith/vi-fighter/engine.(*EventRouter).DispatchAll(0xc00009a180, 0xc00010c1a0)
/home/lixen/git/lixenwraith/vi-fighter/engine/event_router.go:58 +0x22e
github.com/lixenwraith/vi-fighter/engine.(*ClockScheduler).processTick.func1()
/home/lixen/git/lixenwraith/vi-fighter/engine/clock_scheduler.go:304 +0x78
github.com/lixenwraith/vi-fighter/engine.(*World).RunSafe(0xc00010c1a0, 0xc0000a5d80)
/home/lixen/git/lixenwraith/vi-fighter/engine/world.go:135 +0x8f
github.com/lixenwraith/vi-fighter/engine.(*ClockScheduler).processTick(0xc0000a0d00)
/home/lixen/git/lixenwraith/vi-fighter/engine/clock_scheduler.go:303 +0xe6
github.com/lixenwraith/vi-fighter/engine.(*ClockScheduler).schedulerLoop(0xc0000a0d00)
/home/lixen/git/lixenwraith/vi-fighter/engine/clock_scheduler.go:230 +0x625
created by github.com/lixenwraith/vi-fighter/engine.(*ClockScheduler).Start in goroutine 1
/home/lixen/git/lixenwraith/vi-fighter/engine/clock_scheduler.go:147 +0xbf
```

## Task:
[complex, explore, colab]
- Perform a thorough trace and analysis of the panic and provide a report with root cause analysis.
- Consider removal of the patch work done as the workaround for orphaned gold handling that may not be real and can actually be the cause of the panic if racing with event.
- [plan] Based on above 2 tasks, provide a final concise plan.
- based on the plan, provide the code implementation for the fix.

[block]
[[code]]

</User_PROMPT>

---

<Gemini_RESPONSE>

[plan]

## Root Cause Analysis
1.  **Immediate Crash Source:** The stack trace confirms an explicit `panic(nil)` call exists in `engine/event_router.go` at line 58. This debug trap was accidentally left in the code specifically checking for `EventGoldDestroyed`. When `DrainSystem` destroys gold, it emits this event, triggering the panic.
2.  **Recovery Confirmation:** The stack trace output ("GAME LOOP CRASHED...") confirms the `defer recover()` mechanism added to `ClockScheduler` is functioning correctly, trapping the panic and preventing terminal corruption, though the panic itself halts the game logic.
3.  **Event Logic:** The previous fix to `SplashSystem.HandleEvent` correctly added the `EventGoldDestroyed` case. The event flow is now:
    *   `DrainSystem` detects collision -> Emits `EventGoldDestroyed` -> `EventRouter` dispatches -> `SplashSystem` destroys Splash Entity.
4.  **Redundant Polling:** The "orphan detection" logic in `SplashSystem.Update` is a polling fallback that checks `GameState` every tick. With the event handler fixed, this is redundant and masks potential event ordering bugs.

## Implementation Plan
1.  **Remove Debug Panic:** Delete the `if ev.Type == EventGoldDestroyed { panic(nil) }` block from `engine/event_router.go`.
2.  **Clean Up Splash System:** Remove the "orphan detection" polling block from `systems/splash.go`. The system will now rely entirely on the deterministic `EventGoldDestroyed`, `EventGoldTimeout`, and `EventGoldComplete` events to manage the lifecycle of persistent splash timers.

[[code]]

// FILE: engine/event_router.go

// ... existing code ...
// DispatchAll consumes all pending events and routes to handlers
// Events are processed in FIFO order
// All handlers for an event type are called before moving to the next event
// Must be called once per tick, BEFORE World.Update()
func (r *EventRouter) DispatchAll(world *World) {
events := r.queue.Consume()
for _, ev := range events {
handlers := r.handlers[ev.Type]
// FIXED: Removed debug panic that was causing crash on Gold destruction
for _, h := range handlers {
h.HandleEvent(world, ev)
}
}
}

// HasHandlers returns true if any handlers are registered for the given type
// ... existing code ...

// FILE: systems/splash.go

// ... existing code ...
entities := world.Splashes.All()
for _, entity := range entities {
splash, ok := world.Splashes.Get(entity)
if !ok {
continue
}

		switch splash.Mode {
		case components.SplashModeTransient:
			// Check expiry
			if nowNano-splash.StartNano >= splash.Duration {
				toDestroy = append(toDestroy, entity)
			}

		case components.SplashModePersistent:
			// CHANGED: Removed orphan detection polling.
			// Lifecycle is now strictly managed by EventGoldDestroyed/Complete/Timeout events.

			// Update Timer logic
			// Calculate remaining time
			elapsedSeconds := float64(nowNano-splash.StartNano) / float64(time.Second)
			totalSeconds := float64(splash.Duration) / float64(time.Second)
// ... existing code ...

</Gemini_RESPONSE>

---

<User_RESPONSE>

- F*kh! No more 2am debug sessions right after adding a new feature and spaghetti refactor...

- Still <3 `panic()` to check if sh*t breaks.

**DEAD**

<User_RESPONSE>