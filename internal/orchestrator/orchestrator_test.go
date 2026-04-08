package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

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
	engine := &Engine{Go: adapter}
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
	engine := &Engine{Go: adapter}
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

func TestDetectReportsTargetProgressInDeterministicOrder(t *testing.T) {
	reporter := &recordingReporter{}
	adapter := &fakeGoAdapter{
		plansByTarget: map[string]core.TargetPlan{
			"first":  {Decisions: []core.Decision{{ModulePath: "example.com/one", Eligible: true}}},
			"second": {Error: "plan failed"},
		},
	}
	engine := &Engine{
		Go:       adapter,
		Reporter: reporter,
		Now: newFakeClock(
			time.Unix(0, 0),
			time.Unix(1, 0),
			time.Unix(2, 0),
			time.Unix(5, 0),
		).Now,
	}
	cfg := &config.Config{Targets: []config.Target{
		{Name: "first", Ecosystem: "go"},
		{Name: "second", Ecosystem: "go"},
	}}

	result, err := engine.Detect(context.Background(), cfg, []string{"second", "first"})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Targets) != 2 || result.Targets[1].Plan.Error != "plan failed" {
		t.Fatalf("result = %+v, want second target plan failure preserved", result)
	}

	want := []TargetProgress{
		{Mode: "detect", Target: cfg.Targets[0]},
		{Mode: "detect", Target: cfg.Targets[0], Outcome: TargetOutcomeSuccess, Elapsed: time.Second},
		{Mode: "detect", Target: cfg.Targets[1]},
		{Mode: "detect", Target: cfg.Targets[1], Outcome: TargetOutcomeFailure, Elapsed: 3 * time.Second},
	}
	if len(reporter.events) != len(want) {
		t.Fatalf("events = %+v, want %d events", reporter.events, len(want))
	}
	for i, event := range reporter.events {
		assertTargetProgressEqual(t, i, event, want[i])
	}
}

