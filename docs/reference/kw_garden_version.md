## kw_garden version

Print the version

### Synopsis

Output the version of the CLI.

```
kw_garden version [flags]
```

### Examples

```
  > kw version
  v1.2.3

  > kw version -o json
  {"major":"v1","minor":"2","gitVersion":"v1.2.3","gitCommit":"76c01d5337fc9de6e053b4e5bafd5239c8b7a973","gitTreeState":"dirty","buildDate":"2024-04-26T11:29:39+02:00","goVersion":"go1.22.2","compiler":"gc","platform":"darwin/arm64"}

  > kw version -o yaml
  buildDate: "2024-04-26T11:29:39+02:00"
  compiler: gc
  gitCommit: 76c01d5337fc9de6e053b4e5bafd5239c8b7a973
  gitTreeState: dirty
  gitVersion: v1.2.3
  goVersion: go1.22.2
  major: v1
  minor: "2"
  platform: darwin/arm64
```

### Options

```
  -h, --help            help for version
  -o, --output string   Output format. Valid formats are [json, text, yaml]. (default "text")
```

### SEE ALSO

* [kw_garden](kw_garden.md)	 - A kubeswitcher plugin for Gardener landscapes

