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

func TestPlanTargetIncludesIndirectRequirementsOnlyWhenTargetOptsIn(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	dir := mixedRequirementModuleDir(t)
	releases := map[string]candidate{
		"v1.2.0": trustedRelease(now.Add(-24 * time.Hour)),
		"v1.3.0": trustedRelease(now.Add(-24 * time.Hour)),
	}
	versions := map[string][]string{
		"example.com/direct":   {"v1.0.0", "v1.2.0"},
		"example.com/indirect": {"v1.0.0", "v1.3.0"},
	}

	t.Run("omitted option plans only direct requirements", func(t *testing.T) {
		adapter := fakeModuleSelectionAdapter(now, versions, releases)

		plan := adapter.PlanTarget(context.Background(), target(dir, config.Policy{Pins: map[string]string{}}))
		if plan.Error != "" {
			t.Fatalf("plan error = %s", plan.Error)
		}
		if got := planModulePaths(plan); len(got) != 1 || got[0] != "example.com/direct" {
			t.Fatalf("planned modules = %+v, want direct module only", got)
		}
		if got := adapter.versionLookups; len(got) != 1 || got[0] != "example.com/direct" {
			t.Fatalf("version lookups = %+v, want no indirect lookup", got)
		}
	})

	t.Run("explicit false plans only direct requirements", func(t *testing.T) {
		adapter := fakeModuleSelectionAdapter(now, versions, releases)
		target := target(dir, config.Policy{Pins: map[string]string{}})
		target.IncludeIndirect = false

		plan := adapter.PlanTarget(context.Background(), target)
		if plan.Error != "" {
			t.Fatalf("plan error = %s", plan.Error)
		}
		if got := planModulePaths(plan); len(got) != 1 || got[0] != "example.com/direct" {
			t.Fatalf("planned modules = %+v, want direct module only", got)
		}
		if got := adapter.versionLookups; len(got) != 1 || got[0] != "example.com/direct" {
			t.Fatalf("version lookups = %+v, want no indirect lookup", got)
		}
	})

	t.Run("true plans direct and explicitly listed indirect requirements", func(t *testing.T) {
		adapter := fakeModuleSelectionAdapter(now, versions, releases)

		plan := adapter.PlanTarget(context.Background(), targetIncludingIndirect(dir, config.Policy{Pins: map[string]string{}}))
		if plan.Error != "" {
			t.Fatalf("plan error = %s", plan.Error)
		}
		if got := planModulePaths(plan); len(got) != 2 || got[0] != "example.com/direct" || got[1] != "example.com/indirect" {
			t.Fatalf("planned modules = %+v, want direct and indirect modules sorted by path", got)
		}
		if got := plan.Decisions[0]; got.Relationship != core.RelationshipDirect || got.CandidateVersion != "v1.2.0" {
			t.Fatalf("direct decision = %+v, want direct v1.2.0", got)
		}
		if got := plan.Decisions[1]; got.Relationship != core.RelationshipIndirect || got.CandidateVersion != "v1.3.0" {
			t.Fatalf("indirect decision = %+v, want indirect v1.3.0", got)
		}
		if got := adapter.versionLookups; len(got) != 2 || got[0] != "example.com/direct" || got[1] != "example.com/indirect" {
			t.Fatalf("version lookups = %+v, want sorted direct and indirect lookups", got)
		}
	})
}

func TestPlanTargetIndirectRequirementsUseCandidateSelectionRules(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	quarantineDays := 7
	dir := indirectOnlyModuleDir(t, "example.com/indirect", "v1.0.0", "")
	adapter := fakeModuleSelectionAdapter(now, map[string][]string{
		"example.com/indirect": {"v1.0.0", "v1.1.0", "v1.2.0-rc.1", "v1.2.0"},
	}, map[string]candidate{
		"v1.2.0": trustedRelease(now.Add(-24 * time.Hour)),
		"v1.1.0": trustedRelease(now.Add(-10 * 24 * time.Hour)),
	})

	plan := adapter.PlanTarget(context.Background(), targetIncludingIndirect(dir, config.Policy{QuarantineDays: &quarantineDays, Pins: map[string]string{}}))
	decision := onlyDecision(t, plan)
	if decision.Relationship != core.RelationshipIndirect {
		t.Fatalf("relationship = %s, want indirect", decision.Relationship)
	}
	if !decision.Eligible || decision.CandidateVersion != "v1.1.0" {
		t.Fatalf("decision = %+v, want stable non-quarantined fallback v1.1.0", decision)
	}
}

