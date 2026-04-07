# Plan: updtr

> Source PRD: [PRD.md](/Users/mishankov/dev/updtr/PRD.md)

## Architectural decisions

Durable v1 decisions that apply across all phases:

- **Commands**: v1 exposes `updtr init`, `updtr detect`, `updtr apply`, and `updtr version`. Preview and mutation stay separate. No aliases are required in v1.
- **CLI implementation**: use Cobra for subcommands and flags. Use `github.com/mishankov/updtr` as the Go module path.
- **Config parser**: use `github.com/pelletier/go-toml/v2` with strict validation so unknown keys, malformed TOML, duplicate target names, duplicate effective targets, invalid ecosystems, invalid target names, empty policy paths, duplicate allow/deny entries, and invalid paths fail clearly.
- **Config location**: default config is `./updtr.toml` in the current directory only. Do not search upward. `--config <path>` is allowed as an explicit override.
- **Config schema**: `[policy]` is optional. `[[targets]]` entries require `name`, `ecosystem`, and `path`; per-target policy overrides are flat fields on the target entry, not nested `targets.policy`.
- **Target schema**: v1 accepts only `ecosystem = "go"`. Target names must be non-empty and match `[a-z0-9-]+`. Target paths must be relative, non-empty, repo-contained after lexical path cleaning, and may use either `tools/cli` or `./tools/cli` spelling. Absolute paths and lexical escapes are invalid.
- **Path behavior**: do not add symlink-specific validation or traversal logic. Use standard filesystem/path operations as-is. Containment is lexical after normal `.` and `..` cleanup, not realpath-based.
- **Policy fields**: supported policy fields are `quarantine_days`, `allow`, `deny`, and `pin`. No policy field is required. `pin` is a map from exact module path to explicit version only; v1 does not support pin-to-current or automatic downgrades.
- **Policy inheritance**: global policy provides defaults. Target scalar fields override when set. Target `allow` and `deny` lists replace inherited lists, including explicit empty lists. Target `pin` maps merge with global pins, and target entries override matching global keys. Inherited pins cannot be cleared in v1.
- **Policy validation**: unmatched policy rules are ignored silently. Exact overlaps between `allow` and `deny` are allowed and resolved by precedence. Empty module paths and duplicate entries within the same `allow` or `deny` list are invalid.
- **Pin validation**: config decoding only requires `pin` to be a string map. Go version validity is checked while evaluating Go targets; invalid pin versions become target-level operational errors and return `1`.
- **Allow semantics**: omitted `allow` means no allow-list constraint. Explicit `allow = []` means allow-list mode is enabled with no allowed modules, so every otherwise eligible dependency is blocked unless an earlier rule wins.
- **Quarantine semantics**: omitted `quarantine_days` disables quarantine for that effective target. Explicit `quarantine_days = 0` enables zero-day quarantine, so release-date trust is still checked but no waiting period applies. Negative values are invalid. If global quarantine is set, v1 has no target-level disable switch; use `quarantine_days = 0` for immediate updates with metadata trust checks.
- **Policy precedence**: evaluate decisive policy outcomes in this order: `pin_mismatch`, `pinned`, `denied`, `not_allowed`, quarantine. Return one primary reason per blocked dependency.
- **Policy matching**: match Go modules by exact canonical module path only. No globbing, regex, path normalization, version-qualified rules, or replace-only policy subjects in v1.
- **Reason labels**: stable v1 blocked reasons are `pin_mismatch`, `pinned`, `denied`, `not_allowed`, `quarantined`, `missing_release_date`, `untrusted_release_date`, `replaced_dependency`, and `candidate_resolution_failed`.
- **Target selection**: `detect` and `apply` run all configured targets by default. `--target <name>` is repeatable, exact, and case-sensitive. Unknown selected names are command errors. After filtering, execution preserves config order, not flag order.
- **Execution model**: normal runs are config-driven and non-recursive. `detect` and `apply` never discover nested modules on their own; they operate only on configured targets.
- **Init model**: `updtr init` recursively scans the current directory tree for Go modules and creates `./updtr.toml` only when the file does not already exist. If `updtr.toml` exists, print that and do nothing with exit `0`. If no Go modules are found, print that and do nothing with exit `0`. No `--root`, `--write`, or dry-run mode in v1.
- **Init discovery**: discover directories containing `go.mod` and continue scanning below discovered modules. Skip only `.git`, `vendor`, and `node_modules` by directory name. Sort discovered targets by normalized relative path, put root `.` first, derive names from relative paths, normalize generated names to `[a-z0-9-]+`, and append deterministic numeric suffixes on generated-name collisions.
- **Init output**: generated configs always include `[policy] quarantine_days = 7`. Generated target entries include only `name`, `ecosystem`, and `path`. Use `path = "."` for root and `path = "./tools/cli"` style for non-root targets. Print a concise success message like `created updtr.toml with 3 go targets`.
- **Go dependency scope**: v1 manages only direct `require` dependencies from each target's `go.mod`, discovered with the Go module parser. Indirect dependencies may change as Go toolchain side effects, but are not first-class planned items.
- **Go target scope**: each configured Go target path must point to a single module directory containing `go.mod`. `go.work` roots and workspace targets are out of scope in v1.
- **Go candidate resolution**: use the Go toolchain, for example `go list -m -u -mod=readonly -json <module>` or equivalent dependency-specific queries, to resolve candidate versions. Do not infer candidates by manually scraping `go.mod` or by running broad upgrade commands.
- **Version selection**: for stable current versions, choose the newest newer stable version and ignore pre-releases. If the newest available version is a pre-release but an older newer stable version exists, choose the stable version. Pre-release track behavior may follow normal Go resolution when current is already pre-release.
- **Replace handling**: skip direct dependencies affected by any `replace` directive and report `replaced_dependency`. Let Go handle `exclude` and `retract` semantics during candidate resolution.
- **Release metadata**: quarantine uses Go-toolchain-visible module metadata or an equivalent trusted Go module metadata source for the exact `module@version`. Do not scrape Git tags, GitHub releases, or commit dates heuristically in v1. Missing metadata blocks with `missing_release_date`; untrusted or future timestamps block with `untrusted_release_date`.
- **Quarantine time math**: compare exact elapsed duration. A candidate is eligible when `candidate_release_time <= now - quarantine_days * 24h`. The boundary is inclusive. The CLI uses the runtime clock; the policy engine should accept an injected clock for deterministic tests.
- **Apply selection**: `apply` recomputes the plan from current repo state and applies all currently eligible updates for selected targets. It does not consume a saved plan file and does not support dependency-level selection in v1.
- **Apply execution**: process targets sequentially in config order. Within a target, apply selected dependencies sorted by module path with `go get module@version`, then run `go mod tidy` once only if at least one update succeeded. Stop that target on the first mutation failure and continue with the next target.
- **Mutation safety**: do not rely on git, dirty-worktree checks, branch state, reset, or rollback. If a Go command fails after partial mutation, leave the filesystem as produced by the Go toolchain and report the error.
- **Incidental changes**: snapshot direct `require` versions before apply and compare after `go mod tidy`. If direct dependencies changed beyond the planned applied set, emit warning `additional_direct_changes_detected`.
- **Output**: v1 is human-readable only; no `--json` flag yet. Keep internal plan/result models renderer-agnostic so structured output can be added later.
- **Detect rendering**: render every selected target once. Omit empty sections, keep a target summary, and print a global `Total` summary. Show eligible and blocked dependency decisions. Omit current/no-op dependencies, except `pin_mismatch` policy drift. Show release dates only when quarantine is enabled and relevant to eligible/quarantined decisions.
- **Apply rendering**: render every selected target once with applied updates, blocked updates, warnings, errors, target summary, and global `Total` summary. Blocked updates remain visible in apply output.
- **Errors and streams**: normal `detect` and `apply` summaries go to stdout. Fatal command/config errors go to stderr. Target-level runtime errors after successful config load appear in the normal summary only to avoid duplicate CI log lines.
- **Exit codes**: v1 uses only `0` and `1`. Policy-blocked dependencies are normal successful outcomes and return `0` if there are no operational failures. Invalid config, invalid command usage, missing `go` for selected Go targets, target operational failures, and failed Go commands return `1`.
- **Prerequisites**: `detect` and `apply` fail fast if any selected Go target exists and the `go` binary is unavailable. `init` and `version` do not require the Go binary. `version` prints a compact single line and returns `0`.
- **Architecture boundary**: Go support is implemented as an ecosystem adapter behind shared config, target selection, planning, policy, rendering, and apply orchestration. Shared policy logic remains independent of Go resolution and mutation.

