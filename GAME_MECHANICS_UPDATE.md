# Game Mechanics Update - Implementation Tracker

## Overview
Implementing new Heat/Energy/Shield/Drain mechanics per requirements.

## Phase Status
- [x] Phase 1: Constants & Components
- [ ] Phase 2: Shield Lifecycle
- [ ] Phase 3: Drain Spawning
- [ ] Phase 4: Collisions
- [ ] Phase 5: Shield Zone Protection
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
