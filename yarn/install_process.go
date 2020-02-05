package yarn

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudfoundry/packit/pexec"
)

type YarnInstallProcess struct {
	executable Executable
}

func NewYarnInstallProcess(executable Executable) YarnInstallProcess {
	return YarnInstallProcess{
		executable: executable,
	}
}

//go:generate faux --interface Executable --output fakes/executable.go
type Executable interface {
	Execute(pexec.Execution) (stdout, stderr string, err error)
}

func (ip YarnInstallProcess) Execute(workingDir, layerPath string) error {
	err := os.Mkdir(filepath.Join(layerPath, "node_modules"), os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create node_modules directory: %w", err)
	}

	err = os.Symlink(filepath.Join(layerPath, "node_modules"), filepath.Join(workingDir, "node_modules"))
	if err != nil {
		return fmt.Errorf("failed to symlink node_modules into working directory: %w", err)
	}

	_, _, err = ip.executable.Execute(pexec.Execution{
		Args: []string{"install", "--pure-lockfile", "--ignore-engines"},
	})
	if err != nil {
		return fmt.Errorf("failed to execute yarn install: %w", err)
	}

	return nil
}
