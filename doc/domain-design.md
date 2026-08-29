# Multi-instance domain model — vi-fighter

Status: Phases 4 (including 4.16 verification), 5 and 5.5 landed. Rules
D-1..D-15 are implemented unless marked. Supersedes every earlier design note.

## 1. Domains

Two per `World`: **Shared**, identical on every instance and replicated, and
**Player**, this instance's participant and never replicated. One `World` per
local participant. The roster slot lives on `CursorComponent`; it is not part
of the domain tag.

`core.Entity` is `[domain:8][id:56]`. `core.DomainNames` indexes the domain for
seed derivation, telemetry keys and log fields — changing a name re-keys every
stream in that domain.

## 2. Rules

**D-1 Reads.** A player-domain system may read shared state. A shared-domain
system reads shared only. Exceptions are D-12 and D-13, both explicit.

**D-2 Simulation ownership.** Only the instance owning a cursor simulates that
cursor's weapons, projectiles and player species. A remote participant's
player-domain state does not exist locally and is never reconstructed.

**D-3 The crossing.** When a player mechanic affects a shared entity, the
smallest artifact that determines the shared outcome crosses as a Bus event:

| effect | crossing artifact |
|---|---|
| direct hit (rod, cleaner, bullet) | one combat event per shared target |
| area effect (missile impact, dust detonation) | one explosion request: centers, radius, duration, attack family, owner cursor |
| drain fusion | one spawn request: header cell only |
| gold member typed | one composite-member destruction: header, member, typist cursor |

Effects on player targets do not cross. The producer resolves its own domain
*before* pushing the crossing event; the shared consumer resolves only shared
targets.

The gold row is a keystroke crossing: `TypingSystem` is player-domain and
`EventCompositeMemberDestroyed` names a shared member. Its payload carries the
typist (`CompositeMemberDestroyedPayload.Entity`), which is what makes the
credit a function of shared events rather than of who happened to type last.
`GoldSystem` tallies per roster slot and `GoldCompletionPayload.Entity` names
the cursor that typed the most members, ties resolved to the lowest slot so
every instance credits the same one. Timeout and destruction leave it zero.

**D-4 Payload purity.** A Bus payload names only shared entities. Player
emitters are reduced to coordinates and velocity (`HasOrigin`, `OriginX/Y`,
`HasVelocity`, `OriginVelX/Y`). A Local payload may name player entities
freely — `EventFuseSwarmRequest` and the lightning triple do.

**D-5 Derived, not transported.** Events a shared system produces from a Bus
event are re-derived identically on every instance and must never themselves be
transported. `EventExplosionBatchRequest` crosses; the
`EventCombatAttackAreaRequest`s it produces do not.

**D-6 Effect entities are player-domain.** Lightning, flash, fadeout, splash,
motion marker, dust, decay, blossom, orb, bullet, missile and loot are created
from the player counter and may be created conditionally on local view state
(`Player.IsLocal`). They never feed shared simulation. This is what lets a
remote cursor's damage land without its visuals cluttering the screen.

**D-7 Ambient domain.** `World.WithDomain(d, fn)` mirrors `WithOrigin`, and
`PushEventDomain` stamps explicitly for producers outside any scope. One system
can serve both domains without splitting: a nugget-spawned cleaner is created
shared, a weapon-spawned cleaner player, and `CleanerSystem` reads the
request's domain rather than being duplicated. This is the general answer to
generic types (death, timer, flash, spirit, materialize, species lifecycle) —
they are stamped, not statically classified.

**D-8 RNG.** `RandResource.Stream(domain, label)` derives from
`(sessionRoot, domain, label)`. A system resolving both domains holds one
stream per domain and selects by the target's domain; `CombatSystem` and
`SoftCollisionSystem` are the only two. A wholly player-domain system draws one
player stream: `FuseSystem`, `DrainSystem`, `LootSystem`, `LightningSystem`. No
simulation path seeds from a clock; `TimeResource.GameTimeNano` is explicitly
not a seed source.

**D-9 Entity identity.** `World.nextEntityID [2]uint64`; `CreateEntity(domain)`
explicit at every call site; `Clear()` resets both. Zero remains invalid in
both domains. Created and destroyed counts are tracked per domain
(`CreatedCountDomain`, `DestroyedCountDomain`); the aggregate accessors sum
them.

