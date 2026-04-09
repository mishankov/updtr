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

- Go 1.23+
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
