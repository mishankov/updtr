# Plan: Opt-In Indirect Go Dependency Updates

> Source PRD: [plans/0003. indirect dependency opt-in/prd.md](/Users/mishankov/dev/updtr/plans/0003.%20indirect%20dependency%20opt-in/prd.md)

## Architectural decisions

Durable decisions that apply across all phases:

- **Commands**: preserve the existing `detect`, `apply`, and `init` flows. The feature is controlled by target configuration, not by a new CLI flag or dependency-level selection mode.
- **Configuration**: add a target-level boolean opt-in named `include_indirect`, defaulting to false when omitted.
- **Generated configuration**: keep generated targets direct-only by omitting `include_indirect` from generated config output.
- **Dependency scope**: direct Go requirements remain the default planned dependency set. Opted-in targets additionally include only indirect `require` entries already listed in the target's `go.mod`.
- **Transitive boundary**: do not scan or render every module in the resolved Go build list. The Go toolchain remains responsible for final graph resolution during mutation and tidy.
- **Key models**: dependency planning carries whether each requirement is direct or indirect so renderers, summaries, policy decisions, and apply results do not need to re-parse Go-specific state.
- **Decision model**: keep one primary decision per dependency requirement. Decisions should include enough relationship data to label indirect items clearly while preserving the existing direct-dependency output when the opt-in is absent.
- **Policy boundary**: use the same candidate selection and policy controls for direct and indirect dependencies, including semantic-version ordering, prerelease filtering, quarantine fallback, release metadata requirements, pins, deny list, allow list, and replacement handling.
- **Replacement handling**: replaced indirect requirements are skipped or blocked consistently with replaced direct requirements. If unusual input presents a module as both direct and indirect after parsing, direct relationship is authoritative.
- **Output ordering**: keep deterministic module-path sorting and add direct or indirect labeling inline rather than introducing separate unordered sections.
- **Apply behavior**: continue using standard Go module commands to apply selected versions. Do not hand-edit indirect requirements.
- **Tidy behavior**: keep `go mod tidy` behavior unchanged after apply. Preserve existing direct-dependency drift warnings, but do not introduce a broad warning for every indirect graph adjustment caused by tidy.
- **Compatibility contract**: provide no special compatibility guarantee beyond normal Go module semantics and the target project's own validation.

---

## Phase 1: Minimal Opt-In Detect Slice

**User stories**: 1, 2, 3, 10, 12, 13, 14, 15, 16, 17, 18

### What to build

Build the first end-to-end detection path for target-scoped indirect dependency opt-in. A target without the new option should behave exactly as it does today. A target with `include_indirect = true` should plan update candidates for direct requirements plus indirect requirements explicitly listed in that target's `go.mod`, carrying the direct or indirect relationship into the dependency decision so output can distinguish the two.

### Acceptance criteria

- [ ] Existing configs that omit `include_indirect` plan only direct Go requirements.
- [ ] A target with `include_indirect = true` plans update candidates for explicitly listed indirect `require` entries in its `go.mod`.
- [ ] A target with `include_indirect = false` behaves the same as an omitted option.
- [ ] Unknown or misspelled target-scope config keys continue to be rejected by config validation.
- [ ] Generated configs remain direct-only by default and do not include `include_indirect`.
- [ ] Detection output clearly labels indirect candidates without changing the direct-dependency output contract when no indirect candidates are present.
- [ ] Detection output remains deterministically sorted by module path for mixed direct and indirect candidates.
- [ ] Tests prove direct-only behavior is preserved and opt-in behavior includes indirect requirements listed in `go.mod`.

---

## Phase 2: Candidate Selection Safety Parity

**User stories**: 5, 6, 11, 18

### What to build

Extend the opt-in detection path so indirect requirements use the same update-candidate selection behavior as direct requirements. An indirect candidate should follow the existing semantic-version ordering, prerelease filtering, trusted release metadata handling, and quarantine fallback rules, producing the same eligible or blocked decision shape as a direct dependency would under the same policy.

### Acceptance criteria

