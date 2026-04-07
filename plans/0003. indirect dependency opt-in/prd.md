# Opt-In Indirect Go Dependency Updates PRD

## Problem Statement

Users currently can detect and apply updates for direct Go module dependencies, but indirect dependencies listed in `go.mod` are intentionally ignored as first-class planned items. That default is safe and low-noise, but it leaves maintainers without a controlled way to update indirect modules when they deliberately want to keep the module graph fresher, resolve a transitive bug, address a known advisory through an indirect module bump, or reduce dependency drift in CI.

In Go, updating an indirect dependency can be valid because the main module can require a higher version than the minimum requested by a direct dependency, and the selected module graph will use that higher version. However, indirect dependencies are usually implementation details of direct dependencies, so updating them by default would create unnecessary churn and may expose compatibility assumptions in upstream packages. The tool needs an explicit opt-in path that makes indirect updates possible without weakening the current default behavior.

## Solution

Add opt-in support for treating indirect Go module requirements as planned update candidates. Direct dependencies remain the default scope. A target can explicitly include indirect requirements already present in its `go.mod`, causing `detect` and `apply` to evaluate those requirements through the same candidate selection, release metadata, quarantine, allow-list, deny-list, pin, replacement, rendering, and mutation flows used for direct dependencies.

The opt-in should be target-scoped because indirect dependency tolerance can vary by module. A repository may want indirect updates in an internal tool module while keeping application modules direct-only. The config should make the behavior obvious in code review. A boolean such as `include_indirect = true` is acceptable for the initial feature because the product decision is binary: include indirect requirements listed in `go.mod` or keep excluding them.

Indirect support should not expand into scanning every transitive module in the resolved build list. The first version of this feature should only consider modules that are explicitly present as indirect `require` entries in the target's `go.mod`. The Go toolchain remains responsible for final graph resolution and `go mod tidy` may add, remove, or rewrite indirect requirements after updates.

## User Stories

1. As a maintainer, I want indirect dependency updates to remain disabled by default, so that existing `updtr` behavior stays low-noise and predictable.
2. As a maintainer, I want to opt in per Go target to indirect dependency updates, so that I can apply different risk tolerance to different modules in the same repository.
3. As a developer, I want `detect` to show indirect dependency update candidates only when the target opts in, so that the preview output matches the configured scope.
4. As a developer, I want `apply` to update indirect dependencies only when the target opts in, so that mutation never broadens silently beyond the reviewed config.
5. As a security-conscious maintainer, I want to update an indirect dependency that is known to need a newer version, so that I can remediate transitive risk without waiting for every direct dependency to update its minimum requirement.
6. As a maintainer, I want indirect dependency candidates to use the same quarantine policy as direct dependency candidates, so that opt-in does not bypass release-age safety rules.
7. As a maintainer, I want allow-list and deny-list rules to apply to indirect dependencies too, so that existing policy controls remain authoritative after opt-in.
8. As a maintainer, I want pins to apply to indirect dependencies too, so that I can intentionally hold an indirect module at a specific version.
9. As a maintainer, I want replaced indirect dependencies to be skipped or blocked consistently with replaced direct dependencies, so that local replacements are not overwritten by automation.
10. As a developer, I want output to make it clear when an update candidate is indirect, so that review can distinguish direct API dependencies from transitive graph-maintenance changes.
11. As a developer, I want direct and indirect update summaries to remain deterministic, so that repeated CI runs are stable and reviewable.
12. As a developer, I want the tool to avoid scanning the entire resolved transitive graph, so that opt-in indirect support does not explode output size or runtime.
13. As a CI engineer, I want existing configs to keep behaving as direct-only, so that enabling this feature does not create unexpected pull request churn.
14. As a maintainer, I want target-level config validation to reject ambiguous dependency-scope values, so that typos do not silently broaden or narrow update scope.
15. As a developer, I want generated configs to stay direct-only by default, so that new users start from the safer behavior.
16. As a future maintainer, I want the dependency model to carry whether a dependency is direct or indirect, so that rendering and policy decisions can evolve without re-parsing Go-specific state.
17. As a future maintainer, I want tests that prove direct-only behavior is preserved, so that indirect support does not regress the original v1 contract.
18. As a future maintainer, I want tests that prove indirect opt-in works for detect and apply, so that preview and mutation stay consistent.
19. As a maintainer, I want `go mod tidy` side effects to remain visible as warnings when they cause unexpected direct dependency drift, so that indirect updates do not hide graph rewrites.
20. As a developer, I want no special compatibility guarantee beyond normal Go module semantics, so that the feature is honest about indirect-update risk and expects project tests to validate the result.

