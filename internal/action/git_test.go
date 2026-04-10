package action

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestApplyArgsOmitsConfigFlagWhenConfigPathIsUnset(t *testing.T) {
	args := applyArgs(RunOptions{})

	if strings.Join(args, " ") != "apply" {
		t.Fatalf("args = %#v, want only apply when config path is unset", args)
	}
}

func TestApplyArgsIncludesConfigFlagWhenConfigPathIsSet(t *testing.T) {
	args := applyArgs(RunOptions{ConfigPath: "configs/updtr.yml"})

	if strings.Join(args, " ") != "apply --config configs/updtr.yml" {
		t.Fatalf("args = %#v, want apply with explicit config flag", args)
	}
}

func TestCommandGitPrefixesCommandsWithSafeDirectory(t *testing.T) {
	ctx := context.Background()
	runner := &recordingGitRunner{}
	g := commandGit{
		runner:        runner,
		safeDirectory: "/github/workspace",
	}

	if _, err := g.trackedStatus(ctx); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"-c",
		"safe.directory=/github/workspace",
		"status",
		"--porcelain",
		"--untracked-files=no",
	}
	if len(runner.calls) != 1 {
		t.Fatalf("git calls = %d, want 1", len(runner.calls))
	}
	if !reflect.DeepEqual(runner.calls[0], want) {
		t.Fatalf("git args = %#v, want %#v", runner.calls[0], want)
	}
}

func TestCommandGitIgnoresUntrackedFiles(t *testing.T) {
	ctx := context.Background()
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, "tracked.txt"), "base\n")
	runGit(t, repo, "add", "tracked.txt")
	runGit(t, repo, "commit", "-m", "base")

	restore := chdir(t, repo)
	defer restore()

	g := newCommandGit()
	if err := g.EnsureClean(ctx); err != nil {
		t.Fatalf("EnsureClean before changes: %v", err)
	}

	writeFile(t, filepath.Join(repo, "new.txt"), "new\n")

	if err := g.EnsureClean(ctx); err != nil {
		t.Fatalf("EnsureClean with untracked file: %v", err)
	}
	changed, err := g.HasChanges(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("HasChanges = true, want false for untracked file")
	}
	if err := g.StageChanges(ctx, nil); err != nil {
		t.Fatal(err)
	}
	staged, err := g.HasStagedChanges(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if staged {
		t.Fatal("HasStagedChanges = true, want false")
	}
	out := runGit(t, repo, "diff", "--cached", "--name-only")
	if strings.TrimSpace(out) != "" {
		t.Fatalf("staged files = %q, want none", out)
	}
}

func TestCommandGitTracksModifiedFiles(t *testing.T) {
	ctx := context.Background()
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, "tracked.txt"), "base\n")
	runGit(t, repo, "add", "tracked.txt")
	runGit(t, repo, "commit", "-m", "base")

	restore := chdir(t, repo)
	defer restore()

	g := newCommandGit()
	writeFile(t, filepath.Join(repo, "tracked.txt"), "updated\n")

	if err := g.EnsureClean(ctx); err == nil {
		t.Fatal("EnsureClean succeeded with tracked changes, want error")
	}
	changed, err := g.HasChanges(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("HasChanges = false, want true for tracked file")
	}
	if err := g.StageChanges(ctx, nil); err != nil {
		t.Fatal(err)
	}
	staged, err := g.HasStagedChanges(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !staged {
		t.Fatal("HasStagedChanges = false, want true")
	}
	out := runGit(t, repo, "diff", "--cached", "--name-only")
	if !strings.Contains(out, "tracked.txt") {
		t.Fatalf("staged files = %q, want tracked.txt", out)
	}
}

func TestCommandGitStagesRequestedNewFilesWithoutSweepingExistingUntrackedFiles(t *testing.T) {
	ctx := context.Background()
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, "tracked.txt"), "base\n")
	runGit(t, repo, "add", "tracked.txt")
	runGit(t, repo, "commit", "-m", "base")

	restore := chdir(t, repo)
	defer restore()

	writeFile(t, filepath.Join(repo, "existing.log"), "keep me untracked\n")
	writeFile(t, filepath.Join(repo, "go.sum"), "newly created by apply\n")

	g := newCommandGit()
	if err := g.StageChanges(ctx, []string{"go.sum"}); err != nil {
		t.Fatal(err)
	}

	out := runGit(t, repo, "diff", "--cached", "--name-only")
	if !strings.Contains(out, "go.sum") {
		t.Fatalf("staged files = %q, want go.sum", out)
	}
	if strings.Contains(out, "existing.log") {
		t.Fatalf("staged files = %q, want existing.log to remain unstaged", out)
	}
}

