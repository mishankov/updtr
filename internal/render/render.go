package render

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/mishankov/updtr/internal/core"
)

type Status string

const (
	StatusEligible Status = "eligible"
	StatusBlocked  Status = "blocked"
	StatusApplied  Status = "applied"
	StatusWarning  Status = "warning"
	StatusError    Status = "error"
	StatusNoop     Status = "noop"
)

type Options struct {
	Color bool
}

type Row struct {
	Target          string
	Module          string
	FromVersion     string
	ToVersion       string
	Status          Status
	Reason          string
	Vulnerabilities string
}

func Detect(out io.Writer, result core.RunResult) {
	Report(out, result, Options{})
}

func Apply(out io.Writer, result core.RunResult) {
	Report(out, result, Options{})
}

func Report(out io.Writer, result core.RunResult, options Options) {
	rows := Normalize(result)
	if len(rows) == 0 {
		return
	}
	renderTable(out, rows, options)
}

func Normalize(result core.RunResult) []Row {
	var rows []Row
	for _, target := range result.Targets {
		targetRows := normalizeTarget(result.Mode, target)
		if len(targetRows) == 0 {
			targetRows = append(targetRows, Row{
				Target: targetLabel(target.Target.Name, target.Target.NormalizedPath),
				Status: StatusNoop,
				Reason: "no dependency changes detected",
			})
		}
		rows = append(rows, targetRows...)
	}
	return rows
}

func normalizeTarget(mode string, target core.TargetResult) []Row {
	var rows []Row
	targetName := targetLabel(target.Target.Name, target.Target.NormalizedPath)
	appliedByKey := map[string]core.AppliedUpdate{}
	for _, update := range target.Applied {
		appliedByKey[decisionKey(update.ModulePath, update.ToVersion, update.Relationship)] = update
	}

	for _, decision := range target.Plan.Decisions {
		row := Row{
			Target:          targetName,
			Module:          moduleWithRelationship(decision.ModulePath, decision.Relationship),
			FromVersion:     decision.CurrentVersion,
			ToVersion:       decision.CandidateVersion,
			Vulnerabilities: vulnerabilityText(decision.Vulnerabilities),
		}
		switch {
		case decision.Blocked():
			row.Status = StatusBlocked
			if decision.BlockedReason == core.ReasonPinMismatch && decision.PinVersion != "" {
				row.ToVersion = decision.PinVersion
			}
			row.Reason = blockedReasonText(decision)
		case decision.Eligible:
			if update, ok := appliedByKey[decisionKey(decision.ModulePath, decision.CandidateVersion, decision.Relationship)]; ok {
				row.Status = StatusApplied
				row.FromVersion = update.FromVersion
				row.ToVersion = update.ToVersion
				row.Vulnerabilities = vulnerabilityText(update.Vulnerabilities)
				delete(appliedByKey, decisionKey(decision.ModulePath, decision.CandidateVersion, decision.Relationship))
			} else {
				row.Status = StatusEligible
				row.Reason = eligibleReasonText(decision, mode)
			}
		default:
			continue
		}
		rows = append(rows, row)
	}

	if len(appliedByKey) > 0 {
		keys := make([]string, 0, len(appliedByKey))
		for key := range appliedByKey {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			update := appliedByKey[key]
			rows = append(rows, Row{
				Target:          targetName,
				Module:          moduleWithRelationship(update.ModulePath, update.Relationship),
				FromVersion:     update.FromVersion,
				ToVersion:       update.ToVersion,
				Status:          StatusApplied,
				Vulnerabilities: vulnerabilityText(update.Vulnerabilities),
			})
		}
	}

	for _, warning := range target.Warnings {
		rows = append(rows, Row{
			Target: targetName,
			Status: StatusWarning,
			Reason: warning,
		})
	}

	if err := target.EffectiveError(); err != "" {
		rows = append(rows, Row{
			Target: targetName,
			Status: StatusError,
			Reason: err,
		})
	}

	return rows
}

