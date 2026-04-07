package goecosystem

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mishankov/updtr/internal/config"
	"github.com/mishankov/updtr/internal/core"
)

func TestPlanTargetSelectsEligibleFallbackCandidate(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	dir := moduleDir(t, "example.com/lib", "v1.37.0")
	quarantineDays := 7

	adapter := fakeSelectionAdapter(now, []string{"v1.37.0", "v1.48.0", "v1.48.1", "v1.48.2"}, map[string]candidate{
		"v1.48.2": trustedRelease(now.Add(-24 * time.Hour)),
		"v1.48.1": trustedRelease(now.Add(-4 * 24 * time.Hour)),
		"v1.48.0": trustedRelease(now.Add(-11 * 24 * time.Hour)),
	})

	plan := adapter.PlanTarget(context.Background(), target(dir, config.Policy{QuarantineDays: &quarantineDays, Pins: map[string]string{}}))
	decision := onlyDecision(t, plan)
	if !decision.Eligible {
		t.Fatalf("decision = %+v, want eligible fallback", decision)
	}
	if decision.CandidateVersion != "v1.48.0" {
		t.Fatalf("candidate = %s, want v1.48.0", decision.CandidateVersion)
	}
	if decision.ReleaseTime == nil || !decision.ReleaseTime.Equal(now.Add(-11*24*time.Hour)) {
		t.Fatalf("release time = %v, want selected candidate release time", decision.ReleaseTime)
	}
}

func TestPlanTargetSkipsMetadataBlockedCandidates(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	quarantineDays := 0

	cases := []struct {
		name    string
		blocked candidate
	}{
		{
			name:    "missing release date",
			blocked: candidate{Version: "v1.2.0"},
		},
		{
			name:    "untrusted release date",
			blocked: candidate{Version: "v1.2.0", ReleaseTime: timePtr(now.Add(-24 * time.Hour))},
		},
		{
			name:    "future release date",
			blocked: trustedRelease(now.Add(time.Hour)),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := moduleDir(t, "example.com/lib", "v1.0.0")
			adapter := fakeSelectionAdapter(now, []string{"v1.1.0", "v1.2.0"}, map[string]candidate{
				"v1.2.0": tc.blocked,
				"v1.1.0": trustedRelease(now.Add(-24 * time.Hour)),
			})

			plan := adapter.PlanTarget(context.Background(), target(dir, config.Policy{QuarantineDays: &quarantineDays, Pins: map[string]string{}}))
			decision := onlyDecision(t, plan)
			if !decision.Eligible || decision.CandidateVersion != "v1.1.0" {
				t.Fatalf("decision = %+v, want eligible v1.1.0", decision)
			}
		})
	}
}

func TestPlanTargetReportsNewestBlockedCandidateWhenNoFallbackEligible(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	t.Run("quarantined", func(t *testing.T) {
		dir := moduleDir(t, "example.com/lib", "v1.0.0")
		quarantineDays := 7
		newestRelease := now.Add(-24 * time.Hour)
		adapter := fakeSelectionAdapter(now, []string{"v1.1.0", "v1.2.0"}, map[string]candidate{
			"v1.2.0": trustedRelease(newestRelease),
			"v1.1.0": trustedRelease(now.Add(-48 * time.Hour)),
		})

		plan := adapter.PlanTarget(context.Background(), target(dir, config.Policy{QuarantineDays: &quarantineDays, Pins: map[string]string{}}))
		decision := onlyDecision(t, plan)
		if decision.Eligible || decision.BlockedReason != core.ReasonQuarantined {
			t.Fatalf("decision = %+v, want quarantined blocked decision", decision)
		}
		if decision.CandidateVersion != "v1.2.0" {
			t.Fatalf("candidate = %s, want newest blocked v1.2.0", decision.CandidateVersion)
		}
		if decision.ReleaseTime == nil || !decision.ReleaseTime.Equal(newestRelease) {
			t.Fatalf("release time = %v, want newest blocked candidate release time", decision.ReleaseTime)
		}
	})

	t.Run("metadata blocked", func(t *testing.T) {
		dir := moduleDir(t, "example.com/lib", "v1.0.0")
		quarantineDays := 0
		adapter := fakeSelectionAdapter(now, []string{"v1.1.0", "v1.2.0"}, map[string]candidate{
			"v1.2.0": {Version: "v1.2.0"},
			"v1.1.0": {Version: "v1.1.0", ReleaseTime: timePtr(now.Add(-24 * time.Hour))},
		})

		plan := adapter.PlanTarget(context.Background(), target(dir, config.Policy{QuarantineDays: &quarantineDays, Pins: map[string]string{}}))
		decision := onlyDecision(t, plan)
		if decision.Eligible || decision.BlockedReason != core.ReasonMissingReleaseDate {
			t.Fatalf("decision = %+v, want missing release-date blocked decision", decision)
		}
		if decision.CandidateVersion != "v1.2.0" {
			t.Fatalf("candidate = %s, want newest blocked v1.2.0", decision.CandidateVersion)
		}
	})
}