func TestApplyReportsSelectedTargetProgressAndFailures(t *testing.T) {
	reporter := &recordingReporter{}
	adapter := &fakeGoAdapter{
		plansByTarget: map[string]core.TargetPlan{
			"first": {
				Decisions: []core.Decision{
					{
						ModulePath:       "example.com/lib",
						CurrentVersion:   "v1.0.0",
						CandidateVersion: "v1.1.0",
						Eligible:         true,
					},
				},
			},
			"second": {
				Decisions: []core.Decision{
					{
						ModulePath:       "example.com/lib",
						CurrentVersion:   "v1.0.0",
						CandidateVersion: "v1.1.0",
						Eligible:         true,
					},
				},
			},
		},
		beforeByTarget: map[string]map[string]string{
			"first":  {"example.com/lib": "v1.0.0"},
			"second": {"example.com/lib": "v1.0.0"},
		},
		afterByTarget: map[string]map[string]string{
			"first": {"example.com/lib": "v1.1.0"},
		},
		applyErrorsByTarget: map[string]error{
			"second": errors.New("apply failed"),
		},
	}
	engine := &Engine{
		Go:       adapter,
		Reporter: reporter,
		Now: newFakeClock(
			time.Unix(10, 0),
			time.Unix(12, 0),
		).Now,
	}
	cfg := &config.Config{Targets: []config.Target{
		{Name: "first", Ecosystem: "go"},
		{Name: "second", Ecosystem: "go"},
	}}

	result, err := engine.Apply(context.Background(), cfg, []string{"second"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Targets) != 1 {
		t.Fatalf("targets = %+v, want one selected target", result.Targets)
	}
	if got := result.Targets[0].Target.Name; got != "second" {
		t.Fatalf("target name = %q, want second", got)
	}
	if got := result.Targets[0].EffectiveError(); got != "apply failed" {
		t.Fatalf("error = %q, want apply failure", got)
	}
	if len(adapter.applied) != 1 || adapter.applied[0].target != "second" {
		t.Fatalf("applied calls = %+v, want one call for selected target", adapter.applied)
	}

	want := []TargetProgress{
		{Mode: "apply", Target: cfg.Targets[1]},
		{Mode: "apply", Target: cfg.Targets[1], Outcome: TargetOutcomeFailure, Elapsed: 2 * time.Second},
	}
	if len(reporter.events) != len(want) {
		t.Fatalf("events = %+v, want %d events", reporter.events, len(want))
	}
	for i, event := range reporter.events {
		assertTargetProgressEqual(t, i, event, want[i])
	}
}

type fakeGoAdapter struct {
	plan                core.TargetPlan
	plansByTarget       map[string]core.TargetPlan
	before              map[string]string
	beforeByTarget      map[string]map[string]string
	after               map[string]string
	afterByTarget       map[string]map[string]string
	directVersionErrors map[string]error
	applyErrorsByTarget map[string]error
	applied             []applyCall
	planned             []config.Target
}

type applyCall struct {
	target  string
	module  string
	version string
}

func (a *fakeGoAdapter) CheckPrereq() error {
	return nil
}

func (a *fakeGoAdapter) PlanTarget(_ context.Context, target config.Target) core.TargetPlan {
	a.planned = append(a.planned, target)
	if plan, ok := a.plansByTarget[target.Name]; ok {
		return plan
	}
	return a.plan
}

func (a *fakeGoAdapter) ApplyUpdate(_ context.Context, target config.Target, modulePath string, version string) (string, error) {
	a.applied = append(a.applied, applyCall{target: target.Name, module: modulePath, version: version})
	if err := a.applyErrorsByTarget[target.Name]; err != nil {
		return "", err
	}
	return "updated", nil
}

func (a *fakeGoAdapter) Tidy(context.Context, config.Target) (string, error) {
	return "tidied", nil
}

func (a *fakeGoAdapter) directVersionsForTarget(target config.Target) (map[string]string, error) {
	if err := a.directVersionErrors[target.Name]; err != nil {
		return nil, err
	}
	if len(a.beforeByTarget) > 0 || len(a.afterByTarget) > 0 {
		if countAppliedForTarget(a.applied, target.Name) == 0 {
			return a.beforeByTarget[target.Name], nil
		}
		return a.afterByTarget[target.Name], nil
	}
	if len(a.applied) == 0 {
		return a.before, nil
	}
	return a.after, nil
}

func (a *fakeGoAdapter) DirectVersions(target config.Target) (map[string]string, error) {
	return a.directVersionsForTarget(target)
}

type recordingReporter struct {
	events []TargetProgress
}

func (r *recordingReporter) TargetStarted(event TargetProgress) {
	r.events = append(r.events, event)
}

func (r *recordingReporter) TargetFinished(event TargetProgress) {
	r.events = append(r.events, event)
}

type fakeClock struct {
	times []time.Time
	index int
}

func newFakeClock(times ...time.Time) *fakeClock {
	return &fakeClock{times: append([]time.Time(nil), times...)}
}

func (c *fakeClock) Now() time.Time {
	if len(c.times) == 0 {
		return time.Time{}
	}
	if c.index >= len(c.times) {
		return c.times[len(c.times)-1]
	}
	current := c.times[c.index]
	c.index++
	return current
}

func countAppliedForTarget(calls []applyCall, target string) int {
	count := 0
	for _, call := range calls {
		if call.target == target {
			count++
		}
	}
	return count
}

func assertTargetProgressEqual(t *testing.T, index int, got TargetProgress, want TargetProgress) {
	t.Helper()
	if got.Mode != want.Mode ||
		got.Target.Name != want.Target.Name ||
		got.Target.NormalizedPath != want.Target.NormalizedPath ||
		got.Outcome != want.Outcome ||
		got.Elapsed != want.Elapsed {
		t.Fatalf("event[%d] = %+v, want %+v", index, got, want)
	}
}
