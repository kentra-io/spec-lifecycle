# Installing & releasing `lifecycle`

## Install

### Homebrew (macOS / Linux)

```sh
brew install kentra-io/tap/lifecycle
```

The cask is published to [`kentra-io/homebrew-tap`](https://github.com/kentra-io/homebrew-tap)
by the release pipeline — the same tap that already carries the
[`constitution`](https://github.com/kentra-io/adr-sourced-constitution) cask.
The binary is not code-signed; the cask strips the macOS quarantine attribute
on install so it runs without a Gatekeeper prompt.

### `go install`

```sh
go install github.com/kentra-io/spec-lifecycle/cmd/lifecycle@latest
```

`go install` builds locally, so `lifecycle --version` reports the module
pseudo-version + VCS revision rather than a release tag (both are wired in
`cmd/lifecycle/version.go`).

### Release archive (direct download)

Every release publishes per-platform archives plus a `checksums.txt`. Asset
names follow a fixed template:

```
lifecycle_<version>_<os>_<arch>.tar.gz     # linux, darwin
lifecycle_<version>_<os>_<arch>.zip        # windows
```

where `<version>` is the tag without its leading `v` (tag `v0.1.0` -> `0.1.0`).
So the linux/amd64 tarball for `v0.1.0` is at the deterministic URL:

```
https://github.com/kentra-io/spec-lifecycle/releases/download/v0.1.0/lifecycle_0.1.0_linux_amd64.tar.gz
```

### claudebox / Docker

`lifecycle` is a single static Go binary (`CGO_ENABLED=0`) with **no Node.js,
npm, or `openspec` runtime dependency** — the format engine is reimplemented
natively in Go (implementation-plan.md §0.5/"Option B"). Because the asset URL
is deterministic, a container image can install a pinned version with a plain
download-and-extract; no language runtime needs to be provisioned alongside
it. Add to your `.claudebox/Dockerfile` (or any Debian/Ubuntu-based image):

```dockerfile
# Install the lifecycle CLI at a pinned version. Static binary — no Node/npm,
# no `openspec` install, nothing else to provision.
ARG LIFECYCLE_VERSION=0.1.0
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in \
      amd64) goarch=amd64 ;; \
      arm64) goarch=arm64 ;; \
      *) echo "unsupported arch: $arch" >&2; exit 1 ;; \
    esac; \
    base="https://github.com/kentra-io/spec-lifecycle/releases/download/v${LIFECYCLE_VERSION}"; \
    asset="lifecycle_${LIFECYCLE_VERSION}_linux_${goarch}.tar.gz"; \
    cd /tmp; \
    curl -fsSL "$base/$asset" -o "$asset"; \
    curl -fsSL "$base/checksums.txt" -o checksums.txt; \
    # Verify the download against the release checksums before extracting.
    # --ignore-missing checks only the asset we fetched, not every line.
    sha256sum -c --ignore-missing checksums.txt; \
    tar -xzf "$asset" -C /usr/local/bin lifecycle; \
    rm "$asset" checksums.txt; \
    lifecycle --version
```

That the binary is genuinely self-contained can be proven with a
`FROM scratch`-style minimal image — no base OS, no libc, no shell other than
the binary itself:

```dockerfile
FROM scratch
COPY lifecycle /lifecycle
ENTRYPOINT ["/lifecycle"]
```

`docker run --rm my-lifecycle-scratch-image --version` works with nothing
else in the image. Without Docker, the equivalent check is copying the binary
alone into an empty directory and invoking it with a minimal `PATH` (no Go
toolchain, no other tools on the search path) — every verb from `init` through
`guard` runs unassisted.

The `<version>_<os>_<arch>` template is produced by the `archives.name_template`
in [`.goreleaser.yaml`](../.goreleaser.yaml); keep the two in sync if either
changes.

## How a release is cut

Releases are fully automated by [`.github/workflows/release.yml`](../.github/workflows/release.yml),
which triggers on any `v*` tag.

**Prerequisites (one-time, owner-side; implementation-plan.md §10):**

- `kentra-io/spec-lifecycle` exists (public) and the bot has write access.
- `kentra-io/homebrew-tap` exists (public) — already established by the
  constitution.
- `HOMEBREW_TAP_TOKEN` is an **org-level** Actions secret (promoted from the
  constitution's per-repo PAT so every primitive repo shares one token — the
  fine-grained PAT needs `Contents: read/write` on `homebrew-tap`). GoReleaser
  needs it to push the cask cross-repo — the default `GITHUB_TOKEN` is scoped
  to this repo only.

**Cutting a release:**

```sh
git tag v0.1.0
git push origin v0.1.0
```

CI then runs `goreleaser release --clean`, which:

1. Builds all targets (linux/darwin/windows × amd64/arm64).
2. Creates the GitHub Release with the archives and `checksums.txt`.
3. Pushes the updated Homebrew cask to `kentra-io/homebrew-tap` (the second
   cask in that tap, alongside `constitution`).

Validate the config locally before tagging:

```sh
goreleaser check                                    # config is valid
goreleaser release --snapshot --clean --skip=publish  # dry run, builds everything into ./dist
```

`./dist` is git-ignored; snapshot artifacts are never committed.

## If a release fails midway

A tag push that fails partway (e.g. the cask push errors after the GitHub
Release was created) leaves a partial release and a published tag. GoReleaser
does not overwrite an existing release, so re-running against the same tag will
not recover it — tear the partial state down, fix the cause, and re-tag. Delete
the partial release, delete the remote tag, then re-create both once fixed:

```sh
gh release delete v0.1.0 --yes             # remove the partial GitHub Release
git push --delete origin v0.1.0            # remove the remote tag
git tag -d v0.1.0                          # remove the local tag
# ...fix the cause (config, secret, etc.), then re-cut:
git tag v0.1.0
git push origin v0.1.0
```