**D-10 Event domain.** `GameEvent.Domain` stamped at push from the ambient
domain. `World.PushLocal` stamps `DomainPlayer` explicitly, and is the push
site for owner-authored grants (energy, heat, boost, shield, weapon) and D-6
effects (sound, blinks, flash, fadeout, splash, strobe, grayout). The tag never
affects dispatch. Registry classes: `Shared` (emitted and consumed shared,
replicated), `Bus` (player-originated, affects shared, replicated), `Local`
(never replicated), `Stamped` (class determined per-event rather than per-type).
The registry table itself is Phase 6; today the class is documented, not
declared.

*Amendment.* `Stamped` is not only "resolved from the ambient domain at push".
`EventCombatAttackDirectRequest` is the case that forces the distinction: the
same producer, in the same tick, under the same ambient domain, pushes a hit
that crosses when the target is shared and does not when the target is player.
The class is a function of the payload, not of the producer, so **no static
per-type table can carry it**. The predicate exists today only as
`crossingTargets()` in `internal/app/bus_purity_test.go`. Phase 6 closes it one
of two ways: the journal filter carries the same predicate, or combat producers
stamp `GameEvent.Domain` from the target's domain and the filter keys on the
tag. The second is cheaper and D-10 already provides `Stamped` as the
mechanism, but it is a wiring change across every combat producer.

**D-11 Determinism invariants.** Across instances: identical shared event
order, identical shared entity creation order, identical shared RNG derivation,
identical shared component values except where D-13 applies. Verified by
comparing `App.SnapshotShared()` between instances and by stripping player
records from two journals and asserting equality.

**D-12 Claimed geometry.** A shared system that *claims* cells — spawn
footprint clear, composite sweep-over, wall push-out — enumerates both domains
and acts on every occupant. Not a D-1 violation: the shared outcome is a
function of the cell set and protection masks alone, so it is identical on
every instance; player victims differ per instance and are player-domain
effects by D-6. The constraint is on *emission*: victims leave as one death
batch per domain (`internal/system/sweep.go`, `cellSweep`), so a shared record
never names a player entity.

**D-13 Owner-authored shared state.** A shared entity may carry components
written by exactly one instance and replicated as values rather than
re-derived. The complete list: cursor gameplay components (energy, heat, boost,
shield, weapon, combat), `CursorComponent.Control`/`PeerID`, and
`CursorViewComponent`/`PingComponent`/`PulseComponent`, which are pure local
view. D-11 is refined: shared entity *identity* and *creation order* are
identical on every instance; shared component *values* are either re-derived
identically or owner-authored and transported — never both. Owner-authored
state must not appear in a cross-instance digest, and the metric keys mirroring
it are excluded by `denySharedPrefix` in `internal/app/snapshot.go`.

`Shield` and `Combat` are on the D-13 list but are not cursor-exclusive:
`Shield` also carries quasar state and the loot pickup glow, and `Combat` also
carries species state, both re-derived rather than transported. The static
check (`ownerAuthoredStores` in `internal/system/domain_test.go`) therefore
cannot key on store name alone and excludes both; they are covered by the
declared profile of their writers instead.

**D-14 Map bounds authority.** `MapWidth`, `MapHeight` and `CropOnResize` are
shared simulation state with two writers:

- `World.SetupLevel`, driven by `EventLevelSetup` from the map script. Shared
  and replicated; this is the authority.
- `GameContext.HandleResizeLocked`, driven by this instance's terminal, and
  admissible only while `mapSizeLocal()` holds. *When more than one player is
  present, crop is disabled and map size is locked.*

The join race is accepted: a resize already in flight when the second
participant appears may land, a resize after it will not. The window is one
event dispatch and the divergence is bounded by the guard immediately after.
Suppression publishes `context.map_locked` and logs once per resize.

Consequence not yet closed: a map script may branch an FSM guard on
`viewport_width`, `viewport_height`, `camera_x`, `camera_y` or `color_mode`,
which are per-instance. Under a locked map those diverge silently. Keys are
retained; instrumentation is a Phase 6 item. The whole script-visible surface
is `internal/engine/config_access.go` — eight keys, of which `map_width`,
`map_height` and `crop_on_resize` are replicated and the other five are not.

**D-15 Declared classification.** Every system declares its domain profile
(shared, player, dual) and its dependencies (required, optional). The
declaration site is `internal/manifest/definition.go`: `SystemDef.Domain`,
`Requires` and `Optional`, generated into `internal/manifest/build_gen.go` as
`systemProfiles` and resolved by `manifest.ProfileFor(name)`, which
`World.AddSystem(sys, profile)` takes. Systems do not declare `Domain()` or
`Requires()` methods — the methods were deleted rather than asserted, so there
is one declaration site and no drift to detect
(`TestSystemsDeclareNoDomainMethod`). Component domain is declared the same
way, as `ComponentDef.Domain`, generated into
`internal/engine/component_domain_gen.go`.

