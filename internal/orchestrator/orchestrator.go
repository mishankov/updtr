package orchestrator

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishankov/updtr/internal/config"
	"github.com/mishankov/updtr/internal/core"
	"github.com/mishankov/updtr/internal/goecosystem"
)

type Engine struct {
	Go        *goecosystem.Adapter
	goAdapter goAdapter
}

func New() *Engine {
	return &Engine{Go: goecosystem.New()}
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
	for _, target := range targets {
		plan := e.adapterFor(target).PlanTarget(ctx, target)
		result.Targets = append(result.Targets, core.TargetResult{Target: target, Plan: plan})
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
	for _, target := range targets {
		adapter := e.adapterFor(target)
		targetResult := core.TargetResult{Target: target}
		plan := adapter.PlanTarget(ctx, target)
		targetResult.Plan = plan
		if plan.Error != "" {
			result.Targets = append(result.Targets, targetResult)
			continue
		}

		before, err := adapter.DirectVersions(target)
		if err != nil {
			targetResult.Error = err.Error()
			result.Targets = append(result.Targets, targetResult)
			continue
		}

		eligible := eligibleDecisions(plan.Decisions)
		for _, decision := range eligible {
			output, err := adapter.ApplyUpdate(ctx, target, decision.ModulePath, decision.CandidateVersion)
			if err != nil {
				targetResult.Error = err.Error()
				break
			}
			targetResult.Applied = append(targetResult.Applied, core.AppliedUpdate{
				ModulePath:    decision.ModulePath,
				FromVersion:   decision.CurrentVersion,
				ToVersion:     decision.CandidateVersion,
				Relationship:  decision.Relationship,
				CommandOutput: output,
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
	}
	return result, nil
}

type goAdapter interface {
	CheckPrereq() error
	PlanTarget(context.Context, config.Target) core.TargetPlan
	ApplyUpdate(context.Context, config.Target, string, string) (string, error)
	Tidy(context.Context, config.Target) (string, error)
	DirectVersions(config.Target) (map[string]string, error)
}

func (e *Engine) adapterFor(target config.Target) goAdapter {
	if e.goAdapter != nil {
		return e.goAdapter
	}
	return e.Go
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
