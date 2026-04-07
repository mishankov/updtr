package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

var targetNamePattern = regexp.MustCompile(`^[a-z0-9-]+$`)

type rawConfig struct {
	Policy  *rawPolicy  `toml:"policy"`
	Targets []rawTarget `toml:"targets"`
}

type rawPolicy struct {
	QuarantineDays *int              `toml:"quarantine_days"`
	Allow          *[]string         `toml:"allow"`
	Deny           *[]string         `toml:"deny"`
	Pin            map[string]string `toml:"pin"`
}

type rawTarget struct {
	Name            string            `toml:"name"`
	Ecosystem       string            `toml:"ecosystem"`
	Path            string            `toml:"path"`
	IncludeIndirect bool              `toml:"include_indirect"`
	QuarantineDays  *int              `toml:"quarantine_days"`
	Allow           *[]string         `toml:"allow"`
	Deny            *[]string         `toml:"deny"`
	Pin             map[string]string `toml:"pin"`
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
	Allow          []string
	AllowSet       bool
	Deny           []string
	DenySet        bool
	Pins           map[string]string
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = "updtr.toml"
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("load config %s: %w", path, err)
	}
	defer file.Close()

	var raw rawConfig
	decoder := toml.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	absConfig, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve config path %s: %w", path, err)
	}
	cfg := &Config{Path: absConfig, BaseDir: filepath.Dir(absConfig)}
	if err := validateAndResolve(&raw, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func validateAndResolve(raw *rawConfig, cfg *Config) error {
	if len(raw.Targets) == 0 {
		return errors.New("config must define at least one target")
	}

	basePolicy := Policy{Pins: map[string]string{}}
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
	policy := Policy{Pins: map[string]string{}}
	if raw.QuarantineDays != nil {
		if *raw.QuarantineDays < 0 {
			return policy, fmt.Errorf("%s.quarantine_days must be non-negative", label)
		}
		policy.QuarantineDays = intPtr(*raw.QuarantineDays)
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
	if filepath.IsAbs(input) {
		return "", errors.New("absolute paths are not allowed")
	}
	clean := filepath.Clean(input)
	if clean == "." {
		return ".", nil
	}
	slashed := filepath.ToSlash(clean)
	if slashed == ".." || strings.HasPrefix(slashed, "../") {
		return "", errors.New("path escapes the repository")
	}
	return slashed, nil
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

func clonePolicy(in Policy) Policy {
	out := Policy{
		AllowSet: in.AllowSet,
		DenySet:  in.DenySet,
		Pins:     map[string]string{},
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
