# vi-fighter — multi-instance phase plan

Companion to `domain-design.md`. Rules referenced as D-n.

## Phase 4 — Domain boundary · complete

| Step | State |
|---|---|
| 4.1 Cell partition, `DomainScope`, player budget | done |
| 4.2 Shared systems scoped | done |
| 4.3 Combat and soft-collision RNG split | done |
| 4.4 Combat payload reduced to geometry | done |
| 4.5 Player-domain projectiles; cleaner and materialize stamped | done |
| 4.6 Cursor view components (`CursorViewComponent`) | done |
| 4.7 Nugget cursor leak resolved via `ClosestCursor` | done |
| 4.8 Fuse player-domain, geometry-only crossing, spirit stamped | done |
| 4.9 Config split | withdrawn — one struct, no benefit |
| 4.10 Drain flocking | resolved by declaration |
| 4.11 `ViewResource` split | done |
| 4.12 `CommitShared` placement gate | done |
| 4.13 Personal loot, player-domain, per-slot pity | done |
| 4.14 Map-size guard and lock telemetry (D-14) | done |
| 4.15 Per-domain counters, scoped digest, `SnapshotShared` | done |
| 4.16 Verification | done |
| 4.17 System domain profiles, dependency resolver in core, FSM wiring (D-15) | done |

**Exit criterion, met.** `TestSharedSnapshotParityAcrossTerminalSizes`
(`internal/app`) steps two instances of one seed in lockstep on different
terminal sizes and asserts `SnapshotShared()` equality at every step.

### What 4.16 landed

| Test | Package | Asserts |
|---|---|---|
| `TestSystemDomainProfiles` | `system` | Each declared profile against the RNG streams, entity domains and component stores its file touches; resolves hoisted store aliases |
| `TestAllowedDomainAccessIsLive` | `system` | The exemption list cannot outlive the access it excuses |
| `TestHelperFilesArePinned` | `system` | The unattributed file set (`blast`, `interaction`, `sweep`, `targeting`, `telemetry`) is fixed, so a new helper is visible |
| `TestSystemsDeclareNoDomainMethod` | `system` | No system re-declares `Domain()`/`Requires()`; the manifest is the only declaration site |
| `TestCombatKnockbackDrawsFromTheTargetsStream`, `TestSoftCollisionImpulseDrawsFromTheTargetsStream` | `system` | D-8: a player-target impulse never advances the shared RNG stream, with the shared case as its non-vacuity control |
| `TestBusPayloadsNameOnlySharedEntities` | `app` | D-4 over a soak, via a dispatch tap; emitter fields always, target fields only on crossings |
| `TestDomainAuditSoakClean` | `app` | Zero component-domain violations over a 3000-step soak, with retained descriptions |
| `TestMapSizeLockedWithSecondCursor`, `TestMapSizeCropsWithOneCursor` | `app` | D-14, with the crop path as its own negative control |
| `TestSharedGlyphsAreGoldMembersOnly` | `app` | Every shared-domain glyph is a gold composite member |
| `TestSharedSnapshotParityAcrossTerminalSizes` | `app` | The exit criterion |

Supporting machinery: `engine.PinDomainAudit`/`DomainMismatches`/
`DomainViolations`; per-system audit attribution in `UpdateLocked`, falling
back to `"event"` for settle-pass attaches; `ClockScheduler.SetDispatchTap` and
`App.SetDispatchTap`; `ScriptDriver` exported with `Step()` for lockstep
driving; `ScriptOptions.Resizes`/`MapMotionsOnly`; `FastRand.State()`.

**4.16(4) closed by construction, no fixtures regenerated.** There are no
journal fixtures on disk. Every journal test uses the in-memory `app.Capture`
sink, and `internal/journal` — the JSONL reader — has exactly one non-test
importer, `internal/app/play.go`. The step's "regenerate fixtures and digests"
had no artifact to act on. The same is true of Phase 6 item 4.

## Phase 5 — Ownership consolidation · complete

1. **Glyph classified.** Content glyphs are player-domain; gold sequence
   members are the only shared entities carrying `GlyphComponent`. `GlyphBit`
   stays unlisted in `ComponentDef` — it attaches in either domain.
   Player-domain mechanics guard by `e.Domain() != core.DomainPlayer`: one
   invariant stated at the loop, replacing three accidental mechanisms.
2. **Reward tagging.** `World.PushLocal` stamps `Domain = DomainPlayer`,
   applied across owner-authored grants (energy, heat, boost, shield, weapon)
   and D-6 effects (sound, blinks, flash, fadeout, splash, strobe, grayout).
   Behaviour-neutral: the tag never affects dispatch, and these all carry
   `OriginSystem`, so nothing is journaled differently.
