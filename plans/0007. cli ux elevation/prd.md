# CLI UX Elevation PRD

## Problem Statement

`updtr` already reports coarse target lifecycle progress and a plain text final summary, but the current CLI experience still feels rough during real runs. From the user's perspective, the tool is technically functional yet visually underpowered: long-running commands still create uncertainty, live feedback is minimal, and the final report is harder to scan than it needs to be.

The lack of polish matters in two separate ways. First, during slower `detect` and `apply` runs, the user wants ongoing reassurance that the command is still advancing through both targets and dependency work inside those targets. Second, when the run finishes, the user wants the results presented in a denser, more structured table rather than a sequence of textual sections and bullet lines.

The user needs `updtr` to feel like a modern terminal tool: rich interactive progress in real TTY sessions, clear counts for targets and dependencies, tasteful colors and working animations when appropriate, and a single end-of-run table that is easier to read at a glance. At the same time, the tool must remain safe and predictable in non-interactive environments by respecting `NO_COLOR`, disabling animation outside TTYs, and preserving an ASCII-safe fallback.

## Solution

Add a second-generation CLI presentation layer with two operating modes. In interactive TTY sessions, `updtr` should render live progress with target counts, dependency counts within targets, lightweight ASCII-safe animations, and restrained color styling when color is allowed. In non-interactive or color-disabled environments, `updtr` should fall back to deterministic append-only text output without animation, preserving readability in CI logs and copied transcripts.

The progress model should expand beyond target start and finish events. The orchestration layer should continue to emit semantic lifecycle events, but it should now provide enough structured progress data for the CLI to render both target-level and dependency-level advancement. That allows the UX to show concrete progress such as target position within the run and dependency decision progress within the active target, rather than only start and finish markers.

The final report should move to a single consolidated table. Each row should represent one dependency decision or applied update, with extra synthetic rows allowed for target-scoped warnings or errors that do not map cleanly to a single dependency. This keeps the final output compact while preserving the full operational picture in one place.

The implementation should preserve the separation of concerns introduced by the first progress feature. Orchestration should produce semantic events and structured run data, while CLI-facing presentation modules should decide whether to render an interactive experience or a plain fallback. Table row normalization should be isolated from terminal rendering concerns so the same result model can support multiple output modes without duplicating business logic.

## User Stories

