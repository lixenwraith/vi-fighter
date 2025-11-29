# Input Handler State Machine Refactoring: Analysis & Planning

## Directive
[explore, complex, clarify, plan]

**CRITICAL INSTRUCTION**: This task is **ANALYSIS AND PLANNING ONLY**.
- **DO NOT** generate implementation code
- **DO NOT** finalize any approach without user confirmation
- **DO** provide comprehensive analysis with multiple alternatives
- **DO** ask clarifying questions for every ambiguous point
- **DO** present decision matrices for trade-offs
- **DO** wait for user responses before proceeding to final plan

---

## 1. Context & Background

### 1.1 Project Overview
`vi-fighter` is a terminal-based typing game implementing vi-style modal editing. The game uses:
- Go 1.25+ with generics-based ECS
- `tcell` for terminal rendering
- Modal input handling (Normal, Insert, Search, Command, Overlay modes)
- 50ms game tick with decoupled rendering

### 1.2 Current Architecture Location
The input handling system resides in the `modes/` package:
```
modes/
├── input.go           # InputHandler struct and main HandleEvent dispatch
├── capabilities.go    # CommandCapability struct and registry (UNUSED STUB)
├── commands.go        # :command mode execution (`:q`, `:new`, `:debug`, etc.)
├── motions.go         # Cursor motion implementations (word, line, find, etc.)
├── delete_operator.go # Delete operations (x, d{motion}, D)
├── search.go          # Search mode (/, n, N)
└── brackets.go        # Bracket matching (%)
```

### 1.3 Problem Statement

The `InputHandler` in `modes/input.go` has accumulated significant technical debt:

**Symptom 1: Flat Structure**
- `handleNormalMode()` contains 30+ `if/else` branches for individual keys
- Each branch manually handles: count accumulation, state flags, command building, state reset
- Pattern repeats with minor variations for each key

**Symptom 2: Scattered State Management**
- Multiple boolean flags track multi-keystroke state:
```go
  ctx.WaitingForF        // After 'f', waiting for character
  ctx.WaitingForFBackward // After 'F'
  ctx.WaitingForT        // After 't'
  ctx.WaitingForTBackward // After 'T'
  ctx.DeleteOperator     // After 'd', waiting for motion
  ctx.CommandPrefix      // After 'g', waiting for second key
```
- Flags are set/cleared inconsistently across branches
- Easy to forget clearing a flag, causing state leaks

**Symptom 3: Duplicated Logic**
- Count handling (`ctx.MotionCount`) duplicated in every branch
- Command string building (`buildCommandString`) called inconsistently
- `LastCommand` tracking for repeat (`.`) scattered throughout

**Symptom 4: Unused Capability System**
- `capabilities.go` defines `CommandCapability` with metadata:
```go
  type CommandCapability struct {
      AcceptsCount   bool
      MultiKeystroke bool
      RequiresMotion bool
  }
```
- `GetCommandCapability()` exists but is **never called**
- Capability map duplicates knowledge that's also hardcoded in handler

**Symptom 5: Scaling Concerns**
- Adding new keys requires: new branch, manual state handling, testing all interactions
- Planned features (macros, registers, configurable bindings) impossible with current structure

---

## 2. Current Code Patterns (Reference)

### 2.1 Typical Key Handler Pattern (Current)
```go
// Example: 'f' key handler (find character forward)
if char == 'f' {
    h.ctx.WaitingForF = true
    h.ctx.PendingCount = h.ctx.MotionCount
    h.ctx.MotionCount = 0
    // Note: LastCommand not set here, set when target char received
    return true
}

// Later: handling the target character for 'f'
if h.ctx.WaitingForF {
    count := h.ctx.PendingCount
    if count == 0 {
        count = 1
    }
    cmd := h.buildCommandString('f', char, count, false)
    ExecuteFindChar(h.ctx, char, count)
    h.ctx.WaitingForF = false
    h.ctx.PendingCount = 0
    h.ctx.MotionCount = 0
    h.ctx.LastCommand = cmd
    return true
}
```

### 2.2 Count Accumulation Pattern (Current)
```go
// Handle numbers for count
if char >= '0' && char <= '9' {
    // Special case: '0' is motion when not following a number
    if char == '0' && h.ctx.MotionCount == 0 && !h.ctx.DeleteOperator {
        ExecuteMotion(h.ctx, char, 1)
        h.ctx.MotionCommand = ""
        h.ctx.LastCommand = "0"
        return true
    }
    h.ctx.MotionCount = h.ctx.MotionCount*10 + int(char-'0')
    return true
}
```

