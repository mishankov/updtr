package action

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestActionMetadataContract(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(root, "action.yml"))
	if err != nil {
		t.Fatal(err)
	}

	var metadata struct {
		Inputs map[string]struct {
			Default any `yaml:"default"`
		} `yaml:"inputs"`
		Outputs map[string]any `yaml:"outputs"`
		Runs    struct {
			Using string `yaml:"using"`
			Image string `yaml:"image"`
		} `yaml:"runs"`
	}
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}

	if metadata.Runs.Using != "docker" {
		t.Fatalf("runs.using = %q, want docker", metadata.Runs.Using)
	}
	if metadata.Runs.Image != "Dockerfile" {
		t.Fatalf("runs.image = %q, want Dockerfile", metadata.Runs.Image)
	}
	if got := metadata.Inputs["config"].Default; got != nil {
		t.Fatalf("config default = %#v, want omitted so CLI fallback handles updtr.yml", got)
	}
	if got := metadata.Inputs["commit-message"].Default; got != DefaultCommitMessage {
		t.Fatalf("commit-message default = %#v, want %q", got, DefaultCommitMessage)
	}
	if got := metadata.Inputs["pull-request-title"].Default; got != DefaultPullRequestTitle {
		t.Fatalf("pull-request-title default = %#v, want %q", got, DefaultPullRequestTitle)
	}

	for _, output := range []string{"changed", "committed", "branch", "pull_request_operation", "pull_request_number", "pull_request_url"} {
		if _, ok := metadata.Outputs[output]; !ok {
			t.Fatalf("missing output %q", output)
		}
	}
}

func TestDockerfileRunsActionEntrypoint(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `ENTRYPOINT ["/usr/local/bin/updtr", "action"]`) {
		t.Fatalf("dockerfile = %q, want action entrypoint", text)
	}
}

func TestDockerfileRuntimeStageIncludesGoToolchain(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}

	var stages []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "FROM ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			t.Fatalf("malformed FROM line: %q", line)
		}
		stages = append(stages, fields[1])
	}
	if len(stages) == 0 {
		t.Fatal("dockerfile contains no stages")
	}
	if got := stages[len(stages)-1]; !strings.HasPrefix(got, "golang:") {
		t.Fatalf("runtime stage image = %q, want a golang image so the action can run go-based updates", got)
	}
}