func TestPlanTargetPolicyAndReplacementApplyToIndirectRequirements(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	releases := map[string]candidate{
		"v1.1.0": trustedRelease(now.Add(-24 * time.Hour)),
	}
	versions := map[string][]string{
		"example.com/indirect": {"v1.0.0", "v1.1.0"},
	}

	cases := []struct {
		name   string
		policy config.Policy
		reason core.Reason
	}{
		{
			name:   "pinned",
			policy: config.Policy{Pins: map[string]string{"example.com/indirect": "v1.0.0"}},
			reason: core.ReasonPinned,
		},
		{
			name:   "pin mismatch",
			policy: config.Policy{Pins: map[string]string{"example.com/indirect": "v1.0.1"}},
			reason: core.ReasonPinMismatch,
		},
		{
			name:   "denied",
			policy: config.Policy{Deny: []string{"example.com/indirect"}, Pins: map[string]string{}},
			reason: core.ReasonDenied,
		},
		{
			name:   "not allowed",
			policy: config.Policy{AllowSet: true, Allow: []string{"example.com/direct"}, Pins: map[string]string{}},
			reason: core.ReasonNotAllowed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := indirectOnlyModuleDir(t, "example.com/indirect", "v1.0.0", "")
			adapter := fakeModuleSelectionAdapter(now, versions, releases)

			plan := adapter.PlanTarget(context.Background(), targetIncludingIndirect(dir, tc.policy))
			decision := onlyDecision(t, plan)
			if decision.Relationship != core.RelationshipIndirect {
				t.Fatalf("relationship = %s, want indirect; decision=%+v", decision.Relationship, decision)
			}
			if decision.BlockedReason != tc.reason {
				t.Fatalf("blocked reason = %s, want %s; decision=%+v", decision.BlockedReason, tc.reason, decision)
			}
		})
	}

	t.Run("replaced", func(t *testing.T) {
		dir := indirectOnlyModuleDir(t, "example.com/indirect", "v1.0.0", "\nreplace example.com/indirect => ../indirect\n")
		adapter := fakeModuleSelectionAdapter(now, versions, releases)

		plan := adapter.PlanTarget(context.Background(), targetIncludingIndirect(dir, config.Policy{Pins: map[string]string{}}))
		decision := onlyDecision(t, plan)
		if decision.Relationship != core.RelationshipIndirect || decision.BlockedReason != core.ReasonReplacedDependency {
			t.Fatalf("decision = %+v, want replaced indirect decision", decision)
		}
		if len(adapter.versionLookups) != 0 {
			t.Fatalf("version lookups = %+v, replaced dependency should be blocked before candidate resolution", adapter.versionLookups)
		}
	})
}

func TestPlanTargetNormalModeDoesNotInvokeVulnerabilityScanner(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	dir := moduleDir(t, "example.com/lib", "v1.0.0")
	scanner := &fakeVulnerabilityScanner{err: errors.New("scanner should not run")}
	adapter := fakeSelectionAdapter(now, []string{"v1.0.0", "v1.1.0"}, map[string]candidate{
		"v1.1.0": trustedRelease(now.Add(-24 * time.Hour)),
	})
	adapter.vulnerabilityScanner = scanner

	plan := adapter.PlanTarget(context.Background(), target(dir, config.Policy{Pins: map[string]string{}}))
	if plan.Error != "" {
		t.Fatalf("plan error = %s", plan.Error)
	}
	if scanner.calls != 0 {
		t.Fatalf("scanner calls = %d, want 0", scanner.calls)
	}
	decision := onlyDecision(t, plan)
	if !decision.Eligible || decision.CandidateVersion != "v1.1.0" {
		t.Fatalf("decision = %+v, want normal eligible update", decision)
	}
}

func TestPlanTargetVulnerabilityOnlyScannerFailureIsTargetFailure(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	dir := moduleDir(t, "example.com/lib", "v1.0.0")
	scanner := &fakeVulnerabilityScanner{err: errors.New("scan failed")}
	adapter := fakeSelectionAdapter(now, []string{"v1.0.0", "v1.1.0"}, nil)
	adapter.vulnerabilityScanner = scanner

	plan := adapter.PlanTarget(context.Background(), target(dir, vulnerabilityOnlyPolicy(config.Policy{Pins: map[string]string{}})))
	if plan.Error != "scan failed" {
		t.Fatalf("plan error = %q, want scanner failure", plan.Error)
	}
	if scanner.calls != 1 {
		t.Fatalf("scanner calls = %d, want 1", scanner.calls)
	}
}

