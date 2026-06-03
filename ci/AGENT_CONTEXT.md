# Push CI integration context

This document is the working context for adding push version stamping and depot publishing to CI workflows.

## Build package

Projects import the shared build package:

```go
import "github.com/michaelquigley/push/build"
```

The package provides:

- `build.Version`, `build.Hash`, `build.Date`, `build.Builder`, `build.Branch`, and `build.CGO`, stamped at build time by `ci/ldflags.sh`.
- `build.DevVersion`, the fallback base version for unstamped developer builds.
- `build.String()`, a compact version string such as `v0.2.0 [a1b2c3d4]`, falling back to `DevVersion + " [developer build]"`.
- `build.Detail()`, a multi-line metadata block that also includes runtime Go and target information.
- `build.NewVersionCmd(name)`, a reusable Cobra `version` subcommand.

Do not create local build metadata packages in new projects. Import this package and stamp it with the shared script.

## Version command pattern

Create `cmd/{project}/version.go`:

```go
package main

import "github.com/michaelquigley/push/build"

func init() {
    build.DevVersion = "v0.1.x"
    rootCmd.AddCommand(build.NewVersionCmd("myapp"))
}
```

## Version format

| Scenario | Example |
|---|---|
| HEAD is tagged `v0.2.0` | `v0.2.0` |
| Untagged, latest ancestor tag is `v0.2.0` | `v0.2.1-dev.20260602.a1b2c3d4` |
| Untagged, no tags exist | `v0.0.0-dev.20260602.a1b2c3d4` |
| Local developer build without ldflags | `v0.1.x [developer build]` |

The dev version auto-bumps the patch segment relative to the nearest ancestor tag.

## CI scripts

### `ci/version.sh`

Computes a version string from git state. It reads `GITHUB_REF` and `GITHUB_REF_NAME`, matching GitHub Actions and Gitea Actions compatibility variables.

### `ci/ldflags.sh`

Computes the Go build `-ldflags` string. It reads `GITHUB_REF`, `GITHUB_REF_NAME`, and `GITHUB_REPOSITORY`, calls `version.sh`, and targets `github.com/michaelquigley/push/build.*` because all consumers share the same build package.

### `ci/deploy.sh`

Publishes binaries into a push depot. Positional arguments are binary paths or names:

```bash
_push/ci/deploy.sh myapp
_push/ci/deploy.sh tool-a tool-b
```

Required environment:

- `DEPOT`: absolute depot root.
- `PLATFORM`: platform directory to publish into, such as `linux-amd64`.

Optional environment:

- `PROJECT`: depot project name. Defaults to `basename "$GITHUB_REPOSITORY"`.
- `KEEP`: number of build directories to retain. Defaults to `5`.

`PLATFORM` is explicit and required. Do not infer it from `GOOS`/`GOARCH` in the deploy step; those variables are often scoped only to the build step, and host-platform inference misfiles cross-compiled artifacts.

## Depot layout

A deploy writes this structure:

```text
{DEPOT}/
  {project}/
    latest
    builds/
      {short_sha}/
        build.json
        {PLATFORM}/
          <binaries and files>
```

`build.json` records commit metadata and the version computed by `version.sh`. `latest` contains `{SHORT}.{UNIX_EPOCH}` so repeated publishes of the same commit still produce a changed deployment identity.

## Single-binary template

```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    env:
      DEPOT: /mnt/push
      KEEP: 5
      PLATFORM: linux-amd64
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - name: fetch push scripts
        run: git clone https://github.com/michaelquigley/push.git _push
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.7'
      - name: build
        env:
          CGO_ENABLED: '0'
          GOOS: linux
          GOARCH: amd64
        run: |
          LDFLAGS=$(_push/ci/ldflags.sh)
          go build -ldflags "${LDFLAGS}" -o myapp ./cmd/myapp
      - name: verify stamp
        run: |
          out=$(./myapp version)
          if [ -z "${out}" ] || printf '%s\n' "${out}" | grep -F '[developer build]'; then
            printf '%s\n' "${out}"
            exit 1
          fi
      - name: deploy
        run: _push/ci/deploy.sh myapp
```

Use the stamp check for every binary passed to `deploy.sh`. A representative binary is not enough in multi-binary projects, because stamping is per binary.

## Multi-binary template

```yaml
      - name: build
        env:
          CGO_ENABLED: '0'
          GOOS: linux
          GOARCH: amd64
        run: |
          LDFLAGS=$(_push/ci/ldflags.sh)
          go build -ldflags "${LDFLAGS}" -o tool-a ./cmd/tool-a
          go build -ldflags "${LDFLAGS}" -o tool-b ./cmd/tool-b
      - name: verify stamps
        run: |
          for bin in tool-a tool-b; do
            out=$("./${bin}" version)
            if [ -z "${out}" ] || printf '%s\n' "${out}" | grep -F '[developer build]'; then
              printf '%s\n' "${out}"
              exit 1
            fi
          done
      - name: deploy
        run: _push/ci/deploy.sh tool-a tool-b
```

## Adaptation notes

- Keep frontend, system dependency, and CGO setup steps exactly where the project needs them.
- Set `CGO_ENABLED` only when the project can or must do so. `ldflags.sh` stamps the value it sees.
- Use `PROJECT` only when the depot project name should differ from the repository name.
- A single `deploy.sh` invocation is correct for one platform. Coordinated multi-platform publishing needs a stage-and-finalize workflow and is not implemented by this script.
