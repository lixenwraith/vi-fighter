package manifest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/lixenwraith/vi-fighter/internal/engine"
	"github.com/lixenwraith/vi-fighter/internal/system"
)

func TestSystemDomainsMatchManifest(t *testing.T) {
	w := engine.NewWorld()
	engine.NewGameContextWithClock(w, 80, 24, engine.NewManualClock())
	systems, err := BuildSystems(w)
	if err != nil {
		t.Fatal(err)
	}
	if w.Resources.Genetics != nil {
		defer w.Resources.Genetics.Registry.Stop()
	}

	if len(systems) != len(Systems) {
		t.Fatalf("BuildSystems returned %d systems, manifest declares %d", len(systems), len(Systems))
	}
	for i, got := range systems {
		want := Systems[i]
		if got.Name() != want.Name {
			t.Errorf("system %d name = %q, want %q", i, got.Name(), want.Name)
		}
		if got.Domain() != want.Domain {
			t.Errorf("system %q domain = %s, want %s", want.Name, got.Domain(), want.Domain)
		}
	}

	if got := (&system.MetaSystem{}).Domain(); got != engine.SystemDual {
		t.Errorf("meta domain = %s, want dual", got)
	}
	if got := (&system.NetworkSystem{}).Domain(); got != engine.SystemPlayer {
		t.Errorf("network domain = %s, want player", got)
	}
}

func TestSystemDependencyNamesRegistered(t *testing.T) {
	registered := make(map[string]bool, len(Systems))
	for _, def := range Systems {
		if registered[def.Name] {
			t.Fatalf("duplicate system name %q", def.Name)
		}
		registered[def.Name] = true
	}

	for _, def := range Systems {
		seen := make(map[string]string, len(def.Required)+len(def.Optional))
		check := func(strength string, dependencies []string) {
			for _, dependency := range dependencies {
				if !registered[dependency] {
					t.Errorf("system %q has unregistered %s dependency %q", def.Name, strength, dependency)
				}
				if previous := seen[dependency]; previous != "" {
					t.Errorf("system %q declares %q as both %s and %s", def.Name, dependency, previous, strength)
				}
				seen[dependency] = strength
			}
		}
		check("required", def.Required)
		check("optional", def.Optional)
	}
}

func TestSystemDomainBoundaries(t *testing.T) {
	profiles := declaredProfiles()
	playerStores := parsePlayerStores(t)
	entries, err := os.ReadDir("../system")
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "_gen.go") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	found := make(map[string]bool, len(profiles))
	for _, name := range names {
		path := filepath.Join("../system", name)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		profile, ok := fileProfile(t, file, profiles, found)
		if !ok || profile == engine.SystemDual {
			continue
		}
		checkDomainBoundary(t, fset, file, profile, playerStores)
	}

	profileTypes := make([]string, 0, len(profiles))
	for typeName := range profiles {
		profileTypes = append(profileTypes, typeName)
	}
	sort.Strings(profileTypes)
	for _, typeName := range profileTypes {
		if !found[typeName] {
			t.Errorf("declared system type %s has no source declaration", typeName)
		}
	}
}

func declaredProfiles() map[string]engine.SystemDomain {
	profiles := make(map[string]engine.SystemDomain, len(Systems)+2)
	for _, def := range Systems {
		profiles[strings.TrimPrefix(def.Constructor, "New")] = def.Domain
	}
	profiles["MetaSystem"] = engine.SystemDual
	profiles["NetworkSystem"] = engine.SystemPlayer
	return profiles
}

func fileProfile(t *testing.T, file *ast.File, profiles map[string]engine.SystemDomain, found map[string]bool) (engine.SystemDomain, bool) {
	t.Helper()
	var (
		profile engine.SystemDomain
		have    bool
	)
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || !strings.HasSuffix(typeSpec.Name.Name, "System") {
				continue
			}
			if _, ok := typeSpec.Type.(*ast.StructType); !ok {
				continue
			}
			domain, ok := profiles[typeSpec.Name.Name]
			if !ok {
				t.Errorf("system type %s has no declared domain profile", typeSpec.Name.Name)
				continue
			}
			found[typeSpec.Name.Name] = true
			if have && profile != domain {
				t.Errorf("source file mixes %s and %s system profiles", profile, domain)
				return engine.SystemDual, true
			}
			profile, have = domain, true
		}
	}
	return profile, have
}

func parsePlayerStores(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "../engine/component_domain.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	stores := make(map[string]bool)
	ast.Inspect(file, func(node ast.Node) bool {
		value, ok := node.(*ast.ValueSpec)
		if !ok || len(value.Names) != 1 || value.Names[0].Name != "componentDomains" || len(value.Values) != 1 {
			return true
		}
		table, ok := value.Values[0].(*ast.CompositeLit)
		if !ok {
			return false
		}
		for _, entry := range table.Elts {
			pair, ok := entry.(*ast.KeyValueExpr)
			if !ok || qualifiedName(pair.Key) == "" {
				continue
			}
			rule, ok := pair.Value.(*ast.CompositeLit)
			if !ok || len(rule.Elts) < 2 || qualifiedName(rule.Elts[1]) != "core.DomainPlayer" {
				continue
			}
			stores[strings.TrimSuffix(qualifiedName(pair.Key), "Bit")] = true
		}
		return false
	})
	if len(stores) == 0 {
		t.Fatal("component domain table contains no player-only stores")
	}
	return stores
}

func checkDomainBoundary(t *testing.T, fset *token.FileSet, file *ast.File, profile engine.SystemDomain, playerStores map[string]bool) {
	t.Helper()
	wantDomain := "core.DomainPlayer"
	if profile == engine.SystemShared {
		wantDomain = "core.DomainShared"
	}

	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.CallExpr:
			name := callName(value)
			if name == "CreateEntity" || name == "Rand" {
				if len(value.Args) == 0 || qualifiedName(value.Args[0]) != wantDomain {
					t.Errorf("%s: %s system calls %s with domain %q, want %s", fset.Position(value.Pos()), profile, name, firstArgName(value), wantDomain)
				}
			}
			if profile == engine.SystemShared && (name == "GetAllEntitiesAt" || name == "GetAllEntitiesAtInto" || isUnscopedSweep(value)) {
				t.Errorf("%s: shared system performs an unscoped spatial read", fset.Position(value.Pos()))
			}
		case *ast.SelectorExpr:
			if profile != engine.SystemShared || !playerStores[value.Sel.Name] {
				break
			}
			base, ok := value.X.(*ast.SelectorExpr)
			if ok && base.Sel.Name == "Components" {
				t.Errorf("%s: shared system accesses player-only component store %s", fset.Position(value.Pos()), value.Sel.Name)
			}
		}
		return true
	})
}

func callName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	default:
		return ""
	}
}

func firstArgName(call *ast.CallExpr) string {
	if len(call.Args) == 0 {
		return ""
	}
	return qualifiedName(call.Args[0])
}

func qualifiedName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		prefix := qualifiedName(value.X)
		if prefix == "" {
			return value.Sel.Name
		}
		return prefix + "." + value.Sel.Name
	default:
		return ""
	}
}

func isUnscopedSweep(call *ast.CallExpr) bool {
	fun, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || fun.Sel.Name != "collect" {
		return false
	}
	receiver, ok := fun.X.(*ast.SelectorExpr)
	return ok && receiver.Sel.Name == "sweep"
}
