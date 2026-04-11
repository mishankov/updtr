package action

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
	"github.com/mishankov/updtr/internal/progress"
)

func TestDirectRunnerEmitsAppendOnlyProgressAndFinalTable(t *testing.T) {
	restoreEngine := stubActionEngine(func() *orchestrator.Engine {
		return &orchestrator.Engine{
			Go: &actionFakeAdapter{
				plansByTarget: map[string]core.TargetPlan{
					"worker": {
						Decisions: []core.Decision{{
							ModulePath:       "github.com/example/mod",
							CurrentVersion:   "v1.0.0",
							CandidateVersion: "v1.1.0",
							Eligible:         true,
						}},
					},
				},
				beforeByTarget: map[string]map[string]string{
					"worker": {"github.com/example/mod": "v1.0.0"},
				},
				afterByTarget: map[string]map[string]string{
					"worker": {"github.com/example/mod": "v1.1.0"},
				},
			},
			Now: actionFakeNow(
				time.Unix(0, 0),
				time.Unix(3, 0),
			),
		}
	})
	defer restoreEngine()

	configPath := writeActionConfigFile(t, `
targets:
  - name: worker
    ecosystem: go
    path: .
`)

	var out bytes.Buffer
	runner := &directRunner{stdout: &out}

	result, err := runner.Apply(context.Background(), RunOptions{ConfigPath: configPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Targets) != 1 {
		t.Fatalf("targets = %d, want 1", len(result.Targets))
	}

	got := out.String()
	assertInOrder(t, got,
		"apply [1/1 targets] worker (.): started",
		"apply [1/1 targets] worker (.): planning 0/1 dependencies checked",
		"apply [1/1 targets] worker (.): planning 1/1 dependencies checked (github.com/example/mod)",
		"apply [1/1 targets] worker (.): mutating 0/1 updates processed",
		"apply [1/1 targets] worker (.): mutating 1/1 updates processed (github.com/example/mod)",
		"apply [1/1 targets] worker (.): finished SUCCESS in 3s",
		"| TARGET",
	)
	if strings.Contains(got, "\r\033[2K") {
		t.Fatalf("output = %q, want append-only progress without live-update control sequences", got)
	}
}

func TestDirectRunnerAllowsNilStdout(t *testing.T) {
	restoreEngine := stubActionEngine(func() *orchestrator.Engine {
		return &orchestrator.Engine{
			Go: &actionFakeAdapter{
				plansByTarget: map[string]core.TargetPlan{
					"worker": {
						Decisions: []core.Decision{{
							ModulePath:       "github.com/example/mod",
							CurrentVersion:   "v1.0.0",
							CandidateVersion: "v1.1.0",
							Eligible:         true,
						}},
					},
				},
				beforeByTarget: map[string]map[string]string{
					"worker": {"github.com/example/mod": "v1.0.0"},
				},
				afterByTarget: map[string]map[string]string{
					"worker": {"github.com/example/mod": "v1.1.0"},
				},
			},
			Now: actionFakeNow(
				time.Unix(0, 0),
				time.Unix(3, 0),
			),
		}
	})
	defer restoreEngine()

	configPath := writeActionConfigFile(t, `
targets:
  - name: worker
    ecosystem: go
    path: .
`)

	runner := &directRunner{}
	result, err := runner.Apply(context.Background(), RunOptions{ConfigPath: configPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Targets) != 1 {
		t.Fatalf("targets = %d, want 1", len(result.Targets))
	}
	if len(result.Targets[0].Applied) != 1 {
		t.Fatalf("applied updates = %d, want 1", len(result.Targets[0].Applied))
	}
}

type actionFakeAdapter struct {
	plansByTarget  map[string]core.TargetPlan
	beforeByTarget map[string]map[string]string
	afterByTarget  map[string]map[string]string
	applyCalls     map[string]int
}

func (a *actionFakeAdapter) CheckPrereq() error {
	return nil
}

func (a *actionFakeAdapter) PlanTarget(_ context.Context, target config.Target, reports ...func(progress.PlanUpdate)) core.TargetPlan {
	plan := a.plansByTarget[target.Name]
	if plan.Error == "" {
		report := firstActionReport(reports)
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

func (a *actionFakeAdapter) ApplyUpdate(_ context.Context, target config.Target, modulePath string, version string) (string, error) {
	if a.applyCalls == nil {
		a.applyCalls = map[string]int{}
	}
	a.applyCalls[target.Name]++
	return "updated " + target.Name + " " + modulePath + "@" + version, nil
}

func (a *actionFakeAdapter) Tidy(context.Context, config.Target) (string, error) {
	return "tidied", nil
}

func (a *actionFakeAdapter) DirectVersions(target config.Target) (map[string]string, error) {
	if a.applyCalls[target.Name] > 0 {
		return a.afterByTarget[target.Name], nil
	}
	return a.beforeByTarget[target.Name], nil
}

func stubActionEngine(factory func() *orchestrator.Engine) func() {
	previous := newOrchestratorEngine
	newOrchestratorEngine = factory
	return func() {
		newOrchestratorEngine = previous
	}
}

func firstActionReport(reports []func(progress.PlanUpdate)) func(progress.PlanUpdate) {
	if len(reports) == 0 {
		return nil
	}
	return reports[0]
}

func writeActionConfigFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "updtr.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimLeft(body, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func actionFakeNow(times ...time.Time) func() time.Time {
	clock := &actionClock{times: append([]time.Time(nil), times...)}
	return clock.Now
}

type actionClock struct {
	times []time.Time
	index int
}

func (c *actionClock) Now() time.Time {
	if c.index >= len(c.times) {
		return c.times[len(c.times)-1]
	}
	current := c.times[c.index]
	c.index++
	return current
}

func assertInOrder(t *testing.T, haystack string, fragments ...string) {
	t.Helper()
	index := 0
	for _, fragment := range fragments {
		position := strings.Index(haystack[index:], fragment)
		if position < 0 {
			t.Fatalf("output = %q, want fragment %q after index %d", haystack, fragment, index)
		}
		index += position + len(fragment)
	}
}