Example v1 config:

```toml
[policy]
quarantine_days = 7
allow = ["github.com/example/lib"]
deny = ["github.com/example/risky"]
pin = { "golang.org/x/mod" = "v0.23.0" }

[[targets]]
name = "root"
ecosystem = "go"
path = "."

[[targets]]
name = "tools-cli"
ecosystem = "go"
path = "./tools/cli"
quarantine_days = 30
deny = ["github.com/example/cli-risky"]
pin = { "golang.org/x/text" = "v0.14.0" }
```

---

## Phase 1: CLI, Config, Init, and Target Selection

**User stories**: 3, 11, 12, 13, 26, 27, 34, 35

### What to build

Establish the command shell, strict TOML config loading, target model, and `init` generation behavior. This phase should make `updtr` a real Go CLI with a repo-local config contract before adding update planning. Normal execution should be config-driven; recursive discovery exists only in `init`.

### Acceptance criteria

- [ ] `go.mod` exists with module path `github.com/mishankov/updtr`, and the CLI uses Cobra with commands `init`, `detect`, `apply`, and `version`.
- [ ] `version` prints a compact single-line version and returns `0` without requiring the Go binary.
- [ ] Config loads from `./updtr.toml` by default or from explicit `--config <path>`, with no upward discovery.
- [ ] Config validation rejects malformed TOML, unknown keys, missing target identity fields, duplicate target names, duplicate effective targets, unsupported ecosystems, invalid target names, absolute paths, lexical path escapes, empty policy module paths, duplicate allow/deny entries, and negative quarantine values.
- [ ] Valid config supports optional `[policy]` and flat per-target `quarantine_days`, `allow`, `deny`, and `pin` fields with the resolved inheritance rules.
- [ ] `--target <name>` is repeatable, exact, case-sensitive, fails on unknown names, and filtered execution preserves config order.
- [ ] A valid config with zero targets is rejected as a runtime configuration error.
- [ ] `init` creates `./updtr.toml` only when no config file exists and at least one Go module is discovered.
- [ ] `init` scans the current directory tree, skips `.git`, `vendor`, and `node_modules`, includes nested Go modules, sorts targets deterministically, derives safe path-based names, and writes `[policy] quarantine_days = 7`.
- [ ] `init` does nothing when `updtr.toml` already exists and does nothing when no Go modules are found.

