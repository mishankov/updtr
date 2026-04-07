# Plan: YAML Configuration

> Source PRD: [plans/0005. yaml config/prd.md](/Users/mishankov/dev/updtr/plans/0005.%20yaml%20config/prd.md)

## Architectural decisions

Durable decisions that apply across all phases:

- **Config format**: YAML is the only supported public configuration format. JSON, TOML, HCL, CUE, and other formats remain unsupported.
- **Default config path**: when no explicit config path is provided, load `updtr.yaml` from the current working directory.
- **Default fallback**: if local `updtr.yaml` is absent and local `updtr.yml` exists, load `updtr.yml` as the default config. Do not search parent directories.
- **Explicit config paths**: user-provided config paths are authoritative. Explicit `.yaml` and `.yml` paths are parsed as YAML, and explicit paths do not trigger fallback discovery.
- **Unsupported extensions**: unsupported config extensions fail fast with a generic error that lists `.yaml` and `.yml` as the accepted extensions and does not mention prior implementation formats.
- **Loader boundary**: YAML decoding stays behind a small config loader boundary. The validated configuration model remains independent from decoder-specific types and errors.
- **Raw input shape**: decode YAML into raw policy and target structures that preserve unset-versus-empty distinctions for list fields.
- **Schema shape**: keep the current logical keys: optional top-level `policy`, required top-level `targets`, and snake_case keys such as `quarantine_days`, `update_mode`, and `include_indirect`.
- **Key models**: preserve the existing resolved `Config`, `Target`, `Policy`, and `UpdateMode` concepts so planning, policy evaluation, rendering, and apply orchestration do not learn about YAML decoder details.
- **Validation**: preserve strict unknown-field rejection and all existing validation rules for required targets, target names, duplicate target names, duplicate effective targets, supported ecosystems, paths, quarantine days, update modes, allow/deny lists, and pins.
- **Policy semantics**: preserve global policy inheritance, target override behavior, explicit empty allow/deny list semantics, target-scoped indirect opt-in, target-scoped update mode, and target-level pin merging.
- **Path behavior**: preserve config-relative target path resolution from the actual config file loaded. Path containment remains lexical after normal cleanup.
- **Command behavior**: preserve the existing `detect`, `apply`, `init`, and `version` command model. The config format change must not alter dependency planning, policy decisions, rendering, apply ordering, target selection, or exit-code semantics.
- **Generated configuration**: `updtr init` writes YAML to `updtr.yaml`, includes one global quarantine policy and one target per discovered Go module, and omits optional direct-only fields such as `include_indirect`.
- **Init safety**: `updtr init` must not overwrite an existing `updtr.yaml` or `updtr.yml`.
- **Documentation surface**: CLI help, command descriptions, generated output, errors, tests, and docs should describe YAML defaults only.

---

## Phase 1: Default YAML Load Path

**User stories**: 1, 2, 3, 9, 13, 14, 26

### What to build

Build the first end-to-end YAML configuration path for the default `updtr.yaml` file. A simple YAML config should load from the current working directory by default, reject unknown fields, resolve to the same internal target and policy model as before, and drive `detect` and `apply` without changing planning or mutation behavior.

### Acceptance criteria

- [ ] With no explicit config path, config loading reads local `updtr.yaml`.
- [ ] A valid `.yaml` config with a top-level policy and at least one Go target resolves into the existing internal config, target, and policy model.
- [ ] Missing default config errors mention `updtr.yaml`.
- [ ] Strict unknown-field validation rejects misspelled top-level, policy-level, and target-level YAML keys.
- [ ] Malformed YAML produces a parse error that includes the config path.
- [ ] Existing detect and apply command behavior is unchanged after a YAML config has been loaded.
- [ ] YAML decoder-specific details are isolated from planning, policy evaluation, rendering, and orchestration code.
- [ ] Tests prove default `updtr.yaml` loading, strict unknown-field rejection, malformed YAML errors, and unchanged command behavior for a simple valid config.

---

## Phase 2: Explicit Paths and Format Boundaries

**User stories**: 4, 10, 11, 12, 27

### What to build

Complete the public filename contract. Explicit `.yaml` and `.yml` paths should both parse as YAML, default loading should accept local `updtr.yml` only when `updtr.yaml` is absent, and unsupported extensions should fail before parsing with a clear accepted-extension message. Explicit paths remain authoritative and the loader still avoids parent-directory discovery.

### Acceptance criteria

