# Domain Phase 2 audit

This is the ground-truth audit requested before domain primitives are added. It
was performed on 2026-08-22 against `7ab4ff19` plus the Phase 1 dust-explosion
boundary changes in this pull request. It covers production code in `system`,
`engine`, `event`, `mode`, `render`, `network`, and application wiring. Test-only
spatial and player-resource references are listed separately so future API
changes do not discover them by surprise.

This document proposes classifications and migration decisions; it does not
introduce `Domain`, alter entity identity, or restructure the runtime.

## Conventions

| Term | Meaning in this audit |
|---|---|
| Shared | Deterministic simulation state/event that must agree in every instance. |
| Bus | Player-originated event whose consumer mutates shared state. |
| Local | Player-domain, view, input, audio, transport, or debug state/event that is not replicated. |
| Announced | A post-write event reports the applied state. A request that merely caused the write and telemetry mirrors do not count. |
| Silent | There is no post-write event for the applied value. “Threshold only” means only a transition such as crossing zero is announced. |
| View-only | The read selects camera/UI state and must remain instance-local; it is not simulation targeting. |

Line numbers refer to the Phase 1 tree in this pull request.

## Issues to resolve before Phase 3

These are audit findings, not changes requested in Phase 2.

| ID | Conflict | Why it blocks or risks domain primitives | Preparation decision needed |
|---|---|---|---|
| P3-1 | `FuseSystem` converts local drains into shared quasar/swarm state (`internal/system/fuse.go`). | It is an additional player-to-shared crossing beyond the revised Phase 1 drain-heal and dust-explosion buses. The older “only dust explosion crosses” boundary text is therefore already superseded. | Remove the mechanic, give fuse its own accepted Bus contract, and update the boundary specification before domains land. |
| P3-2 | Generic event types span domains: death, timer, species lifecycle, combat, materialize, flash, and fadeout. | Registry-level `Shared`/`Bus`/`Local` annotations cannot truthfully classify one type whose producers and entities span domains. | Split event types or explicitly permit ambient-domain classification for these generic types. |
| P3-3 | Cursor-owned shared components are written by local input/visual systems. | Local systems cannot directly mutate shared cursor state under the boundary rule. Examples include typing error flash, energy blink, ping grid, boost, heat, and weapon requests. | Decide which fields move to a local/view component and which writes become Bus events. |
| P3-4 | `CombatSystem` owns one RNG stream while processing shared, local, and Bus combat. | A player-domain collision can advance the same stream used for shared kinetic results, desynchronizing instances. | Partition combat resolution/streams by domain; the dust Bus resolver must use the Shared stream. |
| P3-5 | The position grid has one per-cell entity capacity before any caller filtering. | Even a correct domain filter cannot recover a shared entity omitted because local entities saturated the cell. | Partition cell storage/capacity by domain or make enumeration lossless before adding filtered queries. |
| P3-6 | Several shared systems use raw occupancy scans. | `CleanerSystem`, `WallSystem`, species systems, targeting, and navigation can currently observe player entities. | Add domain-aware query APIs and migrate the call-site groups below before enforcing the read boundary. |
| P3-7 | Nugget shared events embed `Resources.Player.Entity` (`internal/system/nugget.go:177,278`). | Different instances would stamp different cursor identities into otherwise shared behavior. | Resolve the closest roster cursor deterministically from the nugget position, or define a different shared ownership rule. |
| P3-8 | `GameContext.Config` mixes shared map/simulation settings with local viewport, camera, and color settings. | Copying or comparing a “shared” world can inadvertently depend on per-instance view state. | Separate simulation configuration from instance/view configuration. |
| P3-9 | App, scheduler, event queue, journal, service hub, input router, and renderer are wired around one `World`. | Phase 3 cannot be a local `World` edit; ownership and fixed iteration order must be explicit at the application boundary. | Introduce an instance collection and selection/routing contract before changing scheduling. |
| P3-10 | Events, stamps, and journal records have no domain or player slot. | Replication and the shared-journal equality invariant cannot be enforced or tested. | Land event-domain stamping and journal schema/version decisions together. |
| P3-11 | Player bullets inspect shared cursors/shields, and typing/deletion can inspect both local glyphs and shared composites. | These are more cross-domain paths than the stated boundary allows. | Confirm intended entity domains/mechanics and either remove, split, or explicitly classify their events as Bus. |
| P3-12 | `ExplosionSystem.addCenter` updates `TransientResource` before emitting the dust Bus event. | A future recipient can reproduce Shared damage from the geometry payload but will not reproduce the centre's visual/merge state, even though explosion centres are specified as Shared. | Make the Bus consumer authoritative for Shared centre creation/merge, or explicitly reclassify dust-spawned centres as Player effects and keep only their damage crossing Shared. |

## 1. Writes to cursor-owned components

The cursor-owned component set is the set attached by `CursorSystem.build`:
`Position`, `Cursor`, `Protection`, `Ping`, `Heat`, `Energy`, `Shield`, `Boost`,
`Weapon`, and `Combat`. `Pulse` is an optional cursor-owned visual component.
The table records every production mutation path for those components when the
target entity is a cursor; writes of the same component types on non-cursor
species are outside this table.