func TestCommandGitPushUpdatesExistingManagedBranchFromShallowCheckout(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	seed := filepath.Join(root, "seed")
	clone := filepath.Join(root, "clone")

	runGit(t, root, "init", "--bare", remote)

	mustMkdir(t, seed)
	configureRepo(t, seed)
	writeFile(t, filepath.Join(seed, "file.txt"), "base\n")
	runGit(t, seed, "add", "file.txt")
	runGit(t, seed, "commit", "-m", "base")
	runGit(t, seed, "branch", "-M", "main")
	runGit(t, seed, "remote", "add", "origin", remote)
	runGit(t, seed, "push", "-u", "origin", "main")

	runGit(t, seed, "checkout", "-b", "updtr/test")
	writeFile(t, filepath.Join(seed, "file.txt"), "first\n")
	runGit(t, seed, "commit", "-am", "first")
	runGit(t, seed, "push", "-u", "origin", "updtr/test")

	runGit(t, root, "clone", "--depth", "1", "--branch", "main", "file://"+remote, clone)
	configureRepo(t, clone)

	restore := chdir(t, clone)
	defer restore()

	g := newCommandGit()
	if err := g.CheckoutManagedBranch(ctx, "updtr/test"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(clone, "file.txt"), "second\n")
	runGit(t, clone, "commit", "-am", "second")

	if err := g.Push(ctx, "updtr/test"); err != nil {
		t.Fatalf("Push returned error: %v", err)
	}

	remoteOID := strings.Fields(runGit(t, clone, "ls-remote", "origin", "refs/heads/updtr/test"))
	if len(remoteOID) == 0 {
		t.Fatal("ls-remote returned no output for managed branch")
	}
	localOID := strings.TrimSpace(runGit(t, clone, "rev-parse", "HEAD"))
	if remoteOID[0] != localOID {
		t.Fatalf("remote oid = %s, want %s", remoteOID[0], localOID)
	}
}

func TestCommandGitCheckoutBaseBranchFetchesOverriddenBranchFromShallowClone(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	seed := filepath.Join(root, "seed")
	clone := filepath.Join(root, "clone")

	runGit(t, root, "init", "--bare", remote)

	mustMkdir(t, seed)
	configureRepo(t, seed)
	writeFile(t, filepath.Join(seed, "file.txt"), "main\n")
	runGit(t, seed, "add", "file.txt")
	runGit(t, seed, "commit", "-m", "main")
	runGit(t, seed, "branch", "-M", "main")
	runGit(t, seed, "remote", "add", "origin", remote)
	runGit(t, seed, "push", "-u", "origin", "main")

	runGit(t, seed, "checkout", "-b", "release/1.x")
	writeFile(t, filepath.Join(seed, "file.txt"), "release\n")
	runGit(t, seed, "commit", "-am", "release")
	runGit(t, seed, "push", "-u", "origin", "release/1.x")

	runGit(t, root, "clone", "--depth", "1", "--branch", "main", "file://"+remote, clone)
	configureRepo(t, clone)

	restore := chdir(t, clone)
	defer restore()

	g := newCommandGit()
	if err := g.CheckoutBaseBranch(ctx, "release/1.x"); err != nil {
		t.Fatal(err)
	}

	head := strings.TrimSpace(runGit(t, clone, "rev-parse", "HEAD"))
	release := strings.TrimSpace(runGit(t, clone, "rev-parse", "refs/remotes/origin/release/1.x"))
	if head != release {
		t.Fatalf("HEAD = %s, want release branch %s", head, release)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	configureRepo(t, repo)
	return repo
}

func configureRepo(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		runGit(t, filepath.Dir(dir), "init", dir)
	}
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "config", "user.email", "test@example.com")
}

func chdir(t *testing.T, dir string) func() {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return string(out)
}

type recordingGitRunner struct {
	calls [][]string
}

func (r *recordingGitRunner) CombinedOutput(_ context.Context, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	return []byte(""), nil
}
