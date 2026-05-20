package yarninstall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// isBerryApp returns true if the working directory's package.json declares a
// "packageManager" field for Yarn 2+, e.g. "yarn@4.14.1".
func isBerryApp(workingDir string) bool {
	f, err := os.Open(filepath.Join(workingDir, "package.json"))
	if err != nil {
		return false
	}
	defer f.Close()

	var pkg struct {
		PackageManager string `json:"packageManager"`
	}
	if err := json.NewDecoder(f).Decode(&pkg); err != nil {
		return false
	}

	if strings.HasPrefix(pkg.PackageManager, "yarn@") {
		ver := strings.TrimPrefix(pkg.PackageManager, "yarn@")
		major := strings.SplitN(ver, ".", 2)[0]
		return major >= "2"
	}
	return false
}

// SwitchingInstallProcess selects between a classic and a berry install process
// at runtime based on the app's package.json.
type SwitchingInstallProcess struct {
	classic InstallProcess
	berry   InstallProcess
}

func NewSwitchingInstallProcess(classic, berry InstallProcess) SwitchingInstallProcess {
	return SwitchingInstallProcess{classic: classic, berry: berry}
}

func (s SwitchingInstallProcess) ShouldRun(workingDir string, metadata map[string]interface{}) (bool, string, error) {
	return s.resolve(workingDir).ShouldRun(workingDir, metadata)
}

func (s SwitchingInstallProcess) SetupModules(workingDir, currentModulesLayerPath, nextModulesLayerPath string) (string, error) {
	return s.resolve(workingDir).SetupModules(workingDir, currentModulesLayerPath, nextModulesLayerPath)
}

func (s SwitchingInstallProcess) Execute(workingDir, modulesLayerPath string, launch bool) error {
	return s.resolve(workingDir).Execute(workingDir, modulesLayerPath, launch)
}

func (s SwitchingInstallProcess) resolve(workingDir string) InstallProcess {
	if isBerryApp(workingDir) {
		return s.berry
	}
	return s.classic
}
