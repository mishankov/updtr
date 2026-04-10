# GitHub Action PRD

## Problem Statement

`updtr` already provides a policy-driven CLI for detecting and applying Go dependency updates from a committed `updtr.yaml` file, but teams that want unattended dependency maintenance in GitHub repositories still need to wire together installation, execution, git branch handling, and pull request creation themselves.

The user needs a reusable GitHub-native automation published from this repository that can be consumed from other repositories with a simple `uses:` reference. The automation should read an existing `updtr` config file in the target repository, evaluate and apply eligible dependency updates under that policy, and open a pull request only when the run actually changes dependency files.

## Solution

Create a Docker-based GitHub Action in this repository, implemented in Go, that is reusable from other repositories. The action will run inside GitHub Actions, accept workflow inputs such as config path and branch naming settings, execute `updtr` against the checked-out caller repository, and create or update a pull request when eligible dependency changes are produced.

The action should be designed as a thin GitHub integration layer around existing `updtr` behavior rather than as a second dependency-update engine. `updtr` remains responsible for config parsing, policy evaluation, candidate selection, and repository mutation. The new action owns GitHub Actions inputs and outputs, branch and commit handling, pull request creation behavior, and CI-friendly status reporting.

The first version should prioritize correctness, low surprise, and straightforward reuse:

- caller repositories keep policy in `updtr.yaml`
- the action runs non-interactively on GitHub-hosted Linux runners
- no pull request is opened when no tracked dependency files changed
- the action publishes a stable `v1` tag for cross-repo reuse

## User Stories

1. As a repository maintainer, I want to use `uses: mishankov/updtr@v1`, so that I can adopt dependency automation without copying scripts between repositories.
2. As a repository maintainer, I want the action to read my committed `updtr.yaml`, so that update policy stays reviewable and repo-local.
3. As a repository maintainer, I want to override the config path with an input, so that mono-repos or custom layouts are supported.
4. As a repository maintainer, I want the action to run `updtr` rather than reimplement update logic, so that CLI and action behavior stay consistent.
5. As a repository maintainer, I want the action to apply only updates that `updtr` marks as eligible, so that quarantine, allow, deny, pin, and vulnerability-only policies remain enforced.
6. As a repository maintainer, I want the action to create a pull request only when files actually changed, so that no-op runs stay quiet.
7. As a repository maintainer, I want the pull request branch name to be deterministic, so that repeated scheduled runs do not create branch sprawl.
8. As a repository maintainer, I want the pull request title and commit message to be configurable within safe defaults, so that update PRs fit repository conventions.
9. As a repository maintainer, I want the action to expose outputs indicating whether a change was made and whether a pull request was created, so that downstream workflow steps can react.
10. As a repository maintainer, I want the action to fail clearly when `updtr` encounters an operational error, so that broken automation is visible immediately.
11. As a repository maintainer, I want the action to work when my repository contains multiple configured Go targets, so that one automation run can cover the full repo.
12. As a repository maintainer, I want the action to support `--target`-style narrowing through inputs, so that workflows can update only selected modules when needed.
13. As a repository maintainer, I want the action to commit only repository changes produced by `updtr`, so that unrelated workspace files are not accidentally included.
14. As a repository maintainer, I want the action to run on `ubuntu-latest` without requiring preinstalled `updtr`, so that consumers do not need setup glue.
15. As a repository maintainer, I want published action tags such as `v1`, so that I can consume a stable major version.
16. As a CI engineer, I want action inputs to map cleanly to GitHub Actions conventions, so that workflows are readable and easy to maintain.
17. As a CI engineer, I want the action to read inputs from standard GitHub Action mechanisms, so that it behaves like other Docker-based actions.
18. As a CI engineer, I want the action to write outputs through `GITHUB_OUTPUT`, so that downstream steps can consume structured results.
19. As a CI engineer, I want the action to return a successful status for no-op runs, so that normal “nothing to update” schedules do not appear broken.
20. As a CI engineer, I want the action to require only documented token permissions, so that workflow security can be reviewed up front.
21. As a CI engineer, I want the action to handle existing open update branches predictably, so that scheduled jobs do not constantly create duplicate pull requests.
22. As a CI engineer, I want branch push and PR creation failures to surface as explicit action errors, so that permission or protection issues are diagnosable.
23. As a CI engineer, I want the action to be Linux-container based, so that GitHub can execute it directly without running Go source in the caller workflow.
24. As a developer of `updtr`, I want the action to live in this repository, so that release tags for the CLI and the action can stay aligned.
25. As a developer of `updtr`, I want the action implementation to be a thin wrapper over existing packages or the built CLI, so that business logic is not duplicated.
26. As a future maintainer, I want the action interface to stay small and opinionated, so that upgrades do not become a breaking-input matrix.
27. As a future maintainer, I want tests to cover both GitHub Action metadata and runtime behavior, so that published tags are trustworthy.
28. As a future maintainer, I want PR creation behavior to be encapsulated behind a narrow module, so that the GitHub integration can evolve without affecting update planning logic.
29. As a future maintainer, I want the Docker image build to be reproducible and minimal, so that the action remains fast and secure.
30. As a security-conscious maintainer, I want the action to document the exact permissions it needs, so that repositories can grant only `contents: write` and `pull-requests: write` when PR creation is enabled.

