# Plan: Quarantine-Aware Version Selection

> Source PRD: [plans/quarantine-eligible-version-selection.md](/Users/mishankov/dev/updtr/plans/quarantine-eligible-version-selection.md)

## Architectural decisions

Durable decisions that apply across all phases:

- **Commands**: preserve the existing `detect` and `apply` flows. No new CLI flags, config keys, or user-facing reason labels are introduced.
- **Output model**: keep one primary decision per dependency. If an eligible fallback candidate exists, report that candidate. If none exists, report the most relevant blocked candidate and reason.
- **Key models**: dependencies continue to produce decisions with module path, current version, candidate version, release time, eligibility, blocked reason, and optional message. Applied updates continue to record the version selected during planning.
- **Selection boundary**: add a narrow candidate selection layer that coordinates candidate enumeration, exact-version release metadata lookup, and repeated policy evaluation.
- **Policy boundary**: keep shared policy evaluation candidate-scoped and ecosystem-independent. It evaluates one candidate at a time and returns the existing policy outcomes.
- **Go boundary**: the Go adapter owns Go module version enumeration, stable-versus-prerelease filtering, exact `module@version` release metadata lookup, and Go-native mutation commands.
- **Candidate-specific blockers**: when quarantine is enabled, `quarantined`, `missing_release_date`, and `untrusted_release_date` block only the candidate being evaluated and allow fallback to older candidates.
- **Dependency-level blockers**: pin mismatch, pinned dependency, denied dependency, not-allowed dependency, invalid current version, and version-list failure stop selection for that dependency.
- **Version ordering**: evaluate acceptable newer versions from newest to oldest using semantic-version ordering, preserving existing stable and prerelease track rules.
- **Quarantine time math**: preserve the injected clock and inclusive cutoff behavior. A candidate is eligible when its release time is at or before `now - quarantine_days * 24h`.
- **Metadata trust**: release dates remain exact to the candidate version. Do not infer release times from tags, GitHub releases, commit timestamps, or other repository heuristics.
- **Apply consistency**: `apply` uses the selected candidate from the plan it computes and does not perform a hidden second version selection.

---

## Phase 1: Detect Eligible Fallback Candidate

**User stories**: 1, 2, 4, 5, 8, 16, 17, 18

### What to build

Build the first vertical slice for quarantine-aware fallback in `detect`: for one direct Go dependency, enumerate all newer acceptable versions, evaluate candidates newest-to-oldest, skip a newest candidate that is quarantined, and report the first older candidate that satisfies the configured quarantine window. The slice should preserve one primary dependency decision and keep release dates visible for the selected eligible candidate.

### Acceptance criteria

- [ ] A dependency with current `v1.37.0`, newest newer `v1.48.1` released inside a 7-day quarantine window, and older newer `v1.48.0` released outside the window is reported as eligible for `v1.48.0`.
- [ ] Detection output shows the eligible fallback candidate as the dependency's candidate rather than reporting the newest quarantined version as the primary blocked result.
- [ ] Multiple newer quarantined candidates can be skipped before selecting the newest older eligible candidate.
- [ ] Release-date display remains present for eligible quarantine-related decisions.
- [ ] Candidate selection can be tested deterministically with fake version and metadata providers rather than live Go network calls.
- [ ] Existing quarantine boundary behavior remains inclusive and uses the injected clock.

---

## Phase 2: Metadata Fallback and No-Eligible Reporting

**User stories**: 6, 7, 8

### What to build

Extend candidate fallback so metadata-specific blockers are candidate-local when quarantine is enabled. Missing metadata, untrusted metadata, or future release times should prevent installing only the exact candidate with bad metadata, while allowing the selector to consider older candidates. If no newer candidate is installable, the dependency should still produce a blocked decision with a transparent candidate version, reason, and release date when available.

### Acceptance criteria

- [ ] A newest candidate with missing release-date metadata is skipped when an older newer candidate has trusted metadata and satisfies quarantine.
- [ ] A newest candidate with untrusted or future release-date metadata is skipped when an older newer candidate has trusted metadata and satisfies quarantine.
- [ ] If all newer candidates are quarantined or metadata-blocked, detection reports a blocked decision rather than omitting the dependency silently.
- [ ] The no-eligible-candidate blocked decision prefers a relevant newest blocked candidate so users can understand why the latest available update is not installable.
- [ ] Quarantined blocked decisions continue to include release dates when available.
- [ ] Missing and untrusted release-date reason labels remain unchanged.

---

## Phase 3: Dependency-Level Policy Stop Rules

**User stories**: 9, 10, 11, 12

### What to build

Separate candidate-local fallback outcomes from dependency-level stop outcomes. A dependency that is pinned, denied, not allowed, invalid, or operationally unresolved should not walk older candidates, because changing candidate version cannot make that dependency acceptable under the same policy or operational state.

### Acceptance criteria

- [ ] A pinned dependency remains blocked as pinned even when newer candidates exist that would otherwise satisfy quarantine.
- [ ] A pin mismatch remains blocked as a dependency-level policy drift result and does not fall back to older candidates.
- [ ] A denied dependency stops candidate selection immediately and reports the denied reason.
- [ ] A dependency outside an active allow list stops candidate selection immediately and reports the not-allowed reason.
- [ ] Invalid current versions remain target-level or dependency-level failures and are not converted into no-update outcomes.
- [ ] Version-list failures remain dependency-level candidate resolution failures and are not converted into candidate-specific fallback.
- [ ] Existing policy precedence remains unchanged for pinned, denied, allow-list, and quarantine outcomes.

---

## Phase 4: Version Track Preservation

**User stories**: 13, 14

### What to build

Lock down version-track behavior while adding fallback. Stable current versions should keep ignoring prereleases, and prerelease current versions should retain the existing prerelease behavior except that quarantine-aware fallback now applies among the candidates that are already acceptable for that track.

### Acceptance criteria

- [ ] A stable current version ignores newer prerelease versions while selecting among newer stable candidates.
- [ ] If a stable current version has a newest prerelease and an older newer stable version, fallback selection considers the stable candidate path and does not broaden to prereleases.
- [ ] A prerelease current version preserves the existing acceptable-candidate track while still evaluating quarantine and metadata blockers candidate by candidate.
- [ ] Semantic-version ordering is descending among acceptable candidates so the first eligible candidate is the newest installable version.
- [ ] Invalid semantic versions are skipped or failed consistently with the existing Go version-selection contract.

---

## Phase 5: Apply Uses Selected Candidate

**User stories**: 3, 15

### What to build

Carry the selected fallback candidate through mutation. `apply` should compute the same plan shape as `detect` from current repository state and pass the selected eligible candidate to the Go package manager. The applied summary should therefore match the version users would have seen during detection for the same state and clock.

### Acceptance criteria

- [ ] When detection would select an older eligible fallback candidate, apply mutates the dependency to that same candidate version.
- [ ] Apply does not install the absolute newest version when that version is blocked by quarantine, missing metadata, or untrusted metadata.
- [ ] Applied update reporting records the fallback candidate as the destination version.
- [ ] Blocked dependencies with no eligible candidates remain blocked in apply output and are not mutated.
- [ ] Apply-path tests prove the mutation command receives the selected candidate from planning.
- [ ] Preview and mutation behavior remain consistent without adding a saved-plan file or dependency-level selection flag.
