# push

push is a lightweight deployment agent and version-stamping toolkit for Go projects. It has two parts:

- `github.com/michaelquigley/push/build`, a small package for stamped version output and a reusable Cobra `version` command.
- `push`, a daemon/CLI that syncs binaries from a filesystem depot into local install paths.

The depot is deliberately simple: CI or an operator writes artifacts into a directory tree, and the daemon reads from that tree. The depot can be a shared mount, a synced folder, or any filesystem path available to both the publisher and the agent.

## Install

```bash
go install github.com/michaelquigley/push/cmd/push@latest
```

Or build from source:

```bash
go build -o ~/bin/push ./cmd/push
```

## Configure

Create `~/.config/push/config.yaml`:

```yaml
depot: /mnt/push
interval: 5m

builds:
  - name: myapp
    install_path: ~/bin

  - name: tools
    install_path: ~/opt/tools
```

See [config.example.yaml](config.example.yaml) for a fuller example.

## Usage

```text
push                    sync all builds to latest (default)
push sync               same as above
push daemon             run continuously on the configured interval
push list               list configured builds
push list <name>        list available versions for one build
push status             show sync status for all builds
push vendor <bin> <name> stage a local binary into the depot
push version            show build/version information
```

`push daemon` syncs on startup, then repeats on the configured interval. It reloads the config file on every tick, so build lists, paths, and intervals can change without restarting the daemon.

If push is configured to track its own binary, the daemon exits cleanly after updating itself so the service manager can restart it with the new version.

## Systemd Service

Install the user service:

```bash
cp systemd/push.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now push
```

The shipped unit is generic. If your depot path depends on a mount, add the appropriate `Requires=` and `After=` lines for that mount in your local unit.

## Depot Layout

Publishers write artifacts in this structure:

```text
/mnt/push/
  myapp/
    latest
    builds/
      a1b2c3d4/
        build.json
        linux-amd64/
          myapp
```

The agent detects its own platform string, such as `linux-amd64` or `windows-amd64`, and reads from the matching platform directory. Everything in the platform directory is copied to the configured `install_path`.

## Build Library

Use the `build` package to add stamped version output to a Cobra CLI:

```go
import "github.com/michaelquigley/push/build"

func init() {
    build.DevVersion = "v0.1.x"
    rootCmd.AddCommand(build.NewVersionCmd("myapp"))
}
```

`ci/ldflags.sh` stamps `build.Version`, `build.Hash`, `build.Date`, `build.Builder`, `build.Branch`, and `build.CGO` at build time. Unstamped local builds report the configured developer fallback.

## CI Scripts

The reusable scripts live in [ci/](ci/):

- `version.sh` computes a version from git state.
- `ldflags.sh` computes the Go `-ldflags` string for the shared build package.
- `deploy.sh` publishes one or more already-built binaries into a depot.

A public workflow can fetch the scripts directly:

```yaml
- name: fetch push scripts
  run: git clone https://github.com/michaelquigley/push.git _push

- name: build
  env:
    CGO_ENABLED: '0'
    GOOS: linux
    GOARCH: amd64
  run: |
    LDFLAGS=$(_push/ci/ldflags.sh)
    go build -ldflags "${LDFLAGS}" -o myapp ./cmd/myapp

- name: deploy
  env:
    DEPOT: /mnt/push
    PLATFORM: linux-amd64
  run: _push/ci/deploy.sh myapp
```

`deploy.sh` requires `DEPOT` and `PLATFORM` explicitly. It does not infer platform from the runner, because cross-compiled binaries can otherwise be filed under the wrong platform directory.

See [ci/AGENT_CONTEXT.md](ci/AGENT_CONTEXT.md) for CI integration details and [docs/current/push-design.md](docs/current/push-design.md) for the current system model.
