package state

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/mandelsoft/vfs/pkg/vfs"

	libcontext "github.com/Diaphteiros/kw/pluginlib/pkg/context"
	"github.com/Diaphteiros/kw/pluginlib/pkg/debug"
	liberrors "github.com/Diaphteiros/kw/pluginlib/pkg/errors"
	"github.com/Diaphteiros/kw/pluginlib/pkg/fs"
	libstate "github.com/Diaphteiros/kw/pluginlib/pkg/state"
)

const (
	GardenctlKubeconfigFilename = "kubeconfig.yaml"
	GardenctlTargetFilename     = "target.yaml"
)

var GardenScheme *runtime.Scheme

func init() {
	GardenScheme = runtime.NewScheme()
	for _, err := range []error{
		clientgoscheme.AddToScheme(GardenScheme),
		gardenv1beta1.AddToScheme(GardenScheme),
	} {
		if err != nil {
			panic(fmt.Sprintf("error adding to scheme: %v", err))
		}
	}
}

func GardenctlStateDir(sessionId string) string {
	return filepath.Join(os.Getenv("TMPDIR"), "garden", "sessions", sessionId)
}

type GardenctlState struct {
	Garden           string `json:"garden,omitempty"`
	Project          string `json:"project,omitempty"`
	Seed             string `json:"seed,omitempty"`
	Shoot            string `json:"shoot,omitempty"`
	ControlPlaneFlag bool   `json:"controlPlane,omitempty"`
	KubeconfigName   string `json:"kubeconfigName,omitempty"`
	KubeconfigData   []byte `json:"-"`
}

func (gs *GardenctlState) DeepCopyInto(out *GardenctlState) {
	if gs == nil {
		return
	}
	out.ControlPlaneFlag = gs.ControlPlaneFlag
	out.Garden = gs.Garden
	out.Project = gs.Project
	out.Seed = gs.Seed
	out.Shoot = gs.Shoot
	out.KubeconfigName = gs.KubeconfigName
	out.KubeconfigData = bytes.Clone(gs.KubeconfigData)
}

func (gs *GardenctlState) Id(pluginName string) string {
	sb := strings.Builder{}
	sb.WriteString(pluginName)
	sb.WriteString(":")
	sb.WriteString(gs.Garden)
	if gs.Seed != "" {
		sb.WriteString("/seed:")
		sb.WriteString(gs.Seed)
	}
	if gs.Project != "" {
		sb.WriteString("/")
		sb.WriteString(gs.Project)
	}
	if gs.Shoot != "" {
		sb.WriteString("/")
		sb.WriteString(gs.Shoot)
	}
	if gs.ControlPlaneFlag {
		sb.WriteString("[cp]")
	}
	return sb.String()
}

func (gs *GardenctlState) Notification() string {
	sb := strings.Builder{}
	sb.WriteString("Switched to ")
	if gs.ControlPlaneFlag {
		sb.WriteString("controlplane of ")
	}
	if gs.Shoot != "" {
		sb.WriteString("shoot '")
		sb.WriteString(gs.Shoot)
		sb.WriteString("' in ")
	} else if gs.Seed != "" {
		sb.WriteString("seed '")
		sb.WriteString(gs.Seed)
		sb.WriteString("' in ")
	}
	if gs.Project != "" {
		sb.WriteString("project '")
		sb.WriteString(gs.Project)
		sb.WriteString("' in ")
	}
	sb.WriteString("landscape '")
	sb.WriteString(gs.Garden)
	sb.WriteString("'.")
	return sb.String()
}