- [ ] An opted-in indirect dependency selects the newest eligible candidate according to the same version ordering used for direct dependencies.
- [ ] Stable indirect requirements continue to ignore newer prereleases under the same rules as stable direct requirements.
- [ ] Prerelease indirect requirements preserve the existing prerelease candidate behavior.
- [ ] Quarantine settings apply to indirect candidates and can fall back to older eligible versions when appropriate.
- [ ] Missing, untrusted, or future release metadata blocks indirect candidates consistently with direct candidates.
- [ ] If no eligible indirect candidate exists, detection reports a blocked decision with the relevant candidate version and reason rather than silently omitting the dependency.
- [ ] Selection tests use fake version and metadata providers where possible so behavior is deterministic and does not rely on live network calls.

---

## Phase 3: Policy and Replacement Parity

**User stories**: 7, 8, 9, 11, 18

### What to build

Complete policy parity for opted-in indirect requirements. Existing allow-list, deny-list, pin, and replacement controls should remain authoritative regardless of whether the planned dependency is direct or indirect. This slice should also lock down the relationship-precedence behavior for unusual input so a dependency that parses as direct is treated as direct.

### Acceptance criteria

- [ ] Allow-list rules include or block indirect dependencies under the same target policy as direct dependencies.
- [ ] Deny-list rules block indirect dependencies under the same precedence as direct dependencies.
- [ ] Pins apply to indirect dependencies, including pinned and pin-mismatch outcomes.
- [ ] Replaced indirect dependencies are blocked or skipped consistently with replaced direct dependencies.
- [ ] Relationship labeling survives blocked decisions so users can tell when a blocked item was indirect.
- [ ] Mixed direct and indirect blocked summaries remain deterministic across repeated runs.
- [ ] Tests cover allow-list, deny-list, pin, and replacement behavior for opted-in indirect dependencies.

---

## Phase 4: Opt-In Apply Mutation

**User stories**: 4, 5, 10, 13, 18, 20

### What to build

Carry opted-in indirect decisions through the apply path. `apply` should compute the same plan shape as `detect`, mutate only eligible decisions from that plan, and pass selected indirect dependency versions to the Go toolchain only when the target opted in. Applied output should label indirect updates clearly while disabled or omitted opt-in targets remain direct-only.

### Acceptance criteria

- [ ] Applying a target with `include_indirect = true` mutates an eligible indirect dependency to the selected candidate version.
- [ ] Applying a target without `include_indirect` does not mutate indirect requirements.
- [ ] Applying a target with `include_indirect = false` does not mutate indirect requirements.
- [ ] Blocked indirect decisions are not mutated.
- [ ] Applied summaries label indirect updates clearly enough for review.
- [ ] Apply uses the selected candidate from planning and does not re-select a different version during mutation.
- [ ] Apply-path tests prove preview and mutation behavior are consistent for indirect dependencies.

---

## Phase 5: Tidy, Drift, and Mixed-Target Regression

**User stories**: 2, 11, 13, 17, 19, 20

### What to build

Harden the feature across realistic repository behavior: mixed targets with different opt-in values, deterministic output across repeated runs, generated-config regression, and Go tidy side effects after indirect updates. The direct-dependency drift warning should keep its existing meaning, while expected indirect graph rewrites caused by `go mod tidy` should not produce broad new warnings.

### Acceptance criteria

- [ ] In a multi-target config, `include_indirect = true` affects only the target where it is set.
- [ ] Existing direct-only plans and summaries remain unchanged when the new option is absent.
- [ ] Mixed direct and indirect output remains stable across repeated `detect` runs.
- [ ] Generated config tests prove new users still start with direct-only targets.
- [ ] Applying indirect updates still runs normal tidy behavior after successful mutations.
- [ ] Unexpected direct dependency drift after apply continues to produce the existing warning.
- [ ] Expected indirect changes from tidy do not produce a broad additional warning.
- [ ] A small disposable Go module fixture validates normal Go toolchain behavior around indirect requirements and tidy.
