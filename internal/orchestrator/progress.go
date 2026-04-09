package orchestrator

import "github.com/mishankov/updtr/internal/progress"

type ProgressEvent = progress.Event
type PlanProgress = progress.PlanUpdate
type TargetOutcome = progress.TargetOutcome
type ProgressKind = progress.Kind
type ProgressStage = progress.Stage
type PlanProgressKind = progress.PlanKind

const (
	TargetOutcomeSuccess = progress.TargetOutcomeSuccess
	TargetOutcomeFailure = progress.TargetOutcomeFailure

	ProgressTargetStarted     = progress.KindTargetStarted
	ProgressStageStarted      = progress.KindStageStarted
	ProgressDependencyChecked = progress.KindDependencyChecked
	ProgressMutationProcessed = progress.KindMutationProcessed
	ProgressTargetFinished    = progress.KindTargetFinished

	StagePlanning = progress.StagePlanning
	StageMutating = progress.StageMutating

	PlanProgressStarted = progress.PlanKindStarted
	PlanProgressChecked = progress.PlanKindChecked
)

type ProgressReporter interface {
	Report(progress.Event)
}

type nopProgressReporter struct{}

func (nopProgressReporter) Report(progress.Event) {}