// LoadFromGardenctl fills the receiver state object with the data from the gardenctl state directory.
// The first return value is true if any state was actually loaded, false otherwise.
func (gs *GardenctlState) LoadFromGardenctl(stateDir string) (bool, error) {
	debug.Debug("Loading gardenctl state from '%s'", stateDir)
	targetRead := false
	targetPath := filepath.Join(stateDir, GardenctlTargetFilename)
	data, err := vfs.ReadFile(fs.FS, targetPath)
	if err != nil {
		if !vfs.IsNotExist(err) {
			return false, fmt.Errorf("error reading gardenctl target file '%s': %w", targetPath, err)
		}
		debug.Debug("gardenctl target file '%s' does not exist", targetPath)
		gs.ControlPlaneFlag = false
		gs.Garden = ""
		gs.Project = ""
		gs.Seed = ""
		gs.Shoot = ""
	} else {
		if err := yaml.Unmarshal(data, gs); err != nil {
			return false, fmt.Errorf("error unmarshaling gardenctl target file '%s': %w", targetPath, err)
		}
		targetRead = true
	}
	kcfgPath := filepath.Join(stateDir, GardenctlKubeconfigFilename)
	kcfgPathResolved, err := fs.FS.Readlink(kcfgPath)
	debug.Debug("Gardenctl kubeconfig symlink '%s' resolved to '%s'", kcfgPath, kcfgPathResolved)
	if err != nil {
		if !vfs.IsNotExist(err) {
			return false, fmt.Errorf("error resolving kubeconfig symlink '%s': %w", kcfgPath, err)
		}
		debug.Debug("kubeconfig symlink '%s' does not exist", kcfgPath)
		return targetRead, nil
	}
	tmpStateDir := stateDir
	if !strings.HasSuffix(tmpStateDir, vfs.PathSeparatorString) {
		tmpStateDir += vfs.PathSeparatorString
	}
	gs.KubeconfigName = strings.TrimPrefix(kcfgPathResolved, tmpStateDir)
	gs.KubeconfigData, err = vfs.ReadFile(fs.FS, kcfgPathResolved)
	if err != nil {
		if !vfs.IsNotExist(err) {
			return false, fmt.Errorf("error reading kubeconfig file '%s': %w", kcfgPath, err)
		}
		debug.Debug("kubeconfig file '%s' does not exist", kcfgPathResolved)
	}
	return true, nil
}

// LoadFromKubeswitcher fills the receiver state object with the data from the kubeswitcher state.
// The first return value is true if any state was actually loaded, false otherwise.
func (gs *GardenctlState) LoadFromKubeswitcher(con *libcontext.Context) (bool, error) {
	debug.Debug("Loading gardenctl state from kubeswitcher state")
	ts, err := libstate.LoadTypedState[*GardenctlState](con.GenericStatePath, con.PluginStatePath, con.CurrentPluginName)
	if err != nil {
		return false, liberrors.IgnoreStateFromAnotherPluginError(fmt.Errorf("error loading kubeswitcher state: %w", err))
	}
	return gs.LoadFromKubeswitcherTypedState(con, ts)
}

// LoadFromKubeswitcherTypedState fills the receiver state object with the data from the already loaded kubeswitcher state.
// It additionally reads the kubeswitcher kubeconfig file.
func (gs *GardenctlState) LoadFromKubeswitcherTypedState(con *libcontext.Context, ts *libstate.TypedState[*GardenctlState]) (bool, error) {
	pluginStateRead := false
	if ts != nil {
		ts.PluginState.DeepCopyInto(gs)
		pluginStateRead = true
	}
	var err error
	gs.KubeconfigData, err = vfs.ReadFile(fs.FS, con.KubeconfigPath)
	if err != nil {
		if !vfs.IsNotExist(err) {
			return false, fmt.Errorf("error reading kubeconfig file '%s': %w", con.KubeconfigPath, err)
		}
		debug.Debug("kubeconfig file '%s' does not exist", con.KubeconfigPath)
		return pluginStateRead, nil
	}
	return true, nil
}

