# Progress Visibility PRD

## Problem Statement

`updtr` can spend noticeable time planning and applying dependency updates, especially when a repository has multiple targets or when the Go toolchain and metadata lookups take longer than expected. Today the tool is mostly silent while work is in progress and then prints a final report only after the run finishes. From the user's perspective, that can look indistinguishable from a hang.

The current behavior is especially weak for unattended-but-observed runs, where a maintainer watches the command in a terminal and needs reassurance that `updtr` is actively working through the configured targets. The tool does eventually render the final result, but it does not communicate start, progress, or completion of long-running target work. That creates uncertainty, makes debugging slow runs harder, and reduces confidence in the product.

The user needs `updtr detect` and `updtr apply` to communicate visible forward progress while the run is happening, using simple append-only log lines that work the same way in local terminals and CI logs. The first version should stay intentionally simple: always on, coarse, stdout-only, and compatible with the existing final summary output.

## Solution

Add always-on coarse progress logging for both `detect` and `apply`. The new output should emit append-only lines to `stdout` when work starts on a target and when that target finishes, including elapsed time so the user can see how long each target took. The final rendered report can remain after these progress lines so existing end-of-run detail is preserved.

The progress model should be target-scoped rather than dependency-scoped in the first version. That keeps the output easy to scan, minimizes churn in tests and output contracts, and addresses the core user problem: the tool should make it obvious that it is actively moving through targets instead of appearing stuck.

The implementation should introduce a narrow progress-reporting abstraction so orchestration can report lifecycle events without depending directly on formatting rules. The CLI layer can own the concrete formatting and output destination, while the orchestrator emits semantic events such as target start and target finish. This keeps the progress behavior testable and leaves room for richer UX later without forcing a redesign of planning or rendering.

## User Stories

1. As a maintainer, I want `updtr detect` to print visible progress while it is running, so that I can tell the tool is actively working.
2. As a maintainer, I want `updtr apply` to print visible progress while it is running, so that I can distinguish a real hang from a slow update.
3. As a maintainer, I want progress output to be always enabled, so that I do not need to remember extra flags to understand what the tool is doing.
4. As a maintainer, I want progress output to use plain append-only log lines, so that the logs are readable in terminals, CI systems, and copied command output.
5. As a maintainer, I want progress output for each configured target, so that I can see which target is currently being processed.
6. As a maintainer, I want a visible completion line for each target, so that I can tell that the run is moving forward.
7. As a maintainer, I want elapsed time shown for finished targets, so that I can identify unusually slow targets.
8. As a maintainer, I want the existing final detect report to remain available, so that I still get the full dependency decision summary after progress logging is added.
9. As a maintainer, I want the existing final apply report to remain available, so that I still get the full applied-update summary after progress logging is added.
10. As a developer, I want progress lines to go to `stdout`, so that the full run transcript is available in one output stream.
11. As a developer, I want target progress to appear before the final rendered report, so that the output reads in chronological order.
12. As a developer, I want the first version to stay coarse, so that the tool does not flood logs with per-module noise.
13. As a developer, I want target start and finish logs for both successful and failed targets, so that failures do not look like silent aborts.
14. As a developer, I want progress logging to work the same way in CI and local terminals, so that there is one predictable output contract.
15. As a developer, I want progress logging to avoid spinners and live-updating lines, so that copied logs remain readable and deterministic.
16. As a developer, I want the first version to avoid heartbeat messages, so that long targets do not produce repetitive noise.
17. As a developer, I want progress output to remain understandable when only one target is configured, so that even simple repositories benefit from the feature.
18. As a CI engineer, I want append-only logs only, so that CI systems do not render broken spinner characters or overwritten lines.
19. As a CI engineer, I want progress logging to remain enabled by default in non-interactive environments, so that slow jobs are easier to inspect after the fact.
20. As a CI engineer, I want target durations to be visible in logs, so that I can spot regressions in runtime behavior over time.
21. As a future maintainer, I want progress reporting separated from dependency planning logic, so that UX changes do not spread through the planner.
22. As a future maintainer, I want tests that lock the progress output contract at the target lifecycle level, so that accidental output regressions are caught early.
23. As a future maintainer, I want the first implementation to preserve the final renderer as much as possible, so that the feature can land with low product risk.
24. As a future maintainer, I want the event model to be narrow and semantic, so that richer formatting can be added later without changing orchestration behavior again.
25. As a maintainer, I want a failed target to still emit a finish line with its duration and outcome, so that the point of failure is visible immediately.

