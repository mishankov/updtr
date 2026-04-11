package action

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mishankov/updtr/internal/config"
	"github.com/mishankov/updtr/internal/core"
)

func TestRuntimeNoopRunWritesNoopOutputs(t *testing.T) {
	outputsWriter := &recordingOutputs{}
	runner := &fakeRunner{}
	runtime := Runtime{
		Runner:       runner,
		Git:          &fakeGit{},
		PullRequests: &fakePullRequests{},
		Outputs:      outputsWriter,
	}

	outputs, err := runtime.Run(context.Background(), Config{})
	if err != nil {
		t.Fatal(err)
	}
	if !outputsWriter.called {
		t.Fatal("outputs writer was not called")
	}
	if outputs.Changed {
		t.Fatalf("changed = %t, want false", outputs.Changed)
	}
	if outputs.Committed {
		t.Fatalf("committed = %t, want false", outputs.Committed)
	}
	if outputs.PullRequestOperation != PROperationNone {
		t.Fatalf("pull request operation = %q, want %q", outputs.PullRequestOperation, PROperationNone)
	}
	if runner.opts.ConfigPath != "" {
		t.Fatalf("runner config path = %q, want empty so CLI default resolution can fall back to updtr.yml", runner.opts.ConfigPath)
	}
}

func TestRuntimeChangedRunCommitsPushesAndCreatesPullRequest(t *testing.T) {
	git := &fakeGit{hasChanges: true, stagedChanges: true}
	prs := &fakePullRequests{
		result: PullRequestResult{Operation: PROperationCreated, Number: 42, URL: "https://example.com/pr/42"},
	}
	outputsWriter := &recordingOutputs{}
	runner := &fakeRunner{
		git: git,
		result: core.RunResult{
			Mode: "apply",
			Targets: []core.TargetResult{{
				Target: config.Target{Name: "worker", NormalizedPath: "worker"},
				Applied: []core.AppliedUpdate{{
					ModulePath:  "github.com/example/mod",
					FromVersion: "v1.0.0",
					ToVersion:   "v1.1.0",
				}},
			}},
		},
	}
	runtime := Runtime{
		Runner:       runner,
		Git:          git,
		PullRequests: prs,
		Outputs:      outputsWriter,
	}

	outputs, err := runtime.Run(context.Background(), Config{
		ConfigPath:       "configs/updtr.yaml",
		Targets:          []string{"worker", "app"},
		CommitMessage:    "custom commit",
		PullRequestTitle: "custom title",
		BaseBranch:       "main",
		Repository:       "mishankov/updtr",
		GitHubToken:      "secret",
	})
	if err != nil {
		t.Fatal(err)
	}

	if !outputs.Changed || !outputs.Committed {
		t.Fatalf("outputs = %+v, want changed and committed", outputs)
	}
	if outputs.PullRequestOperation != PROperationCreated {
		t.Fatalf("pull request operation = %q, want %q", outputs.PullRequestOperation, PROperationCreated)
	}
	if outputs.PullRequestNumber != "42" {
		t.Fatalf("pull request number = %q, want 42", outputs.PullRequestNumber)
	}
	if outputs.PullRequestURL != "https://example.com/pr/42" {
		t.Fatalf("pull request url = %q", outputs.PullRequestURL)
	}
	if git.checkedOutBranch == "" {
		t.Fatal("managed branch was not checked out")
	}
	if git.checkedOutBaseBranch != "main" {
		t.Fatalf("checked out base branch = %q, want main", git.checkedOutBaseBranch)
	}
	if git.commitMessage != "custom commit" {
		t.Fatalf("commit message = %q, want custom commit", git.commitMessage)
	}
	if git.pushedBranch != outputs.Branch {
		t.Fatalf("pushed branch = %q, want %q", git.pushedBranch, outputs.Branch)
	}
	if runner.opts.ConfigPath != "configs/updtr.yaml" {
		t.Fatalf("runner config path = %q, want explicit config path", runner.opts.ConfigPath)
	}
	if !slices.Equal(runner.opts.Targets, []string{"worker", "app"}) {
		t.Fatalf("runner targets = %#v, want explicit targets", runner.opts.Targets)
	}
	if !outputsWriter.called {
		t.Fatal("outputs writer was not called")
	}
	if prs.request.Title != "custom title" {
		t.Fatalf("pull request title = %q, want custom title", prs.request.Title)
	}
	if !strings.Contains(prs.request.Body, "github.com/example/mod") {
		t.Fatalf("pull request body = %q, want applied update details", prs.request.Body)
	}
	if !strings.Contains(prs.request.Body, "1 dependency update updated") {
		t.Fatalf("pull request body = %q, want summary counts", prs.request.Body)
	}
}

