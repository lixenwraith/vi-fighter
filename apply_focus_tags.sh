#!/bin/bash
# Script to apply focus tags to all Go files in vi-fighter codebase

set -e

apply_tag() {
    local file="$1"
    local tag="$2"

    if [[ ! -f "$file" ]]; then
        echo "SKIP: $file (not found)"
        return
    fi

    # Check if first line is already a focus tag
    first_line=$(head -1 "$file")

    if [[ "$first_line" == "// @focus:"* ]]; then
        # Replace existing focus line (use | as delimiter to avoid conflicts)
        sed -i "1s|.*|$tag|" "$file"
        echo "REPLACE: $file"
    else
        # Insert new focus line at beginning
        sed -i "1i\\$tag" "$file"
        echo "INSERT: $file"
    fi
}

echo "Applying focus tags to vi-fighter codebase..."
echo ""

# core/
apply_tag "core/entity.go" "// @focus: #arch { ecs, types }"
apply_tag "core/modes.go" "// @focus: #flow { state } #control { types } #input { types }"
apply_tag "core/point.go" "// @focus: #arch { spatial, types }"

# engine/
apply_tag "engine/clock_scheduler.go" "// @focus: #flow { loop, clock } #event { dispatch }"
apply_tag "engine/ecs.go" "// @focus: #arch { ecs, types }"
apply_tag "engine/game_context.go" "// @focus: #flow { context, state } #arch { resource } #event { dispatch } #gameplay { resource, defense } #input { types }"
apply_tag "engine/game_state.go" "// @focus: #flow { state, phase } #gameplay { resource, obstacle, reward } #lifecycle { timer }"
apply_tag "engine/pausable_clock.go" "// @focus: #flow { clock }"
apply_tag "engine/position_store.go" "// @focus: #arch { spatial, ecs }"
apply_tag "engine/query.go" "// @focus: #arch { query, ecs }"
apply_tag "engine/resources.go" "// @focus: #arch { resource, types } #flow { clock } #render { types }"
apply_tag "engine/spatial_grid.go" "// @focus: #arch { spatial }"
apply_tag "engine/store.go" "// @focus: #arch { ecs, types }"
apply_tag "engine/store_interfaces.go" "// @focus: #arch { ecs, types }"
apply_tag "engine/time_provider.go" "// @focus: #flow { clock }"
apply_tag "engine/world.go" "// @focus: #arch { ecs, types } #lifecycle { cull }"
apply_tag "engine/z-index.go" "// @focus: #arch { spatial } #render { types }"

# events/
apply_tag "events/payloads.go" "// @focus: #event { types } #gameplay { resource, defense, obstacle, reward, ability } #vfx { splash } #lifecycle { timer } #meta { reset }"
apply_tag "events/queue.go" "// @focus: #event { queue }"
apply_tag "events/router.go" "// @focus: #event { dispatch }"
apply_tag "events/types.go" "// @focus: #event { types } #gameplay { resource, defense, obstacle, reward, ability } #vfx { splash, ping } #lifecycle { timer } #meta { debug, help, reset }"

# input/
apply_tag "input/intent.go" "// @focus: #input { types }"
apply_tag "input/keytable.go" "// @focus: #input { keys }"
apply_tag "input/machine.go" "// @focus: #input { machine }"
apply_tag "input/state.go" "// @focus: #input { machine, types }"

# modes/
apply_tag "modes/actions.go" "// @focus: #control { action }"
apply_tag "modes/commands.go" "// @focus: #control { command } #meta { debug, help, reset }"
apply_tag "modes/motions.go" "// @focus: #control { motion }"
apply_tag "modes/motions_helpers.go" "// @focus: #control { motion }"
apply_tag "modes/operators.go" "// @focus: #control { operator, action }"
apply_tag "modes/router.go" "// @focus: #control { router } #event { dispatch }"
apply_tag "modes/search.go" "// @focus: #control { search }"
apply_tag "modes/types.go" "// @focus: #control { types }"

