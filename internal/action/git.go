package action

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

type gitRunner interface {
	CombinedOutput(context.Context, ...string) ([]byte, error)
}

type execGitRunner struct{}

func (execGitRunner) CombinedOutput(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	return cmd.CombinedOutput()
}

type commandGit struct {
	runner        gitRunner
	safeDirectory string
}

func newCommandGit() commandGit {
	wd, err := os.Getwd()
	if err != nil {
		wd = ""
	}
	return commandGit{
		runner:        execGitRunner{},
		safeDirectory: wd,
	}
}

func (g commandGit) EnsureClean(ctx context.Context) error {
	status, err := g.trackedStatus(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("repository has changes before updtr action started")
	}
	return nil
}

func (g commandGit) HasChanges(ctx context.Context) (bool, error) {
	status, err := g.trackedStatus(ctx)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(status) != "", nil
}

func (g commandGit) UntrackedFiles(ctx context.Context) ([]string, error) {
	out, err := g.output(ctx, "ls-files", "--others", "--exclude-standard", "--", ".")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	files := strings.Split(out, "\n")
	slices.Sort(files)
	return files, nil
}

func (g commandGit) CheckoutBaseBranch(ctx context.Context, branch string) error {
	remoteRef := "refs/remotes/origin/" + branch
	fetchRef := "+refs/heads/" + branch + ":" + remoteRef
	if err := g.run(ctx, "fetch", "--no-tags", "--depth=1", "origin", fetchRef); err != nil {
		return err
	}
	return g.run(ctx, "checkout", "--detach", remoteRef)
}

func (g commandGit) CheckoutManagedBranch(ctx context.Context, branch string) error {
	return g.run(ctx, "checkout", "-B", branch)
}

func (g commandGit) ConfigureAuthor(ctx context.Context) error {
	if err := g.run(ctx, "config", "user.name", "github-actions[bot]"); err != nil {
		return err
	}
	return g.run(ctx, "config", "user.email", "41898282+github-actions[bot]@users.noreply.github.com")
}

func (g commandGit) StageChanges(ctx context.Context, newFiles []string) error {
	if err := g.run(ctx, "add", "--update", "--", "."); err != nil {
		return err
	}
	if len(newFiles) == 0 {
		return nil
	}
	toStage := slices.Clone(newFiles)
	slices.Sort(toStage)
	args := append([]string{"add", "--"}, toStage...)
	return g.run(ctx, args...)
}

func (g commandGit) HasStagedChanges(ctx context.Context) (bool, error) {
	out, err := g.output(ctx, "diff", "--cached", "--name-only")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (g commandGit) Commit(ctx context.Context, message string) error {
	return g.run(ctx, "commit", "-m", message)
}

func (g commandGit) Push(ctx context.Context, branch string) error {
	remoteRef := "refs/heads/" + branch
	expectedOID, exists, err := g.remoteRefOID(ctx, "origin", remoteRef)
	if err != nil {
		return err
	}
	lease := "--force-with-lease=" + remoteRef + ":"
	if exists {
		lease += expectedOID
	}
	return g.run(ctx, "push", lease, "origin", "HEAD:"+remoteRef)
}

func (g commandGit) status(ctx context.Context) (string, error) {
	return g.output(ctx, "status", "--porcelain")
}

func (g commandGit) trackedStatus(ctx context.Context) (string, error) {
	return g.output(ctx, "status", "--porcelain", "--untracked-files=no")
}

func (g commandGit) remoteRefOID(ctx context.Context, remote string, ref string) (string, bool, error) {
	args := []string{"ls-remote", "--exit-code", remote, ref}
	out, err := g.runner.CombinedOutput(ctx, g.gitArgs(args)...)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
			return "", false, nil
		}
		return "", false, gitError(args, out, err)
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", false, fmt.Errorf("git %s: unexpected empty output", strings.Join(args, " "))
	}
	return fields[0], true, nil
}

func (g commandGit) output(ctx context.Context, args ...string) (string, error) {
	out, err := g.runner.CombinedOutput(ctx, g.gitArgs(args)...)
	if err != nil {
		return "", gitError(args, out, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (g commandGit) run(ctx context.Context, args ...string) error {
	_, err := g.output(ctx, args...)
	return err
}

func (g commandGit) gitArgs(args []string) []string {
	if g.safeDirectory == "" {
		return args
	}
	return append([]string{"-c", "safe.directory=" + g.safeDirectory}, args...)
}

func gitError(args []string, out []byte, err error) error {
	message := strings.TrimSpace(string(bytes.TrimSpace(out)))
	if message == "" {
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return fmt.Errorf("git %s: %s", strings.Join(args, " "), message)
}

type fileOutputWriter struct {
	path string
}

var osWriteFile = func(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (w fileOutputWriter) Write(outputs Outputs) error {
	if w.path == "" {
		return nil
	}
	var body strings.Builder
	writeOutput := func(key string, value string) {
		body.WriteString(key)
		body.WriteByte('=')
		body.WriteString(value)
		body.WriteByte('\n')
	}
	writeOutput("changed", fmt.Sprintf("%t", outputs.Changed))
	writeOutput("committed", fmt.Sprintf("%t", outputs.Committed))
	writeOutput("branch", outputs.Branch)
	writeOutput("pull_request_operation", outputs.PullRequestOperation)
	writeOutput("pull_request_number", outputs.PullRequestNumber)
	writeOutput("pull_request_url", outputs.PullRequestURL)
	return osWriteFile(w.path, []byte(body.String()), 0o644)
}
