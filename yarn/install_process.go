package yarn

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry/packit/fs"
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
	err := os.MkdirAll(filepath.Join(workingDir, "node_modules"), os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create node_modules directory: %w", err)
	}

	err = fs.Move(filepath.Join(workingDir, "node_modules"), filepath.Join(layerPath, "node_modules"))
	if err != nil {
		return fmt.Errorf("failed to move node_modules directory to layer: %w", err)
	}

	err = os.Symlink(filepath.Join(layerPath, "node_modules"), filepath.Join(workingDir, "node_modules"))
	if err != nil {
		return fmt.Errorf("failed to symlink node_modules into working directory: %w", err)
	}

	stdout := bytes.NewBuffer(nil)
	_, _, err = ip.executable.Execute(pexec.Execution{
		Args:   []string{"config", "get", "yarn-offline-mirror"},
		Stdout: stdout,
	})
	if err != nil {
		return fmt.Errorf("failed to execute yarn config: %w", err)
	}

	installArgs := []string{"install", "--pure-lockfile", "--ignore-engines"}

	offlineMirrorPath := strings.TrimSpace(stdout.String())

	fileInfo, err := os.Stat(offlineMirrorPath)
	if err == nil {
		if fileInfo.IsDir() {
			installArgs = append(installArgs, "--offline")
		}
	} else {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to confirm existence of offline mirror directory: %w", err)
		}
	}

	_, _, err = ip.executable.Execute(pexec.Execution{
		Args: installArgs,
	})
	if err != nil {
		return fmt.Errorf("failed to execute yarn install: %w", err)
	}

	return nil
}
