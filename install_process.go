package yarninstall

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paketo-buildpacks/packit/fs"
	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/paketo-buildpacks/packit/scribe"
)

//go:generate faux --interface Summer --output fakes/summer.go
type Summer interface {
	Sum(paths ...string) (string, error)
}

//go:generate faux --interface Executable --output fakes/executable.go
type Executable interface {
	Execute(pexec.Execution) error
}

type YarnInstallProcess struct {
	executable Executable
	summer     Summer
	logger     scribe.Logger
}

func NewYarnInstallProcess(executable Executable, summer Summer, logger scribe.Logger) YarnInstallProcess {
	return YarnInstallProcess{
		executable: executable,
		summer:     summer,
		logger:     logger,
	}
}

func (ip YarnInstallProcess) ShouldRun(workingDir string, metadata map[string]interface{}) (run bool, sha string, err error) {
	ip.logger.Subprocess("Process inputs:")

	_, err = os.Stat(filepath.Join(workingDir, "yarn.lock"))
	if os.IsNotExist(err) {
		ip.logger.Action("yarn.lock -> Not found")
		ip.logger.Break()
		return true, "", nil
	} else if err != nil {
		return true, "", fmt.Errorf("unable to read yarn.lock file: %w", err)
	}

	ip.logger.Action("yarn.lock -> Found")
	ip.logger.Break()

	sum, err := ip.summer.Sum(filepath.Join(workingDir, "yarn.lock"))
	if err != nil {
		return true, "", fmt.Errorf("unable to sum yarn.lock file: %w", err)
	}

	prevSHA, ok := metadata["cache_sha"].(string)
	if (ok && sum != prevSHA) || !ok {
		return true, sum, nil
	}

	return false, "", nil
}

// The build process here relies on yarn install ... --frozen-lockfile note that
// even if we provide a node_modules directory we must run a 'yarn install' as
// this is the ONLY way to rebuild native extensions.

func (ip YarnInstallProcess) Execute(workingDir, modulesLayerPath string) error {
	err := os.MkdirAll(filepath.Join(workingDir, "node_modules"), os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create node_modules directory: %w", err)
	}

	err = fs.Move(filepath.Join(workingDir, "node_modules"), filepath.Join(modulesLayerPath, "node_modules"))
	if err != nil {
		return fmt.Errorf("failed to move node_modules directory to layer: %w", err)
	}

	err = os.Symlink(filepath.Join(modulesLayerPath, "node_modules"), filepath.Join(workingDir, "node_modules"))
	if err != nil {
		return fmt.Errorf("failed to symlink node_modules into working directory: %w", err)
	}

	var variables []string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "PATH=") {
			env = fmt.Sprintf("%s%c%s", env, os.PathListSeparator, filepath.Join("node_modules", ".bin"))
		}

		variables = append(variables, env)
	}

	buffer := bytes.NewBuffer(nil)
	err = ip.executable.Execute(pexec.Execution{
		Args:   []string{"config", "get", "yarn-offline-mirror"},
		Stdout: buffer,
		Stderr: buffer,
		Env:    variables,
	})
	if err != nil {
		return fmt.Errorf("failed to execute yarn config output:\n%s\nerror: %s", buffer.String(), err)
	}

	installArgs := []string{"install", "--ignore-engines", "--frozen-lockfile"}

	info, err := os.Stat(strings.TrimSpace(buffer.String()))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to confirm existence of offline mirror directory: %w", err)
	}

	if info != nil && info.IsDir() {
		installArgs = append(installArgs, "--offline")
	}

	installArgs = append(installArgs, "--modules-folder", filepath.Join(modulesLayerPath, "node_modules"))
	ip.logger.Subprocess("Running yarn %s", strings.Join(installArgs, " "))

	buffer = bytes.NewBuffer(nil)
	err = ip.executable.Execute(pexec.Execution{
		Args:   installArgs,
		Env:    variables,
		Stdout: buffer,
		Stderr: buffer,
	})
	if err != nil {
		ip.logger.Action("%s", buffer)
		return fmt.Errorf("failed to execute yarn install: %w", err)
	}

	return nil
}
