# push design

## Purpose

push makes small tool deployments cheap. A publisher writes build artifacts into a filesystem depot, and a lightweight agent on each host installs the newest build for that host's platform.

```text
git push -> CI builds -> depot -> push daemon installs locally
```

The design intentionally avoids a service API. The filesystem is the transport boundary.

## Components

- `build/`: shared version-stamping package used by push and consuming Go CLIs.
- `ci/`: scripts for computing versions, computing ldflags, and publishing binaries into a depot.
- `cmd/push`: CLI and daemon.
- `internal/config`: config loading, defaults, and validation.
- `internal/platform`: `GOOS-GOARCH` platform detection.
- `internal/sync`: depot reading, latest resolution, installation, manifests, and status/listing helpers.

## Depot layout

```text
{depot}/
  myapp/
    latest
    builds/
      a1b2c3d4/
        build.json
        linux-amd64/
          myapp
        windows-amd64/
          myapp.exe
```

The build directory is keyed by short commit SHA for CI builds, or by short content hash for `push vendor` builds. `latest` contains an identity string `{short}.{unix_epoch}`. The daemon extracts the short prefix and reads `{depot}/{name}/builds/{short}/{platform}/`.

`build.json` stores commit or vendor metadata used by `push list` and status output.

## Agent config

```yaml
depot: /mnt/push
install_path: ~/bin
interval: 5m

builds:
  - name: myapp

  - name: tools
    install_path: ~/opt/tools
```

Config is loaded from the user config directory by default. The state directory defaults beside it and can be overridden.

## Sync behavior

For each configured build, push:

1. Reads `{depot}/{name}/latest`.
2. Compares the short identity to local state.
3. Locates the platform directory for the current host.
4. Copies files into `install_path`.
5. Writes the installed short SHA/hash to local state.
6. Updates a `.files` manifest so removed files can be cleaned up later.

A missing platform directory is not fatal. It means that build was not published for the current host platform.

Single-file installs use a temp-file-and-rename pattern. Multi-file installs are best effort: if an install fails partway through, state is not advanced and the next sync retries.

## Commands

```text
push                           sync all builds to latest
push sync                      sync all builds to latest
push daemon                    sync continuously
push list [name]               list configured builds or versions
push status                    show local/depot state
push vendor <binary> <name>    stage a local binary into the depot
push version                   show build metadata
```

The daemon reloads config each tick. When the daemon updates its own executable, it exits cleanly so a service manager can restart it.

## CI publishing

The public script contract is intentionally narrow:

- `ci/ldflags.sh` stamps `github.com/michaelquigley/push/build` variables.
- `ci/deploy.sh` publishes already-built binaries.
- `DEPOT` and `PLATFORM` are required inputs to `deploy.sh`.
- `PROJECT` defaults to the repository basename and can be overridden.

A single `deploy.sh` invocation publishes one platform correctly. Running concurrent platform matrix legs directly against `deploy.sh` is not a complete publisher, because each invocation writes global metadata, flips `latest`, and prunes. A coordinated matrix publisher would need stage-only platform jobs and one final metadata/latest/prune job.

## Vendoring

`push vendor` stages a local binary using the same depot shape. The binary content hash becomes the build identity, making repeat vendoring idempotent. The platform is detected from the vendoring host, so vendoring should run on the platform that the binary is meant to serve.

## Host integration

The public systemd unit runs `push daemon` and restarts on both failures and clean self-update exits. Operators whose depot path depends on a mount should add their own mount dependency lines to the unit.

Windows hosts can run `push daemon` through Task Scheduler or run `push sync` on a schedule. Path handling uses Go's `filepath` package and accepts normal Windows drive or UNC paths.