func TestPlanTargetVulnerabilityOnlyDirectRemediation(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	dir := moduleDir(t, "example.com/lib", "v1.0.0")
	adapter := fakeSelectionAdapter(now, []string{"v1.0.0", "v1.1.0"}, map[string]candidate{
		"v1.1.0": trustedRelease(now.Add(-24 * time.Hour)),
	})
	adapter.vulnerabilityScanner = &fakeVulnerabilityScanner{vulnerabilities: []core.Vulnerability{
		vulnerability("example.com/lib", "v1.0.0", "v1.1.0", "GO-2026-0001"),
	}}

	plan := adapter.PlanTarget(context.Background(), target(dir, vulnerabilityOnlyPolicy(config.Policy{Pins: map[string]string{}})))
	decision := onlyDecision(t, plan)
	if !decision.Eligible || decision.CandidateVersion != "v1.1.0" {
		t.Fatalf("decision = %+v, want vulnerable direct eligible v1.1.0", decision)
	}
	if len(decision.Vulnerabilities) != 1 || decision.Vulnerabilities[0].AdvisoryIDs[0] != "GO-2026-0001" {
		t.Fatalf("vulnerabilities = %+v, want advisory context", decision.Vulnerabilities)
	}
}

func TestPlanTargetVulnerabilityOnlyOmitsNonCurrentAndOutOfScopeFindings(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	dir := moduleDir(t, "example.com/lib", "v1.0.0")
	adapter := fakeModuleSelectionAdapter(now, map[string][]string{
		"example.com/lib": {"v1.0.0", "v1.1.0"},
	}, map[string]candidate{
		"v1.1.0": trustedRelease(now.Add(-24 * time.Hour)),
	})
	adapter.vulnerabilityScanner = &fakeVulnerabilityScanner{vulnerabilities: []core.Vulnerability{
		vulnerability("example.com/lib", "v0.9.0", "v1.1.0", "GO-2026-0001"),
		vulnerability("example.com/transitive", "v1.0.0", "v1.1.0", "GO-2026-0002"),
	}}

	plan := adapter.PlanTarget(context.Background(), target(dir, vulnerabilityOnlyPolicy(config.Policy{Pins: map[string]string{}})))
	if plan.Error != "" {
		t.Fatalf("plan error = %s", plan.Error)
	}
	if len(plan.Decisions) != 0 {
		t.Fatalf("decisions = %+v, want no in-scope remediation", plan.Decisions)
	}
	if len(adapter.versionLookups) != 0 {
		t.Fatalf("version lookups = %+v, want no candidate lookup for out-of-scope findings", adapter.versionLookups)
	}
}

func TestPlanTargetVulnerabilityOnlyRequiresFixingCandidate(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	dir := moduleDir(t, "example.com/lib", "v1.0.0")
	adapter := fakeSelectionAdapter(now, []string{"v1.0.0", "v1.1.0"}, map[string]candidate{
		"v1.1.0": trustedRelease(now.Add(-24 * time.Hour)),
	})
	adapter.vulnerabilityScanner = &fakeVulnerabilityScanner{vulnerabilities: []core.Vulnerability{
		vulnerability("example.com/lib", "v1.0.0", "v1.2.0", "GO-2026-0001"),
	}}

	plan := adapter.PlanTarget(context.Background(), target(dir, vulnerabilityOnlyPolicy(config.Policy{Pins: map[string]string{}})))
	if plan.Error != "" {
		t.Fatalf("plan error = %s", plan.Error)
	}
	if len(plan.Decisions) != 0 {
		t.Fatalf("decisions = %+v, want no update because newer candidate does not fix", plan.Decisions)
	}
}