func TestPlanTargetDependencyLevelBlockersStopFallback(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	release := trustedRelease(now.Add(-24 * time.Hour))

	cases := []struct {
		name    string
		policy  config.Policy
		reason  core.Reason
		current string
	}{
		{
			name:    "pinned",
			policy:  config.Policy{Pins: map[string]string{"example.com/lib": "v1.0.0"}},
			reason:  core.ReasonPinned,
			current: "v1.0.0",
		},
		{
			name:    "pin mismatch",
			policy:  config.Policy{Pins: map[string]string{"example.com/lib": "v1.0.1"}},
			reason:  core.ReasonPinMismatch,
			current: "v1.0.0",
		},
		{
			name:    "denied",
			policy:  config.Policy{Deny: []string{"example.com/lib"}, Pins: map[string]string{}},
			reason:  core.ReasonDenied,
			current: "v1.0.0",
		},
		{
			name:    "not allowed",
			policy:  config.Policy{AllowSet: true, Allow: []string{"example.com/other"}, Pins: map[string]string{}},
			reason:  core.ReasonNotAllowed,
			current: "v1.0.0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := moduleDir(t, "example.com/lib", tc.current)
			adapter := fakeSelectionAdapter(now, []string{"v1.1.0", "v1.2.0"}, map[string]candidate{
				"v1.2.0": release,
				"v1.1.0": release,
			})

			plan := adapter.PlanTarget(context.Background(), target(dir, tc.policy))
			decision := onlyDecision(t, plan)
			if decision.BlockedReason != tc.reason {
				t.Fatalf("reason = %s, want %s; decision=%+v", decision.BlockedReason, tc.reason, decision)
			}
			if decision.Eligible {
				t.Fatalf("decision = %+v, want blocked", decision)
			}
		})
	}
}

func TestPlanTargetVersionTrackSelection(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	quarantineDays := 0

	t.Run("stable current ignores prereleases", func(t *testing.T) {
		dir := moduleDir(t, "example.com/lib", "v1.0.0")
		adapter := fakeSelectionAdapter(now, []string{"v1.1.0", "v1.2.0-rc.1"}, map[string]candidate{
			"v1.2.0-rc.1": trustedRelease(now.Add(-24 * time.Hour)),
			"v1.1.0":      trustedRelease(now.Add(-24 * time.Hour)),
		})

		plan := adapter.PlanTarget(context.Background(), target(dir, config.Policy{QuarantineDays: &quarantineDays, Pins: map[string]string{}}))
		decision := onlyDecision(t, plan)
		if !decision.Eligible || decision.CandidateVersion != "v1.1.0" {
			t.Fatalf("decision = %+v, want stable v1.1.0", decision)
		}
	})

	t.Run("prerelease current keeps existing candidate track with fallback", func(t *testing.T) {
		dir := moduleDir(t, "example.com/lib", "v1.2.0-rc.1")
		adapter := fakeSelectionAdapter(now, []string{"v1.2.0-rc.2", "v1.2.0"}, map[string]candidate{
			"v1.2.0":      trustedRelease(now.Add(time.Hour)),
			"v1.2.0-rc.2": trustedRelease(now.Add(-24 * time.Hour)),
		})

		plan := adapter.PlanTarget(context.Background(), target(dir, config.Policy{QuarantineDays: &quarantineDays, Pins: map[string]string{}}))
		decision := onlyDecision(t, plan)
		if !decision.Eligible || decision.CandidateVersion != "v1.2.0-rc.2" {
			t.Fatalf("decision = %+v, want prerelease fallback v1.2.0-rc.2", decision)
		}
	})
}

func TestPlanTargetVersionResolutionFailures(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	t.Run("invalid current version", func(t *testing.T) {
		adapter := fakeSelectionAdapter(now, nil, nil)

		decision, show := adapter.selectDecision(context.Background(), target(t.TempDir(), config.Policy{Pins: map[string]string{}}), "example.com/lib", "not-a-version", now)
		if !show {
			t.Fatal("show = false, want candidate resolution failure")
		}
		if decision.BlockedReason != core.ReasonCandidateResolutionFail {
			t.Fatalf("decision = %+v, want candidate resolution failure", decision)
		}
	})

	t.Run("version list failure", func(t *testing.T) {
		dir := moduleDir(t, "example.com/lib", "v1.0.0")
		adapter := &Adapter{
			Now: func() time.Time { return now },
			listVersions: func(context.Context, string, string) ([]string, error) {
				return nil, errors.New("list failed")
			},
		}

		plan := adapter.PlanTarget(context.Background(), target(dir, config.Policy{Pins: map[string]string{}}))
		decision := onlyDecision(t, plan)
		if decision.BlockedReason != core.ReasonCandidateResolutionFail || decision.Message != "list failed" {
			t.Fatalf("decision = %+v, want candidate resolution failure with message", decision)
		}
	})
}

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

func moduleDir(t *testing.T, modulePath string, version string) string {
	t.Helper()
	dir := t.TempDir()
	writeGoMod(t, dir, `module example.com/app

go 1.25.0

require `+modulePath+` `+version+`
`)
	return dir
}

func target(dir string, policy config.Policy) config.Target {
	return config.Target{
		Name:           "app",
		Ecosystem:      "go",
		NormalizedPath: ".",
		AbsPath:        dir,
		Policy:         policy,
	}
}

func fakeSelectionAdapter(now time.Time, versions []string, releases map[string]candidate) *Adapter {
	return &Adapter{
		Now: func() time.Time {
			return now
		},
		listVersions: func(context.Context, string, string) ([]string, error) {
			return versions, nil
		},
		releaseLookup: func(_ context.Context, _ string, _ string, version string) candidate {
			if release, ok := releases[version]; ok {
				if release.Version == "" {
					release.Version = version
				}
				return release
			}
			return candidate{Version: version}
		},
	}
}

func trustedRelease(release time.Time) candidate {
	return candidate{ReleaseTime: &release, ReleaseTrusted: true}
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func onlyDecision(t *testing.T, plan core.TargetPlan) core.Decision {
	t.Helper()
	if plan.Error != "" {
		t.Fatalf("plan error = %s", plan.Error)
	}
	if len(plan.Decisions) != 1 {
		t.Fatalf("decisions = %+v, want exactly one", plan.Decisions)
	}
	return plan.Decisions[0]
}

func writeGoMod(t *testing.T, dir string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
}
