package action

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/mishankov/updtr/internal/core"
)

const (
	DefaultConfigPath       = "updtr.yaml"
	DefaultCommitMessage    = "chore(deps): update dependencies with updtr"
	DefaultPullRequestTitle = "chore(deps): update dependencies with updtr"
	PROperationNone         = "none"
	PROperationCreated      = "created"
	PROperationUpdated      = "updated"
)

type Config struct {
	ConfigPath       string
	Targets          []string
	CommitMessage    string
	PullRequestTitle string
	BaseBranch       string
	Repository       string
	GitHubToken      string
	OutputPath       string
}

type Outputs struct {
	Changed              bool
	Committed            bool
	Branch               string
	PullRequestOperation string
	PullRequestNumber    string
	PullRequestURL       string
}

type RunOptions struct {
	ConfigPath string
	Targets    []string
}

type UpdtrRunner interface {
	Apply(context.Context, RunOptions) (core.RunResult, error)
}

type GitClient interface {
	EnsureClean(context.Context) error
	CheckoutBaseBranch(context.Context, string) error
	UntrackedFiles(context.Context) ([]string, error)
	HasChanges(context.Context) (bool, error)
	CheckoutManagedBranch(context.Context, string) error
	ConfigureAuthor(context.Context) error
	StageChanges(context.Context, []string) error
	HasStagedChanges(context.Context) (bool, error)
	Commit(context.Context, string) error
	Push(context.Context, string) error
}

type PullRequestRequest struct {
	Repository string
	Token      string
	BaseBranch string
	HeadBranch string
	Title      string
	Body       string
}

type PullRequestResult struct {
	Operation string
	Number    int
	URL       string
}

type PullRequestClient interface {
	Ensure(context.Context, PullRequestRequest) (PullRequestResult, error)
}

type OutputWriter interface {
	Write(Outputs) error
}

type Runtime struct {
	Runner       UpdtrRunner
	Git          GitClient
	PullRequests PullRequestClient
	Outputs      OutputWriter
	Log          io.Writer
}

func Run(ctx context.Context, cfg Config, stdout io.Writer, stderr io.Writer) (Outputs, error) {
	runner, err := newExecutableRunner(stdout, stderr)
	if err != nil {
		return Outputs{}, err
	}
	runtime := Runtime{
		Runner:       runner,
		Git:          newCommandGit(),
		PullRequests: newGitHubClient(nil),
		Outputs:      fileOutputWriter{path: cfg.OutputPath},
		Log:          stdout,
	}
	return runtime.Run(ctx, cfg)
}

func (r Runtime) Run(ctx context.Context, cfg Config) (Outputs, error) {
	cfg = cfg.withDefaults()
	outputs := Outputs{PullRequestOperation: PROperationNone}

	if err := r.Git.EnsureClean(ctx); err != nil {
		return outputs, err
	}
	if cfg.BaseBranch != "" {
		r.logf("Checking out base branch %q\n", cfg.BaseBranch)
		if err := r.Git.CheckoutBaseBranch(ctx, cfg.BaseBranch); err != nil {
			return outputs, err
		}
	}
	baselineUntracked, err := r.Git.UntrackedFiles(ctx)
	if err != nil {
		return outputs, err
	}

	if cfg.ConfigPath == "" {
		r.logf("Running updtr apply with default config resolution\n")
	} else {
		r.logf("Running updtr apply with config %q\n", cfg.ConfigPath)
	}
	result, err := r.Runner.Apply(ctx, RunOptions{
		ConfigPath: cfg.ConfigPath,
		Targets:    cfg.Targets,
	})
	if err != nil {
		return outputs, err
	}

	trackedChanged, err := r.Git.HasChanges(ctx)
	if err != nil {
		return outputs, err
	}
	currentUntracked, err := r.Git.UntrackedFiles(ctx)
	if err != nil {
		return outputs, err
	}
	newUntracked := newUntrackedFiles(baselineUntracked, currentUntracked)
	changed := trackedChanged || len(newUntracked) > 0
	outputs.Changed = changed
	if !changed {
		r.logf("No repository changes detected.\n")
		return outputs, r.Outputs.Write(outputs)
	}

	if err := cfg.validateChangedRun(); err != nil {
		return outputs, err
	}

	outputs.Branch = ManagedBranchName(cfg.ConfigPath, cfg.Targets, cfg.BaseBranch)
	r.logf("Preparing managed branch %q\n", outputs.Branch)
	if err := r.Git.CheckoutManagedBranch(ctx, outputs.Branch); err != nil {
		return outputs, err
	}
	if err := r.Git.ConfigureAuthor(ctx); err != nil {
		return outputs, err
	}
	if err := r.Git.StageChanges(ctx, newUntracked); err != nil {
		return outputs, err
	}

	staged, err := r.Git.HasStagedChanges(ctx)
	if err != nil {
		return outputs, err
	}
	if !staged {
		return outputs, fmt.Errorf("detected repository changes after updtr apply, but no staged changes were found")
	}

	if err := r.Git.Commit(ctx, cfg.CommitMessage); err != nil {
		return outputs, err
	}
	outputs.Committed = true
	if err := r.Git.Push(ctx, outputs.Branch); err != nil {
		return outputs, err
	}

	body := RenderPullRequestBody(result)
	prResult, err := r.PullRequests.Ensure(ctx, PullRequestRequest{
		Repository: cfg.Repository,
		Token:      cfg.GitHubToken,
		BaseBranch: cfg.BaseBranch,
		HeadBranch: outputs.Branch,
		Title:      cfg.PullRequestTitle,
		Body:       body,
	})
	if err != nil {
		return outputs, err
	}
	outputs.PullRequestOperation = prResult.Operation
	if prResult.Number > 0 {
		outputs.PullRequestNumber = strconv.Itoa(prResult.Number)
	}
	outputs.PullRequestURL = prResult.URL

	return outputs, r.Outputs.Write(outputs)
}

func (cfg Config) withDefaults() Config {
	if cfg.CommitMessage == "" {
		cfg.CommitMessage = DefaultCommitMessage
	}
	if cfg.PullRequestTitle == "" {
		cfg.PullRequestTitle = DefaultPullRequestTitle
	}
	return cfg
}

func (cfg Config) validateChangedRun() error {
	if cfg.Repository == "" {
		return fmt.Errorf("missing GITHUB_REPOSITORY for changed run")
	}
	if cfg.BaseBranch == "" {
		return fmt.Errorf("missing base branch for changed run")
	}
	if cfg.GitHubToken == "" {
		return fmt.Errorf("missing github-token input for changed run")
	}
	return nil
}

func (r Runtime) logf(format string, args ...any) {
	if r.Log == nil {
		return
	}
	_, _ = fmt.Fprintf(r.Log, format, args...)
}

func newUntrackedFiles(before []string, after []string) []string {
	baseline := make(map[string]struct{}, len(before))
	for _, path := range before {
		baseline[path] = struct{}{}
	}

	var out []string
	for _, path := range after {
		if _, existed := baseline[path]; existed {
			continue
		}
		out = append(out, path)
	}
	return out
}
