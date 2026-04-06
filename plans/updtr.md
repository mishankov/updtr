# Plan: updtr

> Source PRD: [PRD.md](/Users/mishankov/dev/updtr/PRD.md)

## Architectural decisions

Durable decisions that apply across all phases:

- **Commands**: v1 keeps preview and mutation as separate user flows, exposed as distinct CLI commands for detection and apply.
- **Config**: policy is loaded from a repo-local TOML file designed to stay small in v1 while allowing ecosystem-specific sections later.
- **Workspace scope**: execution starts from a repository root and treats each discovered Go module as a unit of planning, reporting, and application.
- **Schema**: the stable internal plan shape distinguishes current dependency state, candidate update state, policy decision, reason codes, and apply results.
- **Key models**: config, workspace, dependency, candidate version, release metadata, policy decision, update plan, applied change, and module-level error.
- **Policy precedence**: pinned dependencies block changes first, deny rules exclude next, and allow rules constrain remaining candidates when allow rules are present.
- **Quarantine**: eligibility is based on candidate version release date, and missing or untrusted release-date metadata must block auto-approval explicitly.
- **Architecture boundary**: Go support is implemented as an ecosystem adapter behind shared discovery, planning, policy, rendering, and apply orchestration.
- **Update execution**: planning and application remain separate deep modules so preview and apply consume the same proposed change set.
- **Output contract**: results distinguish eligible updates, blocked updates with reasons, applied updates, and per-module operational failures, with room for structured output later.
- **Execution mode**: commands run non-interactively in both local and CI contexts and must preserve unrelated working tree changes.

---

## Phase 1: Single-Module Preview/Apply Spine

**User stories**: 1, 2, 11, 17, 18, 19, 22, 23, 31, 35

### What to build

Establish the thinnest end-to-end path for one local Go module: discover dependencies, resolve newer versions through normal Go module mechanisms, build a previewable update plan, and apply selected changes through a separate command. The slice should prove the core command model, show current and candidate versions, mutate only intended dependency state, and leave unrelated repository changes untouched.

### Acceptance criteria

- [ ] A developer can run a detection command against a single Go module repository root and see outdated dependencies with current and candidate versions.
- [ ] A developer can run a separate apply command that consumes the same planning flow and updates only dependencies selected by the plan.
- [ ] Applied changes use standard Go module workflows and leave the repository build metadata consistent for that module.
- [ ] Detection and apply output clearly summarize what was found or changed without requiring implementation knowledge.
- [ ] Running in a dirty repository does not silently reset or overwrite unrelated files.

---

## Phase 2: Config-Driven Quarantine Decisions

**User stories**: 3, 4, 5, 6, 24, 26, 32, 34

### What to build

Add TOML-driven policy loading and validation, then thread quarantine evaluation through the existing preview/apply spine. The slice should make release age part of planning, block candidates that are too new or missing trustworthy release-date metadata, and surface those decisions explicitly in command output.

### Acceptance criteria

- [ ] The tool reads a TOML config file from the repository context and fails fast with actionable validation errors when configuration is invalid.
- [ ] Quarantine policy is evaluated from candidate release date and applies uniformly to patch, minor, and major updates.
- [ ] Detection output shows candidate release age or equivalent quarantine context for each considered update.
- [ ] Candidates with missing or untrusted release-date metadata are blocked explicitly rather than silently approved.
- [ ] Apply respects the same quarantine decisions as preview, so blocked candidates are not mutated by the apply command.

---

## Phase 3: Policy Controls With Explainable Outcomes

**User stories**: 7, 8, 9, 10, 20, 21

### What to build

Extend the planner with allow rules, deny rules, and pinned dependency controls that operate independently from the Go-specific resolver. The slice should make policy precedence deterministic, keep pinned dependencies stable automatically, and explain why any dependency was selected or blocked so a maintainer can review the outcome confidently.

### Acceptance criteria

- [ ] Maintainers can declare allow lists, deny lists, and pinned dependencies in config and have those controls applied during planning.
- [ ] Policy precedence is deterministic and visible in behavior: pinned dependencies win first, deny rules exclude next, and allow rules constrain the remaining candidates when present.
- [ ] Detection output reports blocked updates with clear reason codes or equivalent explanations for quarantine, deny, allow, and pin decisions.
- [ ] Pinned dependencies are excluded from upgrades automatically, including the case where a dependency is pinned to its current version.
- [ ] Apply mutates only dependencies that remain eligible after the shared policy engine finishes evaluation.

---

## Phase 4: Recursive Multi-Module Workspace Runs

**User stories**: 12, 13, 25, 27

### What to build

Expand the same plan/apply workflow from one module to a repository workspace with multiple and nested Go modules. This slice should discover modules recursively, produce per-module reporting, keep successes and failures attributable to the affected module, and handle no-op runs cleanly so monorepo automation remains trustworthy.

### Acceptance criteria

- [ ] The tool can discover multiple Go modules from a single repository root, including nested modules in recursive layouts.
- [ ] Detection output groups or labels planned updates by module so monorepo results are understandable.
- [ ] Workspace-level failures identify the affected module clearly without obscuring successful results from other modules.
- [ ] A run with no eligible updates completes cleanly and reports that state without noisy or misleading output.
- [ ] Apply preserves the distinction between module-level success, blocked updates, and operational failure.

---

## Phase 5: CI-Stable Contracts and Extensibility Guardrails

**User stories**: 14, 15, 16, 28, 29, 30, 33

### What to build

Harden the CLI contract for unattended execution and formalize the ecosystem-agnostic core. This slice should stabilize non-interactive behavior, meaningful exit codes, planner-versus-applier separation, and the adapter boundary so future ecosystems can reuse shared policy and planning logic without rewriting the CLI core.

### Acceptance criteria

- [ ] Detection and apply both run non-interactively and return contractually stable exit codes for success, defined policy states, and operational failures.
- [ ] The CLI layer stays thin while shared planning, policy evaluation, and application orchestration remain reusable across ecosystems.
- [ ] The update planner is a distinct module that produces a complete change set consumable by both preview and apply flows.
- [ ] Shared policy logic remains independent from the Go ecosystem adapter, which owns dependency discovery, version resolution, metadata lookup, and Go-native update execution.
- [ ] Human-readable rendering is implemented in a way that leaves room for structured output support later without redesigning the core plan model.
