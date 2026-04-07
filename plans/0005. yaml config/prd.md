# YAML Config PRD

## Problem Statement

`updtr` needs a repository-local configuration file that is easy to read, review, and commit. The configuration should be friendly to CI and automation workflows, support multiple targets, and make update policy explicit without requiring command-line flags for normal unattended runs.

Because the tool has not been released yet, the first public configuration contract should be YAML-only. The codebase, command help, generated config, validation errors, tests, and documentation should describe only the supported YAML configuration model. There is no user-facing compatibility burden for older config formats.

The user needs `updtr` to read and generate YAML/YML configuration while preserving the existing policy model, target model, command behavior, and fail-fast validation semantics that make the tool safe for unattended CI use.

## Solution

Make YAML the only supported configuration format for `updtr`. The default config file should be `updtr.yaml`, while explicit config paths ending in `.yaml` or `.yml` should both be accepted. The generated configuration from the initialization flow should be YAML and should preserve the current direct-only, low-noise defaults.

The internal validated configuration model should remain independent from YAML decoder details. YAML should decode into a raw input shape, then flow through shared validation and resolution logic. Strict unknown-field rejection, policy inheritance, duplicate-target checks, path normalization, ecosystem validation, update-mode validation, allow/deny list validation, pin validation, and target-scoped indirect opt-in should keep the same external behavior.

Unsupported config file extensions should fail with a generic unsupported-format error that lists `.yaml` and `.yml` as the accepted extensions. The error message should not mention implementation history or unavailable formats.

## User Stories

1. As a maintainer, I want `updtr` to use YAML configuration, so that repository policy is easy to read and review in common automation workflows.
2. As a maintainer, I want the default config filename to be `updtr.yaml`, so that new projects start with one obvious config file.
3. As a maintainer, I want `.yaml` files to be accepted, so that I can use the standard long YAML extension.
4. As a maintainer, I want `.yml` files to be accepted, so that I can match repositories that standardize on the short YAML extension.
5. As a developer, I want `updtr init` to generate YAML, so that freshly initialized projects use the public config contract immediately.
6. As a developer, I want generated YAML to preserve the safe defaults, so that initialization does not broaden dependency update scope.
7. As a developer, I want generated YAML to omit optional direct-only fields when their default is sufficient, so that new configuration stays small.
8. As a developer, I want YAML examples to represent policy and target settings naturally, so that fields such as quarantine days, update mode, allow lists, deny lists, pins, and indirect opt-in are easy to edit.
9. As a developer, I want config semantics to remain stable, so that the same policy decisions produce the same update plans.
10. As a CI engineer, I want `updtr detect` and `updtr apply` to keep accepting an explicit config path, so that pipelines can choose a repository-specific config location.
11. As a CI engineer, I want explicit `.yaml` and `.yml` config paths to be parsed as YAML, so that there is no ambiguity in non-interactive jobs.
12. As a CI engineer, I want unsupported config extensions to fail fast with a clear accepted-extension message, so that invalid pipeline inputs are easy to correct.
13. As a CI engineer, I want missing default config errors to mention `updtr.yaml`, so that failures point users at the expected file.
14. As a maintainer, I want strict unknown-field validation in YAML, so that typos do not silently change update policy.
15. As a maintainer, I want YAML parsing errors to include useful context, so that malformed indentation or invalid values are easy to fix.
16. As a maintainer, I want duplicate target names and duplicate effective targets to be rejected, so that config validation catches ambiguous target definitions.
17. As a maintainer, I want target names, ecosystems, paths, update modes, and dependency lists to be validated consistently, so that invalid policy fails before planning or mutation.
18. As a maintainer, I want policy inheritance to remain available, so that global policy applies to targets unless a target explicitly overrides it.
19. As a maintainer, I want explicitly empty allow and deny lists to keep their distinct semantics, so that unset-versus-empty behavior remains clear.
20. As a maintainer, I want pin maps to be supported in YAML, so that module version holds are easy to express.
21. As a maintainer, I want target-scoped `include_indirect` to be supported in YAML, so that indirect dependency opt-in remains target-specific.
22. As a maintainer, I want target-scoped `update_mode` to be supported in YAML, so that vulnerability-only targets can be configured.
23. As a maintainer, I want config-relative target paths to resolve from the config file's directory, so that multi-directory setups stay portable.
24. As a maintainer, I want `updtr init` to avoid overwriting an existing YAML config, so that initialization remains safe.
25. As a developer, I want the CLI help text and command descriptions to refer to YAML, so that the documented default matches actual behavior.
26. As a future maintainer, I want parsing to be isolated behind a small loader boundary, so that future config decisions do not spread through planning, policy, rendering, or orchestration code.
27. As a future maintainer, I want tests that prove only `.yaml` and `.yml` are accepted, so that the public config contract remains explicit.

