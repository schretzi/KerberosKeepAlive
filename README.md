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

```sh
brew install schretzi/tap/kerberoskeepalive
```

Or download a prebuilt binary from the [Releases](../../releases) page, or
build from source:

```sh
git clone https://github.com/schretzi/KerberosKeepAlive.git
cd KerberosKeepAlive
make build
```

The last option produces `./kerberoskeepalive`. Move it wherever you like on
your `PATH`, e.g. `/usr/local/bin`.

Installing only puts the binary in place — it does **not** start anything.
See [Running in the background](#running-in-the-background) below.

## Configuration

Copy the example config and edit it for your environment:

```sh
mkdir -p ~/.config/kerberoskeepalive
cp testdata/config.example.yaml ~/.config/kerberoskeepalive/config.yaml
```

Each profile needs a Kerberos principal, a reference to an _existing_
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

## Running in the background

Nothing is installed or started automatically — neither `brew install` nor
running any command sets up background refreshing. That is a deliberate,
explicit step:

```sh
kerberoskeepalive daemon install
```

This validates your config (refusing to install if it's invalid), then writes
a **per-user LaunchAgent** to
`~/Library/LaunchAgents/com.<username>.kerberoskeepalive.plist` and loads it
with `launchctl bootstrap gui/<uid>`. The plist points at the absolute path of
the binary you ran the command from, and at the `--config` path you passed
(so pass `--config` here if you don't use the default). Re-running the command
is safe — it unloads and reloads.

From then on `kerberoskeepalive daemon` starts at every login and is restarted
by launchd if it exits non-zero. See [Logs](#logs) for where it writes.

It is a LaunchAgent rather than a system-wide LaunchDaemon on purpose: the
daemon needs your login session's Keychain and credential cache, which a root
daemon cannot reach.

Run `kerberoskeepalive init` interactively **before** installing the agent, so
you can grant the macOS Keychain access prompt while there's a UI to answer
it — the daemon runs unattended and cannot.

To stop and remove it:

```sh
kerberoskeepalive daemon uninstall
```

`brew uninstall kerberoskeepalive` also runs this automatically, so the agent
is never left pointing at a deleted binary. `brew uninstall --zap
kerberoskeepalive` additionally removes your config and logs.

### Logs

Everything lives in `~/Library/Logs/KerberosKeepAlive/`:

| File | Contents |
| --- | --- |
| `daemon.log` | The daemon's own log — this is the one to read. Rotated. |
| `daemon-<timestamp>.log.gz` | Rotated older logs. |
| `launchd.err.log` | Crash capture only: panics, and failures before logging starts. Normally empty. |
| `launchd.out.log` | Same, for stdout. Normally empty. |

```sh
tail -f ~/Library/Logs/KerberosKeepAlive/daemon.log
```

`daemon.log` rotates because a KDC you can't reach — VPN down, laptop off the
corporate network — logs two lines every poll for as long as that lasts. The
defaults keep at most 5 MB plus 3 gzipped backups, dropping anything older
than 28 days; all four knobs are configurable under `daemon.log` in the config
(see [`testdata/config.example.yaml`](testdata/config.example.yaml)). Like
`poll_interval`, changes take effect on daemon restart.

If you installed the LaunchAgent before this was added, re-run
`kerberoskeepalive daemon install` to pick up the new plist, and delete the
stale `daemon.{out,err}.log` files.

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

## AI usage This project's design and implementation were done collaboratively with [Claude Code](https://claude.com/claude-code) (Anthropic's AI coding assistant), directed and reviewed by the repository owner throughout: architecture decisions (e.g. shelling out to `kinit` instead of hand-rolling a Kerberos credential-cache writer, LaunchAgent vs LaunchDaemon for Keychain access) were made jointly after the AI researched the actual library/OS

behavior involved, code was written by the AI against an explicit,
human-approved plan, and functionality was verified end-to-end against the
real `kinit`/`klist`/Keychain/launchd on this machine rather than assumed to
work. Treat commit authorship as human-directed, AI-assisted.
