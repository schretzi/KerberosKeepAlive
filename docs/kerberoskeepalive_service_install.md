## kerberoskeepalive service install

Write the LaunchAgent plist and load it

### Synopsis

Write ~/Library/LaunchAgents/com.schretzi.kerberoskeepalive.plist and load it.

Idempotent: an already-loaded job is unloaded and reloaded, so this is also
how you apply a change to the plist.

```
kerberoskeepalive service install [flags]
```

### Options

```
  -h, --help   help for install
```

### Options inherited from parent commands

```
      --binary string     path to the kerberoskeepalive executable to run (default: the running one)
      --config string     path to config file (default "~/.config/kerberoskeepalive/config.yaml")
      --profile strings   profile name(s) to operate on (default: all configured profiles)
```

### SEE ALSO

* [kerberoskeepalive service](kerberoskeepalive_service.md)	 - Manage the kerberoskeepalive LaunchAgent

