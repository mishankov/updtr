# Product Requirements Document: updtr

## Problem Statement

Projects accumulate outdated dependencies across different ecosystems. Updating them manually is repetitive, easy to postpone, and risky when maintainers accidentally adopt very fresh releases that have not yet had time to surface bugs, regressions, or malicious supply-chain behavior. The user needs a command-line tool that can reliably detect outdated dependencies and apply updates while enforcing a quarantine period, starting with Go modules and leaving room to support other ecosystems later.

## Solution

`updtr` will be a Go CLI for dependency maintenance. In v1, it will support the Go ecosystem only. The CLI will provide separate commands for:

- detecting available dependency updates without changing the repo
- applying eligible dependency updates to the repo

The tool will read configuration from a TOML file, including a quarantine period based on dependency release date. A dependency update is eligible only when the candidate version is old enough to satisfy the configured quarantine window. The same quarantine rule applies equally to patch, minor, and major version updates. The tool will also support allow lists, deny lists, and pinned dependencies. It must work for a single local repository, recursive or multi-module repositories, and non-interactive CI or bot-style execution.

The architecture must be ecosystem-agnostic so that Go is an implementation of a more general updater model rather than a one-off codepath.

## User Stories

1. As a developer, I want to scan my Go project for outdated modules, so that I can see what can be updated before making changes.
2. As a developer, I want update application to be a separate command from detection, so that I can review planned changes before mutating the repository.
3. As a developer, I want the tool to read a TOML config file, so that update policy is explicit, reviewable, and easy to commit to version control.
4. As a developer, I want to define a quarantine period in config, so that newly released dependency versions are not adopted immediately.
5. As a security-conscious maintainer, I want quarantine to be based on dependency release date, so that updates must age before they become eligible.
6. As a maintainer, I want the same quarantine rule to apply to major, minor, and patch releases, so that update behavior is consistent and predictable.
7. As a maintainer, I want to allow specific modules explicitly, so that I can scope automation to approved dependencies.
8. As a maintainer, I want to deny specific modules explicitly, so that I can permanently exclude risky or intentionally unmanaged dependencies.
9. As a maintainer, I want to pin dependencies to a specific version or current version, so that critical packages do not move unexpectedly.
10. As a maintainer, I want the scanner to report why a dependency was not selected for update, so that policy decisions are understandable during review.
11. As a developer, I want to run the tool against a single repository root, so that simple projects work with minimal configuration.
12. As a developer, I want the tool to discover multiple Go modules in one workspace, so that monorepos can be updated in one run.
13. As a developer, I want recursive workspace support, so that nested Go modules are handled without manual enumeration.
14. As a CI engineer, I want the detection command to run non-interactively, so that it can be used in automated checks and scheduled jobs.
15. As a CI engineer, I want meaningful exit codes, so that pipelines can distinguish between success, policy-blocked updates, and operational failure.
16. As a CI engineer, I want machine-readable output support to be possible in the design, so that future CI integrations do not require architectural rewrites.
17. As a developer, I want update application to modify `go.mod` and related dependency state correctly, so that the repository remains buildable after updates.
18. As a developer, I want the tool to operate through normal Go module mechanisms, so that it behaves consistently with the Go toolchain.
19. As a developer, I want the tool to preserve dependencies that are already compliant with policy, so that only intended modules change.
20. As a maintainer, I want pinned modules to be excluded from upgrades automatically, so that local policy wins over available newer versions.
21. As a maintainer, I want allow and deny rules to be deterministic, so that conflicting policies are resolved predictably.
22. As a developer, I want the update command to surface a clear summary of applied changes, so that I can review what changed in each module.
23. As a developer, I want the detection command to show current and candidate versions, so that I can assess upgrade scope quickly.
24. As a developer, I want the tool to indicate release age for candidate versions, so that quarantine decisions are transparent.
25. As a developer, I want workspace-level failures to identify the affected module, so that multi-module runs are debuggable.
26. As a developer, I want configuration errors to fail fast with actionable messages, so that bad policy files can be corrected easily.
27. As a developer, I want the tool to handle repositories with no eligible updates cleanly, so that automated runs stay quiet and trustworthy.
28. As a future maintainer, I want Go support to sit behind an ecosystem interface, so that new ecosystems can be added without rewriting the CLI core.
29. As a future maintainer, I want shared policy evaluation to be independent from Go-specific resolution, so that allow, deny, pin, and quarantine logic can be reused.
30. As a future maintainer, I want update planning to be decoupled from update execution, so that preview and apply flows remain consistent across ecosystems.
31. As a developer, I want the tool to work in a dirty repository without silently resetting unrelated files, so that local work is not lost.
32. As a developer, I want the tool to fail clearly when required release-date metadata cannot be determined, so that quarantine decisions are never guessed silently.
33. As a CI engineer, I want non-zero failures only for real operational or policy states defined by the CLI contract, so that automation remains stable.
34. As a maintainer, I want configuration to remain small in v1, so that the tool solves the core update workflow without premature complexity.
35. As a developer, I want the command set to be understandable without reading implementation details, so that adoption cost stays low.

## Implementation Decisions

