package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadYAMLConfigResolvesPolicyInheritance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	writeConfig(t, path, `
policy:
  quarantine_days: 7
  update_mode: vulnerability_only
  allow:
    - github.com/a/lib
  deny:
    - github.com/a/bad
  pin:
    github.com/a/pin: v1.0.0
targets:
  - name: root
    ecosystem: go
    path: .
  - name: tools-cli
    ecosystem: go
    path: ./tools/cli
    quarantine_days: 3
    update_mode: normal
    include_indirect: true
    allow: []
    deny: []
    pin:
      github.com/a/pin: v1.1.0
      github.com/b/extra: v0.1.0
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Path != path {
		t.Fatalf("config path = %q, want %q", cfg.Path, path)
	}
	if cfg.BaseDir != dir {
		t.Fatalf("base dir = %q, want %q", cfg.BaseDir, dir)
	}
	if len(cfg.Targets) != 2 {
		t.Fatalf("targets = %d, want 2", len(cfg.Targets))
	}
	if got := *cfg.Targets[0].Policy.QuarantineDays; got != 7 {
		t.Fatalf("root quarantine = %d, want 7", got)
	}
	if got := cfg.Targets[0].Policy.UpdateMode; got != UpdateModeVulnerabilityOnly {
		t.Fatalf("root update mode = %s, want vulnerability_only", got)
	}
	if !cfg.Targets[0].Policy.AllowSet || cfg.Targets[0].Policy.Allow[0] != "github.com/a/lib" {
		t.Fatalf("root allow policy = %+v", cfg.Targets[0].Policy)
	}
	if got := cfg.Targets[0].Policy.Pins["github.com/a/pin"]; got != "v1.0.0" {
		t.Fatalf("root pin = %q, want v1.0.0", got)
	}
	tools := cfg.Targets[1]
	if !tools.IncludeIndirect {
		t.Fatalf("include_indirect = false, want true")
	}
	if got := *tools.Policy.QuarantineDays; got != 3 {
		t.Fatalf("target quarantine = %d, want 3", got)
	}
	if got := tools.Policy.UpdateMode; got != UpdateModeNormal {
		t.Fatalf("target update mode = %s, want normal", got)
	}
	if !tools.Policy.AllowSet || len(tools.Policy.Allow) != 0 {
		t.Fatalf("explicit empty allow should enable allow-list mode")
	}
	if !tools.Policy.DenySet || len(tools.Policy.Deny) != 0 {
		t.Fatalf("explicit empty deny should enable deny-list mode")
	}
	if got := tools.Policy.Pins["github.com/a/pin"]; got != "v1.1.0" {
		t.Fatalf("target pin override = %q", got)
	}
	if got := tools.Policy.Pins["github.com/b/extra"]; got != "v0.1.0" {
		t.Fatalf("target pin merge = %q", got)
	}
	if got := tools.AbsPath; got != filepath.Join(dir, "tools", "cli") {
		t.Fatalf("target abs path = %q, want config-relative tools/cli", got)
	}
}

func TestLoadAcceptsExplicitYAMLExtensions(t *testing.T) {
	for _, name := range []string{"updtr.yaml", "updtr.yml"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, name)
			writeConfig(t, path, validConfig("root", "."))

			cfg, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Path != path {
				t.Fatalf("path = %q, want %q", cfg.Path, path)
			}
		})
	}
}

func TestLoadDefaultConfigDiscovery(t *testing.T) {
	t.Run("prefers updtr.yaml over updtr.yml", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, filepath.Join(dir, "updtr.yaml"), validConfig("yaml", "."))
		writeConfig(t, filepath.Join(dir, "updtr.yml"), validConfig("yml", "./sub"))
		withWorkingDir(t, dir, func() {
			cfg, err := Load("")
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Targets[0].Name != "yaml" {
				t.Fatalf("target name = %q, want updtr.yaml to win", cfg.Targets[0].Name)
			}
		})
	})

	t.Run("falls back to updtr.yml", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, filepath.Join(dir, "updtr.yml"), validConfig("yml", "."))
		withWorkingDir(t, dir, func() {
			cfg, err := Load("")
			if err != nil {
				t.Fatal(err)
			}
			if filepath.Base(cfg.Path) != "updtr.yml" {
				t.Fatalf("path = %q, want updtr.yml fallback", cfg.Path)
			}
		})
	})
}

func TestLoadDefaultDoesNotSearchUpward(t *testing.T) {
	parent := t.TempDir()
	writeConfig(t, filepath.Join(parent, "updtr.yaml"), validConfig("root", "."))
	child := filepath.Join(parent, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	withWorkingDir(t, child, func() {
		_, err := Load("")
		if err == nil || !strings.Contains(err.Error(), "updtr.yaml") {
			t.Fatalf("Load default err = %v, want missing local updtr.yaml", err)
		}
	})
}

func TestLoadExplicitPathDoesNotFallback(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, filepath.Join(dir, "updtr.yml"), validConfig("fallback", "."))
	explicit := filepath.Join(dir, "missing.yaml")

	_, err := Load(explicit)
	if err == nil {
		t.Fatal("Load succeeded, want missing explicit config error")
	}
	if !strings.Contains(err.Error(), explicit) {
		t.Fatalf("err = %v, want explicit path", err)
	}
	if strings.Contains(err.Error(), "updtr.yml") {
		t.Fatalf("err = %v, should not mention fallback path", err)
	}
}

func TestLoadRejectsUnsupportedConfigExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "updtr.json")
	writeConfig(t, path, validConfig("root", "."))

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load succeeded, want unsupported extension error")
	}
	if !strings.Contains(err.Error(), ".yaml") || !strings.Contains(err.Error(), ".yml") {
		t.Fatalf("err = %v, want accepted YAML extensions", err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "json") {
		t.Fatalf("err = %v, should not mention unsupported formats", err)
	}
}

func TestLoadRejectsMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "updtr.yaml")
	writeConfig(t, path, `
policy:
  quarantine_days: [
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load succeeded, want parse error")
	}
	if !strings.Contains(err.Error(), "parse config "+path) {
		t.Fatalf("err = %v, want parse error naming config path", err)
	}
}

