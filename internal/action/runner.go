package action

import (
	"context"
	"fmt"
	"io"

	"github.com/mishankov/updtr/internal/config"
	"github.com/mishankov/updtr/internal/core"
	"github.com/mishankov/updtr/internal/orchestrator"
	"github.com/mishankov/updtr/internal/presenter"
)

type directRunner struct {
	stdout io.Writer
}

var (
	loadConfig            = config.Load
	newOrchestratorEngine = orchestrator.New
)

func newExecutableRunner(stdout io.Writer, _ io.Writer) (*directRunner, error) {
	return &directRunner{stdout: stdout}, nil
}

func (r *directRunner) Apply(ctx context.Context, opts RunOptions) (core.RunResult, error) {
	cfg, err := loadConfig(opts.ConfigPath)
	if err != nil {
		return core.RunResult{}, err
	}

	engine := newOrchestratorEngine()
	var progressPresenter presenter.RunPresenter
	if r.stdout != nil {
		progressPresenter = presenter.NewAppendOnly(r.stdout, false)
		engine.Reporter = progressPresenter
	}
	result, err := engine.Apply(ctx, cfg, opts.Targets)
	if err != nil {
		return core.RunResult{}, fmt.Errorf("run updtr apply: %w", err)
	}

	if progressPresenter != nil {
		progressPresenter.Render(result)
	}
	if result.HasOperationalFailures() {
		return result, fmt.Errorf("run updtr apply: one or more targets failed")
	}
	return result, nil
}

func applyArgs(opts RunOptions) []string {
	args := []string{"apply"}
	if opts.ConfigPath != "" {
		args = append(args, "--config", opts.ConfigPath)
	}
	return args
}