func TestPlanTargetVulnerabilityOnlyKeepsPolicyBlockers(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	release := trustedRelease(now.Add(-24 * time.Hour))

	cases := []struct {
		name   string
		policy config.Policy
		reason core.Reason
	}{
		{
			name:   "pinned",
			policy: config.Policy{Pins: map[string]string{"example.com/lib": "v1.0.0"}},
			reason: core.ReasonPinned,
		},
		{
			name:   "denied",
			policy: config.Policy{Deny: []string{"example.com/lib"}, Pins: map[string]string{}},
			reason: core.ReasonDenied,
		},
		{
			name:   "not allowed",
			policy: config.Policy{AllowSet: true, Allow: []string{"example.com/other"}, Pins: map[string]string{}},
			reason: core.ReasonNotAllowed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := moduleDir(t, "example.com/lib", "v1.0.0")
			adapter := fakeSelectionAdapter(now, []string{"v1.0.0", "v1.1.0"}, map[string]candidate{"v1.1.0": release})
			adapter.vulnerabilityScanner = &fakeVulnerabilityScanner{vulnerabilities: []core.Vulnerability{
				vulnerability("example.com/lib", "v1.0.0", "v1.1.0", "GO-2026-0001"),
			}}

			decision := onlyDecision(t, adapter.PlanTarget(context.Background(), target(dir, vulnerabilityOnlyPolicy(tc.policy))))
			if decision.Eligible || decision.BlockedReason != tc.reason {
				t.Fatalf("decision = %+v, want blocked reason %s", decision, tc.reason)
			}
			if len(decision.Vulnerabilities) != 1 {
				t.Fatalf("vulnerabilities = %+v, want blocker context", decision.Vulnerabilities)
			}
		})
	}
}

func TestPlanTargetVulnerabilityOnlyQuarantineBlocksFixingCandidate(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	quarantineDays := 7
	dir := moduleDir(t, "example.com/lib", "v1.0.0")
	adapter := fakeSelectionAdapter(now, []string{"v1.0.0", "v1.1.0", "v1.2.0"}, map[string]candidate{
		"v1.2.0": trustedRelease(now.Add(-24 * time.Hour)),
		"v1.1.0": trustedRelease(now.Add(-10 * 24 * time.Hour)),
	})
	adapter.vulnerabilityScanner = &fakeVulnerabilityScanner{vulnerabilities: []core.Vulnerability{
		vulnerability("example.com/lib", "v1.0.0", "v1.2.0", "GO-2026-0001"),
	}}

	plan := adapter.PlanTarget(context.Background(), target(dir, vulnerabilityOnlyPolicy(config.Policy{QuarantineDays: &quarantineDays, Pins: map[string]string{}})))
	decision := onlyDecision(t, plan)
	if decision.Eligible || decision.BlockedReason != core.ReasonQuarantined || decision.CandidateVersion != "v1.2.0" {
		t.Fatalf("decision = %+v, want fixing candidate blocked by quarantine", decision)
	}
}

func TestPlanTargetVulnerabilityOnlyIndirectOptIn(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	dir := mixedRequirementModuleDir(t)
	versions := map[string][]string{
		"example.com/direct":   {"v1.0.0", "v1.1.0"},
		"example.com/indirect": {"v1.0.0", "v1.2.0"},
	}
	releases := map[string]candidate{
		"v1.1.0": trustedRelease(now.Add(-24 * time.Hour)),
		"v1.2.0": trustedRelease(now.Add(-24 * time.Hour)),
	}
	vulns := []core.Vulnerability{
		vulnerability("example.com/direct", "v1.0.0", "v1.1.0", "GO-2026-0001"),
		vulnerability("example.com/indirect", "v1.0.0", "v1.2.0", "GO-2026-0002"),
	}

	t.Run("disabled", func(t *testing.T) {
		adapter := fakeModuleSelectionAdapter(now, versions, releases)
		adapter.vulnerabilityScanner = &fakeVulnerabilityScanner{vulnerabilities: vulns}

		plan := adapter.PlanTarget(context.Background(), target(dir, vulnerabilityOnlyPolicy(config.Policy{Pins: map[string]string{}})))
		if got := planModulePaths(plan); len(got) != 1 || got[0] != "example.com/direct" {
			t.Fatalf("planned modules = %+v, want direct only", got)
		}
	})

	t.Run("enabled", func(t *testing.T) {
		adapter := fakeModuleSelectionAdapter(now, versions, releases)
		adapter.vulnerabilityScanner = &fakeVulnerabilityScanner{vulnerabilities: vulns}

		plan := adapter.PlanTarget(context.Background(), targetIncludingIndirect(dir, vulnerabilityOnlyPolicy(config.Policy{Pins: map[string]string{}})))
		if got := planModulePaths(plan); len(got) != 2 || got[0] != "example.com/direct" || got[1] != "example.com/indirect" {
			t.Fatalf("planned modules = %+v, want direct and indirect", got)
		}
		if got := plan.Decisions[1].Relationship; got != core.RelationshipIndirect {
			t.Fatalf("indirect relationship = %s, want indirect", got)
		}
	})
}

