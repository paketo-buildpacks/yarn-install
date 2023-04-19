package berry

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

type BerryInstallProcess struct {
	executable Executable
	summer     Summer
	logger     scribe.Emitter
}

func NewBerryInstallProcess(executable Executable, summer Summer, logger scribe.Emitter) BerryInstallProcess {
	return BerryInstallProcess{
		executable: executable,
		summer:     summer,
		logger:     logger,
	}
}

func (ip BerryInstallProcess) ShouldRun(workingDir string, metadata map[string]interface{}) (run bool, sha string, err error) {
	exists, err := fs.Exists(filepath.Join(workingDir, "yarn.lock"))
	if !exists {
		// ip.logger.Action("yarn.lock -> Not found")
		// ip.logger.Break()
		return true, "", nil
	} else if err != nil {
		// return true, "", fmt.Errorf("unable to read yarn.lock file: %w", err)
		panic(err)
	}

	buffer := bytes.NewBuffer(nil)

	err = ip.executable.Execute(pexec.Execution{
		Args:   []string{"info", "-AR", "--json"},
		Stdout: buffer,
		Stderr: buffer,
		Dir:    workingDir,
	})
	if err != nil {
		return true, "", fmt.Errorf("failed to execute yarn info output:\n%s\nerror: %s", buffer.String(), err)
	}

	nodeEnv := os.Getenv("NODE_ENV")
	buffer.WriteString(nodeEnv)

	file, err := os.CreateTemp("", "pkg-info-file")
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

// TODO: Pull this out into common package
func (ip BerryInstallProcess) SetupModules(workingDir, currentModulesLayerPath, nextModulesLayerPath string) (string, error) {

	if currentModulesLayerPath != "" {
		err := fs.Copy(filepath.Join(currentModulesLayerPath, "node_modules"), filepath.Join(nextModulesLayerPath, "node_modules"))
		if err != nil {
			return "", fmt.Errorf("failed to copy node_modules directory: %w", err)
		}

	} else {

		file, err := os.Lstat(filepath.Join(workingDir, "node_modules"))
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return "", fmt.Errorf("failed to stat node_modules directory: %w", err)
			}

		}

		if file != nil && file.Mode()&os.ModeSymlink == os.ModeSymlink {
			err = os.RemoveAll(filepath.Join(workingDir, "node_modules"))
			if err != nil {
				//not tested
				return "", fmt.Errorf("failed to remove node_modules symlink: %w", err)
			}
		}

		err = os.MkdirAll(filepath.Join(workingDir, "node_modules"), os.ModePerm)
		if err != nil {
			//not directly tested
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

func (ip BerryInstallProcess) Execute(workingDir, modulesLayerPath string, launch bool) error {
	environment := os.Environ()
	environment = append(environment, fmt.Sprintf("PATH=%s%c%s", os.Getenv("PATH"), os.PathListSeparator, filepath.Join("node_modules", ".bin")))
	environment = append(environment, fmt.Sprintf("YARN_CACHE_FOLDER=%s", filepath.Join(modulesLayerPath, "yarn-pkgs")))

	installArgs := []string{"install"}

	//TODO: Set --immutable flag based on user-set configuration

	if !launch {
		//TODO: This is deprecated, figure out an alternative.
		// installArgs = append(installArgs, "--production", "false")
	}

	//TODO: Update modules layer specification
	// installArgs = append(installArgs, "--modules-folder", filepath.Join(modulesLayerPath, "node_modules"))

	ip.logger.Subprocess("Running yarn %s", strings.Join(installArgs, " "))

	buffer := bytes.NewBuffer(nil)
	err := ip.executable.Execute(pexec.Execution{
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

//func (ip BerryInstallProcess) executePnP(workingDir, modulesLayerPath string, launch bool) error {
//	environment := os.Environ()
//	environment = append(environment, fmt.Sprintf("PATH=%s%c%s", os.Getenv("PATH"), os.PathListSeparator, filepath.Join("node_modules", ".bin")))

//	buffer := bytes.NewBuffer(nil)

//	installArgs := []string{"install"}

//	//TODO: Set --immutable flag based on user-set configuration

//	if !launch {
//		//TODO: This is deprecated, figure out an alternative.
//		// installArgs = append(installArgs, "--production", "false")
//	}

//	//TODO: Update modules layer specification
//	// installArgs = append(installArgs, "--modules-folder", filepath.Join(modulesLayerPath, "node_modules"))

//	ip.logger.Subprocess("Running yarn %s", strings.Join(installArgs, " "))

//	buffer = bytes.NewBuffer(nil)
//	err := ip.executable.Execute(pexec.Execution{
//		Args:   installArgs,
//		Env:    environment,
//		Stdout: buffer,
//		Stderr: buffer,
//		Dir:    workingDir,
//	})
//	if err != nil {
//		ip.logger.Action("%s", buffer)
//		return fmt.Errorf("failed to execute yarn install: %w", err)
//	}

//	return nil
//}

//func (ip BerryInstallProcess) executeNodeModules(workingDir, modulesLayerPath string, launch bool) error {
//	environment := os.Environ()
//	environment = append(environment, fmt.Sprintf("PATH=%s%c%s", os.Getenv("PATH"), os.PathListSeparator, filepath.Join("node_modules", ".bin")))

//	buffer := bytes.NewBuffer(nil)

//	installArgs := []string{"install"}

//	ip.logger.Subprocess("Running yarn %s", strings.Join(installArgs, " "))

//	buffer = bytes.NewBuffer(nil)
//	err := ip.executable.Execute(pexec.Execution{
//		Args:   installArgs,
//		Env:    environment,
//		Stdout: buffer,
//		Stderr: buffer,
//		Dir:    workingDir,
//	})
//	if err != nil {
//		ip.logger.Action("%s", buffer)
//		return fmt.Errorf("failed to execute yarn install: %w", err)
//	}

//	return nil
//}
