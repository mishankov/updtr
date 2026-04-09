package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mishankov/updtr/internal/config"
	"github.com/mishankov/updtr/internal/core"
	"github.com/mishankov/updtr/internal/orchestrator"
	"github.com/mishankov/updtr/internal/progress"
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

func TestDetectTerminalPolicyRespectsTTYAndNoColor(t *testing.T) {
	restoreTTY := stubTerminalWriterCheck(func(io.Writer) bool { return true })
	defer restoreTTY()

	restoreEnv := stubLookupEnv(func(key string) (string, bool) {
		if key == "NO_COLOR" {
			return "", false
		}
		return "", false
	})
	defer restoreEnv()

	got := detectTerminalPolicy(&bytes.Buffer{})
	if !got.LiveUpdates || !got.Color {
		t.Fatalf("policy = %+v, want live updates and color for tty without NO_COLOR", got)
	}

	restoreEnv = stubLookupEnv(func(key string) (string, bool) {
		if key == "NO_COLOR" {
			return "1", true
		}
		return "", false
	})
	defer restoreEnv()

	got = detectTerminalPolicy(&bytes.Buffer{})
	if !got.LiveUpdates || got.Color {
		t.Fatalf("policy = %+v, want live updates without color when NO_COLOR is set", got)
	}

	restoreTTY = stubTerminalWriterCheck(func(io.Writer) bool { return false })
	defer restoreTTY()
	got = detectTerminalPolicy(&bytes.Buffer{})
	if got.LiveUpdates || got.Color {
		t.Fatalf("policy = %+v, want non-interactive fallback without color for non-tty", got)
	}
}

func TestIsTerminalWriterRejectsDevNull(t *testing.T) {
	file, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if isTerminalWriter(file) {
		t.Fatalf("isTerminalWriter(%q) = true, want false", os.DevNull)
	}
}

func TestDetectCommandUsesAppendOnlyFallbackProgressAndTable(t *testing.T) {
	restorePolicy := stubTerminalPolicy(terminalPolicy{LiveUpdates: false, Color: false})
	defer restorePolicy()

	restoreEngine := stubNewEngine(func() *orchestrator.Engine {
		return &orchestrator.Engine{
			Go: &cliFakeAdapter{
				plansByTarget: map[string]core.TargetPlan{
					"app": {
						Decisions: []core.Decision{{
							ModulePath:       "github.com/a/direct",
							CurrentVersion:   "v1.0.0",
							CandidateVersion: "v1.1.0",
							Eligible:         true,
						}},
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
	defer restoreEngine()

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
	assertInOrder(t, got,
		"detect [1/2 targets] app (.): started",
		"detect [1/2 targets] app (.): planning 0/1 dependencies checked",
		"detect [1/2 targets] app (.): planning 1/1 dependencies checked (github.com/a/direct)",
		"detect [1/2 targets] app (.): finished SUCCESS in 1s",
		"detect [2/2 targets] worker (worker): started",
		"detect [2/2 targets] worker (worker): finished FAILURE in 3s",
		"| TARGET",
	)
	if strings.Contains(got, "\r\033[2K") {
		t.Fatalf("output = %q, want append-only fallback without live-update control sequences", got)
	}
	if !strings.Contains(got, "| app (.)         | github.com/a/direct | v1.0.0 | v1.1.0 | ELIGIBLE | update available |") {
		t.Fatalf("output = %q, want eligible row in final table", got)
	}
	if !strings.Contains(got, "| worker (worker) |                     |        |        | ERROR    | plan failed") {
		t.Fatalf("output = %q, want target error row in final table", got)
	}
}

func TestApplyCommandUsesLivePresenterForTTYPolicy(t *testing.T) {
	restorePolicy := stubTerminalPolicy(terminalPolicy{LiveUpdates: true, Color: false})
	defer restorePolicy()

	restoreEngine := stubNewEngine(func() *orchestrator.Engine {
		return &orchestrator.Engine{
			Go: &cliFakeAdapter{
				plansByTarget: map[string]core.TargetPlan{
					"app": {
						Decisions: []core.Decision{{
							ModulePath:       "github.com/a/direct",
							CurrentVersion:   "v1.0.0",
							CandidateVersion: "v1.1.0",
							Eligible:         true,
						}},
					},
					"worker": {
						Decisions: []core.Decision{{
							ModulePath:       "github.com/a/indirect",
							CurrentVersion:   "v1.0.0",
							CandidateVersion: "v1.1.0",
							Relationship:     core.RelationshipIndirect,
							Eligible:         true,
						}},
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
	defer restoreEngine()

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
	if !strings.Contains(got, "\r\033[2K- apply [1/1 targets] worker (worker): started") {
		t.Fatalf("output = %q, want live presenter carriage-return updates", got)
	}
	if !strings.Contains(got, "apply [1/1 targets] worker (worker): mutating 1/1 updates processed (github.com/a/indirect)") {
		t.Fatalf("output = %q, want mutation progress in live output", got)
	}
	if strings.Contains(got, "app (.)") {
		t.Fatalf("output = %q, want no progress for unselected target", got)
	}
	if !strings.Contains(got, "| worker (worker) | github.com/a/indirect (indirect) | v1.0.0 | v1.1.0 | APPLIED |") {
		t.Fatalf("output = %q, want applied row in final table", got)
	}
	if strings.Contains(got, "\x1b[32m") || strings.Contains(got, "\x1b[31m") {
		t.Fatalf("output = %q, want no color escapes when color is disabled", got)
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

func (a *cliFakeAdapter) PlanTarget(_ context.Context, target config.Target, reports ...func(progress.PlanUpdate)) core.TargetPlan {
	plan := a.plansByTarget[target.Name]
	if plan.Error == "" {
		report := firstReport(reports)
		if report != nil {
			report(progress.PlanUpdate{Kind: progress.PlanKindStarted, TotalDependencies: len(plan.Decisions)})
			for i, decision := range plan.Decisions {
				report(progress.PlanUpdate{
					Kind:                  progress.PlanKindChecked,
					DependenciesCompleted: i + 1,
					TotalDependencies:     len(plan.Decisions),
					ModulePath:            decision.ModulePath,
				})
			}
		}
	}
	return plan
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

func stubTerminalPolicy(policy terminalPolicy) func() {
	previous := resolveTerminalPolicy
	resolveTerminalPolicy = func(io.Writer) terminalPolicy {
		return policy
	}
	return func() {
		resolveTerminalPolicy = previous
	}
}

func stubLookupEnv(fn func(string) (string, bool)) func() {
	previous := lookupEnv
	lookupEnv = fn
	return func() {
		lookupEnv = previous
	}
}

func stubTerminalWriterCheck(fn func(io.Writer) bool) func() {
	previous := terminalWriterCheck
	terminalWriterCheck = fn
	return func() {
		terminalWriterCheck = previous
	}
}

func firstReport(reports []func(progress.PlanUpdate)) func(progress.PlanUpdate) {
	if len(reports) == 0 {
		return nil
	}
	return reports[0]
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