1. As a maintainer, I want `updtr detect` to feel active while it runs, so that I do not mistake a slow run for a stuck process.
2. As a maintainer, I want `updtr apply` to show visible work in progress, so that I can trust the command is moving through planned updates.
3. As a maintainer, I want to see how many targets have been processed out of the total selected targets, so that I understand overall run progress.
4. As a maintainer, I want to see dependency progress inside the current target, so that I understand whether the tool is still evaluating or applying work.
5. As a maintainer, I want progress counts to be concrete, such as `2/4 targets` and `5/35 dependencies checked`, so that progress is easy to interpret.
6. As a maintainer, I want richer progress only in interactive terminals, so that CI logs stay clean and deterministic.
7. As a maintainer, I want subtle animation in TTY mode, so that the tool feels alive without becoming noisy.
8. As a maintainer, I want colors to improve scanability, so that important states stand out while I watch the run.
9. As a maintainer, I want the tool to respect `NO_COLOR`, so that it behaves correctly in environments where color is disabled by policy or preference.
10. As a maintainer, I want an ASCII-safe experience, so that the output remains compatible with conservative terminals and copied logs.
11. As a maintainer, I want the final output to be a table, so that I can scan many dependency outcomes faster.
12. As a maintainer, I want the final table to include target, module, version, status, reason, and vulnerability context, so that I get the important information in one place.
13. As a maintainer, I want applied updates and blocked decisions to appear in the same final table, so that I can review the whole run without reading multiple sections.
14. As a maintainer, I want warnings and errors that are not tied to one dependency to still appear in the table, so that the report stays consolidated.
15. As a maintainer, I want final rows to remain understandable when no update was applied, so that detect-only runs still produce a useful table.
16. As a maintainer, I want the output to remain readable for single-target repositories, so that the richer UX still helps on smaller projects.
17. As a maintainer, I want the output to remain readable for multi-target repositories, so that larger runs do not become visually overwhelming.
18. As a CI engineer, I want non-TTY runs to avoid live-updating output, so that logs remain chronological and copyable.
19. As a CI engineer, I want non-interactive runs to avoid animation entirely, so that build logs do not fill with spinner noise.
20. As a CI engineer, I want color-disabled runs to remain structurally clear, so that loss of styling does not make results ambiguous.
21. As a developer, I want terminal capability detection to be centralized, so that color and animation policy stay consistent across commands.
22. As a developer, I want progress events to remain semantic rather than presentation-specific, so that CLI UX can evolve without leaking rendering concerns into orchestration.
23. As a developer, I want dependency-level progress events to be explicit, so that the CLI never has to infer live counts from incomplete state.
24. As a developer, I want final table rows to be derived from a normalized result model, so that rendering logic stays shallow and testable.
25. As a developer, I want synthetic table rows for target warnings and errors, so that exceptional run outcomes fit the same report contract.
26. As a developer, I want the plain fallback renderer to remain available, so that the tool still behaves predictably outside rich terminals.
27. As a developer, I want interactive rendering isolated behind a small interface, so that library choice or terminal mechanics do not bleed into command orchestration.
28. As a future maintainer, I want progress and result presentation covered by contract-style tests, so that UX regressions are caught early.
29. As a future maintainer, I want stable behavior when `NO_COLOR` is set, so that future polishing does not quietly break accessibility or automation expectations.
30. As a future maintainer, I want the richer UX to preserve existing execution semantics, so that presentation improvements do not change planning or apply behavior.
31. As a user watching a long detect run, I want the active target to be obvious, so that I know where time is being spent.
32. As a user watching a long apply run, I want to distinguish planning progress from mutation progress, so that I can tell whether the tool is still analyzing or actively changing files.
33. As a user reviewing the final table, I want status labels to be visually distinct, so that applied, eligible, blocked, warning, and error outcomes are easy to separate.
34. As a user reviewing the final table, I want vulnerability context surfaced inline when present, so that security-relevant rows stand out without a second report.
35. As a user reviewing a failed run, I want the table to still be complete enough to diagnose what happened, so that a partial failure does not erase useful context.

## Implementation Decisions

- Add a richer CLI presentation layer for both `detect` and `apply`.
- Keep presentation mode environment-sensitive: use interactive live rendering only when stdout is a TTY and animation is allowed; otherwise use deterministic append-only output.
- Respect `NO_COLOR` globally. When `NO_COLOR` is present, disable color styling even in interactive terminals.
- Keep all styling ASCII-safe. Do not require Unicode glyphs for progress, status markers, borders, or animation frames.
- Extend the semantic progress model to include both target-level and dependency-level advancement.
- Preserve target lifecycle events and add explicit dependency progress events instead of deriving counts heuristically in the CLI.
- Represent overall progress as both target position within the selected run and dependency position within the active target.
- Distinguish analysis and mutation progress during `apply`, so the user can tell whether the tool is planning or applying updates.
- Preserve the separation of concerns where orchestration emits semantic events and the CLI owns output strategy and formatting.
- Introduce a terminal capability and style policy module that decides whether color, animation, and live updates are enabled.
- Introduce an interactive progress presenter responsible for live TTY rendering, working animations, and progress summaries.
- Keep a plain fallback presenter responsible for append-only, CI-safe output.
- Replace the existing section-based final summary with a single consolidated table for the richer UX version.
- Define one primary table row as one dependency decision or one applied update.
- Allow synthetic rows in the same table for target warnings and target errors that do not belong to a specific dependency.
- Normalize run results into presentation rows before rendering so table rendering is independent from orchestration internals.
- Include at least the following logical columns in the final table: target, module, from version, to version, status, reason, and vulnerability context.
- Keep status taxonomy explicit and finite, covering at least eligible, blocked, applied, warning, and error outcomes.
- Allow some cells to be empty when a column does not apply to a row type, rather than splitting the report into multiple tables.
- Preserve ASCII-safe table output in fallback mode. If a dependency is added for richer terminal rendering, it must still support the plain ASCII contract.
- It is acceptable to add a terminal rendering dependency if it materially improves the interactive UX and does not compromise fallback behavior.
- Do not change dependency planning rules, candidate selection, policy semantics, target selection, or apply ordering as part of this feature.
- Do not introduce TTY-only semantics that alter what work gets done. TTY detection should affect presentation only.
- Do not require a separate feature flag for the initial rollout; the richer behavior should activate automatically when terminal capabilities permit.
- Preserve a comprehensible non-interactive transcript even if the interactive renderer is more visually sophisticated in TTY mode.

