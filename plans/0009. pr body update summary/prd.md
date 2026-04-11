# GitHub Action PR Body Update Summary PRD

## Problem Statement

`updtr` already has a working GitHub Action that can apply dependency updates, push a managed branch, and create or update a pull request. That action does not currently explain what changed in the pull request body. New pull requests are created with a fixed placeholder body, and repeated runs only refresh the title.

For maintainers using the action, that leaves the pull request itself missing the most important review context: which targets changed, which dependencies moved from which version to which version, whether updates were direct or indirect, and whether any vulnerability-remediating updates were included. Reviewers have to open diffs or re-run tooling mentally to understand the update. That increases review friction and makes the automation look incomplete even when the branch contents are correct.

The user wants the GitHub Action to place useful update information directly in the pull request body so the PR explains itself.

## Solution

Extend the GitHub Action so created and updated pull requests include a deterministic Markdown body generated from the actual `updtr apply` result. The body should summarize the update run in reviewer-friendly language and list the applied dependency changes, grouped by target, with enough detail to understand the branch without digging into raw diffs first.

The action should continue to be a thin integration layer around `updtr`, but it should stop treating `updtr apply` as a write-only side effect. Instead, the action should obtain structured update results from `updtr`, convert them into a pull-request-body view model, and send that Markdown body on both PR creation and PR update.

The first version should stay small and opinionated:

- the body is generated automatically from the actual applied updates
- repeated action runs refresh the body so it matches the latest managed branch state
- the body favors concise Markdown summaries over raw command output
- no new mandatory action inputs are required for basic use
- no PR body is generated for no-op runs because no PR should exist in that case

## User Stories

1. As a repository maintainer, I want update PRs to explain what changed in the body, so that I can review dependency updates without reconstructing them from the diff alone.
2. As a repository maintainer, I want the PR body to list each updated dependency, so that I can quickly scan the scope of the change.
3. As a repository maintainer, I want the PR body to show previous and new versions, so that I can judge the size of each bump.
4. As a repository maintainer, I want updates grouped by configured target, so that multi-module repositories remain readable.
5. As a repository maintainer, I want the body to distinguish direct and indirect dependency updates, so that I can prioritize review attention.
6. As a repository maintainer, I want vulnerability-remediating updates to be visible in the PR body, so that security-driven changes are obvious during review.
7. As a repository maintainer, I want advisory identifiers surfaced when available, so that I can connect the PR to known security findings.
8. As a repository maintainer, I want the PR body to be generated from the actual applied updates, so that the summary matches what landed on the branch.
9. As a repository maintainer, I want the PR body to update on reruns, so that the open PR stays accurate as new eligible updates are added.
10. As a repository maintainer, I want the PR body format to be deterministic, so that repeated runs do not create noisy body churn.
11. As a repository maintainer, I want the PR body to remain concise when many modules change, so that the summary stays useful instead of becoming a dump of logs.
12. As a repository maintainer, I want warnings from the update run reflected when relevant, so that reviewers notice cases like additional direct changes.
13. As a repository maintainer, I want the PR body to avoid including transient command output, so that the content stays stable and reviewable.
14. As a CI engineer, I want PR creation and PR update paths to send the same body-generation contract, so that behavior is consistent across first and repeated runs.
15. As a CI engineer, I want no-op runs to stay quiet and successful, so that the new body feature does not change the existing no-op contract.
16. As a CI engineer, I want failed update runs to avoid publishing misleading summaries, so that PR bodies are only written for successful changed runs.
17. As a CI engineer, I want the action to avoid scraping human-readable terminal output, so that the feature is resilient to future CLI formatting changes.
18. As a developer of `updtr`, I want the action to consume structured result data from the update engine, so that rendering a PR body does not duplicate update-detection logic.
19. As a developer of `updtr`, I want the PR body renderer to be a narrow, testable module, so that formatting can evolve without changing action orchestration.
20. As a future maintainer, I want body generation to be deterministic and easily fixture-tested, so that formatting regressions are cheap to catch.
21. As a future maintainer, I want the action’s GitHub client to update both title and body together, so that create and update behavior stay aligned.
22. As a future maintainer, I want the public action interface to remain small, so that adding PR-body summaries does not expand into an input matrix prematurely.
23. As a reviewer, I want a summary section with counts, so that I can quickly understand how many targets and updates are involved.
24. As a reviewer, I want per-target sections listing applied changes, so that I can jump to the part of the repo I own.
25. As a reviewer, I want empty or blocked targets excluded from the PR body summary of applied changes, so that the body focuses on what actually changed.
26. As a reviewer, I want the PR body wording to make it clear that the branch was generated by `updtr`, so that the automation source is obvious.

