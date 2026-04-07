package initgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverIncludesNestedModulesAndSkipsIgnoredDirs(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "go.mod"))
	touch(t, filepath.Join(root, "tools", "cli", "go.mod"))
	touch(t, filepath.Join(root, "vendor", "vendored", "go.mod"))
	touch(t, filepath.Join(root, "node_modules", "pkg", "go.mod"))
	touch(t, filepath.Join(root, ".git", "modules", "go.mod"))

	targets, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("targets = %+v, want 2 targets", targets)
	}
	if targets[0].Name != "root" || targets[0].Path != "." {
		t.Fatalf("root target = %+v", targets[0])
	}
	if targets[1].Name != "tools-cli" || targets[1].Path != "./tools/cli" {
		t.Fatalf("nested target = %+v", targets[1])
	}
}

func TestRunCreatesConfigAndNoopsWhenExistsOrNoModules(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "go.mod"))

	message, err := Run(root)
	if err != nil {
		t.Fatal(err)
	}
	if message != "created updtr.toml with 1 go targets\n" {
		t.Fatalf("message = %q", message)
	}
	data, err := os.ReadFile(filepath.Join(root, "updtr.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "quarantine_days = 7") || !strings.Contains(string(data), `path = "."`) {
		t.Fatalf("generated config:\n%s", data)
	}
	if strings.Contains(string(data), "include_indirect") {
		t.Fatalf("generated config should stay direct-only by omitting include_indirect:\n%s", data)
	}

	message, err = Run(root)
	if err != nil {
		t.Fatal(err)
	}
	if message != "updtr.toml already exists; nothing to do\n" {
		t.Fatalf("existing message = %q", message)
	}

	empty := t.TempDir()
	message, err = Run(empty)
	if err != nil {
		t.Fatal(err)
	}
	if message != "no go modules found; nothing to do\n" {
		t.Fatalf("empty message = %q", message)
	}
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("module example.com/test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
