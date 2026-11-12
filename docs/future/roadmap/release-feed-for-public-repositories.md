---
title: release feed for public repositories
state: researching
created: 2026-11-11
tags: [feature]
---

Projects hosted on GitHub reach the fleet depot only by hand today: someone downloads a release artifact and runs `push vendor`, as the HQ vendoring procedure describes. Add a feed that does it unattended: watch a configured list of GitHub repositories for published releases, fetch the platform's artifact, verify it, and stage it into the depot the way `push vendor` does.

## why

The public tools (ranger, scry, pane, push itself, and soon metawoo mega) build on GitHub Actions and draft releases there, while the fleet consumes a depot inside HQ. HQ's own CI cannot see GitHub pushes, and a self-hosted runner on a public repository would let a workflow run code inside HQ. A feed that pulls keeps the trust direction right: HQ reaches out, nothing reaches in, and it consumes the same artifact a stranger would download.

## shape

- A list of feeds in the config: repository, the binary names to stage and the depot name for each, the artifact pattern (the house pattern is `<name>-<tag>-linux-amd64.tar.gz`), and whether drafts and prereleases count. Published releases only by default.
- On an interval, ask GitHub for the latest release of each repository. For a tag not yet in the depot: download the archive, extract the named binaries, run each one's `version` command and require the tag in its output and no `[developer build]` marker, then stage exactly as `push vendor <bin> <name> --version <tag>` does, with the release URL in the build's message.
- Nothing is staged when verification fails; the failure is logged with the tag and the reason, and the next interval tries again. A tag already in the depot is never re-staged.
- Unauthenticated GitHub API access is enough for public repositories at this rate; an optional token raises the limit. The feed reads only.
- It runs on one `linux-amd64` host, since `push vendor` files a build under the vendoring host's platform.

## background

Raised 2026-11-11 while planning the move of roundhouse to `github.com/michaelquigley/mega`. Alternatives considered there: a git.hq pull mirror driving HQ's CI (builds from source inside HQ and needs HQ-specific workflow files or mirror-side wiring), and a self-hosted GitHub runner inside HQ (rejected on trust direction). Whether this is a mode of the `push` daemon or a companion command is open; the HQ-side list of repositories and the pinned versions belong in the push-hq repository either way.
