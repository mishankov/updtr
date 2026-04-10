# Plan: GitHub Action

> Source PRD: [plans/0008. github action/prd.md](/Users/mishankov/dev/updtr/plans/0008.%20github%20action/prd.md)

## Architectural decisions

Durable decisions that apply across all phases:

- **Delivery model**: the feature ships as a Docker-based GitHub Action published from this repository, with `action.yml` defining the public contract and a multi-stage `Dockerfile` providing the runtime.
- **Execution boundary**: the action runs inside the checked-out caller repository and does not clone repositories itself. Caller workflows remain responsible for `actions/checkout`.
- **Update engine**: dependency planning and mutation continue to live in `updtr`. The action is a thin GitHub integration layer that invokes `updtr apply` rather than reimplementing policy logic.
- **Config contract**: the default config path remains `updtr.yaml`, with optional override support for alternate layouts. Target narrowing maps to the existing `--target` model.
- **Change detection**: the action decides whether updates occurred by checking git-tracked repository changes after `updtr apply`, not by scraping human-readable CLI output.
- **Git integration boundary**: branch creation, staging, commit creation, pushing, and pull request orchestration live in action-owned modules separate from `updtr` planning logic.
- **Branch strategy**: managed update branches use a deterministic naming strategy so repeated runs converge on the same branch and avoid branch sprawl.
- **Commit scope**: the action stages and commits only repository changes produced by the update run, excluding unrelated workspace files.
- **Action outputs**: externally consumable status is exposed through structured outputs written to `GITHUB_OUTPUT`, including whether tracked changes were made and whether a pull request was created or updated.
- **Runtime support**: the first version targets Linux container execution on GitHub-hosted runners and is intended for reuse via a stable major tag such as `v1`.
- **Security contract**: the published workflow contract documents the minimum token permissions required, with `contents: write` and `pull-requests: write` needed only when branch push and PR creation are enabled.
- **Testing focus**: tests should emphasize stable contracts and observable behavior across metadata, runtime inputs and outputs, no-op runs, changed runs, repeated runs, and explicit failure propagation.

---

## Phase 1: Action Runtime and No-Op Contract

**User stories**: 1, 2, 4, 5, 9, 10, 14, 16, 17, 18, 19, 23, 25, 27

### What to build

Create the first end-to-end vertical slice for a reusable Docker action that can be referenced from another repository, read action inputs from the GitHub Actions environment, run `updtr apply` against the checked-out workspace, detect whether tracked dependency files changed, and emit structured outputs. In this slice, no-op runs must complete successfully and visibly without attempting any git or pull request operations.

### Acceptance criteria

- [ ] The repository contains a runnable Docker-based GitHub Action with a public metadata contract in `action.yml`.
- [ ] The action accepts a small initial input set that includes the config path with a default equivalent to `updtr.yaml`.
- [ ] The action invokes `updtr apply` against the current workspace rather than duplicating update logic.
- [ ] The action reads inputs from standard GitHub Action mechanisms and writes outputs through `GITHUB_OUTPUT`.
- [ ] When `updtr` returns an operational failure, the action fails the step clearly.
- [ ] When `updtr` produces no tracked repository changes, the action exits successfully as a no-op.
- [ ] No-op runs do not attempt branch creation, commit creation, push, or pull request creation.
- [ ] Outputs distinguish between a successful no-op and a run that produced tracked file changes.
- [ ] Tests cover the action metadata contract, input defaulting, no-op behavior, successful change detection, and failure propagation from the `updtr` execution path.

---

## Phase 2: Deterministic Branch and Safe Commit Flow

**User stories**: 6, 7, 9, 13, 22, 28

### What to build

Extend the changed-run path so the action can move from “files changed” to “managed branch prepared.” A successful update run should create or reuse the deterministic action branch, stage only the intended tracked changes from the update run, create a commit with a safe default message, and push the branch to origin.

### Acceptance criteria

- [ ] Changed runs create or reuse one deterministic managed branch rather than a timestamped branch per execution.
- [ ] The branch strategy is stable across repeated runs of the same workflow configuration.
- [ ] The action stages only the tracked repository changes intended for the dependency update commit.
- [ ] Unrelated workspace files are not included in the commit.
- [ ] The action creates a commit only when tracked dependency files changed.
- [ ] Push failures surface as explicit action failures.
- [ ] Outputs indicate that a change was committed and a managed branch was pushed when that path succeeds.
- [ ] Tests cover branch naming determinism, branch reuse, commit gating on changed files, intended staging scope, and explicit push-failure behavior.

