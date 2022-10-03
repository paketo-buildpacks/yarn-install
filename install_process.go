package yarninstall

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paketo-buildpacks/packit/v2/fs"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/scribe"
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
	logger     scribe.Emitter
}

func NewYarnInstallProcess(executable Executable, summer Summer, logger scribe.Emitter) YarnInstallProcess {
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

	buffer := bytes.NewBuffer(nil)

	err = ip.executable.Execute(pexec.Execution{
		Args:   []string{"config", "list", "--silent"},
		Stdout: buffer,
		Stderr: buffer,
		Dir:    workingDir,
	})
	if err != nil {
		return true, "", fmt.Errorf("failed to execute yarn config output:\n%s\nerror: %s", buffer.String(), err)
	}

	nodeEnv := os.Getenv("NODE_ENV")
	buffer.WriteString(nodeEnv)

	file, err := os.CreateTemp("", "config-file")
	if err != nil {
		return true, "", fmt.Errorf("failed to create temp file for %s: %w", file.Name(), err)
	}
	defer file.Close()

	_, err = file.Write(buffer.Bytes())
	if err != nil {
		return true, "", fmt.Errorf("failed to write temp file for %s: %w", file.Name(), err)
	}

	sum, err := ip.summer.Sum(filepath.Join(workingDir, "yarn.lock"), file.Name())
	if err != nil {
		return true, "", fmt.Errorf("unable to sum config files: %w", err)
	}

	prevSHA, ok := metadata["cache_sha"].(string)
	if (ok && sum != prevSHA) || !ok {
		return true, sum, nil
	}

	return false, "", nil
}

func (ip YarnInstallProcess) SetupModules(workingDir, currentModulesLayerPath, nextModulesLayerPath string) (string, error) {
	if currentModulesLayerPath != "" {
		err := fs.Copy(filepath.Join(currentModulesLayerPath, "node_modules"), filepath.Join(nextModulesLayerPath, "node_modules"))
		if err != nil {
			return "", fmt.Errorf("failed to copy node_modules directory: %w", err)
		}

	} else {
		err := os.MkdirAll(filepath.Join(workingDir, "node_modules"), os.ModePerm)
		if err != nil {
			return "", fmt.Errorf("failed to create node_modules directory: %w", err)
		}

		err = fs.Move(filepath.Join(workingDir, "node_modules"), filepath.Join(nextModulesLayerPath, "node_modules"))
		if err != nil {
			return "", fmt.Errorf("failed to move node_modules directory to layer: %w", err)
		}

		err = os.Symlink(filepath.Join(nextModulesLayerPath, "node_modules"), filepath.Join(workingDir, "node_modules"))
		if err != nil {
			return "", fmt.Errorf("failed to symlink node_modules into working directory: %w", err)
		}
	}

	return nextModulesLayerPath, nil
}

// The build process here relies on yarn install ... --frozen-lockfile note that
// even if we provide a node_modules directory we must run a 'yarn install' as
// this is the ONLY way to rebuild native extensions.
func (ip YarnInstallProcess) Execute(workingDir, modulesLayerPath string, launch bool) error {
	environment := os.Environ()
	environment = append(environment, fmt.Sprintf("PATH=%s%c%s", os.Getenv("PATH"), os.PathListSeparator, filepath.Join("node_modules", ".bin")))

	buffer := bytes.NewBuffer(nil)
	err := ip.executable.Execute(pexec.Execution{
		Args:   []string{"config", "get", "yarn-offline-mirror"},
		Stdout: buffer,
		Stderr: buffer,
		Env:    environment,
		Dir:    workingDir,
	})
	if err != nil {
		return fmt.Errorf("failed to execute yarn config output:\n%s\nerror: %s", buffer.String(), err)
	}

	installArgs := []string{"install", "--ignore-engines", "--frozen-lockfile"}

	if !launch {
		installArgs = append(installArgs, "--production", "false")
	}

	// Parse yarn config get yarn-offline-mirror output
	// in case there are any warning lines in the output like:
	// warning You don't appear to have an internet connection.
	var offlineMirrorDir string
	for _, line := range strings.Split(buffer.String(), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "/") {
			offlineMirrorDir = strings.TrimSpace(line)
			break
		}
	}
	info, err := os.Stat(offlineMirrorDir)
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
		Env:    environment,
		Stdout: buffer,
		Stderr: buffer,
		Dir:    workingDir,
	})
	if err != nil {
		ip.logger.Action("%s", buffer)
		return fmt.Errorf("failed to execute yarn install: %w", err)
	}

	return nil
}