---

## Phase 2: Single-Target Go Detection Spine

**User stories**: 1, 10, 14, 18, 19, 23, 24, 28, 29, 30, 32

### What to build

Build the first read-only end-to-end planning path for configured Go targets. The slice should parse direct dependencies from `go.mod`, resolve candidates through the Go toolchain, obtain release metadata for quarantine decisions, evaluate shared policy, and render human-readable detection output.

### Acceptance criteria

- [ ] `detect` fails fast with exit `1` if selected Go targets exist and the `go` binary is unavailable.
- [ ] Each configured Go target path is evaluated as a single module directory containing `go.mod`; missing paths or missing/unreadable `go.mod` files become target-level runtime errors while other targets continue.
- [ ] Direct dependencies are discovered from direct `require` entries in `go.mod`; indirect dependencies are not first-class planned items.
- [ ] Candidate resolution uses dependency-specific Go toolchain queries and chooses the newest newer stable candidate for stable current versions.
- [ ] Direct dependencies affected by `replace` are blocked with `replaced_dependency`; Go's own resolution handles `exclude` and `retract`.
- [ ] Candidate release-date metadata is sourced from trusted Go-toolchain-visible module metadata or equivalent exact `module@version` metadata.
- [ ] Missing release-date metadata blocks with `missing_release_date` when quarantine is enabled; untrusted or future timestamps block with `untrusted_release_date`.
- [ ] Quarantine uses exact elapsed duration and inclusive boundary semantics.
- [ ] Policy evaluation supports `pin_mismatch`, `pinned`, `denied`, `not_allowed`, `quarantined`, and eligible outcomes with one primary reason per blocked dependency.
- [ ] `pin_mismatch` is surfaced even when no newer candidate exists; ordinary current/no-op dependencies are omitted.
- [ ] Isolated dependency-specific candidate resolution failures are blocked with `candidate_resolution_failed` when planning can continue.
- [ ] Detection output renders every selected target once, omits empty sections, includes target summaries and a global `Total` summary, and returns `0` for successful runs even when dependencies are blocked by policy.