3. **Owner-authored write test.** `ownerAuthoredStores` restricted to
   cursor-exclusive components. `Shield` and `Combat` are excluded because they
   also carry quasar, loot and species state that is re-derived rather than
   transported (D-13).
4. **Contested-vs-personal audit done.** All `ClosestCursor` sites are
   contested — deterministic slot order, positions only. One fix:
   `NuggetSystem.emitBeacon` now uses `PushLocal`, so a shared nugget beacon
   draws each instance's own cleaners instead of one shared cleaner carrying
   pure visuals.
5. **`MetaSystem` profile confirmed** `shared`, rationale corrected: its world
   writes are replicated or are the D-14 map-bounds write, and the context
   state it writes is not world state.

Also landed, ahead of its phase: **gold contributor attribution**.
`CompositeMemberDestroyedPayload.Entity` carries the typist, `GoldSystem`
tallies per roster slot, and `GoldCompletionPayload.Entity` names the cursor
that typed the most members — ties to the lowest slot, so every instance
credits the same one. Timeout and destruction leave it zero. This is the D-3
gold crossing row.

**Exit criterion, met.** Every component bit appears in `manifest.Components`,
and every system's declared profile is asserted in both directions.

## Phase 5.5 — Manifest convergence · complete

Was listed under "Deferred, own context" as *component_domain.go / system
profile convergence*. Both tables were stable after Phase 5, so it was pulled
forward.

Component domain and system profile are now data in
`internal/manifest/definition.go`, generated into
`internal/engine/component_domain_gen.go` and `internal/manifest/build_gen.go`.
`System` no longer declares `Domain()` or `Requires()`; `World.AddSystem(sys,
profile)` takes a `manifest.ProfileFor(name)`. The methods were deleted, not
asserted — one declaration site, no drift.
`internal/system/network.go` carries a `TODO(phase7)`: it is written but
registered nowhere, and `domain_test.go` exempts it by name.

## Phase 6 — Event classification and journal completeness

The prerequisite for any wire format. Largest mechanical phase; 167 event types
(`EventTypeCount`, including `EventNone`).

**The finding that shapes this phase.** `EventCombatAttackDirectRequest` is
`Stamped`, and `Stamped` is not "resolved from the ambient domain at push". Its
class is per-instance, resolved by the *target's* domain: the same producer, in
the same tick, under the same ambient domain, pushes a hit that crosses when the
target is shared and does not when the target is player. **No static per-type
table can carry it.** The predicate exists today only as `crossingTargets()` in
`internal/app/bus_purity_test.go`. See the D-10 amendment. Item 1 must decide
this before the table is populated, because the answer changes what the table
means.

1. **Registry classes.** Add `Shared|Bus|Local|Stamped`, derived from a
   doc-comment annotation the way payloads already are. The generator is
   `internal/gen-manifest/main.go` (*not* `internal/manifest/cmd/main.go`,
   which does not exist); `docPayload` already cuts the constant name and an
   optional `(Payload)` off the doc line, so a class marker in the remainder
   parses with a sibling `docClass` and no second mechanism:

   ```
   // EventFoo (FooPayload) [bus] short description
   // EventFoo [local] short description
   ```

   Emit a dense `[EventTypeCount]EventClass` array plus `ClassOf(et)` rather
   than widening `RegisterType`: the journal filter indexes it per record, and
   registration is about name and payload, not replication. Stage the
   enforcement — warn on a missing class while populating, then error, so the
   generator ends up refusing an unclassified constant the way it already
   refuses a forgotten payload annotation.

   **Open question, blocks the rest of the phase.** For `Stamped`, either the
   journal filter carries the `crossingTargets()` predicate, or combat
   producers stamp `GameEvent.Domain` from the target's domain and the filter
   keys on the tag. The second is cheaper and D-10 already provides `Stamped`
   as the mechanism; `PushLocal` establishes the tagging pattern. It is a
   wiring change across every combat producer.

2. **`EmitDeath` stamping.** Add the domain parameter and thread it through.
   Scope is larger than previously recorded: **38 call sites across 11 files** —
   `splash.go` (8), `drain.go` (7), `decay.go` (6), `wall.go` (5), `dust.go`,
   `fuse.go`, `sweep.go`, `typing.go`, `weapon.go` (2 each), `cleaner.go`,
   `composite.go` (1 each). Closes the `TODO(phase6)` in `sweep.go` and
   `fuse.go`.