# components/
apply_tag "components/character.go" "// @focus: #spawn { data } #render { scene }"
apply_tag "components/cleaner.go" "// @focus: #gameplay { ability } #vfx { flash }"
apply_tag "components/cursor.go" "// @focus: #flow { state } #gameplay { resource }"
apply_tag "components/decay.go" "// @focus: #gameplay { obstacle }"
apply_tag "components/drain.go" "// @focus: #gameplay { obstacle }"
apply_tag "components/energy.go" "// @focus: #gameplay { resource }"
apply_tag "components/flash.go" "// @focus: #vfx { flash }"
apply_tag "components/heat.go" "// @focus: #gameplay { resource }"
apply_tag "components/marked_for_death.go" "// @focus: #lifecycle { cull, marker }"
apply_tag "components/materialize.go" "// @focus: #vfx { materialize }"
apply_tag "components/nugget.go" "// @focus: #gameplay { reward }"
apply_tag "components/ping.go" "// @focus: #vfx { ping }"
apply_tag "components/position.go" "// @focus: #arch { spatial, types }"
apply_tag "components/protection.go" "// @focus: #lifecycle { protect }"
apply_tag "components/shield.go" "// @focus: #gameplay { defense }"
apply_tag "components/splash.go" "// @focus: #vfx { splash }"
apply_tag "components/timer.go" "// @focus: #lifecycle { timer }"
apply_tag "components/visuals.go" "// @focus: #render { types }"

# systems/
apply_tag "systems/boost.go" "// @focus: #gameplay { defense } #flow { state }"
apply_tag "systems/cleaner.go" "// @focus: #gameplay { ability, collision } #vfx { flash } #event { dispatch }"
apply_tag "systems/command.go" "// @focus: #meta { debug, help, reset } #event { dispatch }"
apply_tag "systems/cull.go" "// @focus: #lifecycle { cull }"
apply_tag "systems/decay.go" "// @focus: #gameplay { obstacle, collision } #flow { phase } #vfx { flash }"
apply_tag "systems/drain.go" "// @focus: #gameplay { obstacle, collision, resource, reward } #vfx { flash } #event { dispatch }"
apply_tag "systems/energy.go" "// @focus: #gameplay { resource, reward, collision } #event { dispatch }"
apply_tag "systems/flash.go" "// @focus: #vfx { flash } #lifecycle { timer }"
apply_tag "systems/gold.go" "// @focus: #gameplay { reward } #flow { phase } #spawn { placement } #event { dispatch }"
apply_tag "systems/heat.go" "// @focus: #gameplay { resource, ability } #event { dispatch }"
apply_tag "systems/nugget.go" "// @focus: #gameplay { reward } #spawn { placement } #event { dispatch }"
apply_tag "systems/ping.go" "// @focus: #vfx { ping } #event { dispatch }"
apply_tag "systems/shield.go" "// @focus: #gameplay { defense, resource } #event { dispatch }"
apply_tag "systems/spawn.go" "// @focus: #spawn { logic, placement } #flow { phase } #gameplay { obstacle }"
apply_tag "systems/splash.go" "// @focus: #vfx { splash } #event { dispatch } #gameplay { reward }"
apply_tag "systems/time_keeper.go" "// @focus: #lifecycle { timer } #event { dispatch }"

# render/
apply_tag "render/blender.go" "// @focus: #render { color }"
apply_tag "render/buffer.go" "// @focus: #render { buffer, mask, post }"
apply_tag "render/cell.go" "// @focus: #render { buffer, types }"
apply_tag "render/colors.go" "// @focus: #render { color }"
apply_tag "render/context.go" "// @focus: #render { pipeline, types }"
apply_tag "render/interface.go" "// @focus: #render { pipeline, types }"
apply_tag "render/mask.go" "// @focus: #render { mask }"
apply_tag "render/orchestrator.go" "// @focus: #render { pipeline }"
apply_tag "render/priority.go" "// @focus: #render { pipeline }"
apply_tag "render/rgb.go" "// @focus: #render { color }"