## Implementation Decisions

- The deliverable is a Docker-based GitHub Action published from this repository.
- The action is implemented in Go and packaged with `action.yml` plus a multi-stage `Dockerfile`.
- The action consumes standard GitHub Actions inputs, which GitHub exposes to the container as environment variables such as `INPUT_CONFIG`.
- The action writes structured outputs via the file path provided in `GITHUB_OUTPUT`.
- The action runs on the checked-out caller repository. It does not clone repositories on its own.
- Caller workflows remain responsible for `actions/checkout`.
- The action should be usable from other repositories through a stable tag such as `mishankov/updtr@v1`.
- The first version targets GitHub-hosted Linux runners only, because Docker-based actions execute in Linux containers.
- The action should default to using `updtr.yaml` unless an explicit config-path input is provided.
- The action should optionally accept a repeatable or delimited target-selection input that maps to `updtr --target`.
- The action should execute `updtr apply`, not `updtr detect`, because the decision to open a pull request depends on actual repository mutations.
- The action should determine whether updates exist by checking for git-tracked file changes after `updtr apply`, rather than by scraping human-readable CLI output.
- The action should create a deterministic update branch, commit the resulting dependency-file changes, push that branch, and then create or update a pull request.
- The default branch naming strategy should be stable across runs, for example one branch per workflow configuration rather than one branch per execution timestamp.
- The default pull request title and commit message should be sensible and automation-specific, while remaining configurable through inputs.
- The action should treat “no file changes” as a successful no-op and set outputs that allow workflows to detect that case.
- The action should fail the step when `updtr` returns an operational failure or when git push or pull request creation fails.
- The action should touch only files changed by `updtr` and the git metadata needed to create the branch and commit.
- The action should document the required token permissions and expected workflow shape for consumers.
- The action should preserve the repository boundary already established in `updtr`: update planning and application stay in `updtr`; git and pull request orchestration live in the action wrapper.

### Proposed Deep Modules

- Action input/output module: parses GitHub Action inputs, validates them, and emits stable outputs.
- Updtr runner module: invokes `updtr` with the resolved config and target options and returns a normalized execution result.
- Change detector module: determines whether the repository has relevant tracked changes after the update run.
- Git operation module: creates or reuses the update branch, stages the intended files, commits them, and pushes to origin.
- Pull request module: creates a new PR or updates the existing one for the managed branch.
- Action metadata module: owns `action.yml`, documented inputs, outputs, and runtime contract.
- Packaging module: owns the Docker build and published action image contract.

## Testing Decisions

- Good tests should verify externally observable behavior and published contracts rather than helper internals.
- Test the action metadata contract: required and optional inputs, defaults, outputs, and Docker runtime declaration in `action.yml`.
- Test input parsing and validation, including default config-path behavior, optional target selection, and invalid branch or message inputs.
- Test no-op behavior: when `updtr` makes no tracked file changes, the action exits successfully, does not push a branch, does not create a PR, and emits outputs showing no update PR was opened.
- Test change behavior: when `updtr` changes dependency files, the action creates the branch, commits the changes, pushes them, and requests PR creation.
- Test repeated-run behavior against an existing branch so the action does not create duplicate PRs unnecessarily.
- Test failure propagation from `updtr`, git push, and pull request creation.
- Test that only the intended changed files are staged and committed.
- Test Docker packaging by building the image in CI and running at least one fixture-backed invocation.
- Test the published workflow examples or fixture workflows as consumer-level integration checks.
- Prior art in this repo should continue to favor stable contract tests over implementation-detail tests, matching the existing CLI, config, orchestrator, and renderer test style.

## Out of Scope

- Rewriting `updtr` core planning or mutation logic inside the action.
- Adding ecosystems other than Go as part of this action work.
- Automatic merge, auto-approval, or branch protection bypass.
- Running repository-specific test suites, linters, or builds after updates by default.
- Dependency grouping, changelog summarization, or release-note synthesis in the first version.
- Supporting non-GitHub git forges.
- Supporting Windows or macOS as native action runtimes for the first version.
- Building a separate remote service or hosted bot.
- Replacing future reusable-workflow support; a reusable workflow may still be added later as a higher-level orchestration layer.

## Further Notes

- Generic Go-action guidance from the supplied notes is directly applicable here: Docker-based actions are the simplest way to ship a Go implementation, inputs arrive via `INPUT_*` environment variables, outputs should be written through `GITHUB_OUTPUT`, and public reuse should rely on stable major tags such as `v1`.
- Although the user-facing feature is “GitHub Action support,” the design should keep the action thin and avoid duplicating the update engine already present in this repository.
- If future consumers need more orchestration than a single action step can comfortably express, a reusable workflow can be layered on top later without invalidating this action contract.
