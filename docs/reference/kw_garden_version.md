## kw_garden version

Print the version

### Synopsis

Output the version of the CLI.

```
kw_garden version [flags]
```

### Examples

```
  > kw garden version
  v1.2.3

  > kw garden version -o json
  {"version":"v1.2.3-dev-4516e7f4dee0861b3d1a31b53d3a8aabbd084f48","gitTreeState":"dirty","gitCommit":"4516e7f4dee0861b3d1a31b53d3a8aabbd084f48","buildDate":"2026-04-21T13:14:36Z","major":1,"minor":2,"patch":3,"suffix":"dev-4516e7f4dee0861b3d1a31b53d3a8aabbd084f48"}

  > kw garden version -o yaml
	buildDate: "2026-04-21T13:14:36Z"
	gitCommit: 4516e7f4dee0861b3d1a31b53d3a8aabbd084f48
	gitTreeState: dirty
	major: 1
	minor: 2
	patch: 3
	suffix: dev-4516e7f4dee0861b3d1a31b53d3a8aabbd084f48
	version: v1.2.3-dev-4516e7f4dee0861b3d1a31b53d3a8aabbd084f48
```

### Options

```
  -h, --help            help for version
  -o, --output string   Output format. Valid formats are [json, text, yaml]. (default "text")
```

### SEE ALSO

* [kw_garden](kw_garden.md)	 - A kubeswitcher plugin for Gardener landscapes

