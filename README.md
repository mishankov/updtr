# updtr

`updtr` is a policy-driven CLI for detecting and applying Go dependency updates across one or more modules in a repository.

It is built for teams that want dependency updates to follow explicit rules instead of "always take latest". Today, `updtr` supports Go modules and can:

- discover module targets and bootstrap `updtr.yaml`
- detect eligible updates without mutating the repo
- apply eligible updates and run `go mod tidy`
- quarantine freshly released versions
- restrict updates with allow-lists, deny-lists, and pinned versions
- opt into indirect dependency updates
- limit updates to vulnerability-remediating versions

## Installation

Install from source with Go:

```bash
go install github.com/mishankov/updtr@latest
```

Prebuilt binaries are published on the [GitHub Releases](https://github.com/mishankov/updtr/releases) page for tagged versions.

## Requirements

- Go 1.25+
- a directory containing one or more Go modules

## Quick start

Generate an initial config from the current repository:

```bash
updtr init
```

Detect updates:

```bash
updtr detect
```

Apply eligible updates:

```bash
updtr apply
```

Limit execution to a specific target:

```bash
updtr detect --target root
updtr apply --target root
```

Use a non-default config path:

```bash
updtr detect --config ./configs/updtr.yaml
```

## GitHub Action

`updtr` also ships as a Docker-based GitHub Action for scheduled or on-demand dependency updates.

```yaml
name: dependency-updates

on:
  workflow_dispatch:
  schedule:
    - cron: "0 6 * * 1"

permissions:
  contents: write
  pull-requests: write

jobs:
  updtr:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Run updtr
        uses: mishankov/updtr@v1
        with:
          github-token: ${{ github.token }}
```

Notes:

- `actions/checkout` is required. The action runs inside the checked-out repository and does not clone on its own.
- The default `actions/checkout` fetch configuration is sufficient; the action resolves the managed branch state before pushing repeat updates.
- `contents: write` and `pull-requests: write` are only needed for changed runs that push the managed branch and create or update a PR.
- The action treats no-op runs as success and emits structured outputs through `GITHUB_OUTPUT`.

Supported inputs:

- `config`: path to `updtr.yaml` or `updtr.yml`; when omitted, the action uses the CLI default resolution and falls back from `updtr.yaml` to `updtr.yml`
- `targets`: comma- or newline-separated target names mapped to repeatable `--target`
- `base-branch`: optional PR base branch override; when set, the action fetches and runs from `origin/<base-branch>` instead of the workflow checkout ref
- `commit-message`: optional commit message override
- `pull-request-title`: optional pull request title override
- `github-token`: token used for pull request create-or-update API calls

Outputs:

- `changed`: `true` when repository files changed after `updtr apply`
- `committed`: `true` when the action committed and pushed the managed branch
- `branch`: deterministic managed branch name for changed runs
- `pull_request_operation`: `none`, `created`, or `updated`
- `pull_request_number`: PR number for created or updated runs
- `pull_request_url`: PR URL for created or updated runs

## Configuration

By default, `updtr` reads `updtr.yaml` from the current working directory. `updtr.yml` is also supported.

Example:

```yaml
policy:
  quarantine_days: 7
  update_mode: vulnerability_only
  allow:
    - github.com/example/safe-lib
  deny:
    - github.com/example/do-not-touch
  pin:
    github.com/example/pinned-lib: v1.4.2

targets:
  - name: "root"
    ecosystem: "go"
    path: "."

  - name: "tools-cli"
    ecosystem: "go"
    path: "./tools/cli"
    include_indirect: true
    quarantine_days: 3
    update_mode: normal
```

### Top-level policy

`policy` provides defaults inherited by all targets unless overridden:

- `quarantine_days`: block versions released too recently
- `update_mode`: `normal` or `vulnerability_only`
- `allow`: optional allow-list of module paths
- `deny`: optional deny-list of module paths
- `pin`: exact versions that must remain in place

### Targets

Each target defines one Go module to inspect:

- `name`: stable identifier used by `--target`
- `ecosystem`: currently only `"go"`
- `path`: module path relative to the config file
- `include_indirect`: include explicitly listed indirect requirements

Targets can also override any policy field locally.

## Commands

```text
updtr init
updtr detect
updtr apply
updtr version
```

Run `updtr <command> --help` for command-specific flags.