## Implementation Decisions

- The feature is a follow-up to the existing GitHub Action and does not change the action’s core branch-management or PR-lifecycle behavior.
- The PR body should be built from structured apply results produced by `updtr`, not from git diff parsing and not from scraping rendered terminal tables.
- The action runtime should gain access to the applied-update result model for changed runs, including target names, applied module updates, vulnerability metadata, and warnings.
- The action should continue to decide whether a PR exists based on repository changes and branch management, but PR body content should come from the update result model rather than from git state.
- The pull request request model should include both title and body so the GitHub client can create and update the same reviewer-facing content contract.
- Pull request updates should refresh the body as well as the title, so repeated runs keep the open PR accurate.
- The PR body should be deterministic Markdown with a stable section order and stable ordering of updates inside each target.
- The PR body should contain a short summary section with counts such as number of targets changed and number of applied updates.
- The PR body should contain per-target sections that list applied updates with module path, version transition, and dependency relationship.
- When vulnerability metadata is attached to an applied update, the body should surface that context in a concise form, including advisory identifiers when available.
- When the update run produces action-relevant warnings, the body should surface them in a reviewer-readable warning section instead of burying them in logs.
- The first version should describe only applied updates, not blocked candidates, because the pull request branch represents applied repository state rather than the full planning result.
- The first version should not require new user inputs for templating or body customization. The action stays opinionated and generates one built-in Markdown format.
- The body renderer should be separated from GitHub API transport so formatting logic can be tested independently from HTTP behavior.
- The action should write PR bodies only for successful changed runs that proceed through commit and push.
- If PR creation or update fails, the action should fail explicitly rather than silently skipping body publication.
- The design should preserve the repository boundary already present in `updtr`: update planning and mutation remain in the engine; the action layer owns GitHub formatting and API publication.

### Proposed Deep Modules

- Apply result transport module: exposes structured update results from the action’s `updtr` execution path to action orchestration.
- PR body view-model module: converts raw apply results into a stable summary model organized around reviewer-facing concepts.
- PR body renderer module: renders deterministic Markdown from the summary model.
- Pull request client module: sends both title and body for create and update operations.
- Action orchestration module: decides when the body should be rendered and published in relation to change detection, commit, push, and PR lifecycle flow.

## Testing Decisions

- Good tests should verify externally observable behavior and stable contracts rather than helper internals.
- Test that created pull requests receive a generated body containing applied update details rather than the current placeholder text.
- Test that repeated runs against an existing managed PR update the body as well as the title.
- Test that the PR body renderer produces deterministic Markdown for the same apply result input.
- Test that target grouping, update ordering, direct versus indirect labeling, and summary counts are stable and predictable.
- Test that vulnerability metadata is rendered when present and omitted cleanly when absent.
- Test that warnings included in the apply result are surfaced in the PR body in a concise, readable form.
- Test that no-op runs do not attempt PR body rendering or PR API updates.
- Test that failed runs do not publish partial or misleading PR body content.
- Test that GitHub API requests for PR creation and PR update include the expected body field.
- Test the action runtime boundary that transports structured apply results from `updtr` into the PR rendering path.
- Prior art should follow the repository’s existing style of contract-heavy tests around action runtime behavior, GitHub client requests, renderer normalization, and orchestrator result modeling.

## Out of Scope

- Adding user-defined PR body templates, placeholders, or arbitrary Markdown customization.
- Posting release-note excerpts, changelog summaries, or external package metadata fetched from package hosts.
- Rendering blocked updates or full planning diagnostics in the PR body.
- Opening PRs for no-op runs.
- Changing branch naming, commit-message defaults, or token-permission requirements as part of this feature.
- Running repository test suites or policy checks and embedding their results into the PR body.
- Supporting ecosystems other than Go as part of this change.

## Further Notes

- The existing action already has most of the orchestration needed for this feature; the main missing capability is a structured bridge between `updtr apply` and PR-body generation.
- A strong default body is more important than early customization. If customization is ever added later, it should build on a stable generated summary rather than replace it entirely.
- The body should optimize for review utility, not exhaustiveness. Reviewers need a trustworthy summary of what was applied, not a replay of every command executed during the run.
