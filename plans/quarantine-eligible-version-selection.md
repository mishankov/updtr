# Quarantine-Aware Version Selection PRD

## Problem Statement

Users expect `updtr detect` and `updtr apply` to choose the newest dependency version that is actually installable under the configured policy. Today, the Go version detection flow chooses the newest newer stable module version first, then evaluates quarantine against that single version. If that newest version is blocked by quarantine, the dependency is reported as blocked even when an older newer version already satisfies the quarantine window.

Example: on April 6, 2026, a project using `modernc.org/sqlite v1.37.0` with `quarantine_days = 7` should not install `v1.48.1` if it was released on April 3, 2026. But if `v1.48.0` was released on March 27, 2026, `v1.48.0` should be available for detection and application because it is older than the quarantine cutoff. On April 7, 2026, the same conclusion still holds.

This is a policy correctness issue: quarantine should delay only the versions that are too new, not suppress every update for that dependency until the absolute latest version ages out.

## Solution

Change version detection to select the newest installable candidate, not merely the newest available candidate. For each direct dependency, the Go adapter should enumerate acceptable newer versions in descending semantic-version order, fetch trusted release metadata for each candidate as needed, and evaluate candidate-specific policy outcomes until it finds the first candidate that can be installed.

Candidate-specific quarantine outcomes should cause the selector to continue to the next older candidate. These outcomes include `quarantined`, `missing_release_date`, and `untrusted_release_date` when quarantine is enabled. Module-level policy outcomes should still stop selection for the dependency, because older versions cannot make that dependency installable under the same policy. These outcomes include pin mismatch, pinned dependency, denied dependency, and not-allowed dependency.

The output should continue to show one primary decision per dependency. If a newer version is quarantined but an older version is eligible, the eligible older version should be shown as the dependency's candidate. If no newer version is eligible, the output should show the most relevant blocked candidate and reason, preserving transparent quarantine and release-date reporting.

## User Stories

1. As a developer, I want `detect` to show the newest version that can actually be installed, so that I can trust the proposed update plan.
2. As a developer, I want quarantine to block only versions inside the waiting window, so that older safe-enough releases are not hidden.
3. As a maintainer, I want `apply` to install the same candidate shown by `detect`, so that preview and mutation remain consistent.
4. As a maintainer using `quarantine_days = 7`, I want a version released 11 days ago to be eligible even if a version released 4 days ago is not, so that automation can keep moving without violating policy.
5. As a maintainer, I want the newest available but quarantined version to stop blocking an older eligible version, so that release cadence spikes do not freeze dependency updates.
6. As a security-conscious maintainer, I want untrusted or missing release metadata to block only the exact candidate whose metadata cannot be trusted, so that a different older candidate with trusted metadata can still be considered.
7. As a developer, I want a dependency with no eligible candidates to continue showing a blocked reason, so that I can understand why no update is available.
8. As a developer, I want release dates to remain visible for quarantine-related decisions, so that I can audit the cutoff behavior.
9. As a maintainer, I want pinned dependencies to stay pinned regardless of candidate fallback, so that module-level policy remains authoritative.
10. As a maintainer, I want deny-list rules to stop candidate selection immediately, so that denied dependencies are never considered for updates.
11. As a maintainer, I want allow-list rules to stop candidate selection immediately when a dependency is not allowed, so that the allow list remains a dependency-level gate.
12. As a maintainer, I want invalid current versions and version-list failures to remain target-level or dependency-level errors, so that operational failures are not silently converted into no-op updates.
13. As a developer, I want stable current versions to continue ignoring pre-releases, so that this change does not broaden the version track unexpectedly.
14. As a developer using a pre-release current version, I want the existing pre-release behavior to remain unchanged except for quarantine-aware fallback, so that version-track semantics do not regress.
15. As a developer, I want the candidate selected by detection to be the version passed to the package manager during application, so that no hidden second resolution happens.
16. As a future maintainer, I want candidate selection logic to be testable without invoking the Go command, so that quarantine fallback edge cases are easy to cover.
17. As a future maintainer, I want Go-specific version enumeration separated from shared policy evaluation, so that future ecosystems can reuse the same selection concept.
18. As a CI user, I want deterministic behavior around the quarantine boundary, so that runs near midnight or exact cutoff times do not produce surprising results.

