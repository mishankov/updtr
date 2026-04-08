# Plan: Progress Visibility

> Source PRD: [plans/0006. progress visibility/prd.md](/Users/mishankov/dev/updtr/plans/0006.%20progress%20visibility/prd.md)

## Architectural decisions

Durable decisions that apply across all phases:

- **Output model**: progress output is always enabled, append-only, human-readable, and written to `stdout`.
- **Progress granularity**: the first version reports only target lifecycle events. It does not report per-module work, heartbeats, or run-level lifecycle steps.
- **Presentation split**: orchestration emits semantic target progress events while the CLI owns formatting and output writing.
- **Final report**: the existing detect/apply summary remains at the end of the run. Progress lines appear before it in chronological order.
- **Outcome visibility**: target finish output includes elapsed time and a visible success or failure outcome.
- **Execution model**: targets continue running sequentially in deterministic order. This feature adds observability, not concurrency.
- **Testing focus**: most regression coverage should sit at the CLI and orchestrator boundaries where the output contract is observable. Renderer tests should stay focused on the final summary unless rendering behavior changes intentionally.

---

## Phase 1: Target Progress Skeleton

**User stories**: 1, 2, 3, 4, 5, 6, 14, 15, 18, 19, 21, 24

### What to build

Build the first end-to-end progress path for both `detect` and `apply`. A run should emit append-only lines when each selected target starts and when each selected target finishes, while keeping the final summary output intact. The implementation should introduce the durable progress-reporting boundary between orchestration and CLI formatting.

### Acceptance criteria

- [ ] `detect` emits a visible target start line for each selected target.
- [ ] `detect` emits a visible target finish line for each selected target.
- [ ] `apply` emits a visible target start line for each selected target.
- [ ] `apply` emits a visible target finish line for each selected target.
- [ ] Progress output is enabled by default with no extra flag.
- [ ] Progress output uses append-only text lines and does not rely on spinner or carriage-return behavior.
- [ ] Progress output is written to `stdout`.
- [ ] Progress events are emitted in the same deterministic order as target execution.
- [ ] A narrow progress-reporting abstraction exists between orchestration and CLI presentation.
- [ ] Tests prove target start and finish output for both commands and deterministic ordering across multiple targets.

---

## Phase 2: Elapsed Time and Outcome Semantics

**User stories**: 7, 13, 20, 25

### What to build

Make target completion output operationally useful by attaching stable elapsed time and explicit outcome information to each finish line. This slice should make it obvious when a target was slow and whether it completed successfully or with an error, without changing planning or apply behavior.

### Acceptance criteria

- [ ] Every target finish line includes elapsed time in a stable human-readable format.
- [ ] Every target finish line includes an explicit success or failure outcome.
- [ ] Failed targets still emit a finish line before the run proceeds or terminates.
- [ ] Planning failures and apply failures are both reflected in target completion output.
- [ ] Duration reporting is deterministic in tests through injected time or equivalent control, not wall-clock sleeps.
- [ ] Tests prove elapsed time appears on finish lines for successful and failed targets.

---

## Phase 3: Final Output Compatibility

**User stories**: 8, 9, 10, 11, 17, 23

### What to build

Harden the combined output contract so progress lines and the existing final summary coexist cleanly. A user should see chronological target lifecycle logs during the run and then the unchanged detect/apply summary afterward, all on `stdout` and still readable for single-target and multi-target repositories.

### Acceptance criteria

- [ ] Progress lines appear before the final detect summary.
- [ ] Progress lines appear before the final apply summary.
- [ ] The existing final detect report remains unchanged after the new progress lines are ignored or removed.
- [ ] The existing final apply report remains unchanged after the new progress lines are ignored or removed.
- [ ] Single-target runs remain readable and still benefit from visible start and finish output.
- [ ] The command does not split progress and summary across `stdout` and `stderr`.
- [ ] Tests prove chronological ordering and final-summary compatibility for both commands.

---

## Phase 4: Regression and Selection Coverage

**User stories**: 12, 16, 22

### What to build

Lock in the deliberate limits of the first version. The output contract should stay coarse and predictable across selected-target runs, no-op runs, and error paths, with tests that prevent accidental expansion into per-module, heartbeat, or TTY-specific behavior.

### Acceptance criteria

- [ ] Selected-target runs emit progress only for the selected targets.
- [ ] Runs with zero eligible updates or zero applied updates still emit target progress lines.
- [ ] No heartbeat or periodic `still working` messages are emitted.
- [ ] No per-module progress lines are emitted.
- [ ] No TTY-specific formatting behavior is required for the output contract.
- [ ] Deterministic output ordering is preserved in no-op and failure scenarios.
- [ ] Tests cover selected-target runs, no-op runs, and the absence of out-of-scope progress behaviors.
