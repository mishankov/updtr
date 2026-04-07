package policy

import (
	"testing"
	"time"

	"github.com/mishankov/updtr/internal/config"
	"github.com/mishankov/updtr/internal/core"
)

func TestDecidePrecedence(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	age := 7
	release := now.Add(-10 * 24 * time.Hour)
	policy := config.Policy{
		QuarantineDays: &age,
		AllowSet:       true,
		Allow:          []string{"github.com/a/allowed"},
		Deny:           []string{"github.com/a/allowed"},
		Pins:           map[string]string{"github.com/a/pinned": "v1.0.0"},
	}

	cases := []struct {
		name   string
		input  Input
		reason core.Reason
	}{
		{
			name:   "pin mismatch wins without candidate",
			input:  Input{ModulePath: "github.com/a/pinned", CurrentVersion: "v0.9.0"},
			reason: core.ReasonPinMismatch,
		},
		{
			name:   "pinned wins with matching current",
			input:  Input{ModulePath: "github.com/a/pinned", CurrentVersion: "v1.0.0", CandidateVersion: "v1.1.0", ReleaseTime: &release, ReleaseTrusted: true},
			reason: core.ReasonPinned,
		},
		{
			name:   "deny wins over allow",
			input:  Input{ModulePath: "github.com/a/allowed", CurrentVersion: "v1.0.0", CandidateVersion: "v1.1.0", ReleaseTime: &release, ReleaseTrusted: true},
			reason: core.ReasonDenied,
		},
		{
			name:   "not allowed before quarantine",
			input:  Input{ModulePath: "github.com/a/other", CurrentVersion: "v1.0.0", CandidateVersion: "v1.1.0", ReleaseTime: &release, ReleaseTrusted: true},
			reason: core.ReasonNotAllowed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision, show := Decide(policy, tc.input, now)
			if !show {
				t.Fatal("show = false, want true")
			}
			if decision.BlockedReason != tc.reason {
				t.Fatalf("reason = %s, want %s", decision.BlockedReason, tc.reason)
			}
		})
	}
}

func TestDecideQuarantineBoundary(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	days := 7
	cfg := config.Policy{QuarantineDays: &days, Pins: map[string]string{}}
	boundary := now.Add(-7 * 24 * time.Hour)
	inside := boundary.Add(time.Second)

	decision, show := Decide(cfg, Input{ModulePath: "github.com/a/lib", CurrentVersion: "v1.0.0", CandidateVersion: "v1.1.0", ReleaseTime: &boundary, ReleaseTrusted: true}, now)
	if !show || !decision.Eligible {
		t.Fatalf("boundary decision = %+v show=%v, want eligible", decision, show)
	}
	decision, show = Decide(cfg, Input{ModulePath: "github.com/a/lib", CurrentVersion: "v1.0.0", CandidateVersion: "v1.1.0", ReleaseTime: &inside, ReleaseTrusted: true}, now)
	if !show || decision.BlockedReason != core.ReasonQuarantined {
		t.Fatalf("inside decision = %+v show=%v, want quarantined", decision, show)
	}
}

func TestDecideMissingAndUntrustedReleaseMetadata(t *testing.T) {
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	days := 0
	cfg := config.Policy{QuarantineDays: &days, Pins: map[string]string{}}
	future := now.Add(time.Hour)

	decision, show := Decide(cfg, Input{ModulePath: "github.com/a/lib", CurrentVersion: "v1.0.0", CandidateVersion: "v1.1.0"}, now)
	if !show || decision.BlockedReason != core.ReasonMissingReleaseDate {
		t.Fatalf("missing metadata decision = %+v show=%v", decision, show)
	}
	decision, show = Decide(cfg, Input{ModulePath: "github.com/a/lib", CurrentVersion: "v1.0.0", CandidateVersion: "v1.1.0", ReleaseTime: &future, ReleaseTrusted: true}, now)
	if !show || decision.BlockedReason != core.ReasonUntrustedReleaseDate {
		t.Fatalf("future metadata decision = %+v show=%v", decision, show)
	}
}