| Component/field | Writer and site | Trigger | Applied write announced? | Cosmetic? | Domain note |
|---|---|---|---|---|---|
| Full initial set, including `Position` | `CursorSystem.build`, `internal/system/cursor.go:201-224` | `EventCursorSpawnRequest` | Yes: `EventCursorSpawned` and `EventCursorMoved` at `:165-166` | Mixed | Shared/bootstrap. Entity creation order must match across instances. |
| All cursor components and position removed | `CursorSystem.destroy`, `internal/system/cursor.go:186-198`; `World.DestroyEntity`, `internal/engine/world.go:98-113` | `EventCursorDespawnRequest` | Yes: `EventCursorDespawned` | Mixed | Shared. Roster unbind precedes destruction. |
| `Position` | `CursorSystem.move`, `internal/system/cursor.go:103-125` | `EventCursorMoveRequest` | Yes: `EventCursorMoved`, including same-cell reconcile | No | Shared write; local input should cross as Bus intent. |
| `Cursor.ErrorFlashRemaining` | `TypingSystem.emitTypingError`, `internal/system/typing.go:220-225` | Local typing failure | No | Yes | Direct Local-to-Shared write; move field local or introduce a Bus/applied event. |
| `Cursor.ErrorFlashRemaining` | `EnergySystem.Update`, `internal/system/energy.go:201-205` | Tick aging | No | Yes | Same ownership issue; aging belongs with whichever domain owns the field. |
| `Ping.BoundsActive`, `BoundsRadiusX/Y` | `World.UpdateBoundsRadius`, `internal/engine/world.go:260-279` | Cursor spawn/local bind, shield toggle, mode change | No | Yes/view-control | Reads local mode plus shared shield. Strong candidate for a Local view component. |
| `Ping.GridActive`, `GridRemaining` | `PingSystem.handleGridRequest`, `internal/system/ping.go:123-131` | `EventPingGridRequest` | No | Yes | Local. The component currently sits on a shared cursor. |
| `Ping.GridActive`, `GridRemaining` | `PingSystem.Update`, `internal/system/ping.go:97-120` | Tick aging | No | Yes | Local aging path. |
| `Energy.Current` | `EnergySystem.addEnergy`, `internal/system/energy.go:232-313` | `EventEnergyAddRequest` | Threshold only: shield change and `EventEnergyCrossedZero` | No | Mixed producers; classify or split request sources. |
| `Energy.Current` | `EnergySystem.setEnergy`, `internal/system/energy.go:337-356` | `EventEnergySetRequest` | Threshold only | No | Debug/input origin is Bus if the cursor remains Shared. |
| `Energy.Current` | `EnergySystem.handleGlyphConsumed`, `internal/system/energy.go:358-398` | `EventEnergyGlyphConsumed` | Threshold only | No | Player glyph consumption mutates shared cursor energy: Bus. |
| `Energy.BlinkActive`, `BlinkType`, `BlinkLevel`, `BlinkRemaining` | `EnergySystem.startBlink/stopBlink`, `internal/system/energy.go:400-420` | blink events | No | Yes | Move to Local/view state. |
| `Energy.BlinkActive`, `BlinkRemaining` | `EnergySystem.Update`, `internal/system/energy.go:207-216` | Tick aging | No | Yes | Move with blink state. |
| `Heat.Current`, `Overheat`; burst/ember fields | `HeatSystem.addHeat`, `internal/system/heat.go:157-188` | `EventHeatAddRequest` | Threshold only: `EventHeatBurst`; telemetry mirrors values | Mixed: burst flash is cosmetic | Mixed Shared/Bus producers need one contract. |
| `Heat.Current`, `Overheat` | `HeatSystem.setHeat`, `internal/system/heat.go:190-210` | `EventHeatSetRequest` | No; telemetry only | No | Bus if cursor heat is Shared. |
| `Heat.Current`, `Overheat`, `EmberActive`, `EmberDecayTime` | `HeatSystem.Update`, `internal/system/heat.go:76-90` | Tick decay | No; telemetry only | No | Shared simulation aging. |
| `Heat.BurstFlashRemaining` | `HeatSystem.Update`, `internal/system/heat.go:71-74` | Tick aging | No | Yes | Candidate Local/view field. |
| `Shield.Type`, radii/inverses, `Active` | `ShieldSystem.setActive`, `internal/system/shield.go:136-157` | activate/deactivate event | No post-write event | No | Shared cursor state; current event names are commands despite lacking `Request`. |
| `Shield.LastDrainTime` | `ShieldSystem.Update`, `internal/system/shield.go:159-183` | passive drain interval | No | No/bookkeeping | Shared deterministic timer. |
| `Boost.Active`, `Remaining`, `TotalDuration` | `BoostSystem.reward/rewardKill`, `internal/system/boost.go:153-185` | `EventBoostReward` or species kill | No; telemetry only | No | Mixed Local typing and Shared kill producers. |
| `Boost.Active`, `Remaining`, `TotalDuration` | `BoostSystem.activate/deactivate/extend`, `internal/system/boost.go:211-252` | boost command events | No; telemetry only | No | Bus when originated by local input. |
| `Boost.Active`, `Remaining` | `BoostSystem.Update`, `internal/system/boost.go:187-208` | Tick aging | No; telemetry only | No | Shared simulation aging. |
| `Weapon.MainFireCooldown`, per-type `Cooldown` | `WeaponSystem.Update`, `internal/system/weapon.go:155-180` | Tick aging | No | No | Shared cursor/loadout simulation. |
| `Weapon.Charges`, per-type `Cooldown` | `WeaponSystem.addWeapon/removeAllWeapons`, `internal/system/weapon.go:199-233` | weapon add / zero-cross cleanup | No; telemetry only | No | Shared. |
| `Weapon.Orbs` | `WeaponSystem.ensureOrbs`, `destroyOrb`, `destroyCursorOrbs`, `internal/system/weapon.go:245-263,488-509` | loadout reconcile / orb death | No | No | References player-domain orb candidates from a shared cursor component; ownership must be decided. |
| `Weapon.MainFireCooldown`, per-type `Cooldown` | `WeaponSystem.handleFireMain/fireAllWeapons/fireDisruptorWeapon`, `internal/system/weapon.go:519-527,581-675` | `EventWeaponFireRequest` | No | No | Local fire intent should be Bus if weapon/cursor stays Shared. |
| `Pulse` attach, `Pulse.Remaining`, removal | `WeaponSystem.fireDisruptorWeapon/Update`, `internal/system/weapon.go:182-187,696-702` | disruptor fire / tick aging | No | Yes | Local/view component even though attached to shared cursor identity. |
| `Combat` immunity, stun, and hit-flash timers | `CombatSystem.Update`, `internal/system/combat.go:352-402` | Tick aging | No | Mixed: hit flash is cosmetic | Applies to every combat entity, including cursors. Split cosmetic state or keep deterministic timer ownership explicit. |
| `Combat.HitPoints`, `LastDamagedBy`, damage/kinetic immunity, hit flash, stun/enrage | `CombatSystem.applyHitDirect/applyHitArea` and effect helpers, `internal/system/combat.go:404-730` | combat request | No post-resolution event | Mixed | Eye self-destruct can target `CombatEntityCursor` (`internal/profile/combat.go:58-69`). Shared if retained. |
| `Combat.HitPoints` | `CombatSystem.applyHeal`, `internal/system/combat.go:337-350` | `EventCombatHealRequest` | No | No | The generic API could heal a cursor even though current drain producers allow-list swarm/quasar. Constrain by domain/type. |

