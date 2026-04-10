package action

import (
	"slices"
	"testing"
)

func TestConfigFromEnvUsesGitHubActionDefaults(t *testing.T) {
	t.Setenv("INPUT_CONFIG", "")
	t.Setenv("INPUT_TARGETS", "app, worker\napp")
	t.Setenv("INPUT_COMMIT_MESSAGE", "")
	t.Setenv("INPUT_PULL_REQUEST_TITLE", "")
	t.Setenv("INPUT_BASE_BRANCH", "")
	t.Setenv("GITHUB_BASE_REF", "")
	t.Setenv("GITHUB_REF_NAME", "main")
	t.Setenv("GITHUB_REPOSITORY", "mishankov/updtr")
	t.Setenv("INPUT_GITHUB_TOKEN", "secret")
	t.Setenv("GITHUB_OUTPUT", "/tmp/out")

	cfg := ConfigFromEnv()
	if cfg.ConfigPath != "" {
		t.Fatalf("config path = %q, want empty when input is unset", cfg.ConfigPath)
	}
	if !slices.Equal(cfg.Targets, []string{"app", "worker"}) {
		t.Fatalf("targets = %+v, want app,worker", cfg.Targets)
	}
	if cfg.CommitMessage != DefaultCommitMessage {
		t.Fatalf("commit message = %q, want default", cfg.CommitMessage)
	}
	if cfg.PullRequestTitle != DefaultPullRequestTitle {
		t.Fatalf("pull request title = %q, want default", cfg.PullRequestTitle)
	}
	if cfg.BaseBranch != "main" {
		t.Fatalf("base branch = %q, want main", cfg.BaseBranch)
	}
}

func TestManagedBranchNameIsDeterministicAcrossTargetOrder(t *testing.T) {
	first := ManagedBranchName("configs/updtr.yaml", []string{"worker", "app"}, "main")
	second := ManagedBranchName("configs/updtr.yaml", []string{"app", "worker"}, "main")

	if first != second {
		t.Fatalf("branch names differ: %q != %q", first, second)
	}
}
