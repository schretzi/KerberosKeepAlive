## kerberoskeepalive

Manage and keep macOS Kerberos tickets alive

### Options

```
      --config string     path to config file (default "~/.config/kerberoskeepalive/config.yaml")
  -h, --help              help for kerberoskeepalive
      --profile strings   profile name(s) to operate on (default: all configured profiles)
```

### SEE ALSO

* [kerberoskeepalive daemon](kerberoskeepalive_daemon.md)	 - Run in the foreground, keeping configured tickets refreshed (invoked by launchd)
* [kerberoskeepalive init](kerberoskeepalive_init.md)	 - Acquire a fresh ticket now for the configured profile(s) (first-time setup)
* [kerberoskeepalive refresh](kerberoskeepalive_refresh.md)	 - Force re-acquire a ticket now, regardless of current expiry
* [kerberoskeepalive status](kerberoskeepalive_status.md)	 - Show status of configured tickets