`Protection` is initialized and removed with the cursor but has no later
cursor-targeted production mutation. `ProtectAll` is therefore stable for the
cursor lifetime.

## 2. Reads of `Resources.Player.Entity`

There are 60 production reads. “Own cursor” means the per-instance local cursor
after Phase 3; “closest cursor” means replace the singleton identity with a
deterministic roster query; “view-only” means retain local selection without
feeding shared simulation.

| Site(s) | Count | Migration | Rationale |
|---|---:|---|---|
| `internal/app/loop.go:147`; `internal/app/play.go:197` | 2 | View-only | Render camera anchoring for live/caller-driven frames. |
| `internal/engine/world.go:284` | 1 | View-only | Convenience wrapper for local ping bounds. |
| `internal/mode/actions.go:9`; `commands.go:584,613,624,629,639,651,678`; `motions_helpers.go:25`; `operators.go:19`; `search.go:20` | 11 | Own cursor | Input/mode commands act for this instance's participant. |
| `internal/mode/router.go:328,357,377,411,454,471,503,529,573,581,588,595,637,691,696,829,839,854,873,895,1016,1031,1101,1159` | 24 | Own cursor | Router-local intent and cursor-relative movement/search. Reads used only for preview should still resolve through the same local selector. |
| `internal/render/renderer/cursor.go:104`; `ember.go:85`; `heat.go:45`; `ping.go:36`; `pulse.go:42`; `shield.go:207`; `status_bar.go:148` | 7 | View-only | Local HUD/cursor presentation. |
| `internal/system/drain.go:465,648,786` | 3 | Own cursor | Drain is Player-domain and explicitly targets the instance cursor, not roster group 0. |
| `internal/system/dust.go:166,184,232,525` | 4 | Own cursor | Dust ownership, spawn geometry, and explosion credit are Player-domain. |
| `internal/system/motion_marker.go:106,130,210,237` | 4 | View-only | Local motion-hint entities and bounds. |
| `internal/system/splash.go:200,217` | 2 | View-only | Local magnifier/timer effect follows the instance cursor. |
| `internal/system/nugget.go:177,278` | 2 | Closest cursor | Shared cleaner/spawn payloads currently leak the local selection. Resolve from nugget position with deterministic slot tie-break. |

Test-only reads are `internal/app/replay_test.go:118,668` and
`internal/app/soak_test.go:79,104`. They refer to the fixture's local cursor and
should migrate to the own-cursor API.

## 3. Spatial query call sites

### Query policy

The production tree has 189 identity lookups and 110 other position-grid query
calls. The required migration is driven by what a query enumerates, not merely
by its method name.

| Query family | Domain filter? | Rule |
|---|---|---|
| `GetPosition(entity)`, `HasPosition(entity)` | No | Entity identity already selects one domain. Validate the entity at event boundaries instead. |
| `IsOutOfBounds` | No | Reads map geometry only. |
| `HasBlockingWallAt/InArea`, `IsBlocked`, `IsAreaFree`, `FindFreeAreaSpiral`, `FindFreeFromPattern`, `CheckBlockedBatch`, `IsAnyBlockedInSet`, LOS/path/orbit helpers | No API filter | These methods already filter by `WallComponent` and mask; walls are Shared. Their *caller* still needs a domain-correct source/target. |
| `AllEntities`, `Entities`, `CountEntities` | No for lifecycle/digest; optional for metrics | World clear/resize and deterministic digest intentionally cover both domains. Per-domain telemetry may be added separately. |
| `GetAllEntityAt`, `GetAllEntitiesAtInto`, `FindClosestEntityInDirection` | Yes | Raw occupancy must filter Shared or Player at the storage/API boundary. Renderer calls intentionally request both. |

The single-grid capacity issue in P3-5 must be solved before “enumerate then
filter” can be considered correct.

### Raw occupancy and directional enumeration

