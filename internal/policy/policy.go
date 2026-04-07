package policy

import (
	"time"

	"github.com/mishankov/updtr/internal/config"
	"github.com/mishankov/updtr/internal/core"
)

type Input struct {
	ModulePath       string
	CurrentVersion   string
	CandidateVersion string
	ReleaseTime      *time.Time
	ReleaseTrusted   bool
	ResolutionError  string
}

func Decide(policy config.Policy, input Input, now time.Time) (core.Decision, bool) {
	decision := core.Decision{
		ModulePath:       input.ModulePath,
		CurrentVersion:   input.CurrentVersion,
		CandidateVersion: input.CandidateVersion,
		ReleaseTime:      input.ReleaseTime,
	}

	if pin, ok := policy.Pins[input.ModulePath]; ok {
		decision.PinVersion = pin
		if input.CurrentVersion != pin {
			decision.BlockedReason = core.ReasonPinMismatch
			return decision, true
		}
		if input.CandidateVersion != "" {
			decision.BlockedReason = core.ReasonPinned
			return decision, true
		}
		return decision, false
	}

	if input.ResolutionError != "" {
		decision.BlockedReason = core.ReasonCandidateResolutionFail
		decision.Message = input.ResolutionError
		return decision, true
	}

	if input.CandidateVersion == "" {
		return decision, false
	}

	if contains(policy.Deny, input.ModulePath) {
		decision.BlockedReason = core.ReasonDenied
		return decision, true
	}
	if policy.AllowSet && !contains(policy.Allow, input.ModulePath) {
		decision.BlockedReason = core.ReasonNotAllowed
		return decision, true
	}

	if policy.QuarantineDays != nil {
		if input.ReleaseTime == nil {
			decision.BlockedReason = core.ReasonMissingReleaseDate
			return decision, true
		}
		if !input.ReleaseTrusted || input.ReleaseTime.After(now) {
			decision.BlockedReason = core.ReasonUntrustedReleaseDate
			return decision, true
		}
		eligibleAt := now.Add(-time.Duration(*policy.QuarantineDays) * 24 * time.Hour)
		if input.ReleaseTime.After(eligibleAt) {
			decision.BlockedReason = core.ReasonQuarantined
			return decision, true
		}
	}

	decision.Eligible = true
	return decision, true
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
