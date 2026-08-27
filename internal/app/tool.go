package app

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/lixenwraith/vi-fighter/internal/asset"
	"github.com/lixenwraith/vi-fighter/internal/engine"
	"github.com/lixenwraith/vi-fighter/internal/event"
	"github.com/lixenwraith/vi-fighter/internal/fsm"
	"github.com/lixenwraith/vi-fighter/internal/fsm/std"
	"github.com/lixenwraith/vi-fighter/internal/manifest"
	"github.com/lixenwraith/vi-fighter/internal/service"
)

// schemaVersion is the FSM schema contract version consumed by the map editor
const schemaVersion = 1

// Check validates the resolved FSM config and content corpus without starting the game
func Check(cfg Config, w io.Writer) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	event.EnsureRegistry()

	if err := checkFSM(cfg, w); err != nil {
		return err
	}
	return checkContent(cfg, w)
}

// checkFSM loads the resolved FSM config and reports its source
func checkFSM(cfg Config, w io.Writer) error {
	m := fsm.NewMachine[*engine.World]()
	manifest.RegisterFSMComponents(m)

	path, err := ResolveGameConfig(cfg)
	if err != nil {
		return err
	}
	if path == "" {
		if err := fsm.LoadConfigFromFS(m, asset.DefaultFSMConfig, asset.DefaultFSMEntry); err != nil {
			return err
		}
		fmt.Fprintln(w, "config ok: embedded default")
		return checkSystems(m, w)
	}
	if err := fsm.LoadConfigFromPath(m, path); err != nil {
		return err
	}
	fmt.Fprintln(w, "config ok:", path)
	return checkSystems(m, w)
}

// checkSystems validates system references and dependency states.
func checkSystems(m *fsm.Machine[*engine.World], w io.Writer) error {
	warnings, err := validateSystems(m)
	if err != nil {
		return err
	}
	for _, warning := range warnings {
		fmt.Fprintln(w, "warning:", warning)
	}
	fmt.Fprintln(w, "systems ok")
	return nil
}

// validateSystems checks names and required dependencies for every config scope.
func validateSystems(m *fsm.Machine[*engine.World]) ([]string, error) {
	profiles := make(map[string]systemDefView, len(manifest.Systems))
	names := make([]string, 0, len(manifest.Systems))
	for _, def := range manifest.Systems {
		profiles[def.Name] = systemDefView{Required: def.Required, Optional: def.Optional}
		names = append(names, def.Name)
	}
	sort.Strings(names)

	var unknown []string
	check := func(where string, names []string) {
		for _, name := range names {
			if _, exists := profiles[name]; !exists {
				unknown = append(unknown, where+": "+name)
			}
		}
	}
	if sc := m.GetSystemsConfig(); sc != nil {
		check("[systems]", sc.DisabledSystems)
	}
	for _, r := range m.DeclaredRegions() {
		cfg := m.GetRegionConfig(r)
		if cfg == nil {
			continue
		}
		check("region "+r, cfg.EnabledSystems)
		check("region "+r, cfg.DisabledSystems)
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("unknown system names:\n  %s", strings.Join(unknown, "\n  "))
	}

	rootDisabled := make(map[string]bool)
	if cfg := m.GetSystemsConfig(); cfg != nil {
		for _, name := range cfg.DisabledSystems {
			rootDisabled[name] = true
		}
	}

	warned := make(map[string]bool)
	warnings, issues := checkSystemState("[systems]", names, profiles, rootDisabled, warned)
	if len(issues) > 0 {
		return warnings, systemConfigError(issues)
	}

	for _, region := range m.DeclaredRegions() {
		disabled := make(map[string]bool, len(rootDisabled))
		for _, name := range names {
			if rootDisabled[name] {
				disabled[name] = true
			}
		}
		cfg := m.GetRegionConfig(region)
		if cfg != nil {
			for _, name := range cfg.DisabledSystems {
				disabled[name] = true
			}
			for _, name := range cfg.EnabledSystems {
				delete(disabled, name)
			}
		}

		stateWarnings, stateIssues := checkSystemState("region "+region, names, profiles, disabled, warned)
		warnings = append(warnings, stateWarnings...)
		issues = append(issues, stateIssues...)
	}
	if len(issues) > 0 {
		return warnings, systemConfigError(issues)
	}
	return warnings, nil
}

