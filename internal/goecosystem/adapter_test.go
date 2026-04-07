package goecosystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadModuleStateVersionScopedReplacement(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, `module example.com/app

go 1.25.0

require example.com/lib v1.1.0

replace example.com/lib v1.0.0 => ../lib
`)

	state, err := readModuleState(dir)
	if err != nil {
		t.Fatalf("readModuleState() error = %v", err)
	}
	if state.Replaced.Contains("example.com/lib", "v1.1.0") {
		t.Fatal("version-scoped replace for v1.0.0 should not replace v1.1.0")
	}
	if !state.Replaced.Contains("example.com/lib", "v1.0.0") {
		t.Fatal("version-scoped replace for v1.0.0 should replace v1.0.0")
	}
}

func TestReadModuleStatePathWideReplacement(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, `module example.com/app

go 1.25.0

require example.com/lib v1.1.0

replace example.com/lib => ../lib
`)

	state, err := readModuleState(dir)
	if err != nil {
		t.Fatalf("readModuleState() error = %v", err)
	}
	if !state.Replaced.Contains("example.com/lib", "v1.1.0") {
		t.Fatal("path-wide replace should replace every version")
	}
}

func writeGoMod(t *testing.T, dir string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
}
