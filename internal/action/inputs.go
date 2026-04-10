package action

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var branchLabelPattern = regexp.MustCompile(`[^a-z0-9-]+`)

func ConfigFromEnv() Config {
	return Config{
		ConfigPath:       actionInput("config"),
		Targets:          parseTargets(actionInput("targets")),
		CommitMessage:    firstNonEmpty(actionInput("commit-message"), DefaultCommitMessage),
		PullRequestTitle: firstNonEmpty(actionInput("pull-request-title"), DefaultPullRequestTitle),
		BaseBranch:       firstNonEmpty(actionInput("base-branch"), os.Getenv("GITHUB_BASE_REF"), os.Getenv("GITHUB_REF_NAME")),
		Repository:       os.Getenv("GITHUB_REPOSITORY"),
		GitHubToken:      firstNonEmpty(actionInput("github-token"), os.Getenv("GITHUB_TOKEN")),
		OutputPath:       os.Getenv("GITHUB_OUTPUT"),
	}
}

func actionInput(name string) string {
	upper := strings.ToUpper(name)
	return firstNonEmpty(
		os.Getenv("INPUT_"+upper),
		os.Getenv("INPUT_"+strings.ReplaceAll(upper, "-", "_")),
	)
}

func ManagedBranchName(configPath string, targets []string, baseBranch string) string {
	sortedTargets := slices.Clone(targets)
	slices.Sort(sortedTargets)

	label := "all"
	if len(sortedTargets) > 0 {
		label = strings.Join(sortedTargets, "-")
	}
	configLabel := strings.TrimSuffix(filepath.Base(configPath), filepath.Ext(configPath))
	if configLabel == "" || configLabel == "." {
		configLabel = "config"
	}
	joinedLabel := sanitizeBranchLabel(configLabel + "-" + label)
	if len(joinedLabel) > 40 {
		joinedLabel = joinedLabel[:40]
		joinedLabel = strings.Trim(joinedLabel, "-")
	}

	hashInput := strings.Join([]string{
		configPath,
		strings.Join(sortedTargets, ","),
		baseBranch,
	}, "\x00")
	sum := sha1.Sum([]byte(hashInput))
	return fmt.Sprintf("updtr/%s-%s", joinedLabel, hex.EncodeToString(sum[:])[:12])
}

func parseTargets(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	var targets []string
	seen := map[string]struct{}{}
	for _, part := range parts {
		target := strings.TrimSpace(part)
		if target == "" {
			continue
		}
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}
	return targets
}

func sanitizeBranchLabel(value string) string {
	value = strings.ToLower(value)
	value = branchLabelPattern.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "config"
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
