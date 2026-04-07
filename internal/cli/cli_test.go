package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpMentionsYAMLConfigDefault(t *testing.T) {
	var out bytes.Buffer
	cmd := New("test", &out, &out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	help := out.String()
	if !strings.Contains(help, "--config") || !strings.Contains(help, "updtr.yaml") {
		t.Fatalf("help = %q, want --config default updtr.yaml", help)
	}
}

func TestInitHelpMentionsYAMLConfig(t *testing.T) {
	var out bytes.Buffer
	cmd := New("test", &out, &out)
	cmd.SetArgs([]string{"init", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	help := out.String()
	if !strings.Contains(help, "Create updtr.yaml from discovered Go modules") {
		t.Fatalf("init help = %q, want YAML init summary", help)
	}
}

func TestLoadPathUsesDefaultDiscoveryUnlessConfigFlagChanged(t *testing.T) {
	var out bytes.Buffer
	root := New("test", &out, &out)
	detect, _, err := root.Find([]string{"detect"})
	if err != nil {
		t.Fatal(err)
	}

	if got := loadPath(detect, "custom.yml"); got != "" {
		t.Fatalf("load path = %q, want default discovery", got)
	}
	flag := detect.Flag("config")
	if flag == nil {
		t.Fatal("detect command missing inherited config flag")
	}
	flag.Changed = true
	if got := loadPath(detect, "custom.yml"); got != "custom.yml" {
		t.Fatalf("load path = %q, want explicit config path", got)
	}
}
