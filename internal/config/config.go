package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"go.yaml.in/yaml/v3"
)

var targetNamePattern = regexp.MustCompile(`^[a-z0-9-]+$`)

const (
	defaultConfigPath         = "updtr.yaml"
	defaultConfigFallbackPath = "updtr.yml"
)

type rawConfig struct {
	Policy  *rawPolicy  `yaml:"policy"`
	Targets []rawTarget `yaml:"targets"`
}

type rawPolicy struct {
	QuarantineDays *int              `yaml:"quarantine_days"`
	UpdateMode     *string           `yaml:"update_mode"`
	Allow          *[]string         `yaml:"allow"`
	Deny           *[]string         `yaml:"deny"`
	Pin            map[string]string `yaml:"pin"`
}

type rawTarget struct {
	Name            string            `yaml:"name"`
	Ecosystem       string            `yaml:"ecosystem"`
	Path            string            `yaml:"path"`
	IncludeIndirect bool              `yaml:"include_indirect"`
	QuarantineDays  *int              `yaml:"quarantine_days"`
	UpdateMode      *string           `yaml:"update_mode"`
	Allow           *[]string         `yaml:"allow"`
	Deny            *[]string         `yaml:"deny"`
	Pin             map[string]string `yaml:"pin"`
}

type Config struct {
	Path    string
	BaseDir string
	Targets []Target
}

type Target struct {
	Name            string
	Ecosystem       string
	Path            string
	NormalizedPath  string
	AbsPath         string
	IncludeIndirect bool
	Policy          Policy
}

type Policy struct {
	QuarantineDays *int
	UpdateMode     UpdateMode
	Allow          []string
	AllowSet       bool
	Deny           []string
	DenySet        bool
	Pins           map[string]string
}

type UpdateMode string

const (
	UpdateModeNormal            UpdateMode = "normal"
	UpdateModeVulnerabilityOnly UpdateMode = "vulnerability_only"
)

func Load(path string) (*Config, error) {
	resolvedPath, err := configPath(path)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("load config %s: %w", resolvedPath, err)
	}
	defer file.Close()

	var raw rawConfig
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", resolvedPath, err)
	}
	var extra rawConfig
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return nil, fmt.Errorf("parse config %s: %w", resolvedPath, err)
		}
		return nil, fmt.Errorf("parse config %s: multiple YAML documents are not supported", resolvedPath)
	}

	absConfig, err := filepath.Abs(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("resolve config path %s: %w", resolvedPath, err)
	}
	cfg := &Config{Path: absConfig, BaseDir: filepath.Dir(absConfig)}
	if err := validateAndResolve(&raw, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func configPath(path string) (string, error) {
	if path == "" {
		if _, err := os.Stat(defaultConfigPath); err == nil {
			return defaultConfigPath, nil
		} else if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("check config %s: %w", defaultConfigPath, err)
		}
		if _, err := os.Stat(defaultConfigFallbackPath); err == nil {
			return defaultConfigFallbackPath, nil
		} else if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("check config %s: %w", defaultConfigFallbackPath, err)
		}
		return defaultConfigPath, nil
	}
	if !isSupportedConfigPath(path) {
		return "", errors.New("unsupported config extension: accepted extensions are .yaml and .yml")
	}
	return path, nil
}

func isSupportedConfigPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func validateAndResolve(raw *rawConfig, cfg *Config) error {
	if len(raw.Targets) == 0 {
		return errors.New("config must define at least one target")
	}

	basePolicy := Policy{UpdateMode: UpdateModeNormal, Pins: map[string]string{}}
	if raw.Policy != nil {
		policy, err := resolvePolicy("policy", raw.Policy)
		if err != nil {
			return err
		}
		basePolicy = policy
	}

	seenNames := map[string]struct{}{}
	seenTargets := map[string]struct{}{}
	for i, target := range raw.Targets {
		label := fmt.Sprintf("targets[%d]", i)
		if target.Name == "" {
			return fmt.Errorf("%s.name is required", label)
		}
		if !targetNamePattern.MatchString(target.Name) {
			return fmt.Errorf("%s.name %q must match [a-z0-9-]+", label, target.Name)
		}
		if _, ok := seenNames[target.Name]; ok {
			return fmt.Errorf("duplicate target name %q", target.Name)
		}
		seenNames[target.Name] = struct{}{}

		if target.Ecosystem == "" {
			return fmt.Errorf("%s.ecosystem is required", label)
		}
		if target.Ecosystem != "go" {
			return fmt.Errorf("%s.ecosystem %q is unsupported", label, target.Ecosystem)
		}
		normalized, err := normalizeTargetPath(target.Path)
		if err != nil {
			return fmt.Errorf("%s.path: %w", label, err)
		}
		effectiveKey := target.Ecosystem + "\x00" + normalized
		if _, ok := seenTargets[effectiveKey]; ok {
			return fmt.Errorf("duplicate effective target %s:%s", target.Ecosystem, normalized)
		}
		seenTargets[effectiveKey] = struct{}{}

		targetPolicy, err := resolveTargetPolicy(label, basePolicy, target)
		if err != nil {
			return err
		}
		cfg.Targets = append(cfg.Targets, Target{
			Name:            target.Name,
			Ecosystem:       target.Ecosystem,
			Path:            target.Path,
			NormalizedPath:  normalized,
			AbsPath:         filepath.Join(cfg.BaseDir, filepath.FromSlash(normalized)),
			IncludeIndirect: target.IncludeIndirect,
			Policy:          targetPolicy,
		})
	}
	return nil
}