| Call sites | Required view | Notes |
|---|---|---|
| `internal/render/renderer/cursor.go:55`; `marker.go:83` | Both | View-only compositing. Preserve deterministic layer choice. |
| `internal/mode/motions_helpers.go:21,84,110,270,330,365,384`; `motions.go:820,874` | Player | Glyph/local editing and motion filters. If a command deliberately selects Shared composites, expose that as an explicit second query rather than an unfiltered scan. |
| `internal/system/motion_marker.go:195,226,256` | Player | Glyph motion hints. |
| `internal/system/blossom.go:263`; `decay.go:263`; `dust.go:400`; `splash.go:225,345` | Player | Local particle/species interactions. Dust's scan now recognizes drains only. |
| `internal/system/drain.go:740,937,1054` | Split | Spawn/local flock contacts are Player; the explicitly named heal crossing observes Shared swarm/quasar. Use separate filtered scans. |
| `internal/system/explosion.go:330` | Split by explosion type | Dust local effects scan Player; missile/shared explosions scan Shared. Do not retain one ambient unfiltered loop. |
| `internal/system/combat.go:258` | Shared | Dedicated dust Bus resolver must enumerate only Shared combat targets. |
| `internal/system/targeting.go:79` | Shared | Shared targeting/group resolution. |
| `internal/system/eye.go:286`; `quasar.go:316,709`; `snake.go:1025`; `storm.go:414,841`; `swarm.go:283` | Shared | Shared species spawn/collision/placement logic must not observe Player entities. |
| `internal/system/cleaner.go:432` | Shared | Cleaner is Shared; currently a raw scan can observe local glyph/dust/drain. Confirm desired attacks through Bus events rather than reads. |
| `internal/system/wall.go:189,307,713,943,1094,1128` | Shared | Wall displacement/destruction logic must not inspect Player entities. Player-side wall response must be owned locally. |
| `internal/system/typing.go:135` | Split | The cell can contain Player glyphs and Shared composite members. Define explicit local-first/shared-intent behavior and separate the queries/events. |

### Identity lookups

All 189 production `GetPosition(entity)` calls need no query-level filter. This
inventory is exhaustive so migration work can distinguish identity validation
from raw spatial filtering:

| Package | Call sites |
|---|---|
| App/engine | `internal/app/digest.go:58`; `app/loop.go:147`; `app/play.go:197`; `internal/engine/game_context.go:250,428`; `engine/snapshot.go:32`; `engine/world.go:289,333,394` |
| Mode | `internal/mode/actions.go:9`; `mode/router.go:357,377,411,454,471,503,529,637,691,829,839,854,873,895,1162`; `mode/search.go:20,65` |
| Render | `internal/render/renderer/bullet.go:45`; `chargeline.go:67`; `drain.go:38`; `ember.go:94`; `eye.go:167`; `fadeout.go:37`; `flash.go:39`; `glyph.go:39`; `gold.go:43`; `healthbar.go:90`; `lightning.go:63,71`; `orb.go:64,82`; `ping.go:98`; `pylon.go:244,350,445`; `quasar.go:45,136`; `shield.go:229`; `sigil.go:34`; `snake.go:87,231,303,410,436,497,523`; `splash.go:72`; `storm.go:97`; `swarm.go:82`; `tower.go:317,379,439`; `wall.go:46` |
| Systems A-C | `internal/system/bullet.go:133,182,199`; `cleaner.go:175,647`; `combat.go:752,756,810,814,837,854`; `composite.go:101`; `cursor.go:111,246` |
| Systems D-G | `internal/system/death.go:158,367,411`; `drain.go:298,609,622,636,650,788,1169`; `dust.go:167,185,233,478,526,548`; `explosion.go:194`; `eye.go:159,192,457,538,572,598`; `fuse.go:189,190,219,250,295`; `gateway.go:102,207`; `genetic.go:289,302`; `glyph.go:316`; `gold.go:210,469` |
| Systems I-N | `internal/system/interaction.go:32,69,85,103`; `loot.go:575`; `meta.go:194`; `missile.go:155,263,279,298,316`; `motion_marker.go:107,130,210,237`; `navigation.go:365,444,499,558,578,611,862,937`; `nugget.go:152,174,200,298,346` |
| Systems P-S | `internal/system/pylon.go:313,348`; `quasar.go:172,470,575,594,642`; `snake.go:182,202,581,647,740,922,942`; `soft_collision.go:257,264,271,287,392`; `splash.go:200,217,466,520`; `storm.go:759,935,1086,1220`; `swarm.go:183,567,712` |
| Systems T-W | `internal/system/targeting.go:119,148,194,223,315,357`; `typing.go:250,367,407`; `wall.go:518,626,678,696`; `weapon.go:267,324,432,539,553,639` |

### Component-filtered, geometry-only, and global calls

Every remaining production spatial call is covered here. “No filter” means no
domain parameter is required for the query; it does not waive domain validation
for entities passed into it.

| Call sites | Decision |
|---|---|
| `internal/system/pylon.go:277,333`; `tower.go:263,296`; `eye.go:236,242,463`; `storm.go:400,550,1331`; `swarm.go:239,246,573,611,719`; `snake.go:265,271,532,617`; `quasar.go:274,281,538`; `fuse.go:200,273`; `navigation.go:282,292,323,382,384` | No filter: wall-mask placement/LOS. Fuse still has the domain-crossing issue P3-1. |
| `internal/system/dust.go:376`; `blossom.go:240,245`; `decay.go:242,247`; `bullet.go:160,165`; `missile.go:147,243`; `cleaner.go:212,249`; `drain.go:1044`; `glyph.go:330`; `gold.go:489`; `nugget.go:315` | No filter: bounds or wall-mask checks. |
| `internal/system/wall.go:181,292,325` | No filter for wall occupancy/masks. Raw scans in the preceding table still require Shared view. |
| `internal/system/splash.go:428,487`; `loot.go:199,271,365`; `engine/world.go:314,321`; `mode/motions.go:503,530,555`; `mode/motions_helpers.go:58,64` | No filter: wall-mask free-cell/placement helpers. |
| `internal/system/weapon.go:373,435,438,446,458` | No filter: wall-aware orbit/path geometry. |
| `internal/system/glyph.go:231` | No filter: identity existence check. |
| `internal/app/digest.go:57,86`; `internal/engine/game_context.go:418`; `engine/world.go:411`; `engine/clock_scheduler.go:1087` | Both domains: digest, clear/resize lifecycle, and total telemetry. Consider additional per-domain metrics, not filtering these calls. |

Test-only spatial calls are `internal/app/replay_test.go:673,683`,
`app/telemetry_test.go:213`, `internal/system/cleaner_test.go:50`,
`drain_interaction_test.go:83,95`, `dust_explosion_boundary_test.go:55`, and
`multi_cursor_test.go:93`. They are fixture identity lookups or assertions and
should follow the production API selected by the behavior under test.

