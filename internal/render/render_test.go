package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mishankov/updtr/internal/config"
	"github.com/mishankov/updtr/internal/core"
)

func TestNormalizeDetectRowsIncludesEligibleBlockedWarningAndError(t *testing.T) {
	release := time.Date(2026, 4, 6, 10, 0, 0, 0, time.FixedZone("UTC+3", 3*60*60))
	result := core.RunResult{
		Mode: "detect",
		Targets: []core.TargetResult{{
			Target: config.Target{Name: "app", NormalizedPath: "."},
			Plan: core.TargetPlan{
				Decisions: []core.Decision{
					{
						ModulePath:       "github.com/a/direct",
						CurrentVersion:   "v1.0.0",
						CandidateVersion: "v1.1.0",
						Eligible:         true,
						Vulnerabilities:  []core.Vulnerability{{AdvisoryIDs: []string{"GO-2026-0001"}}},
					},
					{
						ModulePath:       "github.com/a/pinned",
						CurrentVersion:   "v0.9.0",
						CandidateVersion: "v1.2.0",
						PinVersion:       "v1.0.0",
						BlockedReason:    core.ReasonPinMismatch,
					},
					{
						ModulePath:       "github.com/a/quarantine",
						CurrentVersion:   "v1.0.0",
						CandidateVersion: "v1.1.0",
						BlockedReason:    core.ReasonQuarantined,
						ReleaseTime:      &release,
					},
				},
			},
			Warnings: []string{core.WarningAdditionalDirectChanges},
			Error:    "apply failed",
		}},
	}

	rows := Normalize(result)
	want := []Row{
		{
			Target:          "app (.)",
			Module:          "github.com/a/direct",
			FromVersion:     "v1.0.0",
			ToVersion:       "v1.1.0",
			Status:          StatusEligible,
			Reason:          "update available",
			Vulnerabilities: "GO-2026-0001",
		},
		{
			Target:      "app (.)",
			Module:      "github.com/a/pinned",
			FromVersion: "v0.9.0",
			ToVersion:   "v1.0.0",
			Status:      StatusBlocked,
			Reason:      "pin_mismatch (pin v1.0.0)",
		},
		{
			Target:      "app (.)",
			Module:      "github.com/a/quarantine",
			FromVersion: "v1.0.0",
			ToVersion:   "v1.1.0",
			Status:      StatusBlocked,
			Reason:      "quarantined released 2026-04-06",
		},
		{
			Target: "app (.)",
			Status: StatusWarning,
			Reason: core.WarningAdditionalDirectChanges,
		},
		{
			Target: "app (.)",
			Status: StatusError,
			Reason: "apply failed",
		},
	}
	assertRowsEqual(t, rows, want)
}

func TestNormalizeApplyPromotesAppliedRowsAndKeepsRemainingEligible(t *testing.T) {
	result := core.RunResult{
		Mode: "apply",
		Targets: []core.TargetResult{{
			Target: config.Target{Name: "worker", NormalizedPath: "worker"},
			Plan: core.TargetPlan{
				Decisions: []core.Decision{
					{
						ModulePath:       "github.com/a/applied",
						CurrentVersion:   "v1.0.0",
						CandidateVersion: "v1.1.0",
						Eligible:         true,
					},
					{
						ModulePath:       "github.com/a/remaining",
						CurrentVersion:   "v1.5.0",
						CandidateVersion: "v1.6.0",
						Eligible:         true,
					},
					{
						ModulePath:       "github.com/a/blocked",
						CurrentVersion:   "v1.0.0",
						CandidateVersion: "v1.1.0",
						Relationship:     core.RelationshipIndirect,
						BlockedReason:    core.ReasonDenied,
					},
				},
			},
			Applied: []core.AppliedUpdate{{
				ModulePath:      "github.com/a/applied",
				FromVersion:     "v1.0.0",
				ToVersion:       "v1.1.0",
				Vulnerabilities: []core.Vulnerability{{AdvisoryIDs: []string{"GO-2026-0002"}}},
			}},
		}},
	}

	rows := Normalize(result)
	want := []Row{
		{
			Target:          "worker (worker)",
			Module:          "github.com/a/applied",
			FromVersion:     "v1.0.0",
			ToVersion:       "v1.1.0",
			Status:          StatusApplied,
			Vulnerabilities: "GO-2026-0002",
		},
		{
			Target:      "worker (worker)",
			Module:      "github.com/a/remaining",
			FromVersion: "v1.5.0",
			ToVersion:   "v1.6.0",
			Status:      StatusEligible,
		},
		{
			Target:      "worker (worker)",
			Module:      "github.com/a/blocked (indirect)",
			FromVersion: "v1.0.0",
			ToVersion:   "v1.1.0",
			Status:      StatusBlocked,
			Reason:      "denied",
		},
	}
	assertRowsEqual(t, rows, want)
}

func TestNormalizeAddsNoopRowForEmptyTarget(t *testing.T) {
	result := core.RunResult{
		Mode: "detect",
		Targets: []core.TargetResult{{
			Target: config.Target{Name: "app", NormalizedPath: "."},
		}},
	}

	rows := Normalize(result)
	want := []Row{{
		Target: "app (.)",
		Status: StatusNoop,
		Reason: "no dependency changes detected",
	}}
	assertRowsEqual(t, rows, want)
}

func TestReportRendersASCIITable(t *testing.T) {
	result := core.RunResult{
		Mode: "detect",
		Targets: []core.TargetResult{{
			Target: config.Target{Name: "app", NormalizedPath: "."},
			Plan: core.TargetPlan{
				Decisions: []core.Decision{{
					ModulePath:       "github.com/a/direct",
					CurrentVersion:   "v1.0.0",
					CandidateVersion: "v1.1.0",
					Eligible:         true,
				}},
			},
		}},
	}

	var out bytes.Buffer
	Report(&out, result, Options{})
	want := `+---------+---------------------+--------+--------+----------+------------------+-----------------+
| TARGET  | MODULE              | FROM   | TO     | STATUS   | REASON           | VULNERABILITIES |
+---------+---------------------+--------+--------+----------+------------------+-----------------+
| app (.) | github.com/a/direct | v1.0.0 | v1.1.0 | ELIGIBLE | update available |                 |
+---------+---------------------+--------+--------+----------+------------------+-----------------+
`
	if got := out.String(); got != want {
		t.Fatalf("output:\n%s\nwant:\n%s", got, want)
	}
}

func TestReportColorsStatusWhenEnabled(t *testing.T) {
	result := core.RunResult{
		Mode: "apply",
		Targets: []core.TargetResult{{
			Target: config.Target{Name: "app", NormalizedPath: "."},
			Applied: []core.AppliedUpdate{{
				ModulePath:   "github.com/a/direct",
				FromVersion:  "v1.0.0",
				ToVersion:    "v1.1.0",
				Relationship: core.RelationshipDirect,
			}},
		}},
	}

	var out bytes.Buffer
	Report(&out, result, Options{Color: true})
	got := out.String()
	if !strings.Contains(got, "\x1b[32mAPPLIED\x1b[0m") {
		t.Fatalf("output = %q, want colored applied status", got)
	}
}

func assertRowsEqual(t *testing.T, got []Row, want []Row) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("rows = %+v, want %d rows", got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
