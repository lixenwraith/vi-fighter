# Focus Tags Report

## Summary

Total .go files tagged: **114**

Files with #all tag: **5**

## Files with #all Tag (Always Included)

core/entity.go →// @focus: #all #core { entity, types }
core/modes.go →// @focus: #all #core { types, state } #input { modes }
core/point.go →// @focus: #all #core { types, spatial }
engine/world.go →// @focus: #all #core { world, ecs, entity, store, lifecycle }
events/types.go →// @focus: #all #events { types } #core { types }

## Files by Directory

| Directory | Count |
|-----------|-------|
| core | 3 |
| engine | 14 |
| components | 18 |
| systems | 16 |
| events | 4 |
| input | 4 |
| modes | 8 |
| render | 23 |
| terminal | 8 |
| audio | 7 |
| content | 1 |
| constants | 6 |
| assets | 1 |
| cmd/vi-fighter | 1 |

## Group Usage Statistics

| Group | Count |
|-------|-------|
| #core | 53 |
| #game | 35 |
| #render | 33 |
| #input | 14 |
| #events | 21 |
| #terminal | 8 |
| #audio | 7 |
| #content | 2 |
| #constants | 6 |

## Sample Tags by Directory

### core/

- **point.go**: `#all #core { types, spatial }`
- **entity.go**: `#all #core { entity, types }`
- **modes.go**: `#all #core { types, state } #input { modes }`

### engine/

- **clock_scheduler.go**: `#core { clock, ecs, lifecycle } #events { dispatch } #game { phase }`
- **position_store.go**: `#core { store, spatial, ecs }`
- **game_state.go**: `#core { state, types } #game { phase, spawn, gold, decay, boost }`
- **resources.go**: `#core { resources, types }`
- **time_provider.go**: `#core { clock }`

### components/

- **cleaner.go**: `#core { ecs, types } #game { cleaner } #render { effects }`
- **materialize.go**: `#core { ecs, types } #game { spawn } #render { effects }`
- **heat.go**: `#core { ecs, types } #game { heat }`
- **visuals.go**: `#core { ecs, types } #render { colors }`
- **nugget.go**: `#core { ecs, types } #game { nugget }`

### systems/

- **cleaner.go**: `#core { ecs, spatial } #game { cleaner, collision } #render { effects } #events { dispatch }`
- **time_keeper.go**: `#core { ecs, lifecycle } #game { timer } #events { dispatch }`
- **cull.go**: `#core { ecs, lifecycle }`
- **heat.go**: `#core { ecs } #game { heat, cleaner } #events { dispatch }`
- **nugget.go**: `#core { ecs, spatial } #game { nugget } #events { dispatch }`

### events/

- **payloads.go**: `#events { payloads } #game { energy, shield, gold, cleaner, splash }`
- **types.go**: `#all #events { types } #core { types }`
- **router.go**: `#events { router, dispatch }`
- **queue.go**: `#events { queue }`

### input/

- **machine.go**: `#input { machine, intent, keys, motion, operator }`
- **state.go**: `#input { machine, modes }`
- **keytable.go**: `#input { keys, machine, motion, operator }`
- **intent.go**: `#input { intent, machine, keys }`

### modes/

- **commands.go**: `#input { commands } #game { spawn, phase }`
- **actions.go**: `#input { char-motion } #game { cursor }`
- **operators.go**: `#input { operator, motion } #events { dispatch }`
- **search.go**: `#input { search, motion }`
- **types.go**: `#input { motion }`

### render/

- **priority.go**: `#render { orchestrator }`
- **interface.go**: `#render { orchestrator }`
- **colors.go**: `#render { colors }`
- **rgb.go**: `#render { colors }`
- **cell.go**: `#render { buffer, cell }`

### terminal/

- **resize_unix.go**: `#terminal { resize }`
- **input.go**: `#terminal { raw-input, keys }`
- **ansi.go**: `#terminal { ansi }`
- **color.go**: `#terminal { color, color-mode }`
- **keys.go**: `#terminal { keys }`

### audio/

- **generator.go**: `#audio { synth }`
- **cache.go**: `#audio { cache }`
- **config.go**: `#audio { engine }`
- **types.go**: `#audio { types }`
- **mixer.go**: `#audio { mixer }`

### content/

- **manager.go**: `#content { manager, loader }`

### constants/

- **content.go**: `#constants { gameplay }`
- **ui.go**: `#constants { ui }`
- **entities.go**: `#constants { entities }`
- **core.go**: `#constants { entities }`
- **audio.go**: `#constants { audio }`

