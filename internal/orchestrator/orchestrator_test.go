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
					Relationship:     core.RelationshipIndirect,
					Vulnerabilities: []core.Vulnerability{
						{ModulePath: "example.com/lib", AffectedVersion: "v1.37.0", FixedVersions: []string{"v1.48.0"}, AdvisoryIDs: []string{"GO-2026-0001"}},
					},
					Eligible: true,
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
	if got := result.Targets[0].Applied[0].Relationship; got != core.RelationshipIndirect {
		t.Fatalf("applied relationship = %s, want indirect", got)
	}
	if got := result.Targets[0].Applied[0].Vulnerabilities; len(got) != 1 || got[0].AdvisoryIDs[0] != "GO-2026-0001" {
		t.Fatalf("applied vulnerabilities = %+v, want selected plan vulnerability context", got)
	}
}

func TestDetectPreservesPerTargetIncludeIndirect(t *testing.T) {
	adapter := &fakeGoAdapter{}
	engine := &Engine{goAdapter: adapter}
	cfg := &config.Config{Targets: []config.Target{
		{Name: "direct", Ecosystem: "go"},
		{Name: "indirect", Ecosystem: "go", IncludeIndirect: true},
	}}

	if _, err := engine.Detect(context.Background(), cfg, nil); err != nil {
		t.Fatal(err)
	}
	if len(adapter.planned) != 2 {
		t.Fatalf("planned targets = %+v, want two", adapter.planned)
	}
	if adapter.planned[0].Name != "direct" || adapter.planned[0].IncludeIndirect {
		t.Fatalf("first planned target = %+v, want direct target without include_indirect", adapter.planned[0])
	}
	if adapter.planned[1].Name != "indirect" || !adapter.planned[1].IncludeIndirect {
		t.Fatalf("second planned target = %+v, want indirect target with include_indirect", adapter.planned[1])
	}
}

type fakeGoAdapter struct {
	plan    core.TargetPlan
	before  map[string]string
	after   map[string]string
	applied []applyCall
	planned []config.Target
}

type applyCall struct {
	module  string
	version string
}

func (a *fakeGoAdapter) CheckPrereq() error {
	return nil
}

func (a *fakeGoAdapter) PlanTarget(_ context.Context, target config.Target) core.TargetPlan {
	a.planned = append(a.planned, target)
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