func TestModuleStateRequirementsDirectRelationshipIsAuthoritative(t *testing.T) {
	state := moduleState{
		Direct:   map[string]string{"example.com/lib": "v1.1.0"},
		Indirect: map[string]string{"example.com/lib": "v1.0.0"},
	}

	requirements := state.Requirements(true)
	if len(requirements) != 1 {
		t.Fatalf("requirements = %+v, want one direct requirement", requirements)
	}
	if got := requirements[0]; got.Relationship != core.RelationshipDirect || got.Version != "v1.1.0" {
		t.Fatalf("requirement = %+v, want direct relationship to be authoritative", got)
	}
}

func TestPlanTargetVersionResolutionFailures(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	t.Run("invalid current version", func(t *testing.T) {
		adapter := fakeSelectionAdapter(now, nil, nil)

		decision, show := adapter.selectDecision(context.Background(), target(t.TempDir(), config.Policy{Pins: map[string]string{}}), requirement("example.com/lib", "not-a-version", core.RelationshipDirect), now, nil)
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

func mixedRequirementModuleDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeGoMod(t, dir, `module example.com/app

go 1.25.0

require (
	example.com/indirect v1.0.0 // indirect
	example.com/direct v1.0.0
)
`)
	return dir
}

func indirectOnlyModuleDir(t *testing.T, modulePath string, version string, extra string) string {
	t.Helper()
	dir := t.TempDir()
	writeGoMod(t, dir, `module example.com/app

go 1.25.0

require `+modulePath+` `+version+` // indirect
`+extra)
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

func targetIncludingIndirect(dir string, policy config.Policy) config.Target {
	target := target(dir, policy)
	target.IncludeIndirect = true
	return target
}

func requirement(modulePath string, version string, relationship core.DependencyRelationship) dependencyRequirement {
	return dependencyRequirement{Path: modulePath, Version: version, Relationship: relationship}
}

type moduleSelectionAdapter struct {
	*Adapter
	versionLookups []string
	releaseLookups []string
}

type fakeVulnerabilityScanner struct {
	vulnerabilities []core.Vulnerability
	err             error
	calls           int
}

func (s *fakeVulnerabilityScanner) Scan(context.Context, config.Target) ([]core.Vulnerability, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.vulnerabilities, nil
}

func fakeModuleSelectionAdapter(now time.Time, versions map[string][]string, releases map[string]candidate) *moduleSelectionAdapter {
	fake := &moduleSelectionAdapter{}
	fake.Adapter = &Adapter{
		Now: func() time.Time {
			return now
		},
		listVersions: func(_ context.Context, _ string, modulePath string) ([]string, error) {
			fake.versionLookups = append(fake.versionLookups, modulePath)
			moduleVersions, ok := versions[modulePath]
			if !ok {
				return nil, errors.New("unexpected version lookup for " + modulePath)
			}
			return moduleVersions, nil
		},
		releaseLookup: func(_ context.Context, _ string, modulePath string, version string) candidate {
			fake.releaseLookups = append(fake.releaseLookups, modulePath+"@"+version)
			if release, ok := releases[version]; ok {
				if release.Version == "" {
					release.Version = version
				}
				return release
			}
			return candidate{Version: version}
		},
	}
	return fake
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

func vulnerabilityOnlyPolicy(policy config.Policy) config.Policy {
	policy.UpdateMode = config.UpdateModeVulnerabilityOnly
	if policy.Pins == nil {
		policy.Pins = map[string]string{}
	}
	return policy
}

func vulnerability(modulePath string, affectedVersion string, fixedVersion string, advisoryID string) core.Vulnerability {
	return core.Vulnerability{
		ModulePath:      modulePath,
		AffectedVersion: affectedVersion,
		FixedVersions:   []string{fixedVersion},
		AdvisoryIDs:     []string{advisoryID},
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

func planModulePaths(plan core.TargetPlan) []string {
	paths := make([]string, 0, len(plan.Decisions))
	for _, decision := range plan.Decisions {
		paths = append(paths, decision.ModulePath)
	}
	return paths
}

func writeGoMod(t *testing.T, dir string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
}
