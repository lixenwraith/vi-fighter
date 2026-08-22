package system

import (
	"testing"

	"github.com/lixenwraith/vi-fighter/internal/component"
	"github.com/lixenwraith/vi-fighter/internal/core"
	"github.com/lixenwraith/vi-fighter/internal/engine"
	"github.com/lixenwraith/vi-fighter/internal/event"
	"github.com/lixenwraith/vi-fighter/internal/parameter"
	"github.com/lixenwraith/vi-fighter/pkg/vmath"
	"github.com/lixenwraith/vi-fighter/pkg/vmath/physics"
)

func addDustExplosionTestUnit(
	w *engine.World,
	combatType component.CombatEntityType,
	x, y int,
) (core.Entity, core.Entity) {
	header := w.CreateEntity()
	member := w.CreateEntity()
	w.Positions.SetPosition(header, component.PositionComponent{X: x, Y: y})
	w.Positions.SetPosition(member, component.PositionComponent{X: x, Y: y})
	w.Components.Header.SetComponent(header, component.HeaderComponent{
		Behavior:      component.BehaviorQuasar,
		Type:          component.CompositeTypeUnit,
		MemberEntries: []component.MemberEntry{{Entity: member}},
	})
	w.Components.Member.SetComponent(member, component.MemberComponent{HeaderEntity: header})
	w.Components.Combat.SetComponent(header, component.CombatComponent{
		OwnerEntity:      header,
		CombatEntityType: combatType,
		HitPoints:        10,
	})
	preciseX, preciseY := vmath.Point{X: x, Y: y}.CenterF()
	w.Components.Kinetic.SetComponent(header, component.KineticComponent{
		Kinetic: physics.Kinetic{PreciseX: preciseX, PreciseY: preciseY},
	})
	return header, member
}

func TestDustCollisionContextContainsOnlyPlayerSpecies(t *testing.T) {
	w, _, _ := testCursorWorld(t)
	dust := NewDustSystem(w).(*DustSystem)

	drain := w.CreateEntity()
	w.Components.Drain.SetComponent(drain, component.DrainComponent{})
	w.Positions.SetPosition(drain, component.PositionComponent{X: 7, Y: 5})
	_, member := addDustExplosionTestUnit(w, component.CombatEntityQuasar, 9, 5)

	ctx := dust.buildCollisionContext()
	if ctx.cellFlags[posKey(7, 5)]&cellFlagDrain == 0 {
		t.Fatal("drain was omitted from dust's local collision context")
	}
	memberPos, _ := w.Positions.GetPosition(member)
	if _, ok := ctx.cellFlags[posKey(memberPos.X, memberPos.Y)]; ok {
		t.Fatal("shared composite member leaked into dust's collision context")
	}
}

func TestDustExplosionUsesDedicatedBusBeforeSharedCombat(t *testing.T) {
	w, cursor, _ := testCursorWorld(t)
	header, _ := addDustExplosionTestUnit(w, component.CombatEntityQuasar, 8, 5)

	dustEntity := w.CreateEntity()
	w.Components.Dust.SetComponent(dustEntity, component.DustComponent{})
	w.Positions.SetPosition(dustEntity, component.PositionComponent{X: 5, Y: 5})

	explosion := NewExplosionSystem(w).(*ExplosionSystem)
	explosion.fireFromDust(cursor)

	var busEvent event.GameEvent
	for _, ev := range w.Resources.Event.Queue.Consume() {
		switch ev.Type {
		case event.EventDustExplosionRequest:
			busEvent = ev
		case event.EventCombatAttackAreaRequest:
			p, _ := ev.Payload.(*event.CombatAttackAreaRequestPayload)
			if p != nil && p.TargetEntity == header {
				t.Fatal("dust explosion emitted a shared combat target before the bus crossing")
			}
		}
	}
	if busEvent.Type != event.EventDustExplosionRequest {
		t.Fatal("dust explosion did not emit its dedicated bus event")
	}
	payload, ok := busEvent.Payload.(*event.DustExplosionRequestPayload)
	if !ok {
		t.Fatalf("bus payload = %T, want *event.DustExplosionRequestPayload", busEvent.Payload)
	}
	if payload.OwnerCursor != cursor || payload.CenterX != 5 || payload.CenterY != 5 ||
		payload.Radius != parameter.ExplosionFieldRadius || payload.AttackType != component.CombatAttackExplosion {
		t.Fatalf("bus payload = %#v", payload)
	}

	combat := NewCombatSystem(w).(*CombatSystem)
	combat.HandleEvent(busEvent)

	areaEvents := 0
	for _, ev := range w.Resources.Event.Queue.Consume() {
		if ev.Type != event.EventCombatAttackAreaRequest {
			continue
		}
		p, ok := ev.Payload.(*event.CombatAttackAreaRequestPayload)
		if !ok || p.TargetEntity != header {
			continue
		}
		areaEvents++
		combat.HandleEvent(ev)
	}
	if areaEvents != 1 {
		t.Fatalf("shared area events = %d, want 1", areaEvents)
	}

	combatComp, _ := w.Components.Combat.GetComponent(header)
	if combatComp.HitPoints != 10-parameter.CombatDamageExplosion {
		t.Fatalf("shared hit points = %d, want %d", combatComp.HitPoints, 10-parameter.CombatDamageExplosion)
	}
	kinetic, _ := w.Components.Kinetic.GetComponent(header)
	if kinetic.VelX == 0 && kinetic.VelY == 0 {
		t.Fatal("shared target received damage without explosion kinetic")
	}
}

func TestSharedMissileExplosionIgnoresPlayerDrain(t *testing.T) {
	w, cursor, _ := testCursorWorld(t)
	drain := w.CreateEntity()
	w.Components.Drain.SetComponent(drain, component.DrainComponent{})
	w.Components.Combat.SetComponent(drain, component.CombatComponent{
		OwnerEntity:      drain,
		CombatEntityType: component.CombatEntityDrain,
		HitPoints:        10,
	})
	w.Positions.SetPosition(drain, component.PositionComponent{X: 5, Y: 5})

	explosion := NewExplosionSystem(w).(*ExplosionSystem)
	explosion.processExplosionArea(cursor, 5, 5, parameter.MissileExplosionRadius, event.ExplosionTypeMissile)

	for _, ev := range w.Resources.Event.Queue.Consume() {
		if ev.Type != event.EventCombatAttackAreaRequest {
			continue
		}
		p, _ := ev.Payload.(*event.CombatAttackAreaRequestPayload)
		if p != nil && p.TargetEntity == drain {
			t.Fatal("shared missile explosion emitted combat against a Player drain")
		}
	}
}