## 4. RNG stream audit

There are 22 production `world.Rand(label)` consumers plus the
`GameContext.Rand` forwarding method.

| Call site | Intended domain | Notes |
|---|---|---|
| `internal/system/adaptation.go:89` | Shared | Shared evolution/adaptation decisions. |
| `internal/system/blossom.go:60` | Player | Local species. |
| `internal/system/combat.go:116` | Split/ambiguous | One system resolves Shared, Player, and Bus attacks. Partition streams or systems; dust Bus kinetic must use Shared. |
| `internal/system/decay.go:60` | Player | Local species. |
| `internal/system/drain.go:107` | Player | Local species. |
| `internal/system/dust.go:103` | Player | Local species. |
| `internal/system/environment.go:43` | Shared | Shared environment scheduling. |
| `internal/system/fuse.go:62` | Bus/ambiguous | Local drain selection creates Shared species; depends on P3-1 decision. |
| `internal/system/gateway.go:43` | Shared | Shared species. |
| `internal/system/glyph.go:106` | Player | Local species. |
| `internal/system/gold.go:59` | Shared | Shared sequence placement. |
| `internal/system/lightning.go:33` | Shared | Lightning entities are Shared in the proposed model. |
| `internal/system/loot.go:72` | Shared | Shared spawn/selection. |
| `internal/system/music.go:69` | Local | Audio sequencer; never replicated. |
| `internal/system/nugget.go:54` | Shared | Shared spawn/behavior. Must also remove local cursor identity leak. |
| `internal/system/pylon.go:46` | Shared | Shared species. |
| `internal/system/quasar.go:53` | Shared | Shared species. |
| `internal/system/snake.go:51` | Shared | Shared species. |
| `internal/system/soft_collision.go:192` | Shared | Drains are excluded from shared soft collisions; retain a Shared stream. |
| `internal/system/storm.go:89` | Shared | Shared species. |
| `internal/system/swarm.go:60` | Shared | Shared species. |
| `internal/system/tower.go:46` | Shared | Shared species. |
| `internal/engine/game_context.go:704` | API must require domain | The forwarding method currently erases intent; replace with `Rand(domain, label)` or remove it. |

Related derivations not expressed as `world.Rand(label)` are
`internal/system/genetic.go:196` and `internal/system/wall.go:60` (both Shared),
and renderer-local `vmath.NewFastRand` usage such as lightning rendering (Local,
view-only). App seed/session advancement is part of the single-World audit below.

## 5. Event type classification

This table covers all 167 constants in `internal/event/type.go`, including the
zero sentinel. “Ambiguous” is deliberate: the current event cannot receive one
registry classification without either changing a mechanic or splitting the
type. An asterisk on a concrete class identifies a migration caveat, not an
unclassified event.

<!-- event-audit-start -->

