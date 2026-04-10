package action

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerActionImageRunsAgainstCommittedSampleRepo(t *testing.T) {
	if os.Getenv("UPDTR_RUN_DOCKER_TESTS") != "1" {
		t.Skip("set UPDTR_RUN_DOCKER_TESTS=1 to run Docker integration tests")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skipf("docker not available: %v", err)
	}
	if out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").CombinedOutput(); err != nil {
		t.Skipf("docker daemon unavailable: %s", strings.TrimSpace(string(out)))
	}

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, "go.mod"), "module example.com/sample\n\ngo 1.23\n")
	writeFile(t, filepath.Join(repo, "updtr.yaml"), "targets:\n  - name: root\n    ecosystem: go\n    path: .\n")
	runGit(t, repo, "add", "go.mod", "updtr.yaml")
	runGit(t, repo, "commit", "-m", "sample repo")

	image := fmt.Sprintf("updtr-action-test-%d", os.Getpid())
	build := exec.Command("docker", "build", "-t", image, ".")
	build.Dir = root
	buildOut, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("docker build failed: %s", strings.TrimSpace(string(buildOut)))
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "rmi", "-f", image).Run()
	})

	run := exec.Command(
		"docker", "run", "--rm",
		"-v", repo+":/workspace",
		"-w", "/workspace",
		"-e", "GITHUB_REF_NAME=main",
		image,
	)
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("docker run failed: %s", strings.TrimSpace(string(out)))
	}
	if !strings.Contains(string(out), "No repository changes detected.") {
		t.Fatalf("docker action output = %q, want noop success log", out)
	}
}
