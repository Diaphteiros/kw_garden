# KubeSwitcher Plugin: Garden

This is a plugin for the [kubeswitcher](https://github.com/Diaphteiros/kw) tool that allows to switch between the different clusters of a Gardener landscape.

Technically, this just implements the kubeswitcher plugin contract and passes the actual work to [gardenctl](https://github.com/gardener/gardenctl-v2). For this to work, `gardenctl` has to be installed and configured on the machine. See below for how to configure this plugin.

## Installation

To install the KubeSwitcher plugin, simply run the following command
```shell
go install github.com/Diaphteiros/kw_garden@latest
```
or clone the repository and run
```shell
task install
```

## Configuration

The plugin takes a small configuration in the kubeswitcher config. It can be completely defaulted, if missing.
```yaml
<...>
- name: garden # under which kw subcommand this plugin will be reachable
  short: "Wrapper around gardenctl" # short message for display in 'kw --help'
  binary: kw_garden # name of or path to the plugin binary
  aliases: # aliases for plugin subcommand (optional)
  - g
  config:
    binary: gardenctl # path to the gardenctl binary (has to be in $PATH if specified without any path separators) (optional, defaults to 'gardenctl')
    configDir: /foo/bar/baz # path to the gardenctl config dir (optional, uses gardenctl default)
    configFileName: gardenctl_config.yaml # name of the gardenctl config file (optional, uses gardenctl default)
    urlRegexes: # regexes for dashboard URL parsing (optional)
    - '^(https:\/\/)?dashboard\.garden\.(?P<garden>[a-zA-Z0-9-]*)\.example\.com\/namespace\/garden-(?P<project>[a-z0-9-]*)\/shoots\/(?P<shoot>[a-z0-9-]*)(\/.*)?$'
```

## Usage

Apart from the subcommands mentioned below, does the plugin not parse any arguments, but simply passes everything to `gardenctl`.

### The 'target' Subcommand

The most used subcommand of `gardenctl` will probably be the `target` one, changing the current kubeconfig to point to the specified target. To improve selecting the desired cluster, this plugin can modify the arguments before passing them to `gardenctl`.

Note that arguments are not parsed properly here - the plugin simply checks whether one of the arguments is `target` and then applies the following logic to all subsequent arguments:
- `-g` will be replaced by `--garden`, `-p` by `--project`, and `-s` by `--shoot`. This is a very primitive way of creating short versions of the respective flags. 
- If any of the aforementioned three arguments is not followed by an argument providing a value - either because it is the last argument, or because the next argument starts with `-`, indicating that it is another command option and not a value - the corresponding value is considered missing. In that case, the plugin will provide a list of possible values and ask the user to choose one of them. Not choosing a valid value will result in the command failing.

The following examples show some plugin calls and the corresponding `gardenctl` calls that will be executed. The examples all assume that the plugin was registered with name `garden` and is therefore reachable via `kw garden`. In the configuration example above, there is an alias `g` registered, so the plugin would also be reachable via `kw g`.
- `kw garden --help` => `gardenctl --help`
- `kw garden target --garden live --project foo --shoot bar` => `gardenctl target --garden live --project foo --shoot bar`
- `kw garden target -g live -p foo -s bar` => `gardenctl target --garden live --project foo --shoot bar`
- `kw garden kubeconfig` => `gardenctl kubeconfig`
- `kw garden target -g live -p -s` => `gardenctl target --garden live --project <project> --shoot <shoot>`, where `<project>` and `<shoot>` have to be selected by the user from a provided list

### Dashboard

The `dashboard` subcommand can be used to target Gardener shoot clusters based on their respective URLs in the Gardener dashboard.
For this to work, a regex has to be provided which matches a valid dashboard URL and captures the `garden`, `project`, and `shoot` names in identically named capturing groups.

The regular expression from the configuration example above would be able to match `dashboard.garden.<garden>.example.com/namespace/garden-<project>/shoots/<shoot>` URLs, optionally with preceding `https://` or following `/`.

If multiple regexes are configured, the command tries them in the specified order and chooses the first one that matches.

The URL can be specified as argument. If no arguments are given, the clipboard contents are interpreted as URL.

Examples:
- `kw garden dashboard` => Parses URL from clipboard and switches to resulting cluster.
- `kw garden dashboard dashboard.garden.live.example.com/namespace/garden-foo/shoots/bar` => `gardenctl target --garden live --project foo --shoot bar`

## Using Gardenctl

Whenever this plugin calls `gardenctl`, it backups the current state of `gardenctl` before, replaces it with any potential state from calls to this plugin before and restores the original `gardenctl` state again afterwards. While somewhat hacky, this basically ensures that calls to `gardenctl` directly and calls to this plugin both maintain their own respective state and don't mess with each other.
