## kerberoskeepalive daemon

Run in the foreground, keeping configured tickets refreshed (invoked by launchd)

### Synopsis

Poll the configured profiles and reacquire any ticket close to expiry, until
interrupted.

This is the process the LaunchAgent runs; it is not how you install or control
that agent. Use `kerberoskeepalive service` for the launchd job.

```
kerberoskeepalive daemon [flags]
```

### Options

```
  -h, --help   help for daemon
```

### Options inherited from parent commands

```
      --config string     path to config file (default "~/.config/kerberoskeepalive/config.yaml")
      --profile strings   profile name(s) to operate on (default: all configured profiles)
```

### SEE ALSO

* [kerberoskeepalive](kerberoskeepalive.md)	 - Manage and keep macOS Kerberos tickets alive

