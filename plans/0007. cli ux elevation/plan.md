# Plan: CLI UX Elevation

> Source PRD: [plans/0007. cli ux elevation/prd.md](/Users/mishankov/dev/updtr/plans/0007.%20cli%20ux%20elevation/prd.md)

## Architectural decisions

Durable decisions that apply across all phases:

- **Commands**: preserve the existing `detect` and `apply` command model. This feature changes CLI presentation only and must not change planning rules, target selection, apply ordering, or exit semantics.
- **Presentation policy**: terminal capability detection is centralized in one CLI-facing policy layer that decides live updates, animation, color, and ASCII-safe fallback based on `stdout` TTY state and `NO_COLOR`.
- **Presentation split**: orchestration emits semantic progress events and structured run data; CLI presentation modules decide whether to use an interactive live presenter or a deterministic append-only fallback presenter.
- **Progress model**: semantic progress must expand beyond target start and finish to include run-level target counts, target-scoped dependency counts, and explicit `apply` stage boundaries so the CLI never infers live progress heuristically.
- **Apply stages**: `apply` progress distinguishes at least planning work from mutation work so the user can tell whether the command is still analyzing or actively changing dependencies.
- **Result normalization**: final reporting flows through a normalized row model before terminal rendering. Both interactive and fallback presentation depend on the same row contract rather than re-deriving output from orchestration internals.
- **Final report**: the end-of-run report becomes one consolidated table with a finite status taxonomy covering at least eligible, blocked, applied, warning, and error outcomes.
- **Synthetic rows**: target-scoped warnings and target-scoped errors that do not map to a single dependency are represented as synthetic rows in the same final table rather than a separate report section.
- **Output safety**: non-interactive runs remain chronological, append-only, animation-free, and readable with ASCII-safe formatting even if interactive TTY rendering uses richer live updates.
- **Execution model**: targets continue to run sequentially in deterministic order. This feature improves observability and scanability, not concurrency.
- **Testing focus**: the stable contracts sit at the terminal policy, semantic progress, normalization, and rendered-output boundaries. Tests should assert externally visible states and table semantics without overfitting incidental terminal mechanics.

---

## Phase 1: Interactive Target Progress Shell

**User stories**: 1, 2, 3, 6, 7, 8, 9, 10, 18, 19, 20, 21, 26, 27, 29, 30

### What to build

Build the end-to-end presentation split for richer CLI output. A run should select an interactive live presenter only when terminal capabilities allow it, and otherwise select a plain append-only fallback presenter. In this first slice, both `detect` and `apply` should show clear target-level progress such as current target position within the run while preserving ASCII safety and `NO_COLOR` behavior.

### Acceptance criteria

- [ ] Terminal capability and style policy are resolved in one place for both `detect` and `apply`.
- [ ] Interactive live rendering is enabled only when `stdout` is a TTY and live updates are allowed.
- [ ] `NO_COLOR` disables color styling even when interactive live rendering remains enabled.
- [ ] Non-TTY runs use append-only output with no carriage-return live updates.
- [ ] Non-interactive runs emit no animation frames or spinner noise.
- [ ] Interactive target progress shows concrete run-level target counts such as `1/4 targets`.
- [ ] The active target is always visible while a run is in progress.
- [ ] Interactive styling and animation remain ASCII-safe.
- [ ] Presenter selection affects presentation only and does not change which work is executed.
- [ ] Tests prove TTY versus non-TTY selection, `NO_COLOR` handling, ASCII-safe fallback behavior, and unchanged command semantics.

---

## Phase 2: Dependency Progress for `detect`

**User stories**: 1, 3, 4, 5, 22, 23, 31

### What to build

Extend semantic progress for `detect` so the active target also reports dependency-level evaluation progress. The user should be able to see both overall run position and concrete dependency counts inside the current target without the CLI guessing from partial planner state.

### Acceptance criteria