## Implementation Decisions

- Add progress visibility for both `detect` and `apply`.
- Keep progress output always enabled in the first version. Do not add a flag to disable or enable it.
- Use only append-only text lines. Do not use spinners, carriage-return updates, progress bars, or ANSI-only UX.
- Write progress lines to `stdout`.
- Keep the existing final end-of-run report for detect and apply. The new progress lines should appear before the existing rendered summary.
- Scope progress reporting to target lifecycle events in the first version.
- Emit at least two target-level events: target started and target finished.
- Include elapsed time in the target finished event.
- Represent target progress as semantic events or callbacks emitted by orchestration, rather than hard-coding formatted strings inside the planning logic.
- Let the CLI layer own concrete progress formatting and output writing, because it already owns command setup and stream selection.
- Keep the progress abstraction narrow. The first version only needs enough event data to identify the command mode, target, outcome, and elapsed duration.
- Ensure target finish events are emitted for both success and failure paths.
- Preserve chronological append-only output. Do not buffer progress lines until the end of the run.
- Avoid per-dependency progress in the first version. The goal is reassurance and coarse observability, not detailed tracing.
- Avoid run-level progress lines such as `loading config` or `checking prerequisites` in the first version.
- Do not change dependency planning rules, candidate selection, policy evaluation, or mutation behavior as part of this feature.
- Do not split output between `stdout` and `stderr` in the first version.
- Keep elapsed time formatting simple and stable enough for human reading and deterministic test assertions.
- Favor a progress reporter module with a small interface over threading raw writers through every layer.
- Preserve sequential target processing. This feature should describe current progress, not introduce concurrency.

## Testing Decisions

- Good tests should assert externally visible behavior: which progress lines appear, when they appear relative to the final report, whether elapsed time is surfaced, and whether success and failure paths both produce understandable output. They should not assert incidental helper internals.
- Add CLI-focused tests proving `detect` emits target start and finish lines before the final detect report.
- Add CLI-focused tests proving `apply` emits target start and finish lines before the final apply report.
- Add tests proving target finish output includes elapsed time in a stable human-readable form.
- Add tests proving a target that ends with a planning or apply error still emits the expected finish line and then contributes to the final error summary.
- Add orchestrator tests proving the target lifecycle events fire once per selected target and in deterministic target order.
- Add tests proving selected-target runs emit progress only for the selected targets.
- Add regression tests proving the existing final render contract remains unchanged after the progress lines are stripped or ignored.
- Add tests for zero eligible updates and zero applied updates, proving that target progress still appears even when the final summary is effectively a no-op.
- Use injected time or a clock abstraction as prior art for deterministic duration assertions rather than relying on wall-clock sleeps.
- Keep renderer tests focused on the final rendered report unless renderer behavior is intentionally extended. Most progress contract tests should live around CLI and orchestration boundaries.

## Out of Scope

- Per-module progress output.
- Heartbeat messages such as `still working`.
- TTY-specific behavior.
- Spinners, progress bars, or live-updating single-line output.
- Colored output or ANSI styling requirements.
- Timestamps on every progress line.
- JSON progress events or machine-readable streaming output.
- A `--verbose`, `--progress`, or `--quiet` flag for this first version.
- Moving progress lines to `stderr`.
- Run-level lifecycle logs such as config load or prerequisite checks.
- Parallel target execution.
- Changing the final detect or apply summary format beyond prepending progress lines.
- Performance optimizations unrelated to progress visibility.

## Further Notes

- The key product rule is that `updtr` should never appear inert while it is actively processing targets. Target start and finish logs are the minimum viable contract that solves that problem without overcomplicating the UX.
- The strongest architectural choice is to make orchestration produce semantic progress events and let the CLI render them. That keeps the domain logic focused on work and the CLI focused on presentation.
- If future iterations need richer visibility, the most natural next steps would be optional run-level events, per-module apply events, or alternate renderers. The first version should not pre-commit to those features in its user-facing contract.