- [ ] Explicit `.yaml` config paths are accepted and parsed as YAML.
- [ ] Explicit `.yml` config paths are accepted and parsed as YAML.
- [ ] Default loading prefers local `updtr.yaml` when both `updtr.yaml` and `updtr.yml` exist.
- [ ] Default loading accepts local `updtr.yml` when `updtr.yaml` is absent.
- [ ] Default loading still does not search parent directories for either YAML filename.
- [ ] An explicit config path does not fall back to another filename when the explicit file is absent.
- [ ] Unsupported config extensions fail fast with an error that lists `.yaml` and `.yml` as the accepted extensions.
- [ ] Unsupported-extension errors do not mention TOML or implementation history.
- [ ] Tests prove accepted extensions, default preference and fallback, explicit path authority, unsupported-extension failure, and no parent-directory search.

---

## Phase 3: Policy and Target Semantics Parity

**User stories**: 8, 9, 15, 16, 17, 18, 19, 20, 21, 22, 23

### What to build

Harden YAML decoding and validation across the full existing configuration model. YAML examples and tests should exercise global policy, multiple targets, policy inheritance, target overrides, empty lists, pins, indirect opt-in, vulnerability-only update mode, duplicate detection, config-relative paths, and invalid values while proving that resolved policy decisions remain stable.

### Acceptance criteria

- [ ] YAML supports global `quarantine_days`, `update_mode`, `allow`, `deny`, and `pin` policy fields.
- [ ] YAML supports target-level `quarantine_days`, `update_mode`, `allow`, `deny`, `pin`, and `include_indirect` fields.
- [ ] Global policy values are inherited by targets unless a target explicitly overrides them.
- [ ] Explicitly empty YAML allow lists and deny lists preserve their distinct unset-versus-empty semantics.
- [ ] YAML pin maps decode correctly and target-level pins merge with or override global pins as before.
- [ ] Target-scoped `include_indirect` remains false when omitted and true only for targets that opt in.
- [ ] Target-scoped `update_mode` supports `normal` and `vulnerability_only`, including global inheritance and target override behavior.
- [ ] Duplicate target names and duplicate effective ecosystem/path targets are rejected.
- [ ] Invalid target names, ecosystems, paths, negative quarantine days, update modes, empty module paths, and duplicate module list entries are rejected consistently.
- [ ] Config-relative target paths resolve from the directory of the actual YAML config file loaded.
- [ ] Tests prove YAML semantics parity for policy inheritance, empty lists, pins, indirect opt-in, update modes, duplicate checks, validation failures, and config-relative paths.

---

## Phase 4: YAML Init Generation Safety

**User stories**: 5, 6, 7, 24

### What to build

Convert the initialization flow to generate the public YAML config contract. `updtr init` should discover Go modules as before, write a minimal `updtr.yaml`, preserve the safe direct-only defaults by omitting optional fields, and avoid overwriting either supported YAML config filename.

### Acceptance criteria

- [ ] `updtr init` writes `updtr.yaml` for repositories with discovered Go modules.
- [ ] Generated YAML includes one global quarantine policy with the existing safe default.
- [ ] Generated YAML includes one target entry per discovered Go module with `name`, `ecosystem`, and `path`.
- [ ] Generated YAML omits `include_indirect` and other optional fields when their defaults are sufficient.
- [ ] Generated root and nested target paths preserve the existing path style and deterministic ordering.
- [ ] `updtr init` does nothing when `updtr.yaml` already exists.
- [ ] `updtr init` does nothing when `updtr.yml` already exists.
- [ ] Existing no-module initialization behavior remains unchanged except for YAML-specific messaging.
- [ ] Tests prove YAML generation, safe defaults, direct-only omission, deterministic discovered targets, and both existing-config no-op cases.

---

## Phase 5: CLI Messaging and Public Contract Hardening

**User stories**: 25, 27

### What to build

Finish the public contract by updating CLI help, command summaries, user-facing messages, dependency metadata, and regression tests so users only see the supported YAML model. The final slice should remove stale TOML references from active code and tests while preserving historical plan files as prior project context.

### Acceptance criteria

- [ ] The root `--config` help default shows `updtr.yaml`.
- [ ] The `init` command summary describes creating `updtr.yaml` or YAML configuration.
- [ ] Init success and existing-config messages mention YAML filenames accurately.
- [ ] Config load, parse, missing-file, and unsupported-extension errors consistently describe YAML behavior.
- [ ] Active tests and fixtures no longer rely on TOML inputs for current config behavior.
- [ ] Runtime dependencies no longer include the TOML decoder if it is unused.
- [ ] Tests prove CLI help and init summaries mention YAML defaults.
- [ ] The full test suite passes with only `.yaml` and `.yml` accepted as current config inputs.
