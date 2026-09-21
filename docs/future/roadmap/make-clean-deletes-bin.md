---
title: make clean deletes /bin
state: inbox
created: 2026-08-13
tags: [defect]
---

`make clean` runs `rm -f $(GOPATH)/bin/*`, and this Makefile never defines `GOPATH` — it is inherited from the environment or empty. With `GOPATH` unset the line expands to **`rm -f /bin/*`**. Scope it to what this repo installs — `rm -f ${GOBIN}/push push` — and add the `GOBIN ?= $(shell go env GOPATH)/bin` line the sibling Makefiles use, so the path resolves through the go toolchain instead of a bare environment variable.

## why

Verified with `env -u GOPATH make -n clean`: the expansion is `rm -f /bin/*`. Unprivileged it fails per-file; run as root, or in a container that builds as root, it removes the system binaries directory. Even with `GOPATH` set, the wildcard removes every binary in that bin directory rather than push's own — and a stale ambient GOPATH points it at a different project's bin entirely.

`$(GOPATH)` is the specific hazard, distinct from the sibling instance in lore: `go env GOPATH` always yields a value (defaulting to `~/go`), while an undefined make variable yields the empty string and roots the wildcard at `/`.
