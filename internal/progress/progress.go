package progress

import (
	"time"

	"github.com/mishankov/updtr/internal/config"
)

type TargetOutcome string
type Kind string
type Stage string
type PlanKind string

const (
	TargetOutcomeSuccess TargetOutcome = "success"
	TargetOutcomeFailure TargetOutcome = "failure"
)

const (
	KindTargetStarted     Kind = "target_started"
	KindStageStarted      Kind = "stage_started"
	KindDependencyChecked Kind = "dependency_checked"
	KindMutationProcessed Kind = "mutation_processed"
	KindTargetFinished    Kind = "target_finished"
)

const (
	StagePlanning Stage = "planning"
	StageMutating Stage = "mutating"
)

const (
	PlanKindStarted PlanKind = "started"
	PlanKindChecked PlanKind = "checked"
)

type Event struct {
	Kind                  Kind
	Mode                  string
	Target                config.Target
	TargetIndex           int
	TotalTargets          int
	Stage                 Stage
	DependenciesCompleted int
	TotalDependencies     int
	MutationsCompleted    int
	TotalMutations        int
	ModulePath            string
	Outcome               TargetOutcome
	Elapsed               time.Duration
}

type PlanUpdate struct {
	Kind                  PlanKind
	DependenciesCompleted int
	TotalDependencies     int
	ModulePath            string
}
