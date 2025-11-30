# Game Mechanics Update - Implementation Tracker

## Overview
Implementing new Heat/Energy/Shield/Drain mechanics per requirements.

## Phase Status
- [x] Phase 1: Constants & Components
- [x] Phase 2: Shield Lifecycle
- [x] Phase 3: Drain Spawning
- [x] Phase 4: Collisions
- [x] Phase 5: Shield Zone Protection
- [ ] Phase 6: Passive Drain & Cleanup

## Key Mechanics Summary
- Heat: 0-100, controls drain count via floor(Heat/10)
- Energy: Can go negative, used for Shield defense
- Shield: Active when Sources != 0 AND Energy > 0
- Drains: Spawn based on Heat, despawn on Heat drop, cursor collision, or drain-drain collision

## Constants Added (Phase 1)
- DrainShieldEnergyDrainAmount = 100 (per tick per drain in shield)
- DrainHeatReductionAmount = 10 (unshielded cursor collision)
- ShieldPassiveDrainAmount = 1 (per second while active)
- ShieldPassiveDrainInterval = 1s
- ShieldSourceBoost = 1 << 0 (bitmask flag)

## Component Changes (Phase 1)
- ShieldComponent.Sources uint8 added (bitmask for activation sources)

## Shield Lifecycle (Phase 2)
- BoostSystem now sets/clears ShieldSourceBoost in Sources bitmask
- Shield component persists; only Sources field changes
- ShieldRenderer checks IsShieldActive() before rendering
- IsShieldActive = Sources != 0 AND Energy > 0

## Drain Spawning (Phase 3)
- Target drain count = floor(Heat / 10), max 10
- Removed energy <= 0 despawn from main Update loop
- Spawn position validation: skip cells with existing drain
- Energy-based despawn moved to Phase 6 (conditional on !ShieldActive)

## Collisions (Phase 4)
- Drain-Drain: If multiple drains at same cell, all involved despawn with flash
- Drain-Cursor (No Shield): -10 Heat, drain despawns
- Drain-Cursor (Shield Active): Energy drain only, no heat loss, drain persists

## Shield Zone Protection (Phase 5)
- Drains inside shield ellipse (not just on cursor) drain 100 energy per interval
- Ellipse check: (dx/rx)^2 + (dy/ry)^2 <= 1
- Energy drain applies to ALL drains in shield, including those on cursor
