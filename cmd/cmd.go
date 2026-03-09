package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	libcontext "github.com/Diaphteiros/kw/pluginlib/pkg/context"
	"github.com/Diaphteiros/kw/pluginlib/pkg/debug"
	libutils "github.com/Diaphteiros/kw/pluginlib/pkg/utils"
	"github.com/Diaphteiros/kw_garden/cmd/dashboard"
	"github.com/Diaphteiros/kw_garden/cmd/version"
	"github.com/Diaphteiros/kw_garden/pkg/config"
	"github.com/Diaphteiros/kw_garden/pkg/state"
)

var RootCmd = &cobra.Command{
	Use:                "kw_garden",
	DisableAutoGenTag:  true,
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	Short:              "A kubeswitcher plugin for Gardener landscapes",
	Long: `A kubeswitcher plugin for Gardener landscapes.
	
This is basically a wrapper around gardenctl, which makes it usable as a kubeswitcher plugin.
gardenctl has to be installed and configured for this to work. See https://github.com/gardener/gardenctl-v2 for more information.

All arguments are passed through to gardenctl.`,
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

		// read state from kubeswitcher
		// if it is from this plugin, overwrite gardenctl state with it
		// otherwise, load gardenctl state
		// so we can later compare to see if anything has changed
		gardenctlSessionId := fmt.Sprintf("kw-%s", con.SessionID)
		oldState := &state.GardenctlState{}
		gardenctlStateDir := state.GardenctlStateDir(gardenctlSessionId)
		ok, err := oldState.LoadFromKubeswitcher(con)
		if err != nil {
			libutils.Fatal(1, "error loading kubeswitcher state: %w", err)
		}
		if ok {
			debug.Debug("Loaded plugin state from kubeswitcher, writing it to gardenctl state dir")
			if err := oldState.StoreToGardenctl(gardenctlStateDir); err != nil {
				libutils.Fatal(1, "error overwriting gardenctl state: %w", err)
			}
		} else {
			debug.Debug("Unable to load plugin state from kubeswitcher (either not found or current state is from a different plugin)")
		}

		// build command environment
		env := map[string]string{
			"GCTL_SESSION_ID": gardenctlSessionId,
		}
		if cfg.ConfigDir != "" {
			env["GCTL_HOME"] = cfg.ConfigDir
		}
		if cfg.ConfigFileName != "" {
			env["GCTL_CONFIG_NAME"] = cfg.ConfigFileName
		}

		args = improveTargetArguments(cmd, cfg, oldState, args)

		// prepare gardenctl execution
		bin := exec.Command(cfg.Binary, args...)
		bin.Env = append(bin.Env, os.Environ()...) // add current env vars
		debug.Debug("environment (in addition to parent process environment):\n")
		for k, v := range env { // add custom env vars
			debug.Debug("  %s=%s\n", k, v)
			bin.Env = append(bin.Env, fmt.Sprintf("%s=%s", k, v))
		}

		errBuffer := libutils.NewWriteBuffer()
		outBuffer := libutils.NewWriteBuffer()

		// set channels
		bin.Stderr = errBuffer
		bin.Stdout = outBuffer
		bin.Stdin = cmd.InOrStdin()

		// run command
		debug.Debug("starting gardenctl execution: %s %s", bin.Path, strings.Join(bin.Args, " "))
		if err := bin.Run(); err != nil {
			errBuffer.Flush(cmd.ErrOrStderr())
			libutils.Fatal(1, "error running gardenctl: %w\n", err)
		}
		debug.Debug("finished gardenctl execution")

		// read new state from gardenctl state dir
		newState := &state.GardenctlState{}
		if ok, err := newState.LoadFromGardenctl(gardenctlStateDir); err != nil {
			libutils.Fatal(1, "error loading gardenctl state: %w", err)
		} else if !ok {
			debug.Debug("No gardenctl state found")
		}
		changed := !oldState.Equal(newState)
		if changed {
			debug.Debug("gardenctl state has changed")
			// throw error if gardenctl state has changed, but does not have a kubeconfig
			if len(newState.KubeconfigData) == 0 {
				libutils.Fatal(1, "gardenctl state has changed, but no kubeconfig found")
			}
			// write new state to kubeswitcher state
			if err := newState.StoreToKubeswitcher(con); err != nil {
				libutils.Fatal(1, "error writing plugin state: %w", err)
			}
		} else {
			debug.Debug("gardenctl state has not changed")
			// gardenctl command was probably a read-only command
			// print output to stdout
			outBuffer.Flush(cmd.OutOrStdout(), "WARN The KUBECONFIG environment variable does not point to the current target of gardenctl. Run `gardenctl kubectl-env --help` on how to configure the KUBECONFIG environment variable accordingly\n", "")
		}
	},
}

