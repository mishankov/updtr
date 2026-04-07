package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidConfigResolvesPolicyInheritance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "updtr.toml")
	writeConfig(t, path, `
[policy]
quarantine_days = 7
allow = ["github.com/a/lib"]
deny = ["github.com/a/bad"]
pin = { "github.com/a/pin" = "v1.0.0" }

[[targets]]
name = "root"
ecosystem = "go"
path = "."

[[targets]]
name = "tools-cli"
ecosystem = "go"
path = "./tools/cli"
allow = []
pin = { "github.com/a/pin" = "v1.1.0", "github.com/b/extra" = "v0.1.0" }
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets) != 2 {
		t.Fatalf("targets = %d, want 2", len(cfg.Targets))
	}
	if got := *cfg.Targets[0].Policy.QuarantineDays; got != 7 {
		t.Fatalf("quarantine = %d, want 7", got)
	}
	if !cfg.Targets[1].Policy.AllowSet || len(cfg.Targets[1].Policy.Allow) != 0 {
		t.Fatalf("explicit empty allow should enable allow-list mode")
	}
	if got := cfg.Targets[1].Policy.Pins["github.com/a/pin"]; got != "v1.1.0" {
		t.Fatalf("target pin override = %q", got)
	}
	if got := cfg.Targets[1].Policy.Pins["github.com/b/extra"]; got != "v0.1.0" {
		t.Fatalf("target pin merge = %q", got)
	}
}

func TestLoadTargetIncludeIndirect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "updtr.toml")
	writeConfig(t, path, `
[[targets]]
name = "omitted"
ecosystem = "go"
path = "."

[[targets]]
name = "disabled"
ecosystem = "go"
path = "./disabled"
include_indirect = false

[[targets]]
name = "enabled"
ecosystem = "go"
path = "./enabled"
include_indirect = true
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Targets[0].IncludeIndirect {
		t.Fatalf("omitted include_indirect = true, want false")
	}
	if cfg.Targets[1].IncludeIndirect {
		t.Fatalf("include_indirect=false parsed as true")
	}
	if !cfg.Targets[2].IncludeIndirect {
		t.Fatalf("include_indirect=true parsed as false")
	}
}

func TestLoadUpdateModeInheritanceAndTargetOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "updtr.toml")
	writeConfig(t, path, `
[policy]
update_mode = "vulnerability_only"

[[targets]]
name = "inherited"
ecosystem = "go"
path = "."

[[targets]]
name = "normal"
ecosystem = "go"
path = "./normal"
update_mode = "normal"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Targets[0].Policy.UpdateMode; got != UpdateModeVulnerabilityOnly {
		t.Fatalf("inherited update mode = %s, want vulnerability_only", got)
	}
	if got := cfg.Targets[1].Policy.UpdateMode; got != UpdateModeNormal {
		t.Fatalf("target update mode = %s, want normal", got)
	}
}

func TestLoadDefaultUpdateModeNormal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "updtr.toml")
	writeConfig(t, path, `
[[targets]]
name = "root"
ecosystem = "go"
path = "."
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Targets[0].Policy.UpdateMode; got != UpdateModeNormal {
		t.Fatalf("update mode = %s, want normal", got)
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	cases := map[string]string{
		"unknown key": `
unexpected = true
[[targets]]
name = "root"
ecosystem = "go"
path = "."
`,
		"unknown target key": `
[[targets]]
name = "root"
ecosystem = "go"
path = "."
include_indirekt = true
`,
		"duplicate target name": `
[[targets]]
name = "root"
ecosystem = "go"
path = "."
[[targets]]
name = "root"
ecosystem = "go"
path = "./tools"
`,
		"path escape": `
[[targets]]
name = "root"
ecosystem = "go"
path = "../outside"
`,
		"duplicate allow": `
[policy]
allow = ["github.com/a", "github.com/a"]
[[targets]]
name = "root"
ecosystem = "go"
path = "."
`,
		"negative quarantine": `
[policy]
quarantine_days = -1
[[targets]]
name = "root"
ecosystem = "go"
path = "."
`,
		"invalid global update mode": `
[policy]
update_mode = "security"
[[targets]]
name = "root"
ecosystem = "go"
path = "."
`,
		"invalid target update mode": `
[[targets]]
name = "root"
ecosystem = "go"
path = "."
update_mode = "security"
`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "updtr.toml")
			writeConfig(t, path, body)
			if _, err := Load(path); err == nil {
				t.Fatal("Load succeeded, want error")
			}
		})
	}
}

func TestLoadDefaultDoesNotSearchUpward(t *testing.T) {
	parent := t.TempDir()
	writeConfig(t, filepath.Join(parent, "updtr.toml"), `
[[targets]]
name = "root"
ecosystem = "go"
path = "."
`)
	child := filepath.Join(parent, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	old, _ := os.Getwd()
	defer func() { _ = os.Chdir(old) }()
	if err := os.Chdir(child); err != nil {
		t.Fatal(err)
	}
	_, err := Load("")
	if err == nil || !strings.Contains(err.Error(), "updtr.toml") {
		t.Fatalf("Load default err = %v, want missing local updtr.toml", err)
	}
}

func writeConfig(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