func resolvePolicy(label string, raw *rawPolicy) (Policy, error) {
	policy := Policy{UpdateMode: UpdateModeNormal, Pins: map[string]string{}}
	if raw.QuarantineDays != nil {
		if *raw.QuarantineDays < 0 {
			return policy, fmt.Errorf("%s.quarantine_days must be non-negative", label)
		}
		policy.QuarantineDays = intPtr(*raw.QuarantineDays)
	}
	if raw.UpdateMode != nil {
		updateMode, err := validateUpdateMode(label+".update_mode", *raw.UpdateMode)
		if err != nil {
			return policy, err
		}
		policy.UpdateMode = updateMode
	}
	if raw.Allow != nil {
		allow, err := validateModuleList(label+".allow", *raw.Allow)
		if err != nil {
			return policy, err
		}
		policy.Allow = allow
		policy.AllowSet = true
	}
	if raw.Deny != nil {
		deny, err := validateModuleList(label+".deny", *raw.Deny)
		if err != nil {
			return policy, err
		}
		policy.Deny = deny
		policy.DenySet = true
	}
	pins, err := validatePinMap(label+".pin", raw.Pin)
	if err != nil {
		return policy, err
	}
	policy.Pins = pins
	return policy, nil
}

func resolveTargetPolicy(label string, base Policy, raw rawTarget) (Policy, error) {
	effective := clonePolicy(base)
	if raw.QuarantineDays != nil {
		if *raw.QuarantineDays < 0 {
			return effective, fmt.Errorf("%s.quarantine_days must be non-negative", label)
		}
		effective.QuarantineDays = intPtr(*raw.QuarantineDays)
	}
	if raw.UpdateMode != nil {
		updateMode, err := validateUpdateMode(label+".update_mode", *raw.UpdateMode)
		if err != nil {
			return effective, err
		}
		effective.UpdateMode = updateMode
	}
	if raw.Allow != nil {
		allow, err := validateModuleList(label+".allow", *raw.Allow)
		if err != nil {
			return effective, err
		}
		effective.Allow = allow
		effective.AllowSet = true
	}
	if raw.Deny != nil {
		deny, err := validateModuleList(label+".deny", *raw.Deny)
		if err != nil {
			return effective, err
		}
		effective.Deny = deny
		effective.DenySet = true
	}
	pins, err := validatePinMap(label+".pin", raw.Pin)
	if err != nil {
		return effective, err
	}
	for module, version := range pins {
		effective.Pins[module] = version
	}
	return effective, nil
}

func normalizeTargetPath(input string) (string, error) {
	if input == "" {
		return "", errors.New("path is required")
	}
	if isAbsoluteTargetPath(input) {
		return "", errors.New("absolute paths are not allowed")
	}
	clean := filepath.Clean(input)
	if clean == "." {
		return ".", nil
	}
	cleanForChecks := filepath.ToSlash(clean)
	if cleanForChecks == ".." || strings.HasPrefix(cleanForChecks, "../") {
		return "", errors.New("path escapes the repository")
	}
	if runtime.GOOS == "windows" {
		return cleanForChecks, nil
	}
	return clean, nil
}

func isAbsoluteTargetPath(input string) bool {
	if filepath.IsAbs(input) {
		return true
	}
	if runtime.GOOS != "windows" {
		return false
	}
	slashed := strings.ReplaceAll(input, "\\", "/")
	if strings.HasPrefix(slashed, "/") || strings.HasPrefix(slashed, "//") {
		return true
	}
	return hasWindowsDrivePrefix(slashed)
}

func hasWindowsDrivePrefix(input string) bool {
	return len(input) >= 2 && input[1] == ':' &&
		((input[0] >= 'a' && input[0] <= 'z') || (input[0] >= 'A' && input[0] <= 'Z'))
}

func validateModuleList(label string, values []string) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			return nil, fmt.Errorf("%s contains an empty module path", label)
		}
		if _, ok := seen[value]; ok {
			return nil, fmt.Errorf("%s contains duplicate module path %q", label, value)
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}

func validatePinMap(label string, values map[string]string) (map[string]string, error) {
	out := map[string]string{}
	for module, version := range values {
		if module == "" {
			return nil, fmt.Errorf("%s contains an empty module path", label)
		}
		out[module] = version
	}
	return out, nil
}

func validateUpdateMode(label string, value string) (UpdateMode, error) {
	switch UpdateMode(value) {
	case UpdateModeNormal, UpdateModeVulnerabilityOnly:
		return UpdateMode(value), nil
	default:
		return "", fmt.Errorf("%s must be one of %q or %q", label, UpdateModeNormal, UpdateModeVulnerabilityOnly)
	}
}

func clonePolicy(in Policy) Policy {
	out := Policy{
		UpdateMode: in.UpdateMode,
		AllowSet:   in.AllowSet,
		DenySet:    in.DenySet,
		Pins:       map[string]string{},
	}
	if in.QuarantineDays != nil {
		out.QuarantineDays = intPtr(*in.QuarantineDays)
	}
	out.Allow = append([]string(nil), in.Allow...)
	out.Deny = append([]string(nil), in.Deny...)
	for module, version := range in.Pins {
		out.Pins[module] = version
	}
	return out
}

func intPtr(value int) *int {
	return &value
}