// StoreToGardenctl stores the receiver state object to the gardenctl state directory.
func (gs *GardenctlState) StoreToGardenctl(stateDir string) error {
	debug.Debug("Storing gardenctl state to '%s'", stateDir)
	// check if state dir exists, create if not
	exists, err := vfs.DirExists(fs.FS, stateDir)
	if err != nil {
		return fmt.Errorf("error checking if gardenctl state dir '%s' exists: %w", stateDir, err)
	}
	if !exists {
		debug.Debug("Gardenctl state dir '%s' does not exist, creating it", stateDir)
		if err := fs.FS.MkdirAll(stateDir, os.ModePerm|os.ModeDir); err != nil {
			return fmt.Errorf("error creating gardenctl state dir '%s': %w", stateDir, err)
		}
	}
	kcfgSymPath := filepath.Join(stateDir, GardenctlKubeconfigFilename)
	kcfgPath := gs.KubeconfigName
	if !strings.HasPrefix(kcfgPath, vfs.PathSeparatorString) {
		kcfgPath = filepath.Join(stateDir, kcfgPath)
	}
	tmp := &GardenctlState{}
	gs.DeepCopyInto(tmp)
	tmp.KubeconfigName = ""
	tmp.KubeconfigData = nil
	targetData, err := yaml.Marshal(tmp)
	if err != nil {
		return fmt.Errorf("error marshaling gardenctl target data: %w", err)
	}
	targetPath := filepath.Join(stateDir, GardenctlTargetFilename)
	if err := vfs.WriteFile(fs.FS, targetPath, targetData, os.ModePerm); err != nil {
		return fmt.Errorf("error writing gardenctl target file '%s': %w", targetPath, err)
	}
	if err := vfs.WriteFile(fs.FS, kcfgPath, gs.KubeconfigData, os.ModePerm); err != nil {
		return fmt.Errorf("error writing kubeconfig file '%s': %w", kcfgPath, err)
	}
	if err := fs.FS.Remove(kcfgSymPath); err != nil && !vfs.IsNotExist(err) {
		return fmt.Errorf("error removing kubeconfig symlink '%s': %w", kcfgSymPath, err)
	}
	if err := fs.FS.Symlink(kcfgPath, kcfgSymPath); err != nil {
		return fmt.Errorf("error creating kubeconfig symlink '%s': %w", kcfgSymPath, err)
	}
	return nil
}

// StoreToKubeswitcher stores the receiver state object to the kubeswitcher state.
func (gs *GardenctlState) StoreToKubeswitcher(con *libcontext.Context) error {
	debug.Debug("Storing gardenctl state to kubeswitcher state")
	if err := con.WritePluginState(gs); err != nil {
		return fmt.Errorf("error writing garden plugin state: %w", err)
	}
	if err := con.WriteKubeconfig(gs.KubeconfigData, gs.Notification()); err != nil {
		return fmt.Errorf("error writing kubeconfig: %w", err)
	}
	if err := con.WriteId(gs.Id(con.CurrentPluginName)); err != nil {
		return fmt.Errorf("error writing id: %w", err)
	}
	return nil
}

func (gs *GardenctlState) Equal(other *GardenctlState) bool {
	if gs == nil && other == nil {
		return true
	}
	if gs == nil || other == nil {
		return false
	}
	if gs.ControlPlaneFlag != other.ControlPlaneFlag {
		return false
	}
	if gs.Garden != other.Garden {
		return false
	}
	if gs.Project != other.Project {
		return false
	}
	if gs.Seed != other.Seed {
		return false
	}
	if gs.Shoot != other.Shoot {
		return false
	}
	if gs.KubeconfigName != other.KubeconfigName {
		return false
	}
	if !bytes.Equal(gs.KubeconfigData, other.KubeconfigData) {
		return false
	}
	return true
}

func (gs *GardenctlState) GenerateClient() (client.Client, error) {
	rest, err := clientcmd.RESTConfigFromKubeConfig(gs.KubeconfigData)
	if err != nil {
		return nil, fmt.Errorf("error creating REST config from kubeconfig data: %w", err)
	}
	cli, err := client.New(rest, client.Options{
		Scheme: GardenScheme,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating client from REST config: %w", err)
	}
	return cli, nil
}
