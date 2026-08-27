package app

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lixenwraith/vi-fighter/internal/engine"
	"github.com/lixenwraith/vi-fighter/internal/event"
	"github.com/lixenwraith/vi-fighter/internal/fsm"
	"github.com/lixenwraith/vi-fighter/internal/manifest"
)

const systemConfigFixture = `
[systems]
disabled_systems = ["%s"]

[regions.main]
initial = "Start"

[states.Start]
parent = "Root"
`

func TestLoadFSMRejectsDisabledRequiredSystem(t *testing.T) {
	event.EnsureRegistry()
	w := engine.NewWorld()
	ctx := engine.NewGameContextWithClock(w, 80, 24, engine.NewManualClock())
	scheduler, _, _ := engine.NewClockScheduler(w, ctx.TimeCtl, time.Millisecond, make(chan struct{}))
	config := fstest.MapFS{
		"game.toml": &fstest.MapFile{Data: []byte(strings.Replace(systemConfigFixture, "%s", "cursor", 1))},
	}
	validate := func(machine *fsm.Machine[*engine.World]) error {
		_, err := validateSystems(machine)
		return err
	}

	err := scheduler.LoadFSMFromFS(fs.FS(config), "game.toml", manifest.RegisterFSMComponents, validate)
	if err == nil {
		t.Fatal("LoadFSMFromFS accepted a disabled required dependency")
	}
	if !strings.Contains(err.Error(), "failed to validate FSM") ||
		!strings.Contains(err.Error(), `system "camera" requires disabled system "cursor"`) {
		t.Fatalf("error does not identify the dependency: %v", err)
	}
}

func TestValidateSystemsReportsOptionalDependencyOnce(t *testing.T) {
	event.EnsureRegistry()
	machine := fsm.NewMachine[*engine.World]()
	manifest.RegisterFSMComponents(machine)
	config := []byte(strings.Replace(systemConfigFixture, "%s", "audio", 1))
	if err := machine.LoadConfig(config); err != nil {
		t.Fatal(err)
	}

	warnings, err := validateSystems(machine)
	if err != nil {
		t.Fatal(err)
	}
	want := `system "music" is enabled without optional dependency "audio"`
	count := 0
	for _, warning := range warnings {
		if strings.Contains(warning, want) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("music/audio warning count = %d, want 1; warnings = %v", count, warnings)
	}
}
