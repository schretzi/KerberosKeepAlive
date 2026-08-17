## kerberoskeepalive daemon

Run in the foreground, keeping configured tickets refreshed (invoked by launchd)

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
* [kerberoskeepalive daemon install](kerberoskeepalive_daemon_install.md)	 - Generate and load a LaunchAgent that runs the daemon at login
* [kerberoskeepalive daemon uninstall](kerberoskeepalive_daemon_uninstall.md)	 - Unload and remove the LaunchAgent

