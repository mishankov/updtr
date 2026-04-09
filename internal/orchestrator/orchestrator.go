package orchestrator

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/mishankov/updtr/internal/config"
	"github.com/mishankov/updtr/internal/core"
	"github.com/mishankov/updtr/internal/goecosystem"
	"github.com/mishankov/updtr/internal/progress"
)

type Engine struct {
	Go       GoAdapter
	Reporter ProgressReporter
	Now      func() time.Time
}

func New() *Engine {
	return &Engine{Go: goecosystem.New(), Now: time.Now}
}

func (e *Engine) Detect(ctx context.Context, cfg *config.Config, selectedNames []string) (core.RunResult, error) {
	targets, err := selectTargets(cfg.Targets, selectedNames)
	if err != nil {
		return core.RunResult{}, err
	}
	if err := e.checkPrereqs(targets); err != nil {
		return core.RunResult{}, err
	}
	result := core.RunResult{Mode: "detect"}
	for i, target := range targets {
		startedAt := e.now()
		e.reporter().Report(progress.Event{
			Kind:         progress.KindTargetStarted,
			Mode:         result.Mode,
			Target:       target,
			TargetIndex:  i + 1,
			TotalTargets: len(targets),
		})
		plan := e.adapterFor(target).PlanTarget(ctx, target, e.planProgressReporter(result.Mode, target, i+1, len(targets)))
		targetResult := core.TargetResult{Target: target, Plan: plan}
		result.Targets = append(result.Targets, targetResult)
		e.reporter().Report(progress.Event{
			Kind:         progress.KindTargetFinished,
			Mode:         result.Mode,
			Target:       target,
			TargetIndex:  i + 1,
			TotalTargets: len(targets),
			Outcome:      outcomeForResult(targetResult),
			Elapsed:      e.now().Sub(startedAt),
		})
	}
	return result, nil
}

func (e *Engine) Apply(ctx context.Context, cfg *config.Config, selectedNames []string) (core.RunResult, error) {
	targets, err := selectTargets(cfg.Targets, selectedNames)
	if err != nil {
		return core.RunResult{}, err
	}
	if err := e.checkPrereqs(targets); err != nil {
		return core.RunResult{}, err
	}
	result := core.RunResult{Mode: "apply"}
	for i, target := range targets {
		startedAt := e.now()
		e.reporter().Report(progress.Event{
			Kind:         progress.KindTargetStarted,
			Mode:         result.Mode,
			Target:       target,
			TargetIndex:  i + 1,
			TotalTargets: len(targets),
		})
		adapter := e.adapterFor(target)
		targetResult := core.TargetResult{Target: target}
		plan := adapter.PlanTarget(ctx, target, e.planProgressReporter(result.Mode, target, i+1, len(targets)))
		targetResult.Plan = plan
		if plan.Error != "" {
			result.Targets = append(result.Targets, targetResult)
			e.reporter().Report(progress.Event{
				Kind:         progress.KindTargetFinished,
				Mode:         result.Mode,
				Target:       target,
				TargetIndex:  i + 1,
				TotalTargets: len(targets),
				Outcome:      outcomeForResult(targetResult),
				Elapsed:      e.now().Sub(startedAt),
			})
			continue
		}

		before, err := adapter.DirectVersions(target)
		if err != nil {
			targetResult.Error = err.Error()
			result.Targets = append(result.Targets, targetResult)
			e.reporter().Report(progress.Event{
				Kind:         progress.KindTargetFinished,
				Mode:         result.Mode,
				Target:       target,
				TargetIndex:  i + 1,
				TotalTargets: len(targets),
				Outcome:      outcomeForResult(targetResult),
				Elapsed:      e.now().Sub(startedAt),
			})
			continue
		}

		eligible := eligibleDecisions(plan.Decisions)
		e.reporter().Report(progress.Event{
			Kind:           progress.KindStageStarted,
			Mode:           result.Mode,
			Target:         target,
			TargetIndex:    i + 1,
			TotalTargets:   len(targets),
			Stage:          progress.StageMutating,
			TotalMutations: len(eligible),
		})
		for idx, decision := range eligible {
			output, err := adapter.ApplyUpdate(ctx, target, decision.ModulePath, decision.CandidateVersion)
			if err != nil {
				targetResult.Error = err.Error()
				break
			}
			e.reporter().Report(progress.Event{
				Kind:               progress.KindMutationProcessed,
				Mode:               result.Mode,
				Target:             target,
				TargetIndex:        i + 1,
				TotalTargets:       len(targets),
				Stage:              progress.StageMutating,
				MutationsCompleted: idx + 1,
				TotalMutations:     len(eligible),
				ModulePath:         decision.ModulePath,
			})
			targetResult.Applied = append(targetResult.Applied, core.AppliedUpdate{
				ModulePath:      decision.ModulePath,
				FromVersion:     decision.CurrentVersion,
				ToVersion:       decision.CandidateVersion,
				Relationship:    decision.Relationship,
				Vulnerabilities: decision.Vulnerabilities,
				CommandOutput:   output,
			})
		}

		if targetResult.Error == "" && len(targetResult.Applied) > 0 {
			if _, err := adapter.Tidy(ctx, target); err != nil {
				targetResult.Error = err.Error()
			}
		}

		if len(targetResult.Applied) > 0 {
			after, err := adapter.DirectVersions(target)
			if err != nil {
				if targetResult.Error == "" {
					targetResult.Error = err.Error()
				}
			} else if additionalDirectChanges(before, after, targetResult.Applied) {
				targetResult.Warnings = append(targetResult.Warnings, core.WarningAdditionalDirectChanges)
			}
		}

		result.Targets = append(result.Targets, targetResult)
		e.reporter().Report(progress.Event{
			Kind:         progress.KindTargetFinished,
			Mode:         result.Mode,
			Target:       target,
			TargetIndex:  i + 1,
			TotalTargets: len(targets),
			Outcome:      outcomeForResult(targetResult),
			Elapsed:      e.now().Sub(startedAt),
		})
	}
	return result, nil
}

