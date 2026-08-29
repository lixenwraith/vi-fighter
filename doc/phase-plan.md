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

## Phase 6 — Event classification and journal completeness · complete

167 event types (`EventTypeCount`, including `EventNone`); 166 classified.

**The finding the phase turned on.** `EventCombatAttackDirectRequest` is
`Stamped`, and `Stamped` is not "the ambient domain at push": the same producer,
in the same tick, under the same ambient domain, pushes a hit that crosses when
the target is shared and does not when the target is player. No static per-type
table can carry it. **Resolved by producer stamping** — the four push sites
resolve the target's own domain and the filter reads the tag. This was the
cheaper of the two options and the one D-10 already had a mechanism for.

**The finding that reshaped the rest.** `core.DomainShared` is the zero value
and the ambient domain defaults to it, so `GameEvent.Domain` is not a
producer-domain oracle: a bare `PushEvent` leaves a record reading "shared"
whatever produced it. `Shared`, `Bus` and `Local` are therefore declarations
checked statically against the pushing system's profile, and only `Stamped` is
read from the tag. A runtime test that assumed otherwise measured stamping
coverage rather than domain truth.

1. **Registry classes · done.** Declared in the `type.go` doc comment, extending
   the payload grammar rather than adding a second mechanism:
   `// EventFoo (FooPayload) [bus] description`. `docPayload` became `docAnnot`
   and consumes both groups in either order. The generator is
   `internal/gen-manifest/main.go` and refuses an unclassified constant the way
   it already refuses a forgotten payload annotation. It emits a dense
   `[EventTypeCount]EventClass` array plus `ClassOf`, rather than widening
   `RegisterType`: the filter indexes it per record, and registration is about
   name and payload, not replication.
2. **`EmitDeath` stamping · done, with no signature change.** The record takes
   the domain of the entities dying, which cannot disagree with them the way a
   passed or ambient domain could, and mixed input splits into one batch per
   domain. That makes D-12's batch purity structural rather than conventional
   and closes the `TODO(phase6)` without touching the 38 call sites the plan
   budgeted for.
3. **Storm red bullets and `internal/mode` · done.** The storm half was already
   closed: all four sites use `PushLocal` and the class table now names those
   events `Local`. `internal/mode` is this instance's input by definition, so 37
   `Local`-class sites and the three `Bus` crossings it originates now stamp
   player. `GameContext` gained `PushLocal`.
4. **Journal filter · done.** `JournalRecord.Domain` already existed and was
   already populated, and the reader already round-tripped it, so the missing
   piece was the predicate: `JournalRecord.Replicated` and
   `journal.Set.Replicated()`. Schema 6 to 7 — no field moved, but `Domain`
   changed meaning, so the two are not comparable. `ConfigFromAnchor` and the
   replay anchor check both read the constant and followed with no change. No
   fixtures existed to regenerate.
5. **View-key instrumentation · done, and far smaller than recorded.** The plan
   pointed at `internal/fsm/config_access.go`, which does not exist, and warned
   that `internal/fsm/std/` had never been read; neither is involved. The
   surface is `internal/engine/config_access.go`: two maps, eight keys. Three
   replicated (`map_width`, `map_height`, `crop_on_resize`), five not. Both
   accessors warn once per key when a non-replicated one is read while
   `World.MapSizeLocal()` is false. `mapSizeLocal` moved to `World`.

**Exit criterion, met.** `TestBusPayloadsNameOnlySharedEntities` reads the class
table rather than its `busEvents` hand-list, and skips a non-replicated record
whole rather than asserting its emitter fields. `crossingTargets` is gone. Every
event type has a class.

### What the classification found

- **The D-3 table had four rows; the code has eleven.**
  `TestEventClassMatchesSystemProfile` checks every statically resolvable push
  against the class and the pusher's profile, and its exemption list,
  `crossingPushes`, is the D-3 table as code. Three crossings the design never
  named came out of it: decay and drain destroying a shared nugget, a dying
  drain donating hit points, and the post-typing cursor advance. Each needs a
  wire path in Phase 7.
- **Cleaner is no longer stamped.** All three producers push
  `core.DomainPlayer` — the nugget beacon since Phase 5 — so D-7's canonical
  example of a dual-domain system was stale. Its events are `Local`.
- **The area attack is not a crossing artifact.** `HitEntities` is a plural list
  that can span domains, so `EventCombatAttackAreaRequest` names resolved
  entities rather than geometry. `Local`, with the weapon pulse's direct push at
  shared targets recorded as a gap.
- **Three unearned `Stamped` declarations.** `EventSpeciesCreated` was pushed
  unstamped by drain while `EventSpeciesKilled` was stamped three ways over;
  `EventMaterializeComplete` did not carry the domain it completed;
  `EventMetaSystemCommandRequest` is `Shared`, since enabling a system on one
  instance and not another diverges the simulation outright.
- **19 `Local` types still push unstamped** from `app`, `engine`, `fsm` and the
  shared species systems. Pinned by `unstampedLocal`, which may only shrink.

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
   `event.Replicated(type, domain)` is the send predicate and `crossingPushes`
   in `internal/system/event_class_test.go` enumerates the eleven producer sites
   it must cover. Two distinctions Phase 6 deliberately left to this phase:
   - **Compared is not sent.** The class table answers "must both instances have
     this record", which is what the journal filter needs. A `Shared` event is
     re-derived identically on both instances and must be compared but never
     sent; a `Bus` event must be sent. The wire set is `Bus` plus `Stamped`
     resolving shared, not the whole transported set.
   - **D-5 chains.** A chain attack is stamped by its target like any other, so
     it is in the transported set. It must not be on the wire: the receiver
     derives it from the root the wire carried, and sending both applies it
     twice.
6. **The unclosed crossings.** The weapon pulse pushes an area attack at shared
   targets with no geometry crossing behind it (see the domain document's gaps),
   and 19 `Local` types still push unstamped. Both need closing before the wire
   carries anything.

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