func TestLoadRejectsMultipleYAMLDocuments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "updtr.yaml")
	writeConfig(t, path, `
targets:
  - name: root
    ecosystem: go
    path: .
---
targets:
  - name: tools
    ecosystem: go
    path: ./tools
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load succeeded, want multiple-documents error")
	}
	if !strings.Contains(err.Error(), "multiple YAML documents are not supported") {
		t.Fatalf("err = %v, want multiple-documents error", err)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	cases := map[string]string{
		"top level": `
unexpected: true
targets:
  - name: root
    ecosystem: go
    path: .
`,
		"policy": `
policy:
  quarantine_dayz: 7
targets:
  - name: root
    ecosystem: go
    path: .
`,
		"target": `
targets:
  - name: root
    ecosystem: go
    path: .
    include_indirekt: true
`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "updtr.yaml")
			writeConfig(t, path, body)
			if _, err := Load(path); err == nil {
				t.Fatal("Load succeeded, want unknown-field error")
			}
		})
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	cases := map[string]string{
		"missing targets": `
policy:
  quarantine_days: 7
`,
		"missing target name": `
targets:
  - ecosystem: go
    path: .
`,
		"invalid target name": `
targets:
  - name: Root
    ecosystem: go
    path: .
`,
		"duplicate target name": `
targets:
  - name: root
    ecosystem: go
    path: .
  - name: root
    ecosystem: go
    path: ./tools
`,
		"duplicate effective target": `
targets:
  - name: root
    ecosystem: go
    path: .
  - name: root-copy
    ecosystem: go
    path: ./
`,
		"missing ecosystem": `
targets:
  - name: root
    path: .
`,
		"unsupported ecosystem": `
targets:
  - name: root
    ecosystem: npm
    path: .
`,
		"missing path": `
targets:
  - name: root
    ecosystem: go
`,
		"absolute path": `
targets:
  - name: root
    ecosystem: go
    path: /tmp/project
`,
		"path escape": `
targets:
  - name: root
    ecosystem: go
    path: ../outside
`,
		"duplicate allow": `
policy:
  allow:
    - github.com/a
    - github.com/a
targets:
  - name: root
    ecosystem: go
    path: .
`,
		"empty deny module": `
policy:
  deny:
    - ""
targets:
  - name: root
    ecosystem: go
    path: .
`,
		"empty pin module": `
policy:
  pin:
    "": v1.0.0
targets:
  - name: root
    ecosystem: go
    path: .
`,
		"negative global quarantine": `
policy:
  quarantine_days: -1
targets:
  - name: root
    ecosystem: go
    path: .
`,
		"negative target quarantine": `
targets:
  - name: root
    ecosystem: go
    path: .
    quarantine_days: -1
`,
		"invalid global update mode": `
policy:
  update_mode: security
targets:
  - name: root
    ecosystem: go
    path: .
`,
		"invalid target update mode": `
targets:
  - name: root
    ecosystem: go
    path: .
    update_mode: security
`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "updtr.yaml")
			writeConfig(t, path, body)
			if _, err := Load(path); err == nil {
				t.Fatal("Load succeeded, want error")
			}
		})
	}
}

func validConfig(name string, path string) string {
	return `
targets:
  - name: ` + name + `
    ecosystem: go
    path: ` + path + `
`
}

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(old); err != nil {
			t.Fatal(err)
		}
	}()
	fn()
}

func writeConfig(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