The declaration is the authority: the boundary suite asserts the code matches
it, and the FSM's `enabled_systems`/`disabled_systems` validation rejects a map
that disables a required dependency. Filename lists are not a classification
mechanism and are not maintained. Dependency order is initialization and
requirement order, resolved topologically by `core.TopoSort`; it is distinct
from `System.Priority()`, which orders `Update()` within a tick, and the two
are permitted to correlate without being conflated.

## 3. Spatial partition

`Cell` = `Count uint8 + SharedCount uint8 + [6]byte + [31]Entity` = 256 bytes,
asserted in `spatial_grid_test.go`. Invariant: shared occupy
`Entities[:SharedCount]`, player occupy `Entities[SharedCount:Count]`.

`engine.DomainScope` — `ScopeShared`, `ScopePlayer`, `ScopeBoth` — with
`Selects(entity)` for component-store iteration.
`parameter.ReservedPlayerPerCell = 12` guarantees 19 slots to shared, so a pile
of local effects can never starve shared placement. Insertion is a soft clip:
`Set` returns false rather than evicting.

Scoped APIs: `GetEntitiesAt(Into)`, `HasAnySharedEntityAt`, `ScanLine(First)`,
`FindClosestEntityInDirection`, `SpatialGrid.HasAnyEntityInArea`, and the
targeting triple (`HasCombatTargetAt`, `FindNearestTargets`,
`FindTargetsInEllipse`). Weapons and missiles pass `ScopeBoth`; shared species
pass `ScopeShared`. `PositionBatch.CommitShared` is the shared placement gate.

Telemetry: `spatial.player_budget_rejects`, `spatial.indexed_shared`.

## 4. Entity classification

| Domain | Entities |
|---|---|
| **Shared** | cursor, quasar, swarm, storm, snake, eye, pylon, tower, gateway, wall, nugget, gold, marker, explosion centers, FSM, time |
| **Player** | content glyph, dust, drain, decay, blossom, bullet, missile, orb, lightning, flash, fadeout, splash, motion marker, loot |
| **Stamped** | cleaner (nugget-spawned shared, weapon-spawned player), materialize (shared when it gates a shared spawn, player for drain), spirit (shared unless the requester is player-domain, which today is always the fuse) |

Cursor components split three ways: shared-and-replicated (position),
owner-authored (energy, heat, boost, shield, weapon, combat — D-13), and pure
local view (`CursorViewComponent`, `PingComponent`, `PulseComponent`).

`TransientResource` holds explosion centers and stays shared: they *are* the
crossing artifact. `ViewResource` (grayout, strobe) is player-domain.

**Glyph.** Content glyphs are player-domain: the corpus and the map are the
only inputs, so every instance derives the same text from its own player
counter and types against its own copy. Gold sequence members are the only
shared entities carrying `GlyphComponent`, which is why the bit stays unlisted
in `ComponentDef` — it attaches in either domain and the audit cannot key on
it. `TestSharedGlyphsAreGoldMembersOnly` asserts the split. Player-domain
mechanics that iterate glyphs — dust conversion, cleaner sweep, decay, blossom,
splash, typing, drain — guard by `e.Domain() != core.DomainPlayer`. One invariant
stated at the loop, replacing three accidental mechanisms (protection masks,
component absence, iteration order) that happened to hold.

**Contested objectives.** Nugget and gold are shared entities that any
participant may claim, and the claim itself is a shared outcome every instance
agrees on: `NuggetSystem.collectionCursor` and `GoldSystem.handleJumpRequest`
resolve over the whole roster and their sequence state is shared. Only the
*reward* is owner-authored (D-13). This is the deliberate opposite of loot,
which is rolled and owned per participant (D-6) precisely because its drop
table reads per-cursor inventory. A mechanic is contested when the outcome is a
function of shared state alone; personal when it reads owner-authored state.

Every `ClosestCursor` site is contested: deterministic slot order over
positions only. The one site that was not, `NuggetSystem.emitBeacon`, now uses
`PushLocal`, so a shared nugget beacon draws each instance's own cleaners
rather than one shared cleaner carrying pure visuals.

## 5. System classification

