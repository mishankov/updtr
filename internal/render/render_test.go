package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mishankov/updtr/internal/config"
	"github.com/mishankov/updtr/internal/core"
)

func TestDetectPinMismatchShowsPinVersion(t *testing.T) {
	result := runResultWithDecision(core.Decision{
		ModulePath:       "github.com/a/pinned",
		CurrentVersion:   "v0.9.0",
		CandidateVersion: "v1.2.0",
		PinVersion:       "v1.0.0",
		BlockedReason:    core.ReasonPinMismatch,
	})

	var out bytes.Buffer
	Detect(&out, result)
	got := out.String()
	if !strings.Contains(got, "github.com/a/pinned v0.9.0 (pin v1.0.0): pin_mismatch") {
		t.Fatalf("output = %q, want pin version in pin mismatch row", got)
	}
	if strings.Contains(got, "github.com/a/pinned v0.9.0 -> v1.2.0: pin_mismatch") {
		t.Fatalf("output = %q, want pin mismatch row not to prefer candidate version", got)
	}
}

func TestDetectQuarantinedBlockedDecisionShowsReleaseDate(t *testing.T) {
	release := time.Date(2026, 4, 6, 10, 0, 0, 0, time.FixedZone("UTC+3", 3*60*60))
	result := runResultWithDecision(core.Decision{
		ModulePath:       "github.com/a/lib",
		CurrentVersion:   "v1.0.0",
		CandidateVersion: "v1.1.0",
		ReleaseTime:      &release,
		BlockedReason:    core.ReasonQuarantined,
	})

	var out bytes.Buffer
	Detect(&out, result)
	if got := out.String(); !strings.Contains(got, "github.com/a/lib v1.0.0 -> v1.1.0: quarantined released 2026-04-06") {
		t.Fatalf("output = %q, want release date in quarantined row", got)
	}
}

func TestDetectDirectOutputContractStaysUnchanged(t *testing.T) {
	result := core.RunResult{
		Mode: "detect",
		Targets: []core.TargetResult{
			{
				Target: config.Target{
					Name:           "app",
					NormalizedPath: ".",
				},
				Plan: core.TargetPlan{
					Decisions: []core.Decision{
						{
							ModulePath:       "github.com/a/direct",
							CurrentVersion:   "v1.0.0",
							CandidateVersion: "v1.1.0",
							Eligible:         true,
						},
					},
				},
			},
		},
	}

	var out bytes.Buffer
	Detect(&out, result)
	want := `Target app (.)
Eligible:
  - github.com/a/direct v1.0.0 -> v1.1.0
Summary: eligible=1 blocked=0 applied=0 errors=0

Total: targets=1 eligible=1 blocked=0 applied=0 errors=0
`
	if got := out.String(); got != want {
		t.Fatalf("output:\n%s\nwant:\n%s", got, want)
	}
}