func TestRuntimeChangedRunStagesNewFilesCreatedByApply(t *testing.T) {
	git := &fakeGit{
		stagedChanges:    true,
		initialUntracked: []string{"notes.txt"},
		currentUntracked: []string{"go.sum", "notes.txt"},
	}
	runtime := Runtime{
		Runner:       &fakeRunner{},
		Git:          git,
		PullRequests: &fakePullRequests{result: PullRequestResult{Operation: PROperationCreated, Number: 7, URL: "https://example.com/pr/7"}},
		Outputs:      &recordingOutputs{},
	}

	outputs, err := runtime.Run(context.Background(), Config{
		BaseBranch:  "main",
		Repository:  "mishankov/updtr",
		GitHubToken: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !outputs.Changed {
		t.Fatal("changed = false, want true when apply creates a new file")
	}
	if !outputs.Committed {
		t.Fatal("committed = false, want true")
	}
	if !slices.Equal(git.stagedNewFiles, []string{"go.sum"}) {
		t.Fatalf("staged new files = %#v, want go.sum only", git.stagedNewFiles)
	}
}

func TestRuntimeChangedRunFailsBeforeGitSideEffectsWhenTokenIsMissing(t *testing.T) {
	git := &fakeGit{hasChanges: true}
	runtime := Runtime{
		Runner:       &fakeRunner{},
		Git:          git,
		PullRequests: &fakePullRequests{},
		Outputs:      &recordingOutputs{},
	}

	_, err := runtime.Run(context.Background(), Config{
		BaseBranch: "main",
		Repository: "mishankov/updtr",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "github-token") {
		t.Fatalf("err = %v, want github-token validation error", err)
	}
	if git.checkedOutBranch != "" {
		t.Fatalf("checked out branch = %q, want no git side effects", git.checkedOutBranch)
	}
}

func TestRuntimeChecksOutBaseBranchBeforeApply(t *testing.T) {
	git := &fakeGit{}
	runner := &fakeRunner{git: git}
	runtime := Runtime{
		Runner:       runner,
		Git:          git,
		PullRequests: &fakePullRequests{},
		Outputs:      &recordingOutputs{},
	}

	_, err := runtime.Run(context.Background(), Config{
		BaseBranch: "release/1.x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if git.checkedOutBaseBranch != "release/1.x" {
		t.Fatalf("checked out base branch = %q, want release/1.x", git.checkedOutBaseBranch)
	}
	if !runner.applySawBaseCheckout {
		t.Fatal("runner.Apply ran before base branch checkout")
	}
}

func TestRuntimePropagatesBaseBranchCheckoutFailures(t *testing.T) {
	git := &fakeGit{checkoutBaseErr: errors.New("fetch failed")}
	runtime := Runtime{
		Runner:       &fakeRunner{},
		Git:          git,
		PullRequests: &fakePullRequests{},
		Outputs:      &recordingOutputs{},
	}

	_, err := runtime.Run(context.Background(), Config{BaseBranch: "release/1.x"})
	if err == nil || !strings.Contains(err.Error(), "fetch failed") {
		t.Fatalf("err = %v, want base branch checkout failure", err)
	}
}

func TestRuntimePropagatesPushFailures(t *testing.T) {
	git := &fakeGit{
		hasChanges:    true,
		stagedChanges: true,
		pushErr:       errors.New("push failed"),
	}
	runtime := Runtime{
		Runner:       &fakeRunner{},
		Git:          git,
		PullRequests: &fakePullRequests{},
		Outputs:      &recordingOutputs{},
	}

	_, err := runtime.Run(context.Background(), Config{
		BaseBranch:  "main",
		Repository:  "mishankov/updtr",
		GitHubToken: "secret",
	})
	if err == nil || !strings.Contains(err.Error(), "push failed") {
		t.Fatalf("err = %v, want push failure", err)
	}
}

type fakeRunner struct {
	err                  error
	git                  *fakeGit
	opts                 RunOptions
	applySawBaseCheckout bool
	result               core.RunResult
}

func (r *fakeRunner) Apply(_ context.Context, opts RunOptions) (core.RunResult, error) {
	r.opts = opts
	if r.git != nil {
		r.applySawBaseCheckout = r.git.checkedOutBaseBranch != ""
	}
	if r.err != nil {
		return core.RunResult{}, r.err
	}
	return r.result, nil
}

type fakeGit struct {
	cleanErr             error
	hasChanges           bool
	hasChangesErr        error
	initialUntracked     []string
	currentUntracked     []string
	untrackedCalls       int
	stagedChanges        bool
	stagedErr            error
	checkoutBaseErr      error
	checkoutErr          error
	configureErr         error
	stageErr             error
	commitErr            error
	pushErr              error
	checkedOutBaseBranch string
	checkedOutBranch     string
	commitMessage        string
	pushedBranch         string
	stagedNewFiles       []string
}

func (g *fakeGit) EnsureClean(context.Context) error {
	return g.cleanErr
}

func (g *fakeGit) CheckoutBaseBranch(_ context.Context, branch string) error {
	g.checkedOutBaseBranch = branch
	return g.checkoutBaseErr
}

func (g *fakeGit) UntrackedFiles(context.Context) ([]string, error) {
	g.untrackedCalls++
	if g.untrackedCalls == 1 {
		return slices.Clone(g.initialUntracked), nil
	}
	return slices.Clone(g.currentUntracked), nil
}

func (g *fakeGit) HasChanges(context.Context) (bool, error) {
	return g.hasChanges, g.hasChangesErr
}

func (g *fakeGit) CheckoutManagedBranch(_ context.Context, branch string) error {
	g.checkedOutBranch = branch
	return g.checkoutErr
}

func (g *fakeGit) ConfigureAuthor(context.Context) error {
	return g.configureErr
}

func (g *fakeGit) StageChanges(_ context.Context, newFiles []string) error {
	g.stagedNewFiles = slices.Clone(newFiles)
	return g.stageErr
}

func (g *fakeGit) HasStagedChanges(context.Context) (bool, error) {
	return g.stagedChanges, g.stagedErr
}

func (g *fakeGit) Commit(_ context.Context, message string) error {
	g.commitMessage = message
	return g.commitErr
}

func (g *fakeGit) Push(_ context.Context, branch string) error {
	g.pushedBranch = branch
	return g.pushErr
}

type fakePullRequests struct {
	result  PullRequestResult
	err     error
	request PullRequestRequest
}

func (p *fakePullRequests) Ensure(_ context.Context, req PullRequestRequest) (PullRequestResult, error) {
	p.request = req
	if p.err != nil {
		return PullRequestResult{}, p.err
	}
	return p.result, nil
}

type recordingOutputs struct {
	called  bool
	outputs Outputs
}

func (w *recordingOutputs) Write(outputs Outputs) error {
	w.called = true
	w.outputs = outputs
	return nil
}