Per D-15 the declarations in `internal/manifest/definition.go` are
authoritative and this document does not restate them — read `Systems` there,
where each entry carries a one-line rationale for its profile. The shape worth
knowing:

- Cursor gameplay systems (`energy`, `heat`, `boost`, `shield`, `weapon`,
  `typing`, `ping`) are **player**, not shared: they author D-13 state for the
  local cursor only.
- Dual-domain systems are the stamped set (D-7): `cleaner`, `materialize`,
  `spirit`, `death`, `timer`, plus the two that hold one RNG stream per domain
  (D-8), `combat` and `soft_collision`.
- Shared species systems with `cellSweep` claimed-geometry sites (D-12): `eye`,
  `quasar`, `swarm`, `snake`, `storm`, `wall`.
- `explosion` is shared and is the crossing artifact, not an effect;
  `marker` is the one shared effect system.

`MetaSystem` is declared in `ContextSystems` because it takes a `GameContext`
rather than a `World`, so `BuildSystems` cannot construct it. Its profile is
`shared` and honest: its world writes are replicated or are the D-14
map-bounds write, and the context state it writes is not world state.
`NetworkSystem` is written but registered nowhere and carries a
`TODO(phase7)`; `internal/system/domain_test.go` exempts it by name.

## 6. Telemetry and snapshots

`status.GroupGate`: `GateAlways`, `GateSentinel` (gated on a roster slot's
entity cell), `GateActivity` (any non-zero member). Declared by prefix in
`activityGatedGroups`; honoured by `VisibleViews`, `Snapshot` and the flight
recorder. Add new wide-but-usually-silent groups by prefix, not by
special-casing a consumer.

Three snapshot views over one reading:

| view | drops | used by |
|---|---|---|
| `Snapshot` | nothing | `:d save`, perturbation test |
| `SnapshotSimulation` | operator surface (`denySim`, session record) | replay vs. source run |
| `SnapshotShared` | owner-authored state (`denySharedPrefix`, view record, local digest scope) | cross-instance comparison |

`SnapshotContext` emits five records. `context` and `world` are shared; `view`
and `session` are this instance's. `player` is presently mixed: `count` is the
shared roster size, but `entity`, `slot`, `x` and `y` are this instance's
binding to a slot and belong in `view` — see §7.
`worldDigestScopedLocked` takes a `DomainScope`, so the shared digest excludes
player entities.

`denySharedPrefix` drops, beyond the `player.` cursor state: this instance's
view of the map (`context.screen_`, `context.camera_`, `context.mode`), the
per-instance effect and cursor-state metric groups, and — as a blunt
instrument — `event.` and `spatial.`. `spatial.indexed_shared` is genuinely
comparable and is dropped with the rest; it wants an allow-list, not a prefix
deny.

## 7. Known gaps

- `event.EmitDeath` writes the queue directly, bypassing `PushEvent`, so
  `WithDomain` does not reach death records. Batches are already domain-pure,
  which is what determinism needs. `TODO(phase6)` in `sweep.go` and `fuse.go`.
- Storm red bullets are player-domain but push shared heat and shield drains.
  Consistent under D-5 only if those records are never transported — must be
  classified `Local`, not `Bus`.
- `internal/mode/` grant pushes carry `OriginCommand`, so they are journaled.
  Retagging them with `PushLocal` changes recorded record domains, which is why
  they were left out of the Phase 5 sweep and are batched with the Phase 6
  journal filter.
- The `player` snapshot record carries the local binding (`entity`, `slot`,
  `x`, `y`) in a record §6 calls shared. It passes today only because both
  instances in `TestSharedSnapshotParityAcrossTerminalSizes` bind slot 0. The
  binding fields move to `view`; `count` stays.
- `World.UpdateBoundsRadius` writes `PingComponent` for every rostered cursor
  including remote ones. Harmless under D-13 — pure local view, reaches no
  digest — but restricting it to the local slot forces `setLocal` to clear the
  departing slot.
- `uint32(entity)` narrowing at `gateway.go` and `adaptation.go` is safe only
  while route-graph anchors are shared (tag 0).
- `internal/journal` (the JSONL reader) has zero test coverage and exactly one
  non-test importer, `internal/app/play.go`. A `core.DomainNames` change
  silently breaks `vif play` with nothing to catch it.
- A recording wider than the terminal is clipped by the render buffer. The pan
  offset in `play.go` is the seam a windowed composite replaces. Deferred; it
  is a presentation problem with no shared-state component.
- Journal schema is 6.
