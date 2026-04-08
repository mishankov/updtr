package orchestrator

import (
	"time"

	"github.com/mishankov/updtr/internal/config"
)

type TargetOutcome string

const (
	TargetOutcomeSuccess TargetOutcome = "success"
	TargetOutcomeFailure TargetOutcome = "failure"
)

type TargetProgress struct {
	Mode    string
	Target  config.Target
	Outcome TargetOutcome
	Elapsed time.Duration
}

type ProgressReporter interface {
	TargetStarted(TargetProgress)
	TargetFinished(TargetProgress)
}

type nopProgressReporter struct{}

func (nopProgressReporter) TargetStarted(TargetProgress)  {}
func (nopProgressReporter) TargetFinished(TargetProgress) {}