3. **Storm red bullets** classified `Local`. `internal/mode/` grant pushes
   retagged in the same batch: they carry `OriginCommand` and so are journaled,
   which is why Phase 5 left them out — retagging changes recorded record
   domains and belongs with the filter that reads them.

4. **Journal filter.** `JournalRecord.Domain` already exists and is already
   populated ("producer domain; replication filters on it"); what is missing is
   the filter. The transported set is `Shared ∪ Bus`. Journal schema bumps
   6 → 7; `ConfigFromAnchor` and `VerifyAnchor` follow. **No fixtures to
   regenerate** — the schema constant and the in-memory round-trip tests are
   the whole surface.

5. **View-key instrumentation.** Much smaller than recorded, and in a different
   package: the script-visible surface is `internal/engine/config_access.go`,
   40 lines, two maps, eight keys. `internal/fsm/config_access.go` does not
   exist and `internal/fsm/std/` is not needed. Replicated: `map_width`,
   `map_height`, `crop_on_resize`. Non-replicated: `viewport_width`,
   `viewport_height`, `camera_x`, `camera_y`, `color_mode`. Warn once when one
   of the five is read while `mapSizeLocal()` is false. Keys stay (D-14).

**Exit criterion.** `TestBusPayloadsNameOnlySharedEntities` runs against the
declared Bus set rather than its hand-list (`busEvents`), and every event type
has a class.

## Phase 7 — Transport

1. **Join handshake.** Carries the D-14 map-size latch, the seed, the session
   counter and the config/content identity. `JournalAnchor` already carries
   exactly this set for replay — extend that shape rather than inventing a
   second one.
2. **`NetworkSystem` wiring.** `NetworkPort.Drain` per tick already exists as a
   poll model, deliberately keeping network goroutines out of the world event
   queue. The interface collapses to a concrete type as the package matures,
   following the audio precedent. Declare it in `manifest.Systems` at the same
   time and drop the `domain_test.go` exemption.
3. **Remote cursor lifecycle.** `EventCursorSpawnRequest{Control: ControlRemote,
   PeerID}` already exists and works; the roster, slots and `CursorSystem` need
   no change. Verify with two local cursors first, one marked remote.
4. **Owner-authored replication.** Periodic value sync for the D-13 component
   set, one direction per cursor. This is the only state transfer in the design;
   everything else re-derives. `Shield` and `Combat` need a field-level split
   here: both carry re-derived species state alongside the cursor state.
5. **Bus event transport**, driven by the Phase 6 classification.

## Phase 8 — Multi-instance verification

Two in-process instances sharing a seed and an event pipe, driven by
`RunScript`, asserting `SnapshotShared()` equality per tick and reporting the
first divergent record via `FirstDiff`/`Diff`. This is Phase 4's exit criterion
applied to a real second participant rather than a second terminal size.

Blocker to resolve first: `headless.go` documents four process-wide values that
no snapshot reaches but that prevent concurrent Apps — the status recorder
trigger hook, the navigation debug pointers in `internal/system`, help's key
table, and vlog's correlation stamp. Two live instances in one process needs
those scoped.

## Carried-forward gaps

Each is small, self-contained, and independent of the phase order.

- **`ctx|player` record split.** `entity`, `slot`, `x`, `y` move from the
  `player` record to `view`; `count` stays. Passes today only because both
  parity instances bind slot 0. `internal/engine/snapshot.go`,
  `internal/app/snapshot.go`.
- **`spatial.indexed_shared` allow-list.** The `spatial.` prefix deny drops one
  genuinely comparable key. Wants an allow-list, not a prefix deny.
  `internal/app/snapshot.go`.
- **`internal/journal` round-trip coverage.** Zero tests; one non-test importer
  (`internal/app/play.go`). A `core.DomainNames` change silently breaks
  `vif play` with nothing to catch it.
- **`World.UpdateBoundsRadius`** writes `PingComponent` for every rostered
  cursor including remote ones. Harmless under D-13 — pure local view, reaches
  no digest — but restricting it to the local slot forces `setLocal` to clear
  the departing slot.
- **`uint32(entity)` narrowing** at `gateway.go` and `adaptation.go`, safe only
  while route-graph anchors are shared (tag 0).

## Deferred, own context

- **Windowed composite / vision box.** A recording or session on a terminal
  smaller than the map is clipped by the render buffer; `play.go`'s pan offset
  is a placeholder. Pure presentation, no shared state, no abuse surface.
  Needs its own focused session.
