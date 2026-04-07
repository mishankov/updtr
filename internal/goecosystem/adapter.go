package goecosystem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mishankov/updtr/internal/config"
	"github.com/mishankov/updtr/internal/core"
	"github.com/mishankov/updtr/internal/policy"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

type Adapter struct {
	Now func() time.Time
}

func New() *Adapter {
	return &Adapter{Now: time.Now}
}

func (a *Adapter) CheckPrereq() error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("go binary is required for selected go targets: %w", err)
	}
	return nil
}

func (a *Adapter) PlanTarget(ctx context.Context, target config.Target) core.TargetPlan {
	plan := core.TargetPlan{Target: target}
	if err := validatePins(target.Policy.Pins); err != nil {
		plan.Error = err.Error()
		return plan
	}

	state, err := readModuleState(target.AbsPath)
	if err != nil {
		plan.Error = err.Error()
		return plan
	}

	modules := make([]string, 0, len(state.Direct))
	for module := range state.Direct {
		modules = append(modules, module)
	}
	sort.Strings(modules)

	now := a.now()
	for _, module := range modules {
		current := state.Direct[module]
		if state.Replaced.Contains(module, current) {
			plan.Decisions = append(plan.Decisions, core.Decision{
				ModulePath:     module,
				CurrentVersion: current,
				BlockedReason:  core.ReasonReplacedDependency,
			})
			continue
		}

		candidate, err := a.resolveCandidate(ctx, target.AbsPath, module, current)
		input := policy.Input{ModulePath: module, CurrentVersion: current}
		if err != nil {
			input.ResolutionError = err.Error()
		} else if candidate.Version != "" {
			input.CandidateVersion = candidate.Version
			input.ReleaseTime = candidate.ReleaseTime
			input.ReleaseTrusted = candidate.ReleaseTrusted
		}
		if decision, show := policy.Decide(target.Policy, input, now); show {
			plan.Decisions = append(plan.Decisions, decision)
		}
	}
	return plan
}

func (a *Adapter) ApplyUpdate(ctx context.Context, target config.Target, modulePath string, version string) (string, error) {
	return runGo(ctx, target.AbsPath, "get", modulePath+"@"+version)
}

func (a *Adapter) Tidy(ctx context.Context, target config.Target) (string, error) {
	return runGo(ctx, target.AbsPath, "mod", "tidy")
}

func (a *Adapter) DirectVersions(target config.Target) (map[string]string, error) {
	state, err := readModuleState(target.AbsPath)
	if err != nil {
		return nil, err
	}
	return state.Direct, nil
}

func (a *Adapter) now() time.Time {
	if a.Now == nil {
		return time.Now()
	}
	return a.Now()
}

type moduleState struct {
	Direct   map[string]string
	Replaced replacedModules
}

type replacedModules map[string]map[string]struct{}

func (m replacedModules) Add(path string, version string) {
	if m[path] == nil {
		m[path] = map[string]struct{}{}
	}
	m[path][version] = struct{}{}
}

func (m replacedModules) Contains(path string, version string) bool {
	versions, ok := m[path]
	if !ok {
		return false
	}
	if _, allVersions := versions[""]; allVersions {
		return true
	}
	_, exactVersion := versions[version]
	return exactVersion
}

func readModuleState(dir string) (moduleState, error) {
	path := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		return moduleState{}, fmt.Errorf("read go.mod for %s: %w", dir, err)
	}
	parsed, err := modfile.Parse(path, data, nil)
	if err != nil {
		return moduleState{}, fmt.Errorf("parse go.mod for %s: %w", dir, err)
	}
	state := moduleState{
		Direct:   map[string]string{},
		Replaced: replacedModules{},
	}
	for _, req := range parsed.Require {
		if req.Indirect {
			continue
		}
		state.Direct[req.Mod.Path] = req.Mod.Version
	}
	for _, replace := range parsed.Replace {
		state.Replaced.Add(replace.Old.Path, replace.Old.Version)
	}
	return state, nil
}

func validatePins(pins map[string]string) error {
	for module, version := range pins {
		if !semver.IsValid(version) {
			return fmt.Errorf("invalid pin version for %s: %q", module, version)
		}
	}
	return nil
}

type candidate struct {
	Version        string
	ReleaseTime    *time.Time
	ReleaseTrusted bool
}

type moduleVersions struct {
	Path     string   `json:"Path"`
	Versions []string `json:"Versions"`
}

type moduleInfo struct {
	Path    string     `json:"Path"`
	Version string     `json:"Version"`
	Time    *time.Time `json:"Time"`
}

func (a *Adapter) resolveCandidate(ctx context.Context, dir string, modulePath string, current string) (candidate, error) {
	if !semver.IsValid(current) {
		return candidate{}, fmt.Errorf("current version %q is not a valid Go module version", current)
	}
	output, err := runGo(ctx, dir, "list", "-m", "-versions", "-json", modulePath)
	if err != nil {
		return candidate{}, err
	}
	var versions moduleVersions
	if err := json.Unmarshal([]byte(output), &versions); err != nil {
		return candidate{}, fmt.Errorf("parse versions for %s: %w", modulePath, err)
	}
	selected := selectCandidate(current, versions.Versions)
	if selected == "" {
		return candidate{}, nil
	}

	infoOutput, err := runGo(ctx, dir, "list", "-m", "-json", modulePath+"@"+selected)
	if err != nil {
		return candidate{Version: selected}, nil
	}
	var info moduleInfo
	if err := json.Unmarshal([]byte(infoOutput), &info); err != nil {
		return candidate{Version: selected}, nil
	}
	return candidate{Version: selected, ReleaseTime: info.Time, ReleaseTrusted: info.Time != nil}, nil
}

func selectCandidate(current string, versions []string) string {
	currentStable := semver.Prerelease(current) == ""
	best := ""
	for _, version := range versions {
		if !semver.IsValid(version) || semver.Compare(version, current) <= 0 {
			continue
		}
		if currentStable && semver.Prerelease(version) != "" {
			continue
		}
		if best == "" || semver.Compare(version, best) > 0 {
			best = version
		}
	}
	return best
}

func runGo(ctx context.Context, dir string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "go", args...)
	command.Dir = dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail == "" {
			detail = err.Error()
		}
		return stdout.String(), fmt.Errorf("go %s failed: %s", strings.Join(args, " "), detail)
	}
	return stdout.String(), nil
}
