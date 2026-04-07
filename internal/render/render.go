package render

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/mishankov/updtr/internal/core"
)

func Detect(out io.Writer, result core.RunResult) {
	render(out, result, false)
}

func Apply(out io.Writer, result core.RunResult) {
	render(out, result, true)
}

func render(out io.Writer, result core.RunResult, apply bool) {
	totalEligible := 0
	totalBlocked := 0
	totalApplied := 0
	totalErrors := 0

	for _, target := range result.Targets {
		_, _ = fmt.Fprintf(out, "Target %s (%s)\n", target.Target.Name, target.Target.NormalizedPath)
		if len(target.Applied) > 0 {
			_, _ = fmt.Fprintln(out, "Applied:")
			for _, update := range target.Applied {
				_, _ = fmt.Fprintf(out, "  - %s %s -> %s%s\n", moduleWithRelationship(update.ModulePath, update.Relationship), update.FromVersion, update.ToVersion, vulnerabilitySuffix(update.Vulnerabilities))
			}
		}

		eligible, blocked := splitDecisions(target.Plan.Decisions)
		if !apply && len(eligible) > 0 {
			_, _ = fmt.Fprintln(out, "Eligible:")
			for _, decision := range eligible {
				_, _ = fmt.Fprintf(out, "  - %s %s -> %s%s%s\n", moduleWithRelationship(decision.ModulePath, decision.Relationship), decision.CurrentVersion, decision.CandidateVersion, releaseSuffix(decision), vulnerabilitySuffix(decision.Vulnerabilities))
			}
		}
		if len(blocked) > 0 {
			_, _ = fmt.Fprintln(out, "Blocked:")
			for _, decision := range blocked {
				_, _ = fmt.Fprintf(out, "  - %s%s: %s%s\n", moduleWithRelationship(decision.ModulePath, decision.Relationship), versionSuffix(decision), decision.BlockedReason, blockedSuffix(decision))
			}
		}
		if len(target.Warnings) > 0 {
			_, _ = fmt.Fprintln(out, "Warnings:")
			for _, warning := range target.Warnings {
				_, _ = fmt.Fprintf(out, "  - %s\n", warning)
			}
		}
		if err := target.EffectiveError(); err != "" {
			_, _ = fmt.Fprintln(out, "Errors:")
			_, _ = fmt.Fprintf(out, "  - %s\n", err)
			totalErrors++
		}

		errorCount := 0
		if target.EffectiveError() != "" {
			errorCount = 1
		}
		_, _ = fmt.Fprintf(out, "Summary: eligible=%d blocked=%d applied=%d errors=%d\n\n", len(eligible), len(blocked), len(target.Applied), errorCount)
		totalEligible += len(eligible)
		totalBlocked += len(blocked)
		totalApplied += len(target.Applied)
	}
	_, _ = fmt.Fprintf(out, "Total: targets=%d eligible=%d blocked=%d applied=%d errors=%d\n", len(result.Targets), totalEligible, totalBlocked, totalApplied, totalErrors)
}

func splitDecisions(decisions []core.Decision) ([]core.Decision, []core.Decision) {
	var eligible []core.Decision
	var blocked []core.Decision
	for _, decision := range decisions {
		if decision.Eligible {
			eligible = append(eligible, decision)
		}
		if decision.Blocked() {
			blocked = append(blocked, decision)
		}
	}
	return eligible, blocked
}

func versionSuffix(decision core.Decision) string {
	if decision.BlockedReason == core.ReasonPinMismatch && decision.PinVersion != "" {
		return fmt.Sprintf(" %s (pin %s)", decision.CurrentVersion, decision.PinVersion)
	}
	switch {
	case decision.CandidateVersion != "":
		return fmt.Sprintf(" %s -> %s", decision.CurrentVersion, decision.CandidateVersion)
	case decision.PinVersion != "":
		return fmt.Sprintf(" %s (pin %s)", decision.CurrentVersion, decision.PinVersion)
	case decision.CurrentVersion != "":
		return " " + decision.CurrentVersion
	default:
		return ""
	}
}

func releaseSuffix(decision core.Decision) string {
	if decision.ReleaseTime == nil {
		return ""
	}
	return " released " + decision.ReleaseTime.UTC().Format(time.DateOnly)
}

func blockedSuffix(decision core.Decision) string {
	suffix := ""
	if decision.BlockedReason == core.ReasonQuarantined {
		suffix += releaseSuffix(decision)
	}
	suffix += messageSuffix(decision)
	suffix += vulnerabilitySuffix(decision.Vulnerabilities)
	return suffix
}

func messageSuffix(decision core.Decision) string {
	if decision.Message == "" {
		return ""
	}
	return " (" + decision.Message + ")"
}

func moduleWithRelationship(modulePath string, relationship core.DependencyRelationship) string {
	if relationship == core.RelationshipIndirect {
		return modulePath + " (indirect)"
	}
	return modulePath
}

func vulnerabilitySuffix(vulnerabilities []core.Vulnerability) string {
	if len(vulnerabilities) == 0 {
		return ""
	}
	advisories := advisoryIDs(vulnerabilities)
	if len(advisories) == 0 {
		return " (vulnerabilities: known)"
	}
	return " (vulnerabilities: " + strings.Join(advisories, ", ") + ")"
}

func advisoryIDs(vulnerabilities []core.Vulnerability) []string {
	seen := map[string]struct{}{}
	var advisories []string
	for _, vulnerability := range vulnerabilities {
		for _, advisory := range vulnerability.AdvisoryIDs {
			if advisory == "" {
				continue
			}
			if _, ok := seen[advisory]; ok {
				continue
			}
			seen[advisory] = struct{}{}
			advisories = append(advisories, advisory)
		}
	}
	sort.Strings(advisories)
	return advisories
}