func init() {
	RootCmd.SetOut(os.Stdout)
	RootCmd.SetErr(os.Stderr)
	RootCmd.SetIn(os.Stdin)

	RootCmd.AddCommand(dashboard.DashboardCmd)
	RootCmd.AddCommand(version.VersionCmd)
}

// getClientForGarden runs 'gardenctl target --garden <gardenName>' and returns a client generated from the resulting gardenctl state.
// This function uses a dummy session ID to avoid overwriting the actual gardenctl state.
func getClientForGarden(cmd *cobra.Command, cfg *config.GardenctlConfig, gardenName string) (client.Client, error) {
	debug.Debug("targeting garden '%s' to generate client", gardenName)

	tmpStateName := "kw-tmp"
	env := map[string]string{
		"GCTL_SESSION_ID": tmpStateName,
	}
	if cfg.ConfigDir != "" {
		env["GCTL_HOME"] = cfg.ConfigDir
	}
	if cfg.ConfigFileName != "" {
		env["GCTL_CONFIG_NAME"] = cfg.ConfigFileName
	}

	// prepare gardenctl execution
	bin := exec.Command(cfg.Binary, "target", "--garden", gardenName)
	bin.Env = append(bin.Env, os.Environ()...) // add current env vars
	debug.Debug("environment (in addition to parent process environment):\n")
	for k, v := range env { // add custom env vars
		debug.Debug("  %s=%s\n", k, v)
		bin.Env = append(bin.Env, fmt.Sprintf("%s=%s", k, v))
	}

	errBuffer := libutils.NewWriteBuffer()
	outBuffer := libutils.NewWriteBuffer()

	// set channels
	bin.Stderr = errBuffer
	bin.Stdout = outBuffer
	bin.Stdin = cmd.InOrStdin()

	// run command
	debug.Debug("starting intermediate gardenctl execution: %s %s", bin.Path, strings.Join(bin.Args, " "))
	if err := bin.Run(); err != nil {
		errBuffer.Flush(cmd.ErrOrStderr())
		libutils.Fatal(1, "error running gardenctl: %w\n", err)
	}
	debug.Debug("finished intermediate gardenctl execution")

	intermediateState := &state.GardenctlState{}
	ok, err := intermediateState.LoadFromGardenctl(state.GardenctlStateDir(tmpStateName))
	if err != nil {
		libutils.Fatal(1, "error loading gardenctl state: %w", err)
	}
	if !ok {
		debug.Debug("No gardenctl state found after targeting garden")
	}
	return intermediateState.GenerateClient()
}

func rbacSubjectToString(subject *rbacv1.Subject) string {
	if subject == nil {
		return "<unknown>"
	}
	switch subject.Kind {
	case rbacv1.UserKind:
		return fmt.Sprintf("[User] %s", subject.Name)
	case rbacv1.GroupKind:
		return fmt.Sprintf("[Group] %s", subject.Name)
	case rbacv1.ServiceAccountKind:
		return fmt.Sprintf("[ServiceAccount] %s/%s", subject.Namespace, subject.Name)
	}
	return fmt.Sprintf("[Unknown] %s", subject.Name)
}

func projectMemberToString(member *gardenv1beta1.ProjectMember) string {
	if member == nil {
		return "<unknown>"
	}
	res := rbacSubjectToString(&member.Subject)
	sb := strings.Builder{}
	sb.WriteString(" (")
	appendedSomething := false
	for _, role := range append([]string{member.Role}, member.Roles...) {
		if role == "" {
			continue
		}
		if appendedSomething {
			sb.WriteString(", ")
		}
		sb.WriteString(role)
		appendedSomething = true
	}
	if appendedSomething {
		sb.WriteString(")")
		res += sb.String()
	}
	return res
}
