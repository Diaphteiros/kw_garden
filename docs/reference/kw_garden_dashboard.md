## kw_garden dashboard

Switch to a cluster based on its URL in the Gardener dashboard

### Synopsis

Switch to a cluster based on its URL in the Gardener dashboard.

The URL must be passed in as argument. If no argument is specified, the command attempts to parse the contents of the clipboard as URL.

The URL is then matched against the configured URL regexes in the plugin configuration. The first matching regex is used to extract
garden, project, and shoot names from the respective capturing groups. These names are then used to switch to the cluster.

Example: `^(https:\/\/)?dashboard\.garden\.(?P<garden>[a-zA-Z0-9-]*)\.example\.com\/namespace\/garden-(?P<project>[a-z0-9-]*)\/shoots\/(?P<shoot>[a-z0-9-]*)(\/.*)?$`
  would match 'dashboard.garden.my-garden.example.com/namespace/garden-my-project/shoots/my-shoot' and capture 'my-garden', 'my-project', and 'my-shoot'.

```
kw_garden dashboard [<url>] [flags]
```

### Options

```
  -h, --help   help for dashboard
```

### SEE ALSO

* [kw_garden](kw_garden.md)	 - A kubeswitcher plugin for Gardener landscapes

