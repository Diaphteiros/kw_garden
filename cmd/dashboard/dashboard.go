package dashboard

import (
	"fmt"
	"regexp"

	"github.com/atotto/clipboard"
	"github.com/mandelsoft/vfs/pkg/vfs"
	"github.com/spf13/cobra"

	libcontext "github.com/Diaphteiros/kw/pluginlib/pkg/context"
	"github.com/Diaphteiros/kw/pluginlib/pkg/debug"
	"github.com/Diaphteiros/kw/pluginlib/pkg/fs"
	libutils "github.com/Diaphteiros/kw/pluginlib/pkg/utils"
	"github.com/Diaphteiros/kw_garden/pkg/config"
)

var DashboardCmd = &cobra.Command{
	Use:     "dashboard [<url>]",
	Aliases: []string{"d"},
	Args:    cobra.RangeArgs(0, 1),
	Short:   "Switch to a cluster based on its URL in the Gardener dashboard",
	Long: `Switch to a cluster based on its URL in the Gardener dashboard.

The URL must be passed in as argument. If no argument is specified, the command attempts to parse the contents of the clipboard as URL.

The URL is then matched against the configured URL regexes in the plugin configuration. The first matching regex is used to extract
garden, project, and shoot names from the respective capturing groups. These names are then used to switch to the cluster.

Example: ` + "`" + `^(https:\/\/)?dashboard\.garden\.(?P<garden>[a-zA-Z0-9-]*)\.example\.com\/namespace\/garden-(?P<project>[a-z0-9-]*)\/shoots\/(?P<shoot>[a-z0-9-]*)(\/.*)?$` + "`" + `
  would match 'dashboard.garden.my-garden.example.com/namespace/garden-my-project/shoots/my-shoot' and capture 'my-garden', 'my-project', and 'my-shoot'.`,
	Run: func(cmd *cobra.Command, args []string) {
		// load context and config
		debug.Debug("Loading kubeswitcher context from environment")
		con, err := libcontext.NewContextFromEnv()
		if err != nil {
			libutils.Fatal(1, "error creating kubeswitcher context from environment (this is a plugin, did you run it as standalone?): %w", err)
		}
		debug.Debug("Kubeswitcher context loaded:\n%s", con.String())
		debug.Debug("Loading plugin configuration")
		cfg, err := config.LoadFromBytes([]byte(con.PluginConfig))
		if err != nil {
			libutils.Fatal(1, "error loading plugin configuration: %w", err)
		}
		debug.Debug("Plugin configuration loaded:\n%s", cfg.String())

		// check if any URL regexes are specified
		if len(cfg.URLs) == 0 {
			libutils.Fatal(1, "no URL regexes specified in plugin configuration")
		}

		// get URL from argument or clipboard
		var url string
		if len(args) > 0 {
			url = args[0]
			debug.Debug("Using URL from argument: %s", url)
		} else {
			url, err = clipboard.ReadAll()
			if err != nil {
				libutils.Fatal(1, "error reading from clipboard: %w\n", err)
			}
			debug.Debug("Using URL from clipboard: %s", url)
		}

		// iterate over regexes, compile them and check for matches
		matched := false
		for idx, urlSpec := range cfg.URLs {
			regex, err := regexp.Compile(urlSpec.Regex)
			if err != nil {
				libutils.Fatal(1, "error compiling URL regex %d: %w", idx, err)
			}
			gardenIdx := -1
			if urlSpec.Garden == "" {
				gardenIdx = regex.SubexpIndex("garden")
				if gardenIdx < 0 {
					libutils.Fatal(1, "garden name not overwritten and no capturing group 'garden' found in URL regex %d", idx)
				}
			}
			projectIdx := regex.SubexpIndex("project")
			if projectIdx < 0 {
				libutils.Fatal(1, "no capturing group 'project' found in URL regex %d", idx)
			}
			shootIdx := regex.SubexpIndex("shoot")
			if shootIdx < 0 {
				libutils.Fatal(1, "no capturing group 'shoot' found in URL regex %d", idx)
			}
			matches := regex.FindStringSubmatch(url)
			if matches == nil {
				continue
			}

			// match found, extract garden, project, and shoot names
			matched = true
			debug.Debug("Match found with regex %d: %s", idx, urlSpec)
			garden := urlSpec.Garden
			if gardenIdx >= 0 {
				garden = matches[gardenIdx]
			}
			if garden == "" {
				libutils.Fatal(1, "garden name must not be empty")
			}
			project := matches[projectIdx]
			shoot := matches[shootIdx]

			// defer actual switching to the 'target' subcommand
			cmdString := fmt.Sprintf("%s target --garden %s --project %s --shoot %s", con.CurrentPluginName, garden, project, shoot)
			debug.Debug("Delegating to 'target' subcommand: %s", cmdString)
			if err := vfs.WriteFile(fs.FS, con.InternalCallPath, []byte(cmdString), vfs.ModePerm); err != nil {
				libutils.Fatal(1, "error writing to internal call path: %w", err)
			}
		}
		if !matched {
			libutils.Fatal(1, "no regex found in plugin configuration that matches the URL '%s'", url)
		}
	},
}
