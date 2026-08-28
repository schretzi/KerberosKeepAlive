## kerberoskeepalive service uninstall

Unload the LaunchAgent and remove its plist

### Synopsis

Unload the job and delete its plist. Logs in ~/Library/Logs are left in place.

```
kerberoskeepalive service uninstall [flags]
```

### Options

```
  -h, --help   help for uninstall
```

### Options inherited from parent commands

```
      --binary string     path to the kerberoskeepalive executable to run (default: the running one)
      --config string     path to config file (default "~/.config/kerberoskeepalive/config.yaml")
      --profile strings   profile name(s) to operate on (default: all configured profiles)
```

### SEE ALSO

* [kerberoskeepalive service](kerberoskeepalive_service.md)	 - Manage the kerberoskeepalive LaunchAgent