func TestDetectLabelsIndirectDecisions(t *testing.T) {
	result := core.RunResult{
		Mode: "detect",
		Targets: []core.TargetResult{
			{
				Target: config.Target{
					Name:           "app",
					NormalizedPath: ".",
				},
				Plan: core.TargetPlan{
					Decisions: []core.Decision{
						{
							ModulePath:       "github.com/a/direct",
							CurrentVersion:   "v1.0.0",
							CandidateVersion: "v1.1.0",
							Relationship:     core.RelationshipDirect,
							Eligible:         true,
						},
						{
							ModulePath:       "github.com/a/indirect",
							CurrentVersion:   "v1.0.0",
							CandidateVersion: "v1.1.0",
							Relationship:     core.RelationshipIndirect,
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
			},
		},
	}

	var out bytes.Buffer
	Detect(&out, result)
	want := `Target app (.)
Eligible:
  - github.com/a/direct v1.0.0 -> v1.1.0
  - github.com/a/indirect (indirect) v1.0.0 -> v1.1.0
Blocked:
  - github.com/a/blocked (indirect) v1.0.0 -> v1.1.0: denied
Summary: eligible=2 blocked=1 applied=0 errors=0

Total: targets=1 eligible=2 blocked=1 applied=0 errors=0
`
	if got := out.String(); got != want {
		t.Fatalf("output:\n%s\nwant:\n%s", got, want)
	}
}

func TestDetectShowsVulnerabilityContext(t *testing.T) {
	result := core.RunResult{
		Mode: "detect",
		Targets: []core.TargetResult{
			{
				Target: config.Target{
					Name:           "app",
					NormalizedPath: ".",
				},
				Plan: core.TargetPlan{
					Decisions: []core.Decision{
						{
							ModulePath:       "github.com/a/direct",
							CurrentVersion:   "v1.0.0",
							CandidateVersion: "v1.1.0",
							Eligible:         true,
							Vulnerabilities: []core.Vulnerability{
								{AdvisoryIDs: []string{"GO-2026-0001"}},
							},
						},
						{
							ModulePath:       "github.com/a/blocked",
							CurrentVersion:   "v1.0.0",
							CandidateVersion: "v1.1.0",
							Relationship:     core.RelationshipIndirect,
							BlockedReason:    core.ReasonDenied,
							Vulnerabilities: []core.Vulnerability{
								{AdvisoryIDs: []string{"GO-2026-0002"}},
							},
						},
					},
				},
			},
		},
	}

	var out bytes.Buffer
	Detect(&out, result)
	got := out.String()
	if !strings.Contains(got, "github.com/a/direct v1.0.0 -> v1.1.0 (vulnerabilities: GO-2026-0001)") {
		t.Fatalf("output = %q, want eligible vulnerability context", got)
	}
	if !strings.Contains(got, "github.com/a/blocked (indirect) v1.0.0 -> v1.1.0: denied (vulnerabilities: GO-2026-0002)") {
		t.Fatalf("output = %q, want blocked vulnerability context", got)
	}
}

func TestApplyLabelsIndirectAppliedUpdates(t *testing.T) {
	result := core.RunResult{
		Mode: "apply",
		Targets: []core.TargetResult{
			{
				Target: config.Target{
					Name:           "app",
					NormalizedPath: ".",
				},
				Applied: []core.AppliedUpdate{
					{
						ModulePath:   "github.com/a/indirect",
						FromVersion:  "v1.0.0",
						ToVersion:    "v1.1.0",
						Relationship: core.RelationshipIndirect,
					},
				},
			},
		},
	}

	var out bytes.Buffer
	Apply(&out, result)
	want := `Target app (.)
Applied:
  - github.com/a/indirect (indirect) v1.0.0 -> v1.1.0
Summary: eligible=0 blocked=0 applied=1 errors=0

Total: targets=1 eligible=0 blocked=0 applied=1 errors=0
`
	if got := out.String(); got != want {
		t.Fatalf("output:\n%s\nwant:\n%s", got, want)
	}
}

func TestApplyShowsVulnerabilityContext(t *testing.T) {
	result := core.RunResult{
		Mode: "apply",
		Targets: []core.TargetResult{
			{
				Target: config.Target{
					Name:           "app",
					NormalizedPath: ".",
				},
				Applied: []core.AppliedUpdate{
					{
						ModulePath:   "github.com/a/direct",
						FromVersion:  "v1.0.0",
						ToVersion:    "v1.1.0",
						Relationship: core.RelationshipDirect,
						Vulnerabilities: []core.Vulnerability{
							{AdvisoryIDs: []string{"GO-2026-0001"}},
						},
					},
				},
			},
		},
	}

	var out bytes.Buffer
	Apply(&out, result)
	if got := out.String(); !strings.Contains(got, "github.com/a/direct v1.0.0 -> v1.1.0 (vulnerabilities: GO-2026-0001)") {
		t.Fatalf("output = %q, want applied vulnerability context", got)
	}
}

func runResultWithDecision(decision core.Decision) core.RunResult {
	return core.RunResult{
		Mode: "detect",
		Targets: []core.TargetResult{
			{
				Target: config.Target{
					Name:           "app",
					NormalizedPath: ".",
				},
				Plan: core.TargetPlan{
					Decisions: []core.Decision{decision},
				},
			},
		},
	}
}
