package initgen

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var unsafeNameChars = regexp.MustCompile(`[^a-z0-9]+`)

func Run(cwd string) (string, error) {
	for _, name := range []string{"updtr.yaml", "updtr.yml"} {
		configPath := filepath.Join(cwd, name)
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Sprintf("%s already exists; nothing to do\n", name), nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("check %s: %w", name, err)
		}
	}

	targets, err := Discover(cwd)
	if err != nil {
		return "", err
	}
	if len(targets) == 0 {
		return "no go modules found; nothing to do\n", nil
	}
	data := RenderConfig(targets)
	configPath := filepath.Join(cwd, "updtr.yaml")
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return "", fmt.Errorf("write updtr.yaml: %w", err)
	}
	return fmt.Sprintf("created updtr.yaml with %d go targets\n", len(targets)), nil
}

type Target struct {
	Name string
	Path string
}

func Discover(root string) ([]Target, error) {
	var relPaths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && path != root && shouldSkip(entry.Name()) {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				if rel == "." {
					relPaths = append(relPaths, ".")
				} else {
					relPaths = append(relPaths, filepath.ToSlash(rel))
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(relPaths, func(i, j int) bool {
		if relPaths[i] == "." {
			return true
		}
		if relPaths[j] == "." {
			return false
		}
		return relPaths[i] < relPaths[j]
	})

	counts := map[string]int{}
	targets := make([]Target, 0, len(relPaths))
	for _, rel := range relPaths {
		name := generatedName(rel)
		counts[name]++
		if counts[name] > 1 {
			name = fmt.Sprintf("%s-%d", name, counts[name])
		}
		targets = append(targets, Target{Name: name, Path: generatedPath(rel)})
	}
	return targets, nil
}

func RenderConfig(targets []Target) []byte {
	var buf bytes.Buffer
	buf.WriteString("policy:\n")
	buf.WriteString("  quarantine_days: 7\n")
	buf.WriteString("targets:\n")
	for _, target := range targets {
		fmt.Fprintf(&buf, "  - name: %q\n", target.Name)
		buf.WriteString("    ecosystem: \"go\"\n")
		fmt.Fprintf(&buf, "    path: %q\n", target.Path)
	}
	return buf.Bytes()
}

func shouldSkip(name string) bool {
	return name == ".git" || name == "vendor" || name == "node_modules"
}

func generatedPath(rel string) string {
	if rel == "." {
		return "."
	}
	return "./" + rel
}

func generatedName(rel string) string {
	if rel == "." {
		return "root"
	}
	name := strings.ToLower(rel)
	name = unsafeNameChars.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "target"
	}
	return name
}
