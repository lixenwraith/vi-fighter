# Phase 6 implementation prompt

Use this prompt in a clean implementation session after the post-Phase 5 tuning PR
has merged.

---

Work in the `vi-fighter` repository. Read these files before changing code:

- `doc/multi-player-enhancement.md`
- `doc/domain-design.md`
- `doc/services-and-networking.md`
- `internal/app/snapshot_codec.go`
- `internal/app/snapshot_shared_state.go`
- `internal/app/snapshot_correction.go`
- `internal/app/correction.go`
- `internal/engine/snapshot_delta.go`
- `internal/system/network.go`

Implement Phase 6: **hash-guided selective correction plus bounded local replay**.

## Current baseline

- The game host owns the canonical Shared-domain world.
- Game guests keep simulating the Shared domain as predictors; do not turn them into
  thin clients.
- Every instance simulates only its own Player domain. D-13 owner-authored cursor
  state is transported separately, on its own sync stream. A capture does carry the
  components — a joiner has to materialise a cursor it has never held — but the
  receiver keeps its own values for the cursors it authors rather than adopting the
  sender's mirror, and no component of a shared entity may name a player-domain
  entity. Selective repair must not change either.
- Ordinary locally produced crossings apply immediately. Remote peers retain the
  receive-side playout lead. Correctly typed shared gold members disappear locally
  before a tick.
- Full captures and exact deltas already use a versioned, bounded deflate envelope.
  At the storm high water, the current wire is about 15.4 KiB per keyframe and
  7.1 KiB per delta, averaging 39.6 KiB/s at 5 Hz with one keyframe in ten.
- Host loss already leaves a guest running an explicit independent local fork with
  persistent `HOST LOST:LOCAL` status. Do not implement election or migration here.

Use the vocabulary **network peer**, **game host**, **game guest**, **Shared
domain** and **Player domain**. Keep role, topology, roster slot and simulation
domain separate. Preserve compatibility with possible relay peers and future
player-domain grouping, but do not implement those ideas.

## Deliverable 1: content-addressed selective correction

Replace steady transmission of whole keyframe/delta bodies with a protocol that
can prove equality cheaply and repair only mismatching state.

1. Define a deterministic, versioned correction manifest over the existing shared
   capture schema. Partition it into stable sections and bounded pages/shards.
   Prefer generated metadata from the snapshot/component manifest; do not create a
   second hand-maintained list of stores.
2. Compute stable hashes with explicit domain separation for manifest root,
   section and page. Hash schema/version, page identity, entity/order information
   and content so reordered or mixed-baseline state cannot compare equal.
3. Send a compact root summary first. If host and guest roots match for the named
   baseline, send no state body and record a hash-only correction.
4. On mismatch, descend through section/page hashes and request or send only the
   mismatching shards. Avoid an all-page hash list on every healthy correction if a
   bounded hierarchy can identify mismatches more cheaply.
5. Every shard must carry enough schema, capture/baseline tick, identity and proof
   information to be validated independently. Reject unknown versions, stale or
   mixed baselines, duplicate-conflicting pages, oversize bodies and invalid
   proofs without mutating the live world.
6. Reconcile only validated shards. A partial repair must not adopt the host tick
   for unrelated state or leave one logical object assembled from different
   baselines. Verify the resulting root before calling the repair successful.
7. Keep periodic compressed keyframes as a bounded recovery fallback for missing
   baselines, lost manifests, proof failure, excessive mismatch or maximum age.
   Supersession must remain safe: a newer repair may replace an older incomplete
   one, never combine with it accidentally.
8. Keep capture under the world lock to a bounded read. Manifest construction,
   hashing, diffing, proof work and compression must run outside the lock.

Do not implement blind one-fifth payload striping as the main protocol. It does not
identify what differs or prove convergence. A rotating coverage scheme is allowed
only if its integrity, maximum repair age and supersession behavior are explicit.

Add telemetry for at least:

- manifest/root bytes sent and received;
- hash-only corrections;
- section/page hashes compared;
- shards requested, sent, received, refused and applied;
- compressed shard and total wire bytes per peer;
- pages/entities/cells repaired;
- proof failures and mixed/stale baseline refusals;
- keyframe fallbacks and keyframe age;
- world-lock capture time versus outside-lock hash/encode time.

Feed actual selective-wire sizes into link pacing and admission. Do not price the
new protocol from the old whole-delta size.

## Deliverable 2: bounded rollback and replay

When a correction describes an earlier host tick, preserve accepted local actions
that happened after that baseline.

1. Retain one canonical suffix of locally produced, accepted D-3 crossings. Reuse
   journal/wire representations where that avoids two definitions of the same
   action.
2. Bound retention by tick span, record count and encoded bytes. Publish overflow
   and unavailable-suffix telemetry.
3. After installing or selectively reconciling host state, replay only this game
   guest's local suffix after the correction baseline, in original deterministic
   order, up to the former predicted present.
4. Do not replay remote artifacts, Shared-derived events, roster creation/removal,
   or a reset from a previous run/session.
5. Deduplicate against artifacts already represented by the correction. Rewards,
   entity creation, gold credit and D-13 owner-state changes must happen exactly
   once.
6. If the canonical suffix is incomplete, fall back safely to the authoritative
   correction and report that local replay was skipped. Never guess.
7. Preserve immediate local cursor response and immediate visual removal of every
   correctly typed gold member.

## Required tests

Add focused tests that prove all of the following:

1. Equal host/guest roots produce only compact hash traffic and no state shard.
2. A single injected component/page mismatch requests and applies only that shard,
   then restores the exact root.
3. Several mismatches in different sections are repaired without sending an
   unrelated section.
4. Player-domain state never appears in manifests, and neither it nor a
   receiver-authored cursor's D-13 owner-authored set changes during selective
   apply — including when the hashed surface disagrees over those cells for good.
5. Reordered entity/store data cannot pass the integrity proof.
6. Corrupt, stale, unknown-version, duplicate-conflicting and oversize shards are
   refused atomically.
7. Loss and supersession reach a bounded compressed keyframe fallback and cannot
   assemble mixed-baseline state.
8. Link pacing and admission use measured manifest/shard wire bytes.
9. Local crossings after a correction baseline survive replay exactly once,
   including rapid cursor movement and a complete gold sequence without an
   intervening tick.
10. A missing replay suffix falls back to authoritative state without corruption.
11. Existing join, reconnect, correction, mesh, replay and deterministic genetic
    continuation tests remain green.
12. A guest continues ticking after host loss and keeps the persistent local-fork
    indicator; no election or migration is introduced.

Extend the storm measurement to report plain schema bytes, compressed full/delta
bytes, manifest/hash bytes, selective shard bytes and projected 2 Hz/5 Hz totals.
Performance values should be logged, not asserted against wall-clock thresholds.
Correctness tests may assert a conservative material byte reduction on the fixed
storm fixture.

Run:

```sh
go generate ./internal/event ./internal/manifest
go test ./...
go vet ./...
```

Also run the two-process shaped-link scripts when the environment permits and
record the commands and observed wire totals in the PR.

## Explicit non-goals

- authentication, TLS or hostile-peer identity;
- coordinated host migration, election or partition merging;
- tower ownership/config changes;
- relay-peer roles, a multi-link CLI or nested Player domains;
- adaptive playout lead;
- replacing guest simulation with host-only simulation.

Document the final protocol and invariants as the code stands. Keep historical
debugging narratives out of the architecture documents; the PR description and
git history hold the implementation chronology.

---
