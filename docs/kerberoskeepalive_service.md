## kerberoskeepalive service

Manage the kerberoskeepalive LaunchAgent

### Synopsis

Manage the launchd job that runs kerberoskeepalive in the background.

  label   com.schretzi.kerberoskeepalive
  plist   ~/Library/LaunchAgents/com.schretzi.kerberoskeepalive.plist
  log     ~/Library/Logs/kerberoskeepalive.log
  stderr  ~/Library/Logs/kerberoskeepalive.err.log

Both logs are rotated by newsyslog, configured in MacbookSetup under
etc/newsyslog.d/kerberoskeepalive.conf.

### Options

```
      --binary string   path to the kerberoskeepalive executable to run (default: the running one)
  -h, --help            help for service
```

### Options inherited from parent commands

```
      --config string     path to config file (default "~/.config/kerberoskeepalive/config.yaml")
      --profile strings   profile name(s) to operate on (default: all configured profiles)
```

### SEE ALSO

* [kerberoskeepalive](kerberoskeepalive.md)	 - Manage and keep macOS Kerberos tickets alive
* [kerberoskeepalive service install](kerberoskeepalive_service_install.md)	 - Write the LaunchAgent plist and load it
* [kerberoskeepalive service restart](kerberoskeepalive_service_restart.md)	 - Unload and reload the LaunchAgent
* [kerberoskeepalive service start](kerberoskeepalive_service_start.md)	 - Load the LaunchAgent
* [kerberoskeepalive service status](kerberoskeepalive_service_status.md)	 - Show whether the LaunchAgent is installed, loaded and running
* [kerberoskeepalive service stop](kerberoskeepalive_service_stop.md)	 - Unload the LaunchAgent
* [kerberoskeepalive service uninstall](kerberoskeepalive_service_uninstall.md)	 - Unload the LaunchAgent and remove its plist