func renderTable(out io.Writer, rows []Row, options Options) {
	headers := []string{"TARGET", "MODULE", "FROM", "TO", "STATUS", "REASON", "VULNERABILITIES"}
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}
	for _, row := range rows {
		cells := rowCells(row)
		for i, cell := range cells {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	border := tableBorder(widths)
	_, _ = fmt.Fprintln(out, border)
	_, _ = fmt.Fprintln(out, tableLine(headers, widths, options, true))
	_, _ = fmt.Fprintln(out, border)
	for _, row := range rows {
		_, _ = fmt.Fprintln(out, tableLine(rowCells(row), widths, options, false))
	}
	_, _ = fmt.Fprintln(out, border)
}

func rowCells(row Row) []string {
	return []string{
		row.Target,
		row.Module,
		row.FromVersion,
		row.ToVersion,
		statusLabel(row.Status),
		row.Reason,
		row.Vulnerabilities,
	}
}

func tableBorder(widths []int) string {
	var parts []string
	for _, width := range widths {
		parts = append(parts, strings.Repeat("-", width+2))
	}
	return "+" + strings.Join(parts, "+") + "+"
}

func tableLine(cells []string, widths []int, options Options, header bool) string {
	rendered := make([]string, len(cells))
	for i, cell := range cells {
		padded := padRight(cell, widths[i])
		if !header && i == 4 {
			padded = colorizeStatus(padded, Status(strings.ToLower(cell)), options.Color)
		}
		rendered[i] = " " + padded + " "
	}
	return "|" + strings.Join(rendered, "|") + "|"
}

func padRight(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
}

func colorizeStatus(value string, status Status, enabled bool) string {
	if !enabled {
		return value
	}
	switch status {
	case StatusEligible:
		return "\x1b[36m" + value + "\x1b[0m"
	case StatusBlocked:
		return "\x1b[33m" + value + "\x1b[0m"
	case StatusApplied:
		return "\x1b[32m" + value + "\x1b[0m"
	case StatusWarning:
		return "\x1b[35m" + value + "\x1b[0m"
	case StatusError:
		return "\x1b[31m" + value + "\x1b[0m"
	default:
		return value
	}
}

func statusLabel(status Status) string {
	return strings.ToUpper(string(status))
}

func targetLabel(name string, path string) string {
	return fmt.Sprintf("%s (%s)", name, path)
}

func decisionKey(modulePath string, candidateVersion string, relationship core.DependencyRelationship) string {
	return string(relationship) + "\x00" + modulePath + "\x00" + candidateVersion
}

func eligibleReasonText(decision core.Decision, mode string) string {
	reason := ""
	if mode == "detect" {
		reason = "update available"
	}
	if decision.ReleaseTime != nil {
		if reason != "" {
			reason += "; "
		}
		reason += "released " + decision.ReleaseTime.UTC().Format(time.DateOnly)
	}
	if decision.Message != "" {
		if reason != "" {
			reason += "; "
		}
		reason += decision.Message
	}
	return reason
}

func blockedReasonText(decision core.Decision) string {
	reason := string(decision.BlockedReason)
	if decision.BlockedReason == core.ReasonPinMismatch && decision.PinVersion != "" {
		reason += " (pin " + decision.PinVersion + ")"
	}
	if decision.BlockedReason == core.ReasonQuarantined && decision.ReleaseTime != nil {
		reason += " released " + decision.ReleaseTime.UTC().Format(time.DateOnly)
	}
	if decision.Message != "" {
		reason += " (" + decision.Message + ")"
	}
	return reason
}

func moduleWithRelationship(modulePath string, relationship core.DependencyRelationship) string {
	if relationship == core.RelationshipIndirect {
		return modulePath + " (indirect)"
	}
	return modulePath
}

func vulnerabilityText(vulnerabilities []core.Vulnerability) string {
	if len(vulnerabilities) == 0 {
		return ""
	}
	advisories := advisoryIDs(vulnerabilities)
	if len(advisories) == 0 {
		return "known"
	}
	return strings.Join(advisories, ", ")
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