## Implementation Decisions

- YAML is the only supported config format.
- The default config filename is `updtr.yaml`.
- Both `.yaml` and `.yml` are accepted as YAML config file extensions.
- Unsupported config file extensions fail with a generic error that lists `.yaml` and `.yml` as accepted extensions.
- When no explicit config path is provided, prefer `updtr.yaml` in the current working directory.
- If `updtr.yaml` is absent but `updtr.yml` exists in the current working directory, allow it as a default YAML config.
- Do not search parent directories for config files. The current-directory-only default behavior remains unchanged.
- Preserve explicit config path behavior. A user-provided path is authoritative and should not trigger fallback discovery.
- Keep the internal resolved configuration model independent of the source file format.
- Decode YAML into a raw input shape that preserves unset-versus-empty distinctions for list fields.
- Preserve strict unknown-field rejection for YAML.
- Preserve all validation rules: required targets, target name format, duplicate target names, duplicate effective targets, supported ecosystems, repository-relative paths, non-negative quarantine days, allowed update mode values, duplicate allow/deny entries, and non-empty module paths.
- Preserve policy inheritance and target override behavior exactly.
- Preserve config-relative target path resolution based on the actual config file that was loaded.
- Keep YAML field names aligned with the logical config keys, including snake_case names such as `quarantine_days`, `update_mode`, and `include_indirect`.
- Use a YAML representation that is natural for lists and maps while retaining the existing data model.
- Keep generated config direct-only by default by omitting `include_indirect` unless a future feature explicitly changes initialization policy.
- Keep generated config minimal: one global quarantine policy and one target entry per discovered Go module.
- Update CLI help, command summaries, user-facing messages, tests, and docs to describe YAML defaults only.
- Ensure initialization treats existing YAML configuration safely. It should not overwrite an existing `updtr.yaml` or `updtr.yml`.
- Do not add a separate config conversion command in the first release.
- Do not change dependency planning, policy evaluation, rendering, or apply behavior as part of this config-format work.

## Testing Decisions

- Good tests should assert externally visible behavior: accepted filenames, parsed config semantics, validation errors, init output, CLI defaults, and no-regression policy resolution. They should not assert decoder internals unless those internals are part of the user-facing contract.
- Add configuration tests for a valid `updtr.yaml` with global policy, multiple targets, pins, allow lists, deny lists, indirect opt-in, and vulnerability-only update mode.
- Add configuration tests for a valid `updtr.yml` with the same semantics as `updtr.yaml`.
- Add tests proving YAML policy inheritance matches the resolved model.
- Add tests proving explicitly empty YAML lists retain the intended allow-list and deny-list semantics.
- Add tests proving YAML pin maps decode and merge with target-level pins correctly.
- Add tests proving YAML unknown top-level, policy-level, and target-level keys are rejected.
- Add tests proving malformed YAML produces a parse error that names the config path.
- Add tests proving invalid YAML values produce validation failures, including negative quarantine days, invalid update modes, duplicate target names, duplicate effective targets, invalid paths, empty module paths, and duplicate module lists.
- Add tests proving default loading prefers `updtr.yaml` in the current working directory.
- Add tests proving default loading accepts `updtr.yml` when `updtr.yaml` is absent.
- Add tests proving explicit config paths do not search or fall back to other filenames.
- Add tests proving unsupported config extensions fail with a generic accepted-extension error.
- Add tests proving default loading still does not search parent directories.
- Add initialization tests proving new configs are written as YAML with the expected safe defaults.
- Add initialization tests proving existing `updtr.yaml` or `updtr.yml` prevents accidental overwrite.
- Add CLI tests proving the default `--config` help and init summary mention YAML.
- Use the existing configuration, initialization, and CLI tests as prior art for table-driven validation and temporary-directory behavior.

## Out of Scope

- Changing dependency update policy semantics.
- Changing target selection, rendering, apply ordering, or exit-code behavior.
- Searching parent directories for configuration.
- Introducing a project-wide configuration discovery algorithm beyond the local default filenames described above.
- Adding a dedicated config conversion command in the first release.
- Automatically rewriting a user's config in place.
- Supporting JSON, HCL, CUE, or other configuration formats.
- Renaming logical config keys.
- Adding new policy options unrelated to YAML configuration.
- Changing generated config defaults to include indirect dependencies or vulnerability-only mode.

## Further Notes

- The most important compatibility rule is that YAML configuration must resolve to the same internal policy model used by planning and apply behavior.
- `updtr.yaml` is the recommended default because it is the more explicit YAML extension. Supporting `updtr.yml` avoids unnecessary friction for repositories that standardize on the short extension.