---

## Phase 3: Pull Request Create-or-Update Flow

**User stories**: 6, 7, 8, 9, 20, 21, 22, 28, 30

### What to build

Complete the main automation loop by adding a narrow pull request integration for the managed branch. When a changed run successfully pushes the managed branch, the action should create a pull request using sensible defaults or update the existing pull request for that branch, then expose structured outputs describing the PR result.

### Acceptance criteria

- [ ] After a successful changed run and branch push, the action creates a pull request for the managed branch when none exists.
- [ ] Repeated runs against an existing managed branch update or reuse the existing pull request instead of creating duplicates.
- [ ] Pull request title and commit message have safe defaults while remaining configurable through inputs.
- [ ] The action exposes structured outputs indicating whether a pull request was created or reused.
- [ ] Pull request creation or update failures surface as explicit action failures.
- [ ] The required workflow token permissions for branch push and PR creation are documented as part of the published contract.
- [ ] Tests cover PR creation, existing-branch repeated-run behavior, configurable title and message handling, and PR-failure propagation.

---

## Phase 4: Config Path and Target Narrowing Inputs

**User stories**: 3, 11, 12, 16, 17, 26

### What to build

Add the repo-shape flexibility needed for real consumers without expanding the interface into a large matrix. The action should support an explicit config path override and target-selection input that narrows execution to selected configured targets while preserving the default full-repository behavior for multi-target repos.

### Acceptance criteria

- [ ] The action supports overriding the default config path through a documented input.
- [ ] The action supports target narrowing through a documented input model that maps cleanly to `updtr --target`.
- [ ] With no target-selection input, the action continues to cover all configured targets in the repository.
- [ ] Invalid or unknown target selections fail clearly before any git or pull request side effects occur.
- [ ] The public input contract remains intentionally small and opinionated rather than exposing internal implementation knobs.
- [ ] Tests cover default config-path behavior, explicit config-path overrides, multi-target repository execution, target narrowing, and invalid input handling.

---

## Phase 5: Repeated-Run Idempotency and Failure Hardening

**User stories**: 6, 9, 10, 19, 21, 22, 27, 28

### What to build

Harden the action so scheduled and repeated runs behave predictably under real CI conditions. This slice focuses on stable branch and PR reuse, explicit error surfaces across each external boundary, and contract tests that prove the action remains quiet on no-op runs while staying diagnosable on partial or failing runs.

### Acceptance criteria

- [ ] Repeated runs against an unchanged repository remain successful no-ops and do not create extra branches or pull requests.
- [ ] Repeated runs against a repository with new eligible updates converge on the same managed branch and pull request.
- [ ] Failures from `updtr`, git push, and pull request operations remain distinct and explicit in action results.
- [ ] The action’s outputs remain coherent across no-op, changed, repeated-run, and failure paths.
- [ ] The action does not leave duplicate PRs for the same managed branch under normal repeated-run behavior.
- [ ] Contract tests cover no-op reruns, changed reruns, existing branch and PR reuse, and explicit failures at each external integration boundary.

---

## Phase 6: Publishing and Consumer Contract

**User stories**: 1, 15, 20, 23, 24, 29, 30

### What to build

Finish the public delivery contract so other repositories can rely on the action. This includes packaging the runtime image reproducibly, wiring the repository’s release flow to publish the action surface confidently, documenting the expected workflow shape and permissions, and establishing the stable major-version consumption story.

### Acceptance criteria

- [ ] The action packaging is reproducible and minimal enough for routine CI use.
- [ ] CI builds and validates the Docker action as part of the repository’s published contract.
- [ ] Consumer-facing documentation includes example workflow usage, required checkout expectations, configurable inputs, outputs, and minimum token permissions.
- [ ] The repository’s release story supports stable major-version consumption such as `mishankov/updtr@v1`.
- [ ] Action metadata, packaging, and documentation stay aligned with the runtime behavior tested in CI.
- [ ] Tests or release validation steps cover the published metadata contract and at least one consumer-style action invocation.