# render/renderers/
apply_tag "render/renderers/characters.go" "// @focus: #render { scene } #spawn { data }"
apply_tag "render/renderers/column_indicators.go" "// @focus: #render { ui }"
apply_tag "render/renderers/cursor.go" "// @focus: #render { ui } #gameplay { resource }"
apply_tag "render/renderers/drain.go" "// @focus: #render { scene } #gameplay { obstacle }"
apply_tag "render/renderers/effects.go" "// @focus: #render { scene } #vfx { flash, materialize } #gameplay { obstacle }"
apply_tag "render/renderers/heat_meter.go" "// @focus: #render { ui } #gameplay { resource }"
apply_tag "render/renderers/line_numbers.go" "// @focus: #render { ui }"
apply_tag "render/renderers/overlay.go" "// @focus: #render { ui } #meta { overlay }"
apply_tag "render/renderers/ping.go" "// @focus: #render { scene } #vfx { ping }"
apply_tag "render/renderers/post_process.go" "// @focus: #render { post }"
apply_tag "render/renderers/shields.go" "// @focus: #render { scene } #gameplay { defense }"
apply_tag "render/renderers/splash.go" "// @focus: #render { ui } #vfx { splash }"
apply_tag "render/renderers/status_bar.go" "// @focus: #render { ui } #flow { state } #input { types }"

# terminal/
apply_tag "terminal/ansi.go" "// @focus: #sys { ansi }"
apply_tag "terminal/color.go" "// @focus: #sys { ansi } #render { color }"
apply_tag "terminal/doc.go" "// @focus: #sys { term }"
apply_tag "terminal/input.go" "// @focus: #sys { io } #input { keys }"
apply_tag "terminal/keys.go" "// @focus: #sys { io } #input { keys }"
apply_tag "terminal/output.go" "// @focus: #sys { io, term }"
apply_tag "terminal/resize_unix.go" "// @focus: #sys { term, io }"
apply_tag "terminal/terminal.go" "// @focus: #sys { term }"

# audio/
apply_tag "audio/cache.go" "// @focus: #sys { audio }"
apply_tag "audio/config.go" "// @focus: #sys { audio } #conf { audio }"
apply_tag "audio/detector.go" "// @focus: #sys { audio }"
apply_tag "audio/engine.go" "// @focus: #sys { audio }"
apply_tag "audio/generator.go" "// @focus: #sys { audio }"
apply_tag "audio/mixer.go" "// @focus: #sys { audio }"
apply_tag "audio/types.go" "// @focus: #sys { audio }"

# content/
apply_tag "content/manager.go" "// @focus: #spawn { loader }"

# constants/
apply_tag "constants/audio.go" "// @focus: #conf { audio } #sys { audio }"
apply_tag "constants/content.go" "// @focus: #conf { gameplay } #spawn { logic }"
apply_tag "constants/core.go" "// @focus: #conf { system } #arch { ecs } #event { queue }"
apply_tag "constants/entities.go" "// @focus: #conf { entity } #gameplay { obstacle, ability } #vfx { flash, materialize }"
apply_tag "constants/gameplay.go" "// @focus: #conf { gameplay } #gameplay { resource, reward }"
apply_tag "constants/ui.go" "// @focus: #conf { visual } #render { ui }"

# assets/
apply_tag "assets/splash_font.go" "// @focus: #vfx { splash } #render { ui }"

# cmd/vi-fighter/
apply_tag "cmd/vi-fighter/main.go" "// @focus: #flow { loop, init } #arch { ecs } #event { dispatch } #render { pipeline }"

# cmd/blend-tester/
apply_tag "cmd/blend-tester/blend.go" "// @focus: #render { color }"
apply_tag "cmd/blend-tester/diag.go" "// @focus: #render { color }"
apply_tag "cmd/blend-tester/effect.go" "// @focus: #render { color, post }"
apply_tag "cmd/blend-tester/main.go" "// @focus: #render { pipeline }"
apply_tag "cmd/blend-tester/palette.go" "// @focus: #render { color }"
apply_tag "cmd/blend-tester/ui.go" "// @focus: #render { ui }"

echo ""
echo "Focus tag application complete!"