### 2.3 Delete Operator Pattern (Current)
```go
if char == 'd' {
    if h.ctx.CommandPrefix == 'd' {
        // dd - delete line
        cmd := h.buildCommandString('d', 'd', count, false)
        ExecuteDeleteMotion(h.ctx, 'd', count)
        h.ctx.MotionCount = 0
        h.ctx.MotionCommand = ""
        h.ctx.CommandPrefix = 0
        h.ctx.LastCommand = cmd
    } else {
        h.ctx.DeleteOperator = true
        h.ctx.CommandPrefix = 'd'
    }
    return true
}

// Later: handling motion after 'd'
if h.ctx.DeleteOperator {
    cmd := h.buildCommandString('d', char, h.ctx.MotionCount, false)
    ExecuteDeleteMotion(h.ctx, char, h.ctx.MotionCount)
    h.ctx.MotionCount = 0
    h.ctx.MotionCommand = ""
    h.ctx.CommandPrefix = 0
    h.ctx.LastCommand = cmd
    return true
}
```

### 2.4 Capability Registry (Currently Unused)
```go
var commandCapabilities = map[rune]CommandCapability{
    'h': {AcceptsCount: true, MultiKeystroke: false, RequiresMotion: false},
    'f': {AcceptsCount: true, MultiKeystroke: true, RequiresMotion: false},
    'd': {AcceptsCount: true, MultiKeystroke: true, RequiresMotion: true},
    // ... 30+ entries
}

func GetCommandCapability(cmd rune) CommandCapability {
    capability, exists := commandCapabilities[cmd]
    if !exists {
        return CommandCapability{} // Default: no count, not multi-keystroke
    }
    return capability
}
```

---

## 3. Analysis Requirements

### 3.1 State Machine Conceptual Model

The input handler naturally models as a state machine:
```
                    ┌─────────────────────────────────────────┐
                    │                                         │
                    ▼                                         │
              ┌──────────┐                                    │
    ┌────────►│  IDLE    │◄───────────────────────────────────┤
    │         └────┬─────┘                                    │
    │              │                                          │
    │         digit│ (not '0' at start)                       │
    │              ▼                                          │
    │    ┌─────────────────┐                                  │
    │    │ COUNT_ACCUMULATE│──────┐                           │
    │    └────────┬────────┘      │                           │
    │             │               │ command key               │
    │    digit    │               ▼                           │
    │    ┌────────┘      ┌────────────────┐                   │
    │    │               │ OPERATOR_PENDING│ (after 'd','c')  │
    │    │               └───────┬────────┘                   │
    │    │                       │ motion key                 │
    │    │                       ▼                            │
    │    │               ┌────────────────┐                   │
    │    │               │    EXECUTE     │───────────────────┤
    │    │               └────────────────┘                   │
    │    │                                                    │
    │    │         ┌─────────────────┐                        │
    │    └────────►│ MULTI_KEYSTROKE │ (after 'f','t','g')    │
    │              └───────┬─────────┘                        │
    │                      │ target key                       │
    │                      ▼                                  │
    │              ┌────────────────┐                         │
    │              │    EXECUTE     │─────────────────────────┘
    │              └────────────────┘
    │
    │  simple command (h,j,k,l,w,b,etc.)
    └──────────────────────────────────────
```

### 3.2 Required Analysis Deliverables

Provide comprehensive analysis covering:

#### A. State Identification
- Enumerate all distinct input states
- Document transitions between states
- Identify state-specific data requirements
- Map current boolean flags to states

#### B. Command Classification
- Categorize all supported commands by behavior type
- Identify shared patterns across commands
- Document special cases and edge conditions
- Analyze the existing capability registry completeness

#### C. Architecture Options
For each viable approach, analyze:
- State representation strategy
- Transition mechanism
- Command dispatch pattern
- Count handling strategy
- Multi-keystroke coordination
- Repeat (`.`) command support
- Integration with existing motion/delete modules

#### D. Migration Strategy
- Identify incremental migration path
- Define compatibility boundaries
- Assess regression risk areas
- Estimate relative complexity

#### E. Future Extensibility
- Configurable key bindings feasibility
- Macro recording/playback compatibility
- Register support (`"a`, `"b`, etc.)
- Visual mode (future) accommodation

---

## 4. Decision Points Requiring User Input

The following areas have multiple valid approaches. **Present analysis for each and request user decision:**

### 4.1 State Representation
- **Option A**: Explicit enum states with switch dispatch
- **Option B**: State objects with polymorphic handlers
- **Option C**: Hierarchical state machine (HSM)
- **Option D**: Table-driven with function pointers

### 4.2 Command Registry Integration
- **Option A**: Enhance existing `capabilities.go` as source of truth
- **Option B**: Replace with new command descriptor system
- **Option C**: Merge into state transition table
- **Option D**: Keep separate, wire at initialization

