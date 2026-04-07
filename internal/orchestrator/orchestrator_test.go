package orchestrator

import (
	"context"
	"testing"

	"github.com/mishankov/updtr/internal/config"
	"github.com/mishankov/updtr/internal/core"
)

func TestApplyPassesSelectedPlanCandidateToAdapter(t *testing.T) {
	adapter := &fakeGoAdapter{
		plan: core.TargetPlan{
			Decisions: []core.Decision{
				{
					ModulePath:       "example.com/lib",
					CurrentVersion:   "v1.37.0",
					CandidateVersion: "v1.48.0",
					Eligible:         true,
				},
				{
					ModulePath:       "example.com/blocked",
					CurrentVersion:   "v1.0.0",
					CandidateVersion: "v1.2.0",
					BlockedReason:    core.ReasonQuarantined,
				},
			},
		},
		before: map[string]string{
			"example.com/lib":     "v1.37.0",
			"example.com/blocked": "v1.0.0",
		},
		after: map[string]string{
			"example.com/lib":     "v1.48.0",
			"example.com/blocked": "v1.0.0",
		},
	}
	engine := &Engine{goAdapter: adapter}
	cfg := &config.Config{Targets: []config.Target{{Name: "app", Ecosystem: "go"}}}

	result, err := engine.Apply(context.Background(), cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(adapter.applied) != 1 {
		t.Fatalf("applied calls = %+v, want one", adapter.applied)
	}
	if adapter.applied[0].module != "example.com/lib" || adapter.applied[0].version != "v1.48.0" {
		t.Fatalf("applied call = %+v, want selected fallback candidate v1.48.0", adapter.applied[0])
	}
	if len(result.Targets) != 1 || len(result.Targets[0].Applied) != 1 {
		t.Fatalf("result = %+v, want one applied update", result)
	}
	if got := result.Targets[0].Applied[0].ToVersion; got != "v1.48.0" {
		t.Fatalf("applied ToVersion = %s, want v1.48.0", got)
	}
}

type fakeGoAdapter struct {
	plan    core.TargetPlan
	before  map[string]string
	after   map[string]string
	applied []applyCall
}

type applyCall struct {
	module  string
	version string
}

func (a *fakeGoAdapter) CheckPrereq() error {
	return nil
}

func (a *fakeGoAdapter) PlanTarget(context.Context, config.Target) core.TargetPlan {
	return a.plan
}

func (a *fakeGoAdapter) ApplyUpdate(_ context.Context, _ config.Target, modulePath string, version string) (string, error) {
	a.applied = append(a.applied, applyCall{module: modulePath, version: version})
	return "updated", nil
}

func (a *fakeGoAdapter) Tidy(context.Context, config.Target) (string, error) {
	return "tidied", nil
}

func (a *fakeGoAdapter) DirectVersions(config.Target) (map[string]string, error) {
	if len(a.applied) == 0 {
		return a.before, nil
	}
	return a.after, nil
}