type systemDefView struct {
	Required []string
	Optional []string
}

// checkSystemState reports enabled dependents whose dependencies are disabled.
func checkSystemState(scope string, names []string, profiles map[string]systemDefView, disabled map[string]bool, warned map[string]bool) ([]string, []string) {
	var warnings, issues []string
	for _, name := range names {
		if disabled[name] {
			continue
		}
		profile := profiles[name]
		for _, dependency := range profile.Required {
			if disabled[dependency] {
				issues = append(issues, fmt.Sprintf("%s: system %q requires disabled system %q", scope, name, dependency))
			}
		}
		for _, dependency := range profile.Optional {
			key := name + "\x00" + dependency
			if disabled[dependency] && !warned[key] {
				warnings = append(warnings, fmt.Sprintf("%s: system %q is enabled without optional dependency %q", scope, name, dependency))
				warned[key] = true
			}
		}
	}
	return warnings, issues
}

func systemConfigError(issues []string) error {
	return fmt.Errorf("invalid system configuration:\n  %s", strings.Join(issues, "\n  "))
}

// checkContent loads the corpus and reports accepted and rejected files
func checkContent(cfg Config, w io.Writer) error {
	src, err := ResolveContent(cfg)
	if err != nil {
		return fmt.Errorf("content path: %w", err)
	}

	svc := service.NewContentService(src, 0) // validation only; block order is irrelevant
	if err := svc.Init(); err != nil {
		return err
	}

	c := svc.Corpus()
	fmt.Fprintf(w, "content ok: %s (%d files, %d blocks, %d lines)\n",
		svc.Label(), len(c.Sources), c.BlockCount(), c.LineCount())

	for _, s := range c.Sources {
		fmt.Fprintf(w, "  ok    %-32s %4d blocks %6d lines\n", s.Name, len(s.Blocks), s.Lines)
	}
	for _, r := range c.Rejected {
		fmt.Fprintf(w, "  skip  %-32s %s\n", r.Name, r.Reason)
	}
	return nil
}

// Schema writes the machine schema as JSON for the map editor
// Requires no terminal, services, or World instance
func Schema(w io.Writer) error {
	event.EnsureRegistry()

	m := fsm.NewMachine[*engine.World]()
	manifest.RegisterFSMComponents(m)

	type field struct {
		Name   string `json:"name"`    // toml tag (authoring name)
		GoName string `json:"go_name"` // reflection fallback name
		Type   string `json:"type"`
	}
	type eventSchema struct {
		Name   string  `json:"name"`
		Fields []field `json:"fields,omitempty"`
	}
	schema := struct {
		SchemaVersion    int           `json:"schema_version"`
		Events           []eventSchema `json:"events"`
		Guards           []string      `json:"guards"`
		Actions          []string      `json:"actions"`
		Ops              []string      `json:"ops"`
		ConfigIntFields  []string      `json:"config_int_fields"`
		ConfigBoolFields []string      `json:"config_bool_fields"`
	}{
		SchemaVersion:    schemaVersion,
		Guards:           m.RegisteredGuards(),
		Actions:          m.RegisteredActions(),
		Ops:              std.Ops(),
		ConfigIntFields:  engine.ConfigIntFields(),
		ConfigBoolFields: engine.ConfigBoolFields(),
	}

	event.RangeEvents(func(name string, et event.EventType, payload any) {
		es := eventSchema{Name: name}
		if payload != nil {
			t := reflect.TypeOf(payload)
			if t.Kind() == reflect.Ptr {
				t = t.Elem()
			}
			for f := range t.Fields() {
				tag := f.Tag.Get("toml")
				n := f.Name
				if tag != "" && tag != "-" {
					if idx := strings.Index(tag, ","); idx >= 0 {
						tag = tag[:idx]
					}
					n = tag
				}
				es.Fields = append(es.Fields, field{Name: n, GoName: f.Name, Type: f.Type.String()})
			}
		}
		schema.Events = append(schema.Events, es)
	})
	sort.Slice(schema.Events, func(i, j int) bool { return schema.Events[i].Name < schema.Events[j].Name })

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(schema)
}