- [ ] The semantic progress model includes explicit dependency-level progress for `detect`.
- [ ] `detect` exposes dependency counts within the active target such as `5/35 dependencies checked`.
- [ ] The active target remains obvious while dependency progress updates.
- [ ] Dependency progress ordering is deterministic and reflects actual planner advancement.
- [ ] The CLI does not infer dependency counts from final plan state or other incomplete state.
- [ ] Fallback output remains append-only and readable while still surfacing dependency progress meaningfully.
- [ ] Tests prove dependency-progress emission, concrete count rendering, deterministic ordering, and fallback readability.

---

## Phase 3: Planning vs Mutation Progress for `apply`

**User stories**: 2, 4, 5, 22, 23, 32

### What to build

Extend the richer progress model through `apply` so users can distinguish planning work from actual mutation work. This slice should show concrete dependency progress during planning and then visible mutation progress as eligible updates are applied, without changing the underlying apply contract.

### Acceptance criteria

- [ ] The semantic progress model includes explicit `apply` stage boundaries for planning and mutation.
- [ ] Interactive `apply` progress makes it clear whether the tool is analyzing updates or actively mutating dependencies.
- [ ] `apply` progress includes concrete dependency counts during planning.
- [ ] `apply` progress includes concrete mutation counts as eligible updates are attempted or completed.
- [ ] Failed planning, failed mutation, and partial-apply cases still surface meaningful progress state before the final report.
- [ ] Fallback output remains chronological and free of live-update artifacts while preserving stage visibility.
- [ ] Tests prove planning-versus-mutation visibility, deterministic counts, and meaningful behavior across success and failure cases.

---

## Phase 4: Single Normalized Final Table

**User stories**: 11, 12, 13, 15, 16, 24, 33, 34

### What to build

Replace the section-based final summary with one consolidated table driven by a normalized row model. Each primary row should represent one dependency decision or applied update, with consistent columns and explicit status labels so detect and apply runs become easier to scan at a glance.

### Acceptance criteria

- [ ] A normalized result-row model exists between run results and terminal rendering.
- [ ] The final report is rendered as one consolidated table rather than multiple output sections.
- [ ] The table includes logical columns for target, module, from version, to version, status, reason, and vulnerability context.
- [ ] Eligible, blocked, and applied dependency outcomes appear in the same table with a finite explicit status taxonomy.
- [ ] Detect-only no-op runs still render a useful table even when no updates are applied.
- [ ] Single-target and multi-target runs remain readable with the consolidated format.
- [ ] Vulnerability context is surfaced inline when present without requiring a second report.
- [ ] Status labels remain visually distinct in interactive mode and structurally clear in fallback mode.
- [ ] Tests prove row normalization, column ordering, status labels, empty-cell behavior, vulnerability rendering, and coherent output for detect and apply.

---

## Phase 5: Warnings, Errors, and Mixed-Run Hardening

**User stories**: 14, 17, 25, 28, 35

### What to build

Finish the reporting contract by folding target-scoped warnings and errors into the same normalized table through synthetic rows, then harden the whole UX with mixed-run and regression coverage. The final output should remain complete enough to diagnose partial failures without splitting exceptional outcomes into separate report sections.

### Acceptance criteria

- [ ] Target-scoped warnings that are not tied to one dependency produce synthetic rows in the final table.
- [ ] Target-scoped errors that are not tied to one dependency produce synthetic rows in the final table.
- [ ] Mixed runs with applied updates, blocked decisions, warnings, and errors remain coherent in one final report.
- [ ] Partial failures preserve enough completed context in the final table to support diagnosis.
- [ ] Interactive and fallback presenters both consume the same normalized row contract for warning and error rows.
- [ ] Contract-style tests cover mixed multi-target runs, single-target no-op runs, warning rows, error rows, and partial-failure behavior.
- [ ] Regression tests prove richer presentation does not alter execution semantics, target ordering, or policy behavior.
