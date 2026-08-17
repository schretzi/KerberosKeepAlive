# KerberosKeepAlive

A macOS CLI and background daemon that keeps one or more Kerberos tickets
refreshed automatically. Ticket passwords are read from existing macOS
Keychain entries (never written by this tool), tickets are acquired via the
system `kinit`, and each ticket is written to a standard Kerberos credential
cache (ccache) file that other Kerberos-aware tools can consume via
`KRB5CCNAME` / `klist -c`.

## Features

- Manage multiple named ticket "profiles" from a single YAML config.
- `status` / `init` / `refresh` for on-demand inspection and (re)acquisition.
- `daemon` mode that polls and reacquires tickets before they expire,
  installable as a per-user macOS LaunchAgent (`daemon install`).
- Passwords are looked up read-only from the macOS Keychain by
  service+account — this tool never creates or modifies Keychain items.

## Installation

Download a prebuilt binary from the [Releases](../../releases) page, or
build from source:

```sh
git clone https://github.com/schretzi/KerberosKeepAlive.git
cd KerberosKeepAlive
make build
```

This produces `./kerberoskeepalive`. Move it wherever you like on your
`PATH`, e.g. `/usr/local/bin`.

## Configuration

Copy the example config and edit it for your environment:

```sh
mkdir -p ~/.config/kerberoskeepalive
cp testdata/config.example.yaml ~/.config/kerberoskeepalive/config.yaml
```

Each profile needs a Kerberos principal, a reference to an *existing*
Keychain generic-password item (service + account), and a path to write its
credential cache to. Create the Keychain item yourself, e.g.:

```sh
security add-generic-password -a jdoe -s KerberosKeepAlive-corp -w
```

See [`testdata/config.example.yaml`](testdata/config.example.yaml) for the
full schema.

## Usage

```sh
# Show status of all configured tickets
kerberoskeepalive status

# Acquire a ticket for the first time (run this interactively once, so any
# macOS Keychain access prompt can be granted before the daemon runs
# unattended)
kerberoskeepalive init

# Force re-acquire regardless of current expiry
kerberoskeepalive refresh

# Run the poll loop in the foreground (this is what the LaunchAgent invokes)
kerberoskeepalive daemon

# Install/uninstall the LaunchAgent that runs `daemon` at login
kerberoskeepalive daemon install
kerberoskeepalive daemon uninstall
```

All commands accept `--config <path>` (default
`~/.config/kerberoskeepalive/config.yaml`) and `--profile <name>` (repeatable;
defaults to all configured profiles).

Full command reference: [`docs/kerberoskeepalive.md`](docs/kerberoskeepalive.md)
(generated from the CLI itself — see below).

## Development

This project uses a `Makefile` as its local pipeline; see `make help` for
the full target list. The same checks run in CI on every push/PR.

```sh
make test      # go test ./...
make lint      # golangci-lint
make security  # govulncheck + gosec
make build     # go build
make docs      # regenerate docs/ from the Cobra command tree
```

### Git hooks

[lefthook](https://github.com/evilmartians/lefthook) runs
[gitleaks](https://github.com/gitleaks/gitleaks) on staged changes before
every commit, to catch accidentally-committed secrets. Install the hook
once after cloning:

```sh
lefthook install
```

### CI/CD

Every push/PR to `main` runs four separate jobs (all on `macos-latest`, since
this project links cgo against macOS frameworks and can't build on
Linux/Windows): `test`, `lint` (golangci-lint), `security` (govulncheck +
gosec), and `build`. These mirror `make pipeline` — run that locally before
pushing to catch the same issues early.

Releases are built with [GoReleaser](https://goreleaser.com) and published
via the "Release" GitHub Actions workflow, which is **manual only**
(`workflow_dispatch`) — pushing a tag alone does not publish anything. To
cut a release:

```sh
git tag v0.1.0
git push origin v0.1.0
```

then go to the repo's **Actions → Release → Run workflow**, and select that
tag in the branch/tag dropdown before running it.

## AI usage

This project's design and implementation were done collaboratively with
[Claude Code](https://claude.com/claude-code) (Anthropic's AI coding
assistant), directed and reviewed by the repository owner throughout:
architecture decisions (e.g. shelling out to `kinit` instead of hand-rolling
a Kerberos credential-cache writer, LaunchAgent vs LaunchDaemon for Keychain
access) were made jointly after the AI researched the actual library/OS
behavior involved, code was written by the AI against an explicit,
human-approved plan, and functionality was verified end-to-end against the
real `kinit`/`klist`/Keychain/launchd on this machine rather than assumed to
work. Treat commit authorship as human-directed, AI-assisted.
