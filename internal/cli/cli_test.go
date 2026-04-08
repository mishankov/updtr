package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mishankov/updtr/internal/config"
	"github.com/mishankov/updtr/internal/core"
	"github.com/mishankov/updtr/internal/orchestrator"
)

func TestHelpMentionsYAMLConfigDefault(t *testing.T) {
	var out bytes.Buffer
	cmd := New("test", &out, &out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	help := out.String()
	if !strings.Contains(help, "--config") || !strings.Contains(help, "updtr.yaml") {
		t.Fatalf("help = %q, want --config default updtr.yaml", help)
	}
}

func TestInitHelpMentionsYAMLConfig(t *testing.T) {
	var out bytes.Buffer
	cmd := New("test", &out, &out)
	cmd.SetArgs([]string{"init", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	help := out.String()
	if !strings.Contains(help, "Create updtr.yaml from discovered Go modules") {
		t.Fatalf("init help = %q, want YAML init summary", help)
	}
}

func TestLoadPathUsesDefaultDiscoveryUnlessConfigFlagChanged(t *testing.T) {
	var out bytes.Buffer
	root := New("test", &out, &out)
	detect, _, err := root.Find([]string{"detect"})
	if err != nil {
		t.Fatal(err)
	}

	if got := loadPath(detect, "custom.yml"); got != "" {
		t.Fatalf("load path = %q, want default discovery", got)
	}
	flag := detect.Flag("config")
	if flag == nil {
		t.Fatal("detect command missing inherited config flag")
	}
	flag.Changed = true
	if got := loadPath(detect, "custom.yml"); got != "custom.yml" {
		t.Fatalf("load path = %q, want explicit config path", got)
	}
}

func TestDetectCommandPrintsProgressBeforeFinalReport(t *testing.T) {
	restore := stubNewEngine(func() *orchestrator.Engine {
		return &orchestrator.Engine{
			Go: &cliFakeAdapter{
				plansByTarget: map[string]core.TargetPlan{
					"app": {
						Decisions: []core.Decision{
							{
								ModulePath:       "github.com/a/direct",
								CurrentVersion:   "v1.0.0",
								CandidateVersion: "v1.1.0",
								Eligible:         true,
							},
						},
					},
					"worker": {Error: "plan failed"},
				},
			},
			Now: cliFakeNow(
				time.Unix(0, 0),
				time.Unix(1, 0),
				time.Unix(2, 0),
				time.Unix(5, 0),
			),
		}
	})
	defer restore()

	configPath := writeConfigFile(t, `
targets:
  - name: app
    ecosystem: go
    path: .
  - name: worker
    ecosystem: go
    path: worker
`)

	var out bytes.Buffer
	cmd := New("test", &out, &out)
	cmd.SetArgs([]string{"detect", "--config", configPath})

	err := cmd.Execute()
	if !IsSilentExit(err) {
		t.Fatalf("err = %v, want silentExit", err)
	}

	got := out.String()
	progress := []string{
		"detect: target app (.) started",
		"detect: target app (.) finished outcome=success elapsed=1s",
		"detect: target worker (worker) started",
		"detect: target worker (worker) finished outcome=failure elapsed=3s",
	}
	assertInOrder(t, got, progress...)

	wantSummary := `Target app (.)
Eligible:
  - github.com/a/direct v1.0.0 -> v1.1.0
Summary: eligible=1 blocked=0 applied=0 errors=0

Target worker (worker)
Errors:
  - plan failed
Summary: eligible=0 blocked=0 applied=0 errors=1

Total: targets=2 eligible=1 blocked=0 applied=0 errors=1
`
	if stripped := stripProgressLines(got); stripped != wantSummary {
		t.Fatalf("summary after stripping progress:\n%s\nwant:\n%s", stripped, wantSummary)
	}
}

func TestApplyCommandPrintsSelectedTargetProgressBeforeFinalReport(t *testing.T) {
	restore := stubNewEngine(func() *orchestrator.Engine {
		return &orchestrator.Engine{
			Go: &cliFakeAdapter{
				plansByTarget: map[string]core.TargetPlan{
					"app": {
						Decisions: []core.Decision{
							{
								ModulePath:       "github.com/a/direct",
								CurrentVersion:   "v1.0.0",
								CandidateVersion: "v1.1.0",
								Eligible:         true,
							},
						},
					},
					"worker": {
						Decisions: []core.Decision{
							{
								ModulePath:       "github.com/a/indirect",
								CurrentVersion:   "v1.0.0",
								CandidateVersion: "v1.1.0",
								Relationship:     core.RelationshipIndirect,
								Eligible:         true,
							},
						},
					},
				},
				beforeByTarget: map[string]map[string]string{
					"worker": {"github.com/a/indirect": "v1.0.0"},
				},
				afterByTarget: map[string]map[string]string{
					"worker": {"github.com/a/indirect": "v1.1.0"},
				},
			},
			Now: cliFakeNow(
				time.Unix(10, 0),
				time.Unix(12, 0),
			),
		}
	})
	defer restore()

	configPath := writeConfigFile(t, `
targets:
  - name: app
    ecosystem: go
    path: .
  - name: worker
    ecosystem: go
    path: worker
`)

	var out bytes.Buffer
	cmd := New("test", &out, &out)
	cmd.SetArgs([]string{"apply", "--config", configPath, "--target", "worker"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	progress := []string{
		"apply: target worker (worker) started",
		"apply: target worker (worker) finished outcome=success elapsed=2s",
	}
	assertInOrder(t, got, progress...)
	if strings.Contains(got, "apply: target app (.)") {
		t.Fatalf("output = %q, want no progress for unselected target", got)
	}

	wantSummary := `Target worker (worker)
Applied:
  - github.com/a/indirect (indirect) v1.0.0 -> v1.1.0
Summary: eligible=1 blocked=0 applied=1 errors=0

Total: targets=1 eligible=1 blocked=0 applied=1 errors=0
`
	if stripped := stripProgressLines(got); stripped != wantSummary {
		t.Fatalf("summary after stripping progress:\n%s\nwant:\n%s", stripped, wantSummary)
	}
}

type cliFakeAdapter struct {
	plansByTarget  map[string]core.TargetPlan
	beforeByTarget map[string]map[string]string
	afterByTarget  map[string]map[string]string
	applyCalls     map[string]int
}

func (a *cliFakeAdapter) CheckPrereq() error {
	return nil
}

func (a *cliFakeAdapter) PlanTarget(_ context.Context, target config.Target) core.TargetPlan {
	return a.plansByTarget[target.Name]
}

func (a *cliFakeAdapter) ApplyUpdate(_ context.Context, target config.Target, modulePath string, version string) (string, error) {
	if a.applyCalls == nil {
		a.applyCalls = map[string]int{}
	}
	a.applyCalls[target.Name]++
	return "updated " + target.Name + " " + modulePath + "@" + version, nil
}

func (a *cliFakeAdapter) Tidy(context.Context, config.Target) (string, error) {
	return "tidied", nil
}

func (a *cliFakeAdapter) DirectVersions(target config.Target) (map[string]string, error) {
	if a.applyCalls[target.Name] > 0 {
		if after, ok := a.afterByTarget[target.Name]; ok {
			return after, nil
		}
	}
	return a.beforeByTarget[target.Name], nil
}

func stubNewEngine(factory func() *orchestrator.Engine) func() {
	previous := newEngine
	newEngine = factory
	return func() {
		newEngine = previous
	}
}

func writeConfigFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "updtr.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimLeft(body, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func cliFakeNow(times ...time.Time) func() time.Time {
	clock := &cliClock{times: append([]time.Time(nil), times...)}
	return clock.Now
}

type cliClock struct {
	times []time.Time
	index int
}

func (c *cliClock) Now() time.Time {
	if c.index >= len(c.times) {
		return c.times[len(c.times)-1]
	}
	current := c.times[c.index]
	c.index++
	return current
}

func assertInOrder(t *testing.T, text string, fragments ...string) {
	t.Helper()
	index := 0
	for _, fragment := range fragments {
		next := strings.Index(text[index:], fragment)
		if next == -1 {
			t.Fatalf("output = %q, want fragment %q after offset %d", text, fragment, index)
		}
		index += next + len(fragment)
	}
}

func stripProgressLines(text string) string {
	var kept []string
	for _, line := range strings.SplitAfter(text, "\n") {
		if strings.HasPrefix(line, "detect: target ") || strings.HasPrefix(line, "apply: target ") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "")
}