| Event type(s) | Class | Evidence and migration note |
|---|---|---|
| `EventNone` | Local | Non-event sentinel; never push or replicate. |
| `EventLevelSetup` | Shared | Changes shared map dimensions/lifecycle. The scheduler must deliver it identically to every instance. |
| `EventScreenResize` | Local | Terminal/view geometry. If crop-on-resize changes the shared map, emit a separate Shared level change. |
| `EventSoundRequest`, `EventSoundMuteToggle`, `EventAudioMuteChanged` | Local | Audio is explicitly provisional Local. |
| `EventMusicStart`, `EventMusicStop`, `EventBeatPatternRequest`, `EventMelodyNoteRequest`, `EventMelodyPatternRequest`, `EventMusicIntensityChange`, `EventMusicTempoChange`, `EventMusicSeedRequest`, `EventMusicSwingRequest` | Local | Music/audio sequencer state is instance-local. |
| `EventNetworkConnect`, `EventNetworkDisconnect`, `EventRemoteInput`, `EventStateSync`, `EventNetworkEvent`, `EventNetworkError` | Local | Transport events are explicitly provisional Local. Decoded replicated game events must be pushed separately with their declared class/domain. |
| `EventGameResetRequest` | Shared* | Resets shared and own-player state. Treat as scheduler-wide control and fan it to all worlds; do not independently replicate one world's reset handling. |
| `EventMetaDebugRequest`, `EventMetaHelpRequest`, `EventMetaAboutRequest`, `EventMetaStatusMessageRequest` | Local | Debug/overlay/status presentation. |
| `EventMetaSystemCommandRequest` | Local* | Debug is provisional Local, but this command currently enables/disables simulation systems. Split local presentation commands from synchronized simulation control. |
| `EventGamePauseRequest`, `EventGamePauseChanged`, `EventGameSpeedRequest`, `EventGameSpeedChanged`, `EventGameStepRequest` | Local* | Scheduler control, not world replication. One scheduler must apply the result to all instances because Time is Shared. |
| `EventCycleDamageMultiplierIncrease`, `EventCycleDamageMultiplierReset`, `EventFSMRegionRequest` | Shared | Shared progression/FSM state. |
| `EventNuggetCollected`, `EventNuggetDestroyed` | Shared | Shared species outcomes. Collection credit must carry a deterministic roster cursor. |
| `EventNuggetJumpRequest` | Bus | Local player intent moves a shared cursor to a shared nugget. |
| `EventCleanerDirectionalRequest` | Ambiguous | Emitted by Shared nugget and player-originated weapon paths. Split Shared autonomous spawn from Bus fire intent. |
| `EventCleanerSweepingRequest` | Bus | Player command creates Shared cleaners. |
| `EventGoldSpawnRequest`, `EventGoldSpawnFailed`, `EventGoldSpawned`, `EventGoldCompleted`, `EventGoldTimeout`, `EventGoldDestroyed`, `EventGoldCancel` | Shared | Shared gold lifecycle. |
| `EventGoldJumpRequest` | Bus | Local player intent moves a shared cursor. |
| `EventSplashTimerRequest`, `EventSplashTimerCancel` | Local | Splash/timer visual is Player-domain. |
| `EventEnergyAddRequest` | Ambiguous | Shared species and player systems both produce it for a shared cursor. Split Shared mutation from Bus intent or constrain producers. |
| `EventEnergySetRequest` | Bus | Local command sets shared cursor energy. |
| `EventEnergyCrossedZero` | Shared | Applied shared energy transition. |
| `EventEnergyGlyphConsumed` | Bus | Player glyph destruction changes shared cursor energy. |
| `EventEnergyBlinkStart`, `EventEnergyBlinkStop` | Local* | Cosmetic. Move blink fields off the shared Energy component. |
| `EventShieldActivate`, `EventShieldDeactivate` | Shared | Applied/requested shield state derived from shared energy. Names should be clarified if the registry distinguishes request from result. |
| `EventShieldDrainRequest` | Ambiguous | Shared species and Player bullet paths both drain shared cursor energy. Split Shared and Bus producers. |
| `EventWeaponAddRequest` | Ambiguous | Shared loot and local debug/command producers grant a shared loadout. Split or normalize at a Bus boundary. |
| `EventWeaponFireRequest` | Bus | Player intent causes shared cleaner/missile/combat effects. |
| `EventFireSpecialRequest` | Local | Spawns Player-domain dust/blossom/decay mechanics. |
| `EventHeatAddRequest` | Ambiguous | Shared species, Player bullet/drain, typing, loot, and nugget producers target shared cursor heat. |
| `EventHeatSetRequest` | Bus | Local command sets shared cursor heat. |
| `EventHeatBurst` | Shared | Applied shared heat transition; its visual substate should be Local. |
| `EventBoostActivate`, `EventBoostDeactivate`, `EventBoostExtend`, `EventBoostReward` | Ambiguous | Local typing/commands and Shared energy/species-kill paths all mutate shared cursor boost. Split by source or move the mechanic. |
| `EventCharacterTyped` | Local* | Input is provisional Local, but the current handler directly mutates shared composites/cursor feedback. Those consequences need explicit Bus events. |
| `EventDeleteRequest` | Ambiguous | One operation can target Player glyphs or Shared composite members. Split Local deletion from Bus shared-combat intent. |
| `EventPingGridRequest` | Local* | View effect, but currently writes `PingComponent` on a shared cursor. Move the state local. |
| `EventMaterializeRequest` | Local | Current single-entity producer/consumer path materializes drains. |
| `EventMaterializeComplete` | Ambiguous | Generic completion inherits the materialized entity's domain; split the type or permit an ambient-domain generic event. |
| `EventMaterializeAreaRequest` | Ambiguous | Shared storm and Bus fuse paths share this type. P3-1 decides the fuse side. |
| `EventFlashSpawnOneRequest`, `EventFlashSpawnBatchRequest` | Ambiguous | Death/effect producers exist in both domains. Effect entities inherit the spawning mechanic's domain. |
| `EventExplosionRequest` | Shared | After the Phase 1 change, eye and missile are its production sources; dust uses its dedicated Bus request. |
| `EventDustExplosionRequest` | Bus | Explicit crossing carrying centre, radius, owner cursor, and attack type; resolves Shared health and kinetic effects. Shared centre replication itself remains P3-12. |
| `EventDustSpawnOneRequest`, `EventDustSpawnBatchRequest`, `EventDustAllRequest` | Local | Dust is Player-domain. |
| `EventBlossomSpawnOne`, `EventBlossomSpawnBatch`, `EventBlossomWave` | Local | Blossom is Player-domain. |
| `EventDecaySpawnOne`, `EventDecaySpawnBatch`, `EventDecayWave` | Local | Decay is Player-domain. |
| `EventDeathBatch` | Ambiguous | Uniform pooled death API targets entities in both domains. Split by event type or make ambient domain part of the generic API contract. |
| `EventTimerStart` | Ambiguous | Generic lifecycle timers can own Shared or Player entities. Same decision as death. |
| `EventCompositeMemberDestroyed` | Ambiguous | Player typing and Shared species self-management both produce it for Shared composites. A Bus typing request should be distinct from the Shared applied outcome. |
| `EventCompositeIntegrityBreach`, `EventCompositeDestroyRequest` | Shared | Shared composite lifecycle. If a future Player composite uses these, split the generic types first. |
| `EventCursorSpawnRequest`, `EventCursorDespawnRequest` | Ambiguous | Bootstrap/FSM and participant/debug control both mutate the replicated cursor roster. Define scheduler/bootstrap versus Bus request paths. |
| `EventCursorSpawned`, `EventCursorSpawnFailed`, `EventCursorDespawned` | Shared | Applied shared roster lifecycle. |
| `EventCursorMoveRequest` | Ambiguous | Local movement/jump is Bus; shared resize/reconcile can be Shared. Split intents from deterministic reconcile. |
| `EventCursorMoved` | Shared | Applied shared cursor position. |
| `EventCursorSetLocalRequest`, `EventCursorLocalChanged` | Local | Selects which replicated roster cursor this instance controls/follows. |
| `EventSpeciesCreated`, `EventSpeciesKilled` | Ambiguous | Generic lifecycle announcements currently cover Player drain and all Shared species. Split or ambient-stamp by entity domain. |
| `EventFuseQuasarRequest`, `EventFuseSwarmRequest` | Bus* | Local drains create Shared species. This is the extra, unresolved crossing in P3-1; the revised Phase 1 already accepts drain-heal and dust-explosion buses. |
| `EventDrainPause`, `EventDrainResume` | Local | Drain is Player-domain. |
| `EventQuasarSpawnRequest`, `EventQuasarCancelRequest` | Shared | Shared species lifecycle. |
| `EventSwarmSpawnRequest`, `EventSwarmCancelRequest` | Shared | Shared species lifecycle. |
| `EventStormSpawnRequest`, `EventStormCancelRequest` | Shared | Shared species lifecycle. |
| `EventGrayoutStart`, `EventGrayoutEnd`, `EventStrobeRequest` | Local | Post-processing/view state. |
| `EventSpiritSpawnRequest`, `EventSpiritDespawnRequest` | Shared | Spirit is listed in the Shared domain. Fuse-originated spirit creation remains dependent on P3-1. |
| `EventLightningSpawnRequest`, `EventLightningUpdateRequest`, `EventLightningDespawnRequest` | Shared | Lightning is listed in the Shared domain; audio triggered by it remains Local. |
| `EventCombatAttackDirectRequest`, `EventCombatAttackAreaRequest` | Ambiguous | Shared species attacks, player weapon Bus paths, and domain-generic combat share these types. Keep the post-Bus shared resolver internal or split attack intents by domain. |
| `EventCombatHealRequest` | Bus | Explicit drain-to-shared healing crossing; current targets are swarm/quasar. |
| `EventLootSpawnRequest` | Shared | Loot is Shared. Player-originated kills must first resolve as a Shared combat outcome. |
| `EventMissileSpawnRequest` | Shared | Missile is Shared in the proposed model; local fire intent reaches it through Bus. |
| `EventBulletSpawnRequest` | Local* | Bullet is Player-domain, but current bullets inspect shared cursors/shields. Resolve P3-11. |
| `EventMarkerSpawnRequest` | Shared | Marker is listed in the Shared domain. |
| `EventMotionMarkerShowColored`, `EventMotionMarkerClearColored` | Local | Player motion hints. |
| `EventModeChanged` | Local | Input/view mode; shared cursor-facing consequences require a separate Bus event. |
| `EventWallSpawnRequest`, `EventWallBatchSpawnRequest`, `EventWallCompositeSpawnRequest`, `EventWallPatternSpawnRequest`, `EventMazeSpawnRequest`, `EventWallDespawnRequest`, `EventWallMaskChangeRequest`, `EventWallPushCheckRequest`, `EventWallSpawned`, `EventWallDespawned`, `EventWallDespawnAll` | Shared | Wall state and applied lifecycle are Shared. Local entities must react from their own side. |
| `EventFadeoutSpawnOne`, `EventFadeoutSpawnBatch` | Ambiguous | Generic effect spawned from deaths in both domains; inherit/split by source domain. |
| `EventPylonSpawnRequest`, `EventPylonSpawnFailed`, `EventPylonCancelRequest` | Shared | Shared species lifecycle. |
| `EventSnakeSpawnRequest`, `EventSnakeCancelRequest` | Shared | Shared species lifecycle. |
| `EventTargetGroupUpdate`, `EventTargetGroupRemove`, `EventNavigationRegraph`, `EventRouteGraphRequest`, `EventRouteGraphComputed` | Shared | Shared navigation/targeting state. Player drain targeting must not join these groups. |
| `EventEyeSpawnRequest`, `EventEyeCancelRequest` | Shared | Shared species lifecycle. |
| `EventTowerSpawnRequest`, `EventTowerSpawnFailed`, `EventTowerCancelRequest` | Shared | Shared species lifecycle. |
| `EventGatewaySpawnRequest`, `EventGatewayDespawnRequest`, `EventGatewayDespawned` | Shared | Shared species lifecycle. |
| `EventGeneticRegisterSpecies`, `EventGeneticAbandonEval` | Shared | Shared evolution state. Player-domain species need a separate/local genetics path if added later. |
| `EventDebugFlowToggle`, `EventDebugGraphToggle` | Local | Debug events are explicitly provisional Local. |

