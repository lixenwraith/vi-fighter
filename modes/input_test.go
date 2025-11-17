package modes

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/lixenwraith/vi-fighter/engine"
	"github.com/lixenwraith/vi-fighter/systems"
)

// Mock screen for testing
type mockScreen struct {
	tcell.Screen
}

func (m *mockScreen) Size() (int, int) {
	return 80, 24
}

// Test DeleteOperator is reset on mode transitions
func TestDeleteOperatorResetOnModeTransition(t *testing.T) {
	// Create a mock screen
	screen := &mockScreen{}
	ctx := engine.NewGameContext(screen)
	scoreSystem := systems.NewScoreSystem(ctx)
	handler := NewInputHandler(ctx, scoreSystem)

	// Set DeleteOperator to true
	ctx.DeleteOperator = true
	ctx.Mode = engine.ModeNormal

	// Simulate pressing 'i' to enter insert mode
	ev := tcell.NewEventKey(tcell.KeyRune, 'i', tcell.ModNone)
	handler.handleKeyEvent(ev)

	// Verify DeleteOperator was reset
	if ctx.DeleteOperator {
		t.Error("DeleteOperator should be reset when entering insert mode")
	}
	if ctx.Mode != engine.ModeInsert {
		t.Error("Should be in insert mode")
	}
}

// Test DeleteOperator reset when entering search mode
func TestDeleteOperatorResetOnSearchMode(t *testing.T) {
	screen := &mockScreen{}
	ctx := engine.NewGameContext(screen)
	scoreSystem := systems.NewScoreSystem(ctx)
	handler := NewInputHandler(ctx, scoreSystem)

	// Set DeleteOperator to true
	ctx.DeleteOperator = true
	ctx.Mode = engine.ModeNormal

	// Simulate pressing '/' to enter search mode
	ev := tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone)
	handler.handleKeyEvent(ev)

	// Verify DeleteOperator was reset
	if ctx.DeleteOperator {
		t.Error("DeleteOperator should be reset when entering search mode")
	}
	if ctx.Mode != engine.ModeSearch {
		t.Error("Should be in search mode")
	}
}

// Test DeleteOperator reset when pressing Escape from insert mode
func TestDeleteOperatorResetOnEscapeFromInsert(t *testing.T) {
	screen := &mockScreen{}
	ctx := engine.NewGameContext(screen)
	scoreSystem := systems.NewScoreSystem(ctx)
	handler := NewInputHandler(ctx, scoreSystem)

	// Enter insert mode and set DeleteOperator
	ctx.Mode = engine.ModeInsert
	ctx.DeleteOperator = true

	// Simulate pressing Escape
	ev := tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
	handler.handleKeyEvent(ev)

	// Verify DeleteOperator was reset and back to normal mode
	if ctx.DeleteOperator {
		t.Error("DeleteOperator should be reset when escaping from insert mode")
	}
	if ctx.Mode != engine.ModeNormal {
		t.Error("Should be in normal mode after escape")
	}
}

// Test DeleteOperator reset when pressing Escape from search mode
func TestDeleteOperatorResetOnEscapeFromSearch(t *testing.T) {
	screen := &mockScreen{}
	ctx := engine.NewGameContext(screen)
	scoreSystem := systems.NewScoreSystem(ctx)
	handler := NewInputHandler(ctx, scoreSystem)

	// Enter search mode and set DeleteOperator
	ctx.Mode = engine.ModeSearch
	ctx.DeleteOperator = true

	// Simulate pressing Escape
	ev := tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
	handler.handleKeyEvent(ev)

	// Verify DeleteOperator was reset and back to normal mode
	if ctx.DeleteOperator {
		t.Error("DeleteOperator should be reset when escaping from search mode")
	}
	if ctx.Mode != engine.ModeNormal {
		t.Error("Should be in normal mode after escape")
	}
}