## Testing Decisions

- Good tests should assert externally visible behavior and stable presentation contracts: which progress states appear, when live rendering is enabled or disabled, how rows are normalized, and whether the final table communicates the right outcomes. They should not overfit internal terminal implementation details.
- Add tests for terminal capability and style policy, including TTY versus non-TTY behavior, `NO_COLOR` handling, and ASCII-safe fallback selection.
- Add tests for semantic progress event plumbing, proving target-level and dependency-level progress are emitted with deterministic counts and ordering.
- Add tests for `apply` phase semantics, proving the presentation can distinguish planning progress from mutation progress.
- Add tests for interactive presenter state transitions, focusing on externally meaningful rendered states rather than incidental frame timing internals.
- Add tests for the plain fallback presenter, proving non-TTY output remains append-only, animation-free, and readable.
- Add tests for result row normalization, proving eligible, blocked, applied, warning, and error cases all map to the expected single-table row model.
- Add tests proving target-scoped warnings and errors produce synthetic rows in the final table.
- Add tests for final table rendering that lock column ordering, status labels, empty-cell behavior, and ASCII-safe formatting.
- Add tests proving vulnerability context is rendered inline when present for both decision rows and applied rows.
- Add tests for mixed runs containing multiple targets, blocked decisions, applied updates, warnings, and errors, proving the final table remains coherent in one report.
- Add tests for single-target no-op runs, proving the final output stays useful even when no updates are applied.
- Add tests for detect and apply command integration, proving the CLI selects the correct presenter based on terminal capabilities and still emits a complete final report.
- Use existing CLI, renderer, and orchestrator contract tests as prior art for deterministic output assertions and fake-time control where elapsed progress semantics matter.

## Out of Scope

- Changing dependency policy semantics, eligibility rules, quarantine behavior, or vulnerability resolution.
- Changing target execution order or introducing parallel execution.
- Adding non-ASCII-only visual polish such as box-drawing characters, emoji, or Unicode spinners.
- Building a full-screen terminal UI with panes, keyboard input, or alternate-screen behavior.
- Adding machine-readable streaming output formats such as JSON progress events.
- Introducing configurable themes, custom palettes, or user-defined animation styles in the first version.
- Redesigning configuration, initialization, or non-CLI product surfaces.
- Adding presentation-only flags unless implementation pressure makes one necessary later.
- Changing the underlying run result semantics solely to beautify the table beyond what semantic progress and normalized row mapping require.

## Further Notes

- The core product balance is polish without fragility. Interactive terminals should get a richer, more modern experience, but CI and copied logs must remain boring, stable, and readable.
- The strongest architectural move is to keep three layers distinct: semantic progress production, terminal capability policy, and concrete presentation. That separation allows `updtr` to evolve its UX without turning orchestration into a terminal renderer.
- The deepest reusable module in this design is the result-to-row normalization layer. If it has a small stable interface, both the interactive and fallback renderers can depend on it while tests validate report semantics independently of terminal mechanics.
- A terminal library may be worthwhile, but it should be adopted only if it supports strict fallback behavior and does not force alternate-screen or Unicode-centric assumptions onto the default experience.