## Implementation Decisions

- Modify the Go candidate resolution flow to enumerate all newer acceptable versions rather than returning a single newest candidate before policy evaluation.
- Keep semantic-version filtering behavior unchanged: stable current versions should consider newer stable versions and ignore pre-releases.
- Evaluate candidates from newest to oldest so the first eligible candidate is the newest installable version.
- Treat quarantine, missing release date, and untrusted release date as candidate-specific blockers that allow fallback to older candidates.
- Treat pin mismatch, pinned, denied, not allowed, and dependency-wide candidate resolution failure as dependency-level blockers that stop candidate fallback.
- Preserve one primary decision per dependency in the core plan and renderer to avoid expanding the output model in this change.
- If at least one candidate is eligible, render and apply the newest eligible candidate.
- If no candidate is eligible, report the most relevant blocked candidate. Prefer the newest candidate's quarantine or metadata decision, because it explains why the latest available update is not installable.
- Keep the policy engine responsible for evaluating one candidate at a time. Add a narrow selection layer that coordinates candidate enumeration, release metadata lookup, and repeated policy evaluation.
- Keep release metadata lookup exact to `module@version`; do not infer release times from repository tags, GitHub releases, or commit timestamps.
- Preserve the existing injected clock behavior for deterministic quarantine tests.
- Avoid changing configuration syntax, reason labels, or CLI flags for this feature.

## Testing Decisions

- Good tests should assert externally visible behavior: selected candidate version, blocked reason, release-date handling, and apply target version. They should not assert incidental helper function call order unless that order is part of the public behavior.
- Add table-driven tests for candidate selection with a newer quarantined candidate and an older eligible candidate.
- Add table-driven tests for multiple quarantined candidates before the first eligible candidate.
- Add table-driven tests for missing release-date metadata on the newest candidate with an older trusted eligible candidate.
- Add table-driven tests for untrusted or future release dates on the newest candidate with an older trusted eligible candidate.
- Add tests proving deny-list, allow-list, and pin rules stop fallback and still report the appropriate dependency-level blocked reason.
- Add tests proving stable current versions still ignore pre-releases while selecting among stable candidates.
- Add tests proving no eligible candidates still produce a blocked quarantine or metadata decision with the candidate version and release date when available.
- Add integration-style adapter tests using fake version and metadata providers rather than live network calls.
- Add apply-path tests proving the mutation command receives the same candidate selected during planning.
- Use existing policy quarantine boundary tests as prior art for exact cutoff behavior.
- Use existing renderer tests as prior art for release-date display and blocked decision formatting.

## Out of Scope

- New configuration options for choosing whether to show all skipped candidates.
- Rendering multiple decisions for a single dependency.
- Changelog, release note, or vulnerability-aware version selection.
- Broadening Go version-track rules beyond the existing stable-versus-pre-release behavior.
- Supporting downgrade or sidegrade behavior.
- Changing quarantine duration semantics or the inclusive cutoff rule.
- Adding non-Go ecosystem support.
- Fetching release metadata from untrusted repository heuristics.

## Further Notes

- `pkg.go.dev` shows `modernc.org/sqlite v1.48.1` published on April 3, 2026. That validates the example's latest-version quarantine case under a 7-day quarantine on April 6 or April 7, 2026.
- The user-reported `v1.48.0` release date of March 27, 2026 is the motivating eligible fallback case. The implementation should not hard-code this module; it should use the same exact-version metadata path for every Go dependency.
- This change should make `detect` and `apply` more useful without weakening quarantine, because only candidates whose own release metadata satisfies policy become eligible.