type GoAdapter interface {
	CheckPrereq() error
	PlanTarget(context.Context, config.Target, ...func(progress.PlanUpdate)) core.TargetPlan
	ApplyUpdate(context.Context, config.Target, string, string) (string, error)
	Tidy(context.Context, config.Target) (string, error)
	DirectVersions(config.Target) (map[string]string, error)
}

func (e *Engine) adapterFor(target config.Target) GoAdapter {
	if e.Go != nil {
		return e.Go
	}
	return goecosystem.New()
}

func (e *Engine) checkPrereqs(targets []config.Target) error {
	for _, target := range targets {
		if target.Ecosystem == "go" {
			return e.adapterFor(target).CheckPrereq()
		}
	}
	return nil
}

func selectTargets(targets []config.Target, selectedNames []string) ([]config.Target, error) {
	if len(selectedNames) == 0 {
		return append([]config.Target(nil), targets...), nil
	}
	selected := map[string]struct{}{}
	for _, name := range selectedNames {
		selected[name] = struct{}{}
	}
	for name := range selected {
		found := false
		for _, target := range targets {
			if target.Name == name {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown target %q", name)
		}
	}
	var out []config.Target
	for _, target := range targets {
		if _, ok := selected[target.Name]; ok {
			out = append(out, target)
		}
	}
	return out, nil
}

func eligibleDecisions(decisions []core.Decision) []core.Decision {
	var eligible []core.Decision
	for _, decision := range decisions {
		if decision.Eligible {
			eligible = append(eligible, decision)
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		return eligible[i].ModulePath < eligible[j].ModulePath
	})
	return eligible
}

func (e *Engine) reporter() ProgressReporter {
	if e.Reporter != nil {
		return e.Reporter
	}
	return nopProgressReporter{}
}

func (e *Engine) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func (e *Engine) planProgressReporter(mode string, target config.Target, targetIndex int, totalTargets int) func(progress.PlanUpdate) {
	return func(update progress.PlanUpdate) {
		switch update.Kind {
		case progress.PlanKindStarted:
			e.reporter().Report(progress.Event{
				Kind:              progress.KindStageStarted,
				Mode:              mode,
				Target:            target,
				TargetIndex:       targetIndex,
				TotalTargets:      totalTargets,
				Stage:             progress.StagePlanning,
				TotalDependencies: update.TotalDependencies,
			})
		case progress.PlanKindChecked:
			e.reporter().Report(progress.Event{
				Kind:                  progress.KindDependencyChecked,
				Mode:                  mode,
				Target:                target,
				TargetIndex:           targetIndex,
				TotalTargets:          totalTargets,
				Stage:                 progress.StagePlanning,
				DependenciesCompleted: update.DependenciesCompleted,
				TotalDependencies:     update.TotalDependencies,
				ModulePath:            update.ModulePath,
			})
		}
	}
}

func outcomeForResult(result core.TargetResult) progress.TargetOutcome {
	if result.EffectiveError() != "" {
		return progress.TargetOutcomeFailure
	}
	return progress.TargetOutcomeSuccess
}

func additionalDirectChanges(before map[string]string, after map[string]string, applied []core.AppliedUpdate) bool {
	expected := map[string]string{}
	for _, update := range applied {
		expected[update.ModulePath] = update.ToVersion
	}
	for module, beforeVersion := range before {
		afterVersion, exists := after[module]
		if !exists {
			return true
		}
		if expectedVersion, ok := expected[module]; ok {
			if afterVersion != expectedVersion {
				return true
			}
			continue
		}
		if afterVersion != beforeVersion {
			return true
		}
	}
	for module := range after {
		if _, existed := before[module]; !existed {
			return true
		}
	}
	return false
}
