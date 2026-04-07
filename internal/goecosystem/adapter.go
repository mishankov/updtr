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
	Now                  func() time.Time
	listVersions         func(context.Context, string, string) ([]string, error)
	releaseLookup        func(context.Context, string, string, string) candidate
	vulnerabilityScanner VulnerabilityScanner
}

func New() *Adapter {
	return &Adapter{Now: time.Now, vulnerabilityScanner: GovulncheckScanner{}}
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

	vulnerabilities := []core.Vulnerability(nil)
	if target.Policy.UpdateMode == config.UpdateModeVulnerabilityOnly {
		scanner := a.scanner()
		vulnerabilities, err = scanner.Scan(ctx, target)
		if err != nil {
			plan.Error = err.Error()
			return plan
		}
	}

	now := a.now()
	for _, requirement := range state.Requirements(target.IncludeIndirect) {
		relevantVulnerabilities := relevantVulnerabilitiesForRequirement(vulnerabilities, requirement)
		if target.Policy.UpdateMode == config.UpdateModeVulnerabilityOnly && len(relevantVulnerabilities) == 0 {
			continue
		}

		if state.Replaced.Contains(requirement.Path, requirement.Version) {
			decision := core.Decision{
				ModulePath:      requirement.Path,
				CurrentVersion:  requirement.Version,
				Relationship:    requirement.Relationship,
				Vulnerabilities: relevantVulnerabilities,
				BlockedReason:   core.ReasonReplacedDependency,
			}
			plan.Decisions = append(plan.Decisions, decision)
			continue
		}

		var candidateFilter func(string) bool
		if target.Policy.UpdateMode == config.UpdateModeVulnerabilityOnly {
			candidateFilter = func(version string) bool {
				return len(vulnerabilitiesFixedByCandidate(relevantVulnerabilities, version)) > 0
			}
		}

		if decision, show := a.selectDecision(ctx, target, requirement, now, candidateFilter); show {
			if target.Policy.UpdateMode == config.UpdateModeVulnerabilityOnly {
				decision.Vulnerabilities = relevantVulnerabilitiesForDecision(relevantVulnerabilities, decision)
			}
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

func (a *Adapter) scanner() VulnerabilityScanner {
	if a.vulnerabilityScanner != nil {
		return a.vulnerabilityScanner
	}
	return GovulncheckScanner{}
}

type moduleState struct {
	Direct   map[string]string
	Indirect map[string]string
	Replaced replacedModules
}

type dependencyRequirement struct {
	Path         string
	Version      string
	Relationship core.DependencyRelationship
}

func (s moduleState) Requirements(includeIndirect bool) []dependencyRequirement {
	requirements := make([]dependencyRequirement, 0, len(s.Direct)+len(s.Indirect))
	for module, version := range s.Direct {
		requirements = append(requirements, dependencyRequirement{
			Path:         module,
			Version:      version,
			Relationship: core.RelationshipDirect,
		})
	}
	if includeIndirect {
		for module, version := range s.Indirect {
			if _, direct := s.Direct[module]; direct {
				continue
			}
			requirements = append(requirements, dependencyRequirement{
				Path:         module,
				Version:      version,
				Relationship: core.RelationshipIndirect,
			})
		}
	}
	sort.Slice(requirements, func(i, j int) bool {
		if requirements[i].Path != requirements[j].Path {
			return requirements[i].Path < requirements[j].Path
		}
		return requirements[i].Relationship < requirements[j].Relationship
	})
	return requirements
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
		Indirect: map[string]string{},
		Replaced: replacedModules{},
	}
	for _, req := range parsed.Require {
		if req.Indirect {
			if _, direct := state.Direct[req.Mod.Path]; !direct {
				state.Indirect[req.Mod.Path] = req.Mod.Version
			}
			continue
		}
		state.Direct[req.Mod.Path] = req.Mod.Version
		delete(state.Indirect, req.Mod.Path)
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

func (a *Adapter) selectDecision(ctx context.Context, target config.Target, requirement dependencyRequirement, now time.Time, candidateFilter func(string) bool) (core.Decision, bool) {
	input := policy.Input{
		ModulePath:     requirement.Path,
		CurrentVersion: requirement.Version,
		Relationship:   requirement.Relationship,
	}

	if !semver.IsValid(requirement.Version) {
		input.ResolutionError = fmt.Sprintf("current version %q is not a valid Go module version", requirement.Version)
		return policy.Decide(target.Policy, input, now)
	}

	versions, err := a.moduleVersions(ctx, target.AbsPath, requirement.Path)
	if err != nil {
		input.ResolutionError = err.Error()
		return policy.Decide(target.Policy, input, now)
	}

	candidates := selectCandidates(requirement.Version, versions)
	if len(candidates) == 0 {
		if candidateFilter != nil {
			return core.Decision{}, false
		}
		return policy.Decide(target.Policy, input, now)
	}

	var firstBlocked *core.Decision
	filteredCandidates := 0
	for _, version := range candidates {
		if candidateFilter != nil && !candidateFilter(version) {
			continue
		}
		filteredCandidates++

		candidate := a.candidateRelease(ctx, target.AbsPath, requirement.Path, version)
		candidateInput := input
		candidateInput.CandidateVersion = candidate.Version
		candidateInput.ReleaseTime = candidate.ReleaseTime
		candidateInput.ReleaseTrusted = candidate.ReleaseTrusted

		decision, show := policy.Decide(target.Policy, candidateInput, now)
		if !show {
			continue
		}
		if decision.Eligible {
			return decision, true
		}
		if candidateLocalBlocker(target.Policy, decision.BlockedReason) {
			if firstBlocked == nil {
				blocked := decision
				firstBlocked = &blocked
			}
			continue
		}
		return decision, true
	}

	if firstBlocked != nil {
		return *firstBlocked, true
	}
	if candidateFilter != nil && filteredCandidates == 0 {
		return core.Decision{}, false
	}
	return policy.Decide(target.Policy, input, now)
}

func candidateLocalBlocker(policy config.Policy, reason core.Reason) bool {
	if policy.QuarantineDays == nil {
		return false
	}
	switch reason {
	case core.ReasonQuarantined, core.ReasonMissingReleaseDate, core.ReasonUntrustedReleaseDate:
		return true
	default:
		return false
	}
}

func (a *Adapter) moduleVersions(ctx context.Context, dir string, modulePath string) ([]string, error) {
	if a.listVersions != nil {
		return a.listVersions(ctx, dir, modulePath)
	}
	output, err := runGo(ctx, dir, "list", "-m", "-versions", "-json", modulePath)
	if err != nil {
		return nil, err
	}
	var versions moduleVersions
	if err := json.Unmarshal([]byte(output), &versions); err != nil {
		return nil, fmt.Errorf("parse versions for %s: %w", modulePath, err)
	}
	return versions.Versions, nil
}

func (a *Adapter) candidateRelease(ctx context.Context, dir string, modulePath string, version string) candidate {
	if a.releaseLookup != nil {
		candidate := a.releaseLookup(ctx, dir, modulePath, version)
		candidate.Version = version
		return candidate
	}
	return a.goCandidateRelease(ctx, dir, modulePath, version)
}

func (a *Adapter) goCandidateRelease(ctx context.Context, dir string, modulePath string, version string) candidate {
	infoOutput, err := runGo(ctx, dir, "list", "-m", "-json", modulePath+"@"+version)
	if err != nil {
		return candidate{Version: version}
	}
	var info moduleInfo
	if err := json.Unmarshal([]byte(infoOutput), &info); err != nil {
		return candidate{Version: version}
	}
	return candidate{Version: version, ReleaseTime: info.Time, ReleaseTrusted: info.Time != nil}
}

func selectCandidates(current string, versions []string) []string {
	currentStable := semver.Prerelease(current) == ""
	var candidates []string
	for _, version := range versions {
		if !semver.IsValid(version) || semver.Compare(version, current) <= 0 {
			continue
		}
		if currentStable && semver.Prerelease(version) != "" {
			continue
		}
		candidates = append(candidates, version)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return semver.Compare(candidates[i], candidates[j]) > 0
	})
	return candidates
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
