# Review Findings

## Finding 1: [P2] Version-specific replacements block unrelated versions

**File:** `internal/goecosystem/adapter.go:132-133`

This records every replace directive by module path only, ignoring `replace.Old.Version`. A directive like `replace example.com/lib v1.0.0 => ../lib` only affects `v1.0.0`, but a target requiring `v1.1.0` will still be reported as `replaced_dependency` and skipped, causing valid updates to be hidden.

**Recommended fix:** Track replaced dependencies by path plus version, and only block all versions when the replace has no old version.

## Finding 2: [P3] Pin mismatch hides the configured pin when a candidate exists

**File:** `internal/render/render.go:85-90`

For a pin mismatch with an available newer candidate, the decision contains both `CandidateVersion` and `PinVersion`, but `versionSuffix` prefers `CandidateVersion`. The output becomes `current -> candidate: pin_mismatch` instead of showing the expected pin, making the policy drift harder to diagnose.

**Recommended fix:** Prefer `PinVersion` for `ReasonPinMismatch` or check `PinVersion` before `CandidateVersion` for that reason.

## Finding 3: [P3] Quarantined updates omit release date

**File:** `internal/render/render.go:41-45`

Blocked quarantine decisions are rendered without the release timestamp even though the decision carries `ReleaseTime`. That makes `quarantined` output less actionable and misses the stated transparency requirement to show release age/date for quarantine decisions.

**Recommended fix:** Include `releaseSuffix(decision)` for quarantined blocked rows, or otherwise render the candidate release date/age for that reason.
