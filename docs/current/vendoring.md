# vendoring third-party binaries

## Overview

`push vendor` stages an existing local binary into a push depot. It is useful for tools that are built elsewhere but should still be distributed through the same daemon/sync path.

```bash
push vendor <binaryPath> <depotName> [--version <ver>] [--message <msg>]
```

## Arguments and flags

- `binaryPath`: path to an existing binary on disk. Leading `~/` is expanded.
- `depotName`: build name under the depot.
- `--version`: version string written to `build.json`. Defaults to the short content hash.
- `--message`: metadata message. Defaults to `vendored`.

Examples:

```bash
push vendor ~/local/bin/zrok zrok
push vendor ~/local/bin/hugo hugo --version v0.145.0 --message "hugo static site generator"
```

## Behavior

1. Load config to find `depot`.
2. Validate the source binary.
3. Hash the binary with SHA-256.
4. Use the first 8 hex characters as the build identity.
5. Exit early if `latest` already points at that identity.
6. Detect the vendoring host platform, such as `linux-amd64`.
7. Copy the binary into `{depot}/{name}/builds/{short}/{platform}/` with executable permissions.
8. Write `build.json`.
9. Write `{short}.{unix_epoch}` to `latest`.

The command copies the binary byte-for-byte. It does not stamp or modify the binary. If the binary embeds version metadata, build and verify that metadata before vendoring.

## Depot layout produced

```text
{depot}/{depotName}/
  latest
  builds/
    {short}/
      build.json
      linux-amd64/
        {basename of binaryPath}
```

The platform directory is detected from the host running `push vendor`, not from the binary. Run vendoring on the target platform unless you are deliberately publishing for the vendoring host's platform.

## Consuming machines

Add the vendored build to config:

```yaml
builds:
  - name: zrok
    install_path: ~/bin
```

No special sync configuration is needed. Vendored builds use the same depot shape as CI-published builds.
