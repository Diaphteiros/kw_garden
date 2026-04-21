package version

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"sigs.k8s.io/yaml"

	libutils "github.com/Diaphteiros/kw/pluginlib/pkg/utils"

	"github.com/Diaphteiros/kw_garden/internal/version"
)

// variables for holding the flags
var (
	output libutils.OutputFormat
)

// VersionCmd represents the version command
var VersionCmd = &cobra.Command{
	Use:     "version",
	Aliases: []string{"v"},
	Args:    cobra.NoArgs,
	Short:   "Print the version",
	Long:    `Output the version of the CLI.`,
	Example: `  > kw garden version
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
	version: v1.2.3-dev-4516e7f4dee0861b3d1a31b53d3a8aabbd084f48`,
	Run: func(cmd *cobra.Command, args []string) {
		ver := version.Get()
		switch output {
		case libutils.OUTPUT_TEXT:
			cmd.Print(ver.String())
		case libutils.OUTPUT_JSON:
			data, err := json.Marshal(ver)
			if err != nil {
				libutils.Fatal(1, "error converting version to json: %w\n", err)
			}
			cmd.Println(string(data))
		case libutils.OUTPUT_YAML:
			data, err := yaml.Marshal(ver)
			if err != nil {
				libutils.Fatal(1, "error converting version to yaml: %w\n", err)
			}
			cmd.Print(string(data))
		default:
			libutils.Fatal(1, "unknown output format '%s'\n", string(output))
		}
	},
}

func init() {
	libutils.AddOutputFlag(VersionCmd.Flags(), &output, libutils.OUTPUT_TEXT)
}
