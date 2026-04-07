package core

import (
	"time"

	"github.com/mishankov/updtr/internal/config"
)

type Reason string

const (
	ReasonPinMismatch              Reason = "pin_mismatch"
	ReasonPinned                   Reason = "pinned"
	ReasonDenied                   Reason = "denied"
	ReasonNotAllowed               Reason = "not_allowed"
	ReasonQuarantined              Reason = "quarantined"
	ReasonMissingReleaseDate       Reason = "missing_release_date"
	ReasonUntrustedReleaseDate     Reason = "untrusted_release_date"
	ReasonReplacedDependency       Reason = "replaced_dependency"
	ReasonCandidateResolutionFail  Reason = "candidate_resolution_failed"
	WarningAdditionalDirectChanges        = "additional_direct_changes_detected"
)

type Decision struct {
	ModulePath       string
	CurrentVersion   string
	CandidateVersion string
	PinVersion       string
	ReleaseTime      *time.Time
	Eligible         bool
	BlockedReason    Reason
	Message          string
}

func (d Decision) Blocked() bool {
	return d.BlockedReason != ""
}

type TargetPlan struct {
	Target    config.Target
	Decisions []Decision
	Error     string
}

type AppliedUpdate struct {
	ModulePath    string
	FromVersion   string
	ToVersion     string
	CommandOutput string
}

type TargetResult struct {
	Target   config.Target
	Plan     TargetPlan
	Applied  []AppliedUpdate
	Warnings []string
	Error    string
}

func (r TargetResult) EffectiveError() string {
	if r.Error != "" {
		return r.Error
	}
	return r.Plan.Error
}

type RunResult struct {
	Mode    string
	Targets []TargetResult
}

func (r RunResult) HasOperationalFailures() bool {
	for _, target := range r.Targets {
		if target.EffectiveError() != "" {
			return true
		}
	}
	return false
}