- The CLI will expose at least two primary behaviors: discovery and apply. Exact command names may evolve, but preview and mutation must remain separate user flows.
- The first supported ecosystem is Go modules. The product design must treat Go as one ecosystem adapter behind common interfaces for discovery, metadata lookup, planning, and application.
- The core execution model will be: load config, discover workspaces or modules, collect current dependencies, resolve candidate updates, fetch release-date metadata, evaluate policy, build an update plan, then either print the plan or apply it.
- The quarantine period is measured against the candidate dependency version release date. If release-date metadata is missing or cannot be trusted, the candidate must not be auto-approved silently.
- Quarantine policy applies uniformly to patch, minor, and major updates. Version category is not a policy distinction in v1.
- Policy evaluation will support three user-managed controls in v1:
  - allow list
  - deny list
  - pinned dependencies
- Policy precedence must be explicit and deterministic. A practical v1 order is: pinned dependencies block change first, deny rules exclude next, allow rules then constrain remaining eligible dependencies when allow rules are present.
- The config file will be TOML-based and repo-friendly. It should be designed so future ecosystems can have ecosystem-specific sections while sharing a common top-level policy model.
- Multi-module and monorepo support will be part of the core design rather than a later add-on. Workspace scanning should identify each Go module as a unit of planning and reporting.
- Planning and application should be separate deep modules:
  - a planner that produces a complete proposed change set and reason codes
  - an applier that consumes the plan and performs repository mutations
- Shared policy logic should live outside the Go adapter. This allows future ecosystems to reuse quarantine, allow, deny, and pin semantics.
- The Go adapter should own:
  - reading module state from Go projects
  - resolving available versions
  - obtaining release-date metadata for candidate versions
  - applying selected updates using standard Go module workflows
- The CLI layer should remain thin. Its job is argument parsing, invoking the orchestration layer, rendering results, and returning contractually stable exit codes.
- Result reporting should distinguish:
  - eligible updates
  - blocked updates and the blocking reason
  - applied updates
  - module-level operational errors
- The architecture should anticipate future structured output support, even if human-readable text is the first renderer.
- Repositories may be local developer workspaces or CI environments. The tool must not rely on interactive prompts for normal operation.
- v1 should prefer correctness and traceability over maximum update aggressiveness. Clear explanations for why an update was or was not applied are part of the product behavior.

### Proposed Deep Modules

- Configuration module: parses and validates TOML config into a stable internal model shared by all commands.
- Workspace discovery module: finds target Go modules across single-repo and recursive layouts and emits a normalized workspace list.
- Ecosystem adapter interface: defines how an ecosystem discovers dependencies, resolves candidates, fetches metadata, and applies updates.
- Go ecosystem adapter: implements the ecosystem interface for Go modules only in v1.
- Release metadata provider: resolves candidate-version release dates with a narrow interface so the policy engine does not depend on transport or registry details.
- Policy engine: applies quarantine, allow, deny, and pin rules and returns decision results with reason codes.
- Update planner: combines current state, candidate state, and policy decisions into a complete plan that can be rendered or applied.
- Update applier: executes a plan and reports per-module results without owning policy logic.
- Output renderer: formats human-readable summaries now and can later be extended for structured output.

## Testing Decisions

- Good tests should verify external behavior and stable contracts, not internal implementation details. Tests should assert outcomes such as selected updates, blocked updates, rendered summaries, config validation behavior, and repository mutations that a user would observe.
- The configuration module should be tested with valid and invalid TOML inputs, default handling, and policy precedence edge cases.
- The workspace discovery module should be tested against single-module and multi-module repository fixtures.
- The policy engine should be tested extensively with table-driven cases for quarantine thresholds, allow and deny conflicts, pinned dependencies, and missing metadata behavior.
- The update planner should be tested with fake ecosystem and metadata providers so plan outputs and reason codes can be validated deterministically.
- The Go ecosystem adapter should be tested at the boundary of its contract, favoring fixture-backed or controlled integration tests over assertions about internal helper behavior.
- The update applier should be tested against disposable repository fixtures to confirm that planned updates are applied only when eligible and that summaries reflect actual changes.
- The CLI layer should have end-to-end command tests that validate exit codes, non-interactive behavior, and user-facing output for common flows.
- Since the repo currently has no implementation prior art, the initial testing style should establish the standard:
  - table-driven unit tests for policy and config behavior
  - fixture-based integration tests for workspace discovery and apply flows
  - thin command-level end-to-end tests for CLI contracts

## Out of Scope

- Supporting ecosystems other than Go in v1
- Automatic pull request creation, branch management, or git commits
- Vulnerability scanning or security advisory interpretation beyond the quarantine rule
- Rich policy types beyond allow lists, deny lists, and pinned dependencies
- Special handling that treats patch, minor, and major updates differently
- Interactive TUI or wizard flows
- Dependency changelog summarization or release note generation
- Automatic test execution after updates
- Automatic rollback when an updated project later fails its own tests or builds
- Remote service hosting; the tool is a local CLI first

## Further Notes

- The BRD describes a broader multi-ecosystem direction, so the main architectural risk is accidentally baking Go-specific assumptions into the core planner and policy layers. The PRD intentionally treats ecosystem support as a plugin-style boundary from the start.
- Command naming, exact output format, and exact exit-code mapping can be finalized during implementation, but the separation between preview and apply must not change.
- If release-date data proves difficult for some Go module sources, the implementation should favor explicit blockage and transparent reporting instead of inventing dates or bypassing quarantine.
