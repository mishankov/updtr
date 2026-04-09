package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/mishankov/updtr/internal/core"
	"github.com/mishankov/updtr/internal/orchestrator"
	"github.com/mishankov/updtr/internal/progress"
	"github.com/mishankov/updtr/internal/render"
)

type runPresenter interface {
	orchestrator.ProgressReporter
	Render(core.RunResult)
}

func newRunPresenter(out io.Writer) runPresenter {
	policy := resolveTerminalPolicy(out)
	if policy.LiveUpdates {
		return &livePresenter{out: out, color: policy.Color}
	}
	return &fallbackPresenter{out: out, color: policy.Color}
}

type fallbackPresenter struct {
	out   io.Writer
	color bool
}

func (p *fallbackPresenter) Report(event progress.Event) {
	line := appendOnlyLine(event, p.color)
	if line == "" {
		return
	}
	_, _ = fmt.Fprintln(p.out, line)
}

func (p *fallbackPresenter) Render(result core.RunResult) {
	render.Report(p.out, result, render.Options{Color: p.color})
}

type livePresenter struct {
	out        io.Writer
	color      bool
	frameIndex int
}

func (p *livePresenter) Report(event progress.Event) {
	line := liveLine(event, p.color, spinnerFrame(p.frameIndex))
	if line == "" {
		return
	}
	if event.Kind == progress.KindTargetFinished {
		_, _ = fmt.Fprintf(p.out, "\r\033[2K%s\n", line)
		return
	}
	p.frameIndex++
	_, _ = fmt.Fprintf(p.out, "\r\033[2K%s", line)
}

func (p *livePresenter) Render(result core.RunResult) {
	_, _ = fmt.Fprint(p.out, "\r\033[2K")
	render.Report(p.out, result, render.Options{Color: p.color})
}

func appendOnlyLine(event progress.Event, color bool) string {
	prefix := eventPrefix(event)
	switch event.Kind {
	case progress.KindTargetStarted:
		return prefix + "started"
	case progress.KindStageStarted:
		switch event.Stage {
		case progress.StagePlanning:
			return prefix + fmt.Sprintf("planning 0/%d dependencies checked", event.TotalDependencies)
		case progress.StageMutating:
			return prefix + fmt.Sprintf("mutating 0/%d updates processed", event.TotalMutations)
		}
	case progress.KindDependencyChecked:
		return prefix + fmt.Sprintf("planning %d/%d dependencies checked (%s)", event.DependenciesCompleted, event.TotalDependencies, event.ModulePath)
	case progress.KindMutationProcessed:
		return prefix + fmt.Sprintf("mutating %d/%d updates processed (%s)", event.MutationsCompleted, event.TotalMutations, event.ModulePath)
	case progress.KindTargetFinished:
		return prefix + fmt.Sprintf("finished %s in %s", outcomeLabel(event.Outcome, color), event.Elapsed)
	}
	return ""
}

func liveLine(event progress.Event, color bool, frame string) string {
	prefix := eventPrefix(event)
	switch event.Kind {
	case progress.KindTargetStarted:
		return frame + " " + prefix + "started"
	case progress.KindStageStarted:
		switch event.Stage {
		case progress.StagePlanning:
			return frame + " " + prefix + fmt.Sprintf("planning 0/%d dependencies checked", event.TotalDependencies)
		case progress.StageMutating:
			return frame + " " + prefix + fmt.Sprintf("mutating 0/%d updates processed", event.TotalMutations)
		}
	case progress.KindDependencyChecked:
		return frame + " " + prefix + fmt.Sprintf("planning %d/%d dependencies checked (%s)", event.DependenciesCompleted, event.TotalDependencies, event.ModulePath)
	case progress.KindMutationProcessed:
		return frame + " " + prefix + fmt.Sprintf("mutating %d/%d updates processed (%s)", event.MutationsCompleted, event.TotalMutations, event.ModulePath)
	case progress.KindTargetFinished:
		return prefix + fmt.Sprintf("finished %s in %s", outcomeLabel(event.Outcome, color), event.Elapsed)
	}
	return ""
}

func eventPrefix(event progress.Event) string {
	return fmt.Sprintf("%s [%d/%d targets] %s (%s): ", event.Mode, event.TargetIndex, event.TotalTargets, event.Target.Name, event.Target.NormalizedPath)
}

func spinnerFrame(index int) string {
	frames := []string{"-", "\\", "|", "/"}
	if index < 0 {
		index = 0
	}
	return frames[index%len(frames)]
}

func outcomeLabel(outcome progress.TargetOutcome, color bool) string {
	label := strings.ToUpper(string(outcome))
	if !color {
		return label
	}
	switch outcome {
	case progress.TargetOutcomeSuccess:
		return "\x1b[32m" + label + "\x1b[0m"
	case progress.TargetOutcomeFailure:
		return "\x1b[31m" + label + "\x1b[0m"
	default:
		return label
	}
}