<!-- event-audit-end -->

### Classification decisions required before registry annotations

| Decision | Affected types |
|---|---|
| Split generic lifecycle/effect types or allow ambient-domain generic events | death, timer, species lifecycle, materialize completion, flash, fadeout |
| Split local intent from shared applied combat | character/delete, cleaner, weapon, combat direct/area, cursor movement |
| Separate shared cursor simulation fields from local presentation fields | energy blink, heat burst flash, cursor error flash, ping grid/bounds, pulse |
| Resolve mixed Shared/Bus resource mutations | energy, heat, shield drain, boost, weapon add |
| Decide whether fuse and bullet mechanics remain | fuse requests, materialize area/spirit side effects, bullet/shield/heat interaction |

## 6. Single-`World` assumptions

| Area and site | Current single-world assumption | Phase 3 requirement |
|---|---|---|
| `App`, `internal/app/app.go:27-46` | Stores one `world`, `GameContext`, mode router, renderer set, scheduler, and frame/update channel pair. | Store local world instances in fixed participant order and identify the active local instance for input/render. |
| World construction, `internal/app/app.go:139-205` | Calls `NewWorld` once, binds services once, builds one manifest system set, and advances one RNG session. | Build every instance from one session root; derive identical Shared streams and per-instance Player streams without order-dependent root advancement. |
| Scheduler construction, `internal/app/app.go:232-254` | Registers handlers from one world into one event router. | One scheduler owns the ordered world list; each world retains its own queue/router/handler set. |
| Frame loops, `internal/app/loop.go`, `play.go`, `headless.go` | Lock, tick, settle, snapshot, and render one world. `frameReady`/`gameUpdateDone` acknowledge one update. | Tick all instances before acknowledging the frame; render only the selected local instance unless a multi-view UI is deliberately added. |
| `GameContext`, `internal/engine/game_context.go:16-75` | Holds one `World`, `GameState`, `TimeControl`, reset channel, local input state, and terminal/view state. | Separate scheduler-global time/control from per-instance world state and per-view context. Avoid sharing mutable input/view state across participants. |
| `ConfigResource`, `internal/engine/resource.go:90-118` | Map dimensions and crop behavior coexist with viewport, camera, and color mode in every world. | Split Shared simulation config from Local view config (P3-8). |
| `ClockScheduler`, `internal/engine/clock_scheduler.go:20-96,124-153` | Owns one world, one queue router, one `fsm.Machine[*World]`, one telemetry set, and one reset/session/journal path. | Drive `settle -> FSM -> UpdateLocked` for each world in fixed instance order. Specify whether FSM is per world (as designed) and verify identical Shared transitions. |
| Tick body, `internal/engine/clock_scheduler.go:1014-1065` | Begins one queue tick, updates one TimeResource, settles one queue, then updates one FSM/world. | Stamp the same tick on all queues and execute the mandated fixed instance sequence without a goroutine race. |
| Reset/session, `internal/engine/clock_scheduler.go:1001-1006` | Advances one `RandResource` session and anchors one journal. | Advance/anchor every instance consistently from the same session generation. |
| `World`, `internal/engine/world.go:14-73` | One scalar `nextEntityID`, one component mask map, one lock, one origin, one component-store set, and one position grid. | Add two domain counters, entity-domain identity, ambient domain, and reset both counters. Keep one lock per world instance. |
| `Resource`, `internal/engine/resource.go:22-54` | One Time, Config, Game, Player roster/local selector, Event queue, Rand root, targeting/navigation/genetics/transient/status set, plus bridged services. | Mark each resource Shared, Player, Local-view, or service-global; do not shallow-share mutable resources between worlds. Resolve explosion-centre ownership per P3-12. |
| Player resource, `internal/engine/resource.go:134-224` | One roster plus one `Entity`/local slot selector lives inside the only world. | Each world duplicates the identical roster but selects its own local slot/entity. Shared systems use roster APIs, never `Entity`. |
| RNG resource, `internal/engine/resource.go:226-267` | Stream derivation is `(sessionRoot, label)` and each world would independently own/advance a session counter. | Derive `(sessionRoot, domain, label)` and coordinate session advancement across instances. |
| Component stores/position grid, `internal/engine/component_store_gen.go`, `internal/engine/position.go` | Stores point back to one world; position cells mix every entity and cap occupancy globally. | Preserve per-world stores while making enumeration/capacity domain-safe (P3-5). |
| Event queue, `internal/event/queue.go:9-74` | One MPSC queue has one sequence, stamp, journal, dispatch counters, and consumer. `GameEvent` has no domain. | Keep one queue per world, stamp ambient domain at push, and replicate only Shared/Bus records without merging live queues. |
| Journal schema, `internal/event/journal.go:10-64,83-159` | Schema 5 has no domain or roster slot; one journal is bound to one queue. | Bump schema; add domain and player slot metadata; retain merge key `(Run, Tick, Boundary, Seq)` and add the shared-record equality test. |
| Replay, `internal/app/replay.go:208-292` | `ReplayDriver` owns one `App`, advances one world, and injects groups into one queue. | Reconstruct the instance set, route each record by instance/domain/slot, and advance the shared scheduler once per tick. |
| Event routing, `internal/event/router.go`; scheduler registration | One router dispatches one queue to handlers bound to one world. | Retain a router per world or make world identity explicit; never dispatch one payload object concurrently to multiple mutable handlers. |
| Mode/input router, `internal/mode/router.go:34-74` | One context, input machine, macro state, undo ring, search history, mouse state, and fire cadence target one local cursor. | Bind the router to the selected local instance. Keep participant input history Local. |
| Render setup and frame, `internal/app/app.go:207-215`; `internal/render/orchestrator.go:14-76` | Renderers capture one `GameContext`; `RenderFrame` locks and draws one supplied world. | Construct/select renderer context for the active instance. The current single-world render call can remain once selection is explicit. |
| Services, `internal/service/hub.go:14-44`; `App.initServices/initWorld` | One process hub is bound once into one world's resources. | Keep I/O services process-global but expose per-instance adapters/ports; do not bind one mutable event or network endpoint blindly into every world. |
| Network, `internal/system/network.go:12-74`; `internal/network/inbound.go:3-17` | One `NetworkSystem` drains one port into one world; inbound records have no target instance/domain. App wiring remains marked TODO at `internal/app/app.go:191-192`. | Add instance/domain routing and a Shared/Bus replication envelope before enabling transport. Transport notifications remain Local. |
| FSM/config loading, `internal/app/app.go:245-254`; `internal/fsm` | One machine is loaded against one world and its event queue. | Load equivalent machine state per instance or define immutable shared config with per-world machine state; assert shared transition equality. |
| Telemetry/status | One status registry is displayed as the world/global truth. System metric keys are not instance-qualified. | Decide active-instance versus aggregate metrics and add stable instance/domain dimensions without changing simulation order. |
| Snapshot/digest/journal verification, `internal/engine/snapshot.go`; `internal/app/digest.go` | Snapshot and digest read one world's mixed state; replay verifies one output. | Add shared-only canonical snapshots/digests and compare them across instances after stripping Player records/state. |

## Phase 3 entry criteria from this audit

Before domain primitives are introduced, the design should explicitly answer:

1. Whether fuse and Player bullets remain, and if so which dedicated Bus events
   define their crossings.
2. Whether generic lifecycle/effect events are split or classified from ambient
   domain despite registry-level annotations.
3. Which cursor visual fields move to Local components versus which mutations
   become Bus requests against Shared cursor components.
4. How domain-aware position cells avoid local occupancy hiding Shared entities.
5. How one scheduler owns the fixed instance order while retaining one event
   queue, journal, RNG resource, and FSM state per world.

Those decisions are preparatory constraints. None requires Phase 2 to add
`Domain`, change entity encoding, or build the multi-world scheduler.