## Implementation Decisions

- Direct dependencies remain the default dependency scope for every Go target.
- Add an explicit target-level opt-in for indirect requirements, with a simple boolean configuration option such as `include_indirect = true`.
- The opt-in includes only indirect `require` entries already listed in the target's `go.mod`.
- Do not scan or render every module from the resolved transitive build list as part of this feature.
- Preserve the current behavior for generated configs: omit the indirect opt-in option and therefore generate direct-only targets.
- Extend the internal dependency model so a planned dependency carries its relationship to the main module: direct or indirect.
- Keep one primary decision per dependency requirement. The decision should include enough dependency-kind information for renderers and apply summaries to label indirect items clearly.
- Apply the same candidate selection rules to direct and indirect dependencies, including semantic-version ordering, prerelease filtering, quarantine fallback, and trusted release metadata requirements.
- Apply existing policy controls to both direct and indirect dependencies: pins, deny list, allow list, quarantine, and replacement handling.
- Continue to use standard Go module commands for applying selected versions. The tool should not hand-edit indirect requirements.
- Keep `go mod tidy` behavior unchanged after apply. It may remove indirect requirements that are no longer needed or adjust graph state according to normal Go rules.
- Preserve existing direct-dependency drift warnings. Do not add a broad warning for every indirect change caused by `go mod tidy`, because indirect graph adjustments are expected when this feature is enabled.
- Sort output deterministically. A practical default is to keep module-path sorting and include direct or indirect labeling in the rendered line rather than creating separate unordered sections.
- If a module appears as both direct and indirect due to malformed or unusual input, treat the direct relationship as authoritative after Go module parsing normalizes the file.
- Keep the configuration surface minimal for the first version. Do not add per-dependency indirect-only rules, separate quarantine periods for indirect dependencies, or a separate apply mode.

## Testing Decisions

- Good tests should assert externally visible behavior: which dependencies are planned, whether indirect items are labeled, selected candidate versions, blocked reasons, rendered output, and apply mutations. They should not assert incidental helper call order unless order is part of deterministic output.
- Add adapter tests proving direct-only behavior skips indirect requirements by default.
- Add adapter tests proving opt-in includes indirect requirements already listed in `go.mod`.
- Add tests proving opt-in does not include modules that appear only in the full resolved transitive graph and are not listed in `go.mod`.
- Add config tests for target-level `include_indirect = true`, default false behavior, and unknown-field rejection for misspelled config keys.
- Add planning tests proving direct and indirect dependencies both use the same candidate fallback and quarantine behavior.
- Add policy tests or planning tests proving allow list, deny list, and pins apply to indirect dependencies after opt-in.
- Add replacement tests proving replaced indirect requirements are blocked consistently with replaced direct requirements.
- Add renderer tests proving indirect decisions are visibly labeled without breaking existing direct-dependency output.
- Add apply-path tests proving an eligible indirect dependency is passed to the Go mutation command when opt-in is enabled.
- Add apply-path tests proving indirect dependencies are not mutated when opt-in is disabled.
- Add regression tests proving existing direct-only plans and summaries remain unchanged when the new config option is absent.
- Use fake version and metadata providers for most selection tests so they stay deterministic and do not rely on live network calls.
- Use disposable Go module fixtures for a smaller number of integration-style tests that validate normal Go toolchain behavior around indirect requirements and `go mod tidy`.

## Out of Scope

- Enabling indirect dependency updates by default.
- Scanning every module in the resolved transitive build list.
- Vulnerability scanning or advisory interpretation.
- Separate quarantine settings for direct versus indirect dependencies.
- Separate allow, deny, or pin syntax for indirect dependencies.
- Dependency-level CLI selection.
- Saved plan files.
- Automatic test execution after applying updates.
- Compatibility guarantees beyond normal Go module semantics and the target project's own test suite.
- Non-Go ecosystem dependency-scope design.
- Automatic pull request creation, branch management, or git commits.

## Further Notes

- Updating an indirect dependency can affect direct dependencies because the main module can select a higher version of a transitive module than a direct dependency's minimum requirement. This is normal Go behavior, but it still carries compatibility risk when upstream modules rely on undocumented behavior or when the indirect module does not preserve compatibility well.
- The opt-in design intentionally makes indirect updates a conscious repository policy decision rather than a surprise side effect of upgrading `updtr`.
- If this feature later needs more nuance, a future migration could replace the boolean with an enum-style dependency scope. The first version should avoid that complexity unless implementation reveals an immediate need.