---

## Phase 3: Apply Spine and Mutation Reporting

**User stories**: 2, 17, 18, 19, 20, 22, 31

### What to build

Add mutation by having `apply` recompute the same plan from current state and apply all currently eligible updates for selected targets. The applier should use normal Go module commands, avoid git assumptions, report partial failures truthfully, and preserve the shared planner's blocked-dependency explanations.

### Acceptance criteria

- [ ] `apply` reuses the same planner as `detect` and does not consume a saved plan file.
- [ ] `apply` processes selected targets sequentially in config order and applies eligible dependencies within each target sorted by module path.
- [ ] Each planned update is applied with `go get module@version`; `go mod tidy` runs once after successful updates and only when at least one update was applied.
- [ ] `apply` does not run `go get -u`, `go test`, `go build`, broad verification, git dirty-worktree checks, reset, or rollback.
- [ ] On the first `go get` or `go mod tidy` failure within a target, `apply` stops that target, reports already-applied changes and the error, leaves the filesystem as produced by the Go toolchain, and continues with the next target.
- [ ] Apply output includes applied updates, blocked updates, warnings, errors, target summaries, and a global `Total` summary.
- [ ] Direct dependency versions are snapshotted before and after mutation; unexpected direct dependency drift beyond planned applied updates emits warning `additional_direct_changes_detected`.
- [ ] Target-level runtime errors after config load appear in normal output only, while fatal command/config errors go to stderr.
- [ ] The command returns `0` for successful apply runs, including runs with no eligible updates or policy-blocked updates, and returns `1` when any operational failure occurs.

---

## Phase 4: Multi-Target Robustness and CI Contracts

**User stories**: 12, 13, 14, 15, 16, 25, 27, 33

### What to build

Harden the configured multi-target behavior for unattended use. This phase should make all-target default runs, target filtering, per-target continuation, output aggregation, and the simplified `0`/`1` exit-code contract dependable under CI.

### Acceptance criteria

- [ ] `detect` and `apply` run all configured targets by default and support repeatable `--target` filters without changing config-order execution.
- [ ] Runtime failure in one target does not prevent remaining selected targets from being processed.
- [ ] Every selected target appears exactly once in output, including targets with only runtime errors.
- [ ] No-op targets with no direct dependencies, no newer candidates, or no eligible updates report a clean target summary without noisy empty sections.
- [ ] Global `Total` summary is always printed, including single-target runs.
- [ ] Exit code `0` means command completed without operational failures, even if policy blocked updates. Exit code `1` covers invalid usage, invalid config, missing prerequisites, target runtime failures, and failed Go commands.
- [ ] `detect` and `apply` are fully non-interactive and do not prompt during normal local or CI execution.
- [ ] The renderer stays human-readable-only in v1 while consuming structured result data so JSON output can be added later.

---

## Phase 5: Extensibility Guardrails and Test Coverage

**User stories**: 16, 28, 29, 30

### What to build

Formalize the ecosystem-agnostic seams and test strategy so future ecosystems can be added without rewriting the CLI core. This phase should lock down shared config, policy, planner, renderer, and applier interfaces while keeping Go-specific behavior inside the adapter.

### Acceptance criteria

- [ ] Shared config loading, target selection, policy evaluation, plan/result models, rendering, and apply orchestration are independent of Go-specific module mechanics.
- [ ] The Go adapter owns Go module parsing, candidate resolution, release metadata lookup, replace handling, and Go-native mutation commands.
- [ ] The policy engine accepts an injected clock/time value so quarantine behavior is deterministic under test.
- [ ] Table-driven tests cover config validation, policy inheritance, policy precedence, unset-versus-empty list semantics, quarantine boundaries, missing/untrusted release metadata, and pin mismatch behavior.
- [ ] Fixture-based tests cover `init` discovery and generated TOML, target filtering, single-module detection, multi-target continuation, replaced dependencies, apply command sequencing, tidy gating, partial mutation failure reporting, and incidental direct dependency drift warnings.
- [ ] Command-level tests cover stdout/stderr split, global summaries, no-op output, invalid config errors, missing target errors, missing Go prerequisite handling, and the `0`/`1` exit-code contract.
- [ ] Tests assert external behavior and stable contracts rather than private helper implementation details.
