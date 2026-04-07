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
