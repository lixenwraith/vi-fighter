# Gap closure brief

A working brief for the session that closes the open items in
[multi-player-enhancement.md](multi-player-enhancement.md) §8. Delete it when
they are closed.

Written 2026-09-07, after the §8 review. Gap 1 is deferred by decision and is not
in scope. Gap 10 was reworded rather than dropped and needs no code. Read §8 for
what each item actually is; this file says what to do about it and in what order.

## Ground rules

- The code is the source of truth. The docs were audited on 2026-09-07 and should
  agree; where they do not, fix the doc in the same commit.
- Domain rules D-1..D-24 in [domain-design.md](domain-design.md) are binding.
- Every behavioural change lands with a test that fails without it. Prefer one
  test that proves the rule over several that exercise the path.
- Full gates before any push: `go generate ./internal/event ./internal/manifest`,
  `go build ./...`, `go test ./...`, `go test -race ./internal/app/...`,
  `go vet ./...`, `gofmt -l` on changed files, and `test/scenario.sh all`.

## Order of work

### 1. Gap 6 — guard the embedder's shared-domain mutations

Smallest, wholly determined, and it removes a class of silent divergence.

`App.SetupLevel` and `App.Region` (`internal/app/headless.go`) push `ClassShared`
events — `EventLevelSetup` and `EventFSMRegionRequest` — with
`PushEventOrigin(..., event.OriginDebug)`, which is a local push. On a guest in a
live session that changes this instance's map bounds or FSM regions and nobody
else's.

`App.Reset` in the same file already does the right thing: it checks
`world.LiveSession()`, refuses with a status message when
`!world.IsSessionCoordinator()`, and publishes a crossing otherwise. Give both
methods that shape. Then check the rest of the exported `App` surface for the
same pattern — anything that pushes a `ClassShared` event locally.

Test: a two-instance harness where the guest calls each method and neither world
changes, and where the authority calls it and both do.

### 2. Gap 2 — wire the playout lead, then close it as an issue

The plumbing is complete: `BarrierDelayTicks` travels in `SessionOffer`, survives
in `HandoffRecord`, reaches `NetworkResource` and sets `NetworkSystem.delayTicks`.
Only the *choice* is missing — `internal/app/host.go` and `internal/app/session.go`
both write `parameter.NetworkBarrierDelayTicks` and nothing else ever writes it.

The link probe (`MsgLinkProbe`/`MsgLinkEcho`) already measures round trips. Derive
the session's lead from the worst measured round trip at lobby close, clamp it to
a sane range, and let a relayed peer's hop count raise it. Keep the constant as
the floor and the default.

Confirm the problem first, so the fix has a before and an after:

```sh
tc qdisc add dev lo root netem delay 100ms     # 200 ms RTT, past the 150 ms lead
./bin/vif -serve :7777 -size 120x40 -l -lv info -lt 50
./bin/vif -join 127.0.0.1:7777 -l -lv info -lt 50
tc qdisc del dev lo root
```

`network.barrier_late` climbs on the host, `network.lag_ticks` rises on the guest
and `network.stale` latches. All three are zero on an undelayed local session.

This is a tracked issue rather than an architectural gap: raise it as one, fix it,
and drop the entry from §8.

### 3. Gaps 3, 4, 5 — reachability under `-authority migrate`

One gap seen from three sides. The design is written up in
[multi-player-enhancement.md](multi-player-enhancement.md) §5.3 and should be
read in full before any code.

**Settle §5.3's four open decisions with the user before starting.** They change
the shape of the work: mesh shape, what a participant that declines advertisement
may do, the default listen port, and whether a departed guest keeps the map for
rejoin.

Then, in this order:

1. A guest binds a listening port. `-listen <addr>` pins one; the default is the
   host's port, falling back to an OS-assigned one, declaring what was actually
   bound. A bind that fails entirely leaves the participant a leaf, not an error.
2. The host's bind-confirmation dial. Until the round trip succeeds, the address
   is declared and not confirmed, and only confirmed addresses are published.
3. The five-second warning on the guest, and the host's five-second hold after
   confirmation. Quitting inside the window publishes nothing.
4. `MsgPeerList` (0x20, reserved and unused) carries the map. It rides the
   authority's term and is refused below the term the receiver holds.
5. The map is a **separate table keyed by identity**, carried beside the roster in
   the offer, the handoff and the broadcast. It must not become a field on
   `SessionParticipant`: `SameRoster` compares that struct by value through
   `slices.Equal`, and `HandoffRecord.Validate` refuses a roster that is not
   byte-identical, so an address change would break every handoff.
6. Peers dial from the map; the lower identity dials the higher, so a pair opens
   one link.
7. `DesignatedSuccessor` gains the map: the lowest surviving identity **that the
   map confirms**. A local dial failure must never be an input — the split-brain
   rule needs every survivor to compute the same answer from replicated state.

Tests worth having: a three-participant mesh that survives losing the authority
and then the successor; a roster whose lowest identity is unconfirmed, skipped in
favour of the next; a handoff that still validates after a peer rebound its port.

### 4. Gap 7 — domain-boundary debt

Long outstanding, so do it in the cheapest order rather than as a redesign:

1. Make every exemption explicit at its site — ambient-local stamping,
   `event.EmitDeath`'s direct queue path in `internal/event/api.go`, route-anchor
   casts. A named, commented exemption is a boundary; an unremarked one is debt.
2. Delete the ones that turn out to be unnecessary once named.
3. Only then consider splitting the mixed combat telemetry. `combat.` and
   `kills.` are on `deniedSharedKey`'s prefix list in
   `internal/snapshot/surface.go` precisely because they aggregate both domains;
   splitting them per domain is what would let them back into the compared
   surface, and that is a bigger change than the first two steps.

### 5. Gap 8 — tower ownership

**Blocked on a decision.** A tower's `CombatComponent.OwnerEntity` is whatever
cursor the spawn request named, and that comes from the FSM's `player_entity`
capture variable — one cursor for the whole machine. Attribution in
`CombatSystem.applyHitDirect` and its area sibling then credits one participant
for a structure the session shares.

Two shapes, for the user to choose:

- a hybrid shared/local ownership like the cursor and its shield, or
- an explicit "every player" ownership value that attribution expands at the
  point of use.

Do not start until that is settled. The drain half of the old gap 8 was wrong and
has been removed: `kills.drain` gating quasar escalation is correct behaviour.

## Not in scope

- **Gap 1**, deferred by decision.
- **Gap 9.** The correctness half is closed by the D-14 map latch; what remains is
  a windowed composite and optional presentation interpolation, which are
  presentation work independent of simulation ordering.
- **Gap 10.** Reworded. Optionally, stop `network.digest_mismatches` from being
  tripped by one-ULP float differences on a mixed-platform session — a diagnostic
  refinement, not a simulation fix.