### 4.3 Context State Fields
- **Option A**: Keep in `GameContext`, add state machine field
- **Option B**: Move all input state to dedicated `InputState` struct
- **Option C**: Encapsulate in state machine, expose via interface

### 4.4 Backward Compatibility Approach
- **Option A**: Big-bang replacement (higher risk, cleaner result)
- **Option B**: Parallel implementation with feature flag
- **Option C**: Incremental migration (state-by-state)

### 4.5 Error Handling Strategy
- Invalid key sequences
- Interrupted multi-keystroke commands (ESC)
- State corruption recovery

---

## 5. Constraints & Requirements

### 5.1 Hard Constraints
- Must maintain exact behavioral compatibility with current implementation
- Must work with existing `GameContext` and ECS architecture
- Must not break existing mode transitions (Normal ↔ Insert ↔ Search ↔ Command)
- Go 1.25+ idioms; no external state machine libraries

### 5.2 Soft Constraints (Preferences)
- Prefer compile-time safety over runtime flexibility
- Prefer explicit over clever
- Minimize allocations in hot path (key handling)
- Keep cognitive complexity manageable

### 5.3 Non-Goals (Explicitly Out of Scope)
- Configurable key bindings (future, but design should not preclude)
- Visual mode implementation
- Macro recording
- Plugin system

---

## 6. Analysis Report Structure

Provide your analysis in this structure:
```
## 1. Executive Summary
   - Problem scope assessment
   - Recommended approach (high-level)
   - Risk assessment
   - Effort estimate (relative)

## 2. Current State Analysis
   ### 2.1 State Inventory
   ### 2.2 Command Catalog
   ### 2.3 Pain Points (Ranked)
   ### 2.4 Technical Debt Quantification

## 3. Architecture Analysis
   ### 3.1 Option A: [Name]
       - Description
       - Advantages
       - Disadvantages
       - Implementation complexity
       - Risk factors
   ### 3.2 Option B: [Name]
       ... (repeat for each option)

## 4. Recommendation Matrix
   | Criterion | Option A | Option B | Option C |
   |-----------|----------|----------|----------|
   | ...       | ...      | ...      | ...      |

## 5. Decision Points
   (Questions requiring user input before proceeding)

## 6. Migration Path Options
   ### 6.1 Approach A
   ### 6.2 Approach B

## 7. Risk Analysis
   ### 7.1 Technical Risks
   ### 7.2 Behavioral Regression Risks
   ### 7.3 Mitigation Strategies

## 8. Open Questions
   (Clarifications needed from user)
```

---

## 7. Clarifying Questions Template

Before finalizing analysis, ask questions in this format:
```
### Clarification Required: [Topic]

**Context**: [Why this matters]

**Question**: [Specific question]

**Options**:
- A: [Option description]
- B: [Option description]

**Recommendation**: [Your suggested answer, if any]

**Impact of Choice**: [How the answer affects the design]
```

---

## 8. Success Criteria for This Analysis

The analysis is complete when:

1. [ ] All current input states are identified and documented
2. [ ] All commands are catalogued with their behavioral characteristics
3. [ ] At least 3 architectural approaches are analyzed with trade-offs
4. [ ] Decision points are clearly presented with options
5. [ ] Migration strategies are outlined
6. [ ] User has answered all clarifying questions
7. [ ] User has confirmed approach selection
8. [ ] Final plan is documented (still no implementation code)

---

## 9. Reference: Full Command List

For completeness, here are all commands currently handled:

**Motions (Normal Mode):**
- `h`, `j`, `k`, `l` - Basic movement
- `w`, `W`, `b`, `B`, `e`, `E` - Word motions
- `0`, `^`, `$` - Line motions
- `H`, `M`, `L` - Screen motions
- `G`, `gg` - Document motions
- `{`, `}` - Paragraph motions
- `%` - Bracket matching
- `f`, `F`, `t`, `T` - Find/till character
- `;`, `,` - Repeat find

**Operators:**
- `d` - Delete (requires motion)
- `D` - Delete to end of line
- `x` - Delete character

**Mode Switches:**
- `i` - Insert mode
- `/` - Search mode
- `:` - Command mode
- `ESC` - Return to normal / Cancel

**Search:**
- `n` - Next match
- `N` - Previous match

**Special:**
- `.` - Repeat last command (implicit via LastCommand)
- Digits `1-9` - Count prefix
- `0` - Count prefix OR line start (context-dependent)

---

## 10. Begin Analysis

Start your analysis by:
1. Acknowledging the scope
2. Identifying any immediate clarifying questions
3. Proceeding with state inventory
4. Building toward the full analysis report structure

**Remember: NO implementation code. Analysis and planning only.**