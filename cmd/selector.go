package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	authzv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	"github.com/Diaphteiros/kw/pluginlib/pkg/debug"
	"github.com/Diaphteiros/kw/pluginlib/pkg/selector"
	libutils "github.com/Diaphteiros/kw/pluginlib/pkg/utils"

	"github.com/Diaphteiros/kw_garden/pkg/config"
	"github.com/Diaphteiros/kw_garden/pkg/state"
)

const (
	NoneString    = "<none>"
	UnknownString = "<unknown>"
)

// improveTargetArguments modifies the given arguments by
// - replacing '-g', '-p', and '-s' that come after a 'target' argument with their respective long versions '--garden', '--project', and '--shoot'
// - interactively querying the user for garden, project, and/or shoot, if the respective arguments are missing
// It returns the modified arguments.
// This is meant to be used for the 'target' subcommand!
func improveTargetArguments(cmd *cobra.Command, cfg *config.GardenctlConfig, oldState *state.GardenctlState, args []string) []string {
	// convenience flag short versions
	// if 'target' is found in the arguments, it is expected to be the 'target' subcommand
	// in all later arguments, the all occurrences of each of the following short flags are replaced:
	// '-g' -> '--garden'
	// '-p' -> '--project'
	// '-s' -> '--shoot'
	// Additionally, figure out if any of the '--garden', '--project', or '--shoot' flags are missing their values
	// and mark it for being chosen interactively later
	tIdx := slices.Index(args, "target")
	if tIdx < 0 {
		// 'target' not found in arguments, return them as is
		return args
	}
	selectGarden := -10
	selectProject := -10
	selectShoot := -10
	if tIdx >= 0 {
		for i := tIdx + 1; i < len(args); i++ {
			switch args[i] {
			case "-g":
				args[i] = "--garden"
				if i+1 >= len(args) || args[i+1][0] == '-' {
					selectGarden = i
				}
			case "-p":
				args[i] = "--project"
				if i+1 >= len(args) || args[i+1][0] == '-' {
					selectProject = i
				}
			case "-s":
				args[i] = "--shoot"
				if i+1 >= len(args) || args[i+1][0] == '-' {
					selectShoot = i
				}
			}
		}
	}

	// interactively choose missing target parameters if needed
	var gardenName string
	if selectGarden < 0 && (selectProject >= 0 || selectShoot >= 0) {
		// if we might need the garden name later and will not select it interactively, let's figure it out now
		if selectGarden < 0 {
			gardenArgIdx := slices.Index(args, "--garden")
			if gardenArgIdx < 0 {
				gardenArgIdx = slices.Index(args, "-g") // could also be the short version
			}
			if gardenArgIdx >= 0 && gardenArgIdx+1 < len(args) && !strings.HasPrefix(args[gardenArgIdx+1], "-") {
				// garden is specified as an argument to this command
				gardenName = args[gardenArgIdx+1]
				debug.Debug("identified garden name '%s' from command arguments", gardenName)
			}
			if gardenName == "" {
				debug.Debug("no garden name specified in arguments, extracting it from current state")
				gardenName = oldState.Garden
				if gardenName == "" {
					debug.Debug("unable to determine garden name, this might cause problems later")
				} else {
					debug.Debug("identified garden name '%s' from current state", gardenName)
				}
			}
		}
	}
	if selectGarden >= 0 {
		errBuffer := libutils.NewWriteBuffer()
		debug.Debug("fetching and evaluating gardenctl config to select garden interactively")
		// parse garden config to figure out available gardens
		tmpCmd := exec.Command(cfg.Binary, "config", "view", "--output", "json")
		tmpCmd.Env = append(tmpCmd.Env, os.Environ()...)
		data, err := tmpCmd.Output()
		if err != nil {
			_ = errBuffer.Flush(cmd.ErrOrStderr())
			libutils.Fatal(1, "error running '%s %s': %w\n", tmpCmd.Path, strings.Join(tmpCmd.Args, " "), err)
		}
		gctlConfig := &gardenctlConfig{}
		if err := json.Unmarshal(data, gctlConfig); err != nil {
			libutils.Fatal(1, "error unmarshaling gardenctl config: %w\n", err)
		}
		gardens := []gardenSelector{}
		for _, g := range gctlConfig.Gardens {
			debug.Debug("evaluating garden '%s'", g.Identity)
			gs := gardenSelector{
				Garden:         g.Identity,
				KubeconfigPath: g.Kubeconfig,
			}
			// try to get api server from kubeconfig
			kcfg, err := libutils.ParseKubeconfigFromFile(g.Kubeconfig)
			if err != nil {
				debug.Debug("unable to parse kubeconfig '%s' for garden '%s': %v", g.Kubeconfig, g.Identity, err)
				continue
			}
			apiServer, err := libutils.GetCurrentApiserverHost(kcfg)
			if err != nil {
				debug.Debug("unable to get current apiserver from kubeconfig '%s' for garden '%s': %v", g.Kubeconfig, g.Identity, err)
				continue
			}
			gs.APIServer = apiServer
			gardens = append(gardens, gs)
		}

		// sort by identity, but reversed so the fuzzy finder shows them ordered from top to bottom
		slices.SortFunc(gardens, func(a, b gardenSelector) int {
			return -strings.Compare(a.Garden, b.Garden)
		})

		// select garden
		_, selectedGarden, err := selector.New[gardenSelector]().
			WithPrompt("Select Garden: ").
			WithFatalOnAbort("No garden selected.").
			WithFatalOnError("error selecting garden: %w").
			WithPreview(func(elem gardenSelector, _, _ int) string {
				return fmt.Sprintf("Garden: %s\n\nAPI Server: %s\nKubeconfig Path: %s", elem.Garden, elem.APIServer, elem.KubeconfigPath)
			}).
			From(gardens, func(elem gardenSelector) string {
				return elem.Garden
			}).
			Select()
		if err != nil {
			libutils.Fatal(1, "error selecting garden: %w\n", err)
		}

		// insert selected garden into args
		debug.Debug("selected garden: %s", selectedGarden.Garden)
		args = append(args[:selectGarden+1], append([]string{selectedGarden.Garden}, args[selectGarden+1:]...)...)
		selectProject += 1
		selectShoot += 1
	}
	var selectedProject projectSelector
	var gardenClient client.Client
	if selectProject >= 0 {
		// there are three different scenarios here:
		// 1. The kubeconfig was already targeting a garden before the command was executed.
		// 2. The garden is specified as an argument to this command.
		// 3. The garden has just been selected interactively in the previous step.
		// this only works if gardenctl targets a garden cluster
		debug.Debug("project name is missing, identifying garden to fetch available projects")
		var err error
		gardenClient, err = getClientForGarden(cmd, cfg, gardenName)
		if err != nil {
			libutils.Fatal(1, "error creating client for garden cluster: %w\n", err)
		}

		debug.Debug("fetching projects from targeted garden cluster to select project interactively")
		// check which projects the user has access to
		ssrr := &authzv1.SelfSubjectRulesReview{
			Spec: authzv1.SelfSubjectRulesReviewSpec{
				Namespace: "*",
			},
		}
		ssrr.SetName(gardenName)
		ctx := context.Background()
		if err := gardenClient.Create(ctx, ssrr); err != nil {
			libutils.Fatal(1, "error creating SelfSubjectRulesReview in garden '%s': %w\n", gardenName, err)
		}
		projectNames := sets.New[string]()
		for _, rule := range ssrr.Status.ResourceRules {
			// search for projects where the user has access
			if slices.Contains(rule.APIGroups, "core.gardener.cloud") && slices.Contains(rule.Resources, "projects") && slices.Contains(rule.Verbs, "get") {
				for _, rn := range rule.ResourceNames {
					projectNames.Insert(rn)
				}
			}
		}

		// try to fetch each project to get more information
		// but ignore errors, as it is not critical
		projects := make([]projectSelector, 0, projectNames.Len())
		for projectName := range projectNames {
			cur := projectSelector{
				Project: projectName,
			}
			project := &gardenv1beta1.Project{}
			project.SetName(cur.Project)
			if err := gardenClient.Get(ctx, client.ObjectKeyFromObject(project), project); err != nil {
				debug.Debug("error fetching project '%s': %v", cur.Project, err)
				continue
			}
			cur.Owner = rbacSubjectToString(project.Spec.Owner)
			for _, m := range project.Spec.Members {
				cur.Users = append(cur.Users, projectMemberToString(&m))
			}
			slices.Sort(cur.Users)
			cur.CreatedBy = rbacSubjectToString(project.Spec.CreatedBy)
			if project.Spec.Description != nil {
				cur.Description = *project.Spec.Description
			} else {
				cur.Description = NoneString
			}
			if project.Spec.Purpose != nil {
				cur.Purpose = *project.Spec.Purpose
			} else {
				cur.Purpose = NoneString
			}
			projects = append(projects, cur)
		}

		// sort projects in inverse alphabetical order so the fuzzy finder shows them from top to bottom
		slices.SortFunc(projects, func(a, b projectSelector) int {
			return -strings.Compare(a.Project, b.Project)
		})

		// select project
		_, selectedProject, err = selector.New[projectSelector]().
			WithPrompt("Select Project: ").
			WithFatalOnAbort("No project selected.").
			WithFatalOnError("error selecting project: %w").
			WithPreview(func(elem projectSelector, _, _ int) string {
				userString := strings.Join(elem.Users, "\n- ")
				if userString != "" {
					userString = "\n- " + userString
				}
				return fmt.Sprintf("Project: %s\n\nDescription: %s\nPurpose: %s\n\nCreated By: %s\nOwner:      %s\n\nUsers:%s", elem.Project, elem.Description, elem.Purpose, elem.CreatedBy, elem.Owner, userString)
			}).
			From(projects, func(elem projectSelector) string {
				return elem.Project
			}).
			Select()
		if err != nil {
			libutils.Fatal(1, "error selecting project: %w\n", err)
		}

		// insert selected project into args
		debug.Debug("selected project: %s", selectedProject.Project)
		args = append(args[:selectProject+1], append([]string{selectedProject.Project}, args[selectProject+1:]...)...)
		selectShoot += 1
	}
	if selectShoot >= 0 {
		// there are three different scenarios here:
		// 1. The kubeconfig was already targeting a project before the command was executed.
		// 2. The project is specified as an argument to this command.
		// 3. The project has just been selected interactively in the previous step.
		// this only works if gardenctl targets a garden cluster
		debug.Debug("shoot name is missing, identifying project namespace to fetch available shoots")
		var projectName string
		if selectProject >= 0 {
			// scenario 3: project was just selected interactively in the previous step
			projectName = selectedProject.Project
			debug.Debug("identified project name '%s' from previous selection", projectName)
		} else {
			projectArgIdx := slices.Index(args, "--project")
			if projectArgIdx < 0 {
				projectArgIdx = slices.Index(args, "-p") // could also be the short version
			}
			if projectArgIdx >= 0 && projectArgIdx+1 < len(args) && !strings.HasPrefix(args[projectArgIdx+1], "-") {
				// scenario 2: project is specified as an argument to this command
				projectName = args[projectArgIdx+1]
				debug.Debug("identified project name '%s' from command arguments", projectName)
			}
		}
		if projectName == "" {
			// scenario 1: kubeconfig was already targeting a project before the command was executed
			debug.Debug("no project name specified in arguments, extracting it from current state")
			projectName = oldState.Project
			if projectName == "" {
				libutils.Fatal(1, "unable to determine project name for shoot selection: no project specified in arguments and no project found in current state\n")
			} else {
				debug.Debug("identified project name '%s' from current state", projectName)
			}
		}
		debug.Debug("fetching project to determine project namespace")
		if gardenClient == nil {
			if gardenName == "" {
				libutils.Fatal(1, "unable to determine garden name for shoot selection: no garden specified in arguments and no garden found in current state\n")
			}
			var err error
			gardenClient, err = getClientForGarden(cmd, cfg, gardenName)
			if err != nil {
				libutils.Fatal(1, "error creating client for garden cluster: %w\n", err)
			}
		}
		project := &gardenv1beta1.Project{}
		project.SetName(projectName)
		if err := gardenClient.Get(cmd.Context(), client.ObjectKeyFromObject(project), project); err != nil {
			libutils.Fatal(1, "error fetching project '%s': %w\n", projectName, err)
		}
		if project.Spec.Namespace == nil || *project.Spec.Namespace == "" {
			libutils.Fatal(1, "project '%s' does not have a namespace specified\n", projectName)
		}
		projectNamespace := *project.Spec.Namespace

		debug.Debug("fetching shoots from targeted garden cluster and project namespace to select shoot interactively")
		shootList := &gardenv1beta1.ShootList{}
		if err := gardenClient.List(cmd.Context(), shootList, client.InNamespace(projectNamespace)); err != nil {
			libutils.Fatal(1, "error listing shoots in project '%s' (namespace '%s'): %w\n", projectName, projectNamespace, err)
		}

		shoots := make([]shootSelector, 0, len(shootList.Items))
		for _, shoot := range shootList.Items {
			cur := shootSelector{
				Name:       shoot.Name,
				Namespace:  shoot.Namespace,
				CreatedAt:  shoot.CreationTimestamp.Format(time.RFC3339),
				Hibernated: shoot.Status.IsHibernated,
			}
			if creator, ok := shoot.Annotations["gardener.cloud/created-by"]; ok {
				cur.CreatedBy = creator
			} else {
				cur.CreatedBy = UnknownString
			}
			if len(shoot.Labels) > 0 {
				labels := make([]string, 0, len(shoot.Labels))
				for k, v := range shoot.Labels {
					labels = append(labels, fmt.Sprintf("\n  %s: %s", k, v))
				}
				slices.Sort(labels)
				cur.Labels = strings.Join(labels, "")
			} else {
				cur.Labels = NoneString
			}
			if shoot.Spec.Kubernetes.Version != "" {
				cur.KubernetesVersion = shoot.Spec.Kubernetes.Version
			} else {
				cur.KubernetesVersion = UnknownString
			}
			shoots = append(shoots, cur)
		}
		slices.SortFunc(shoots, func(a, b shootSelector) int {
			return -strings.Compare(a.Name, b.Name)
		})

		// select shoot
		_, selectedShoot, err := selector.New[shootSelector]().
			WithPrompt("Select Shoot: ").
			WithFatalOnAbort("No shoot selected.").
			WithFatalOnError("error selecting shoot: %w").
			WithPreview(func(elem shootSelector, _, _ int) string {
				return fmt.Sprintf("Name: %s\nNamespace: %s\n\nCreated At: %s\nCreated By: %s\n\nKubernetes Version: %s\n\nHibernated: %t\n\nLabels:%s", elem.Name, elem.Namespace, elem.CreatedAt, elem.CreatedBy, elem.KubernetesVersion, elem.Hibernated, elem.Labels)
			}).
			From(shoots, func(elem shootSelector) string {
				return elem.Name
			}).
			Select()
		if err != nil {
			libutils.Fatal(1, "error selecting shoot: %w\n", err)
		}

		// insert selected shoot into args
		debug.Debug("selected shoot: %s", selectedShoot.Name)
		args = append(args[:selectShoot+1], append([]string{selectedShoot.Name}, args[selectShoot+1:]...)...)
	}

	return args
}

type gardenctlConfig struct {
	Gardens []gardenConfig `json:"gardens"`
}

type gardenConfig struct {
	Identity   string `json:"identity"`
	Kubeconfig string `json:"kubeconfig"`
}

type gardenSelector struct {
	Garden         string `json:"garden"`
	APIServer      string `json:"apiServer"`
	KubeconfigPath string `json:"kubeconfigPath"`
}

type projectSelector struct {
	Project     string   `json:"project"`
	CreatedBy   string   `json:"createdBy"`
	Description string   `json:"description"`
	Purpose     string   `json:"purpose"`
	Owner       string   `json:"owner"`
	Users       []string `json:"users"`
}

type shootSelector struct {
	Name              string `json:"name"`
	Namespace         string `json:"namespace"`
	CreatedBy         string `json:"createdBy"`
	CreatedAt         string `json:"createdAt"`
	Hibernated        bool   `json:"hibernated"`
	KubernetesVersion string `json:"kubernetesVersion"`
	Labels            string `json:"labels"`
}
