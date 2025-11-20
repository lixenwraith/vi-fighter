package components

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func TestPositionComponent(t *testing.T) {
	tests := []struct {
		name string
		x, y int
	}{
		{"Origin", 0, 0},
		{"Positive values", 10, 20},
		{"Negative values", -5, -10},
		{"Mixed values", -3, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := PositionComponent{X: tt.x, Y: tt.y}
			if pos.X != tt.x {
				t.Errorf("Expected X to be %d, got %d", tt.x, pos.X)
			}
			if pos.Y != tt.y {
				t.Errorf("Expected Y to be %d, got %d", tt.y, pos.Y)
			}
		})
	}
}

func TestCharacterComponent(t *testing.T) {
	tests := []struct {
		name  string
		rune  rune
		style tcell.Style
	}{
		{"Letter", 'a', tcell.StyleDefault},
		{"Number", '5', tcell.StyleDefault.Foreground(tcell.ColorRed)},
		{"Special char", '@', tcell.StyleDefault.Bold(true)},
		{"Unicode", '🎮', tcell.StyleDefault.Background(tcell.ColorBlue)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			char := CharacterComponent{Rune: tt.rune, Style: tt.style}
			if char.Rune != tt.rune {
				t.Errorf("Expected Rune to be %v, got %v", tt.rune, char.Rune)
			}
			// Style comparison is complex, just verify it's set
			if char.Style != tt.style {
				t.Errorf("Expected Style to match")
			}
		})
	}
}

func TestSequenceComponent(t *testing.T) {
	tests := []struct {
		name  string
		id    int
		index int
		sType SequenceType
		level SequenceLevel
	}{
		{"Green dark sequence", 1, 0, SequenceGreen, LevelDark},
		{"Red normal sequence", 2, 5, SequenceRed, LevelNormal},
		{"Blue bright sequence", 3, 9, SequenceBlue, LevelBright},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := SequenceComponent{
				ID:    tt.id,
				Index: tt.index,
				Type:  tt.sType,
				Level: tt.level,
			}
			if seq.ID != tt.id {
				t.Errorf("Expected ID to be %d, got %d", tt.id, seq.ID)
			}
			if seq.Index != tt.index {
				t.Errorf("Expected Index to be %d, got %d", tt.index, seq.Index)
			}
			if seq.Type != tt.sType {
				t.Errorf("Expected Type to be %v, got %v", tt.sType, seq.Type)
			}
			if seq.Level != tt.level {
				t.Errorf("Expected Level to be %v, got %v", tt.level, seq.Level)
			}
		})
	}
}

func TestSequenceTypeValues(t *testing.T) {
	if SequenceGreen != 0 {
		t.Errorf("Expected SequenceGreen to be 0, got %d", SequenceGreen)
	}
	if SequenceRed != 1 {
		t.Errorf("Expected SequenceRed to be 1, got %d", SequenceRed)
	}
	if SequenceBlue != 2 {
		t.Errorf("Expected SequenceBlue to be 2, got %d", SequenceBlue)
	}
}

func TestSequenceLevelValues(t *testing.T) {
	if LevelDark != 0 {
		t.Errorf("Expected LevelDark to be 0, got %d", LevelDark)
	}
	if LevelNormal != 1 {
		t.Errorf("Expected LevelNormal to be 1, got %d", LevelNormal)
	}
	if LevelBright != 2 {
		t.Errorf("Expected LevelBright to be 2, got %d", LevelBright)
	}
}

func TestDrainComponent(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name          string
		x, y          int
		lastMoveTime  time.Time
		lastDrainTime time.Time
		isActive      bool
	}{
		{"Initial state at origin", 0, 0, now, now, true},
		{"Active at position", 10, 20, now, now.Add(-100 * time.Millisecond), true},
		{"Inactive state", 5, 15, now.Add(-1 * time.Second), now, false},
		{"Negative position", -3, -7, now, now, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drain := DrainComponent{
				X:             tt.x,
				Y:             tt.y,
				LastMoveTime:  tt.lastMoveTime,
				LastDrainTime: tt.lastDrainTime,
				IsActive:      tt.isActive,
			}
			if drain.X != tt.x {
				t.Errorf("Expected X to be %d, got %d", tt.x, drain.X)
			}
			if drain.Y != tt.y {
				t.Errorf("Expected Y to be %d, got %d", tt.y, drain.Y)
			}
			if !drain.LastMoveTime.Equal(tt.lastMoveTime) {
				t.Errorf("Expected LastMoveTime to be %v, got %v", tt.lastMoveTime, drain.LastMoveTime)
			}
			if !drain.LastDrainTime.Equal(tt.lastDrainTime) {
				t.Errorf("Expected LastDrainTime to be %v, got %v", tt.lastDrainTime, drain.LastDrainTime)
			}
			if drain.IsActive != tt.isActive {
				t.Errorf("Expected IsActive to be %v, got %v", tt.isActive, drain.IsActive)
			}
		})
	}
}

func TestDrainComponentCreation(t *testing.T) {
	// Test creating a drain component with zero values
	drain := DrainComponent{}
	if drain.X != 0 {
		t.Errorf("Expected default X to be 0, got %d", drain.X)
	}
	if drain.Y != 0 {
		t.Errorf("Expected default Y to be 0, got %d", drain.Y)
	}
	if !drain.LastMoveTime.IsZero() {
		t.Errorf("Expected default LastMoveTime to be zero time")
	}
	if !drain.LastDrainTime.IsZero() {
		t.Errorf("Expected default LastDrainTime to be zero time")
	}
	if drain.IsActive {
		t.Errorf("Expected default IsActive to be false, got %v", drain.IsActive)
	}
}
