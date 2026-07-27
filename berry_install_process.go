package yarninstall

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paketo-buildpacks/packit/v2/fs"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/scribe"
)

// BerryInstallProcess handles `yarn install` for Yarn Berry (v2+).
// Berry does not support --frozen-lockfile, --ignore-engines, or
// --modules-folder; it uses --immutable instead.
//
// If the app commits a Berry binary and declares it via yarnPath in
// .yarnrc.yml, that binary is invoked as `node <yarnPath>`. Otherwise the
// buildpack-delivered @yarnpkg/cli-dist binary (on PATH as `yarn`) is used.
type BerryInstallProcess struct {
	yarnExecutable Executable
	nodeExecutable Executable
	summer         Summer
	logger         scribe.Emitter
}

func NewBerryInstallProcess(yarnExecutable Executable, nodeExecutable Executable, summer Summer, logger scribe.Emitter) BerryInstallProcess {
	return BerryInstallProcess{
		yarnExecutable: yarnExecutable,
		nodeExecutable: nodeExecutable,
		summer:         summer,
		logger:         logger,
	}
}

// yarnPathFromRC reads the yarnPath value from .yarnrc.yml in workingDir.
// Returns an empty string (no error) when .yarnrc.yml doesn't exist or has no
// yarnPath entry — callers must treat "" as "not set".
func yarnPathFromRC(workingDir string) (string, error) {
	rcPath := filepath.Join(workingDir, ".yarnrc.yml")
	f, err := os.Open(rcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to open .yarnrc.yml: %w", err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "yarnPath:") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "yarnPath:"))
			value = strings.Trim(value, `"'`)
			if value == "" {
				return "", nil
			}
			if filepath.IsAbs(value) {
				return value, nil
			}
			return filepath.Join(workingDir, value), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read .yarnrc.yml: %w", err)
	}
	return "", nil
}

func (ip BerryInstallProcess) ShouldRun(workingDir string, metadata map[string]interface{}) (run bool, sha string, err error) {
	return shouldRun(workingDir, metadata, ip.summer, ip.logger, func() ([]byte, error) {
		return []byte(os.Getenv("NODE_ENV")), nil
	}, []string{".yarnrc.yml", ".pnp.cjs", "pnp.loader.mjs"})
}

func (ip BerryInstallProcess) SetupModules(workingDir, currentModulesLayerPath, nextModulesLayerPath string) (string, error) {
	appNodeModules := filepath.Join(workingDir, "node_modules")
	layerNodeModules := filepath.Join(nextModulesLayerPath, "node_modules")

	// remove any prior symlink. RemoveAll function, in case of a symlink, only unlinks the path.
	if err := os.RemoveAll(appNodeModules); err != nil {
		return "", fmt.Errorf("failed to clear node_modules in working directory: %w", err)
	}

	// If the node_modules directory is not empty, means that probably the build if statement ran.
	// In that case, we want to reuse the node_modules directory from the build if statement.
	if currentModulesLayerPath != "" {
		err := fs.Copy(filepath.Join(currentModulesLayerPath, "node_modules"), layerNodeModules)
		if err != nil {
			return "", fmt.Errorf("failed to copy node_modules directory: %w", err)
		}
	} else {
		if err := os.MkdirAll(layerNodeModules, os.ModePerm); err != nil {
			return "", fmt.Errorf("failed to create node_modules in layer: %w", err)
		}
	}

	// Point the app at this layer so yarn install writes into the correct place.
	if err := os.Symlink(layerNodeModules, appNodeModules); err != nil {
		return "", fmt.Errorf("failed to symlink node_modules to layer: %w", err)
	}

	return nextModulesLayerPath, nil
}

// Execute runs `yarn install --immutable` for Yarn Berry.
//
// If the app declares yarnPath in .yarnrc.yml pointing to a committed .cjs
// binary, that binary is invoked via `node <yarnPath>` — giving the app full
// control over the Berry version. Otherwise the buildpack-delivered Berry
// (on PATH as `yarn`) is used.
//
// SetupModules must be called first for pointing the node_modules directory at the correct layer.
func (ip BerryInstallProcess) Execute(workingDir, modulesLayerPath string, launch bool) error {
	environment := os.Environ()

	// Keep install-state.gz in the layer instead of the app directory as it is not expected to be committed.
	installStateDir := filepath.Join(modulesLayerPath, ".yarn")
	if err := os.MkdirAll(installStateDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create install-state directory in layer: %w", err)
	}
	environment = append(environment, fmt.Sprintf("YARN_INSTALL_STATE_PATH=%s", filepath.Join(installStateDir, "install-state.gz")))

	if !launch {
		environment = append(environment, "NODE_ENV=development")
	}

	// Determine which executable + args to use.
	var exe Executable
	var execArgs []string

	yarnBin, err := yarnPathFromRC(workingDir)
	if err != nil {
		return fmt.Errorf("failed to read yarnPath from .yarnrc.yml: %w", err)
	}

	if yarnBin != "" {
		// App provides its own Berry binary — invoke it via node.
		exe = ip.nodeExecutable
		execArgs = []string{yarnBin, "install", "--immutable"}
		ip.logger.Subprocess("Running 'node %s install --immutable' (app-provided yarnPath)", yarnBin)
	} else {
		exe = ip.yarnExecutable
		execArgs = []string{"install", "--immutable"}
		ip.logger.Subprocess("Running 'yarn install --immutable' (buildpack-provided Berry)")
	}

	err = exe.Execute(pexec.Execution{
		Args:   execArgs,
		Env:    environment,
		Stdout: ip.logger.ActionWriter,
		Stderr: ip.logger.ActionWriter,
		Dir:    workingDir,
	})
	if err != nil {
		return fmt.Errorf("failed to execute yarn install: %w", err)
	}

	// If yarn replaced the symlink with a real directory, move it into the layer.
	appNodeModules := filepath.Join(workingDir, "node_modules")
	layerNodeModules := filepath.Join(modulesLayerPath, "node_modules")
	info, statErr := os.Lstat(appNodeModules)
	if statErr == nil && info.Mode()&os.ModeSymlink == 0 && info.IsDir() {
		if err := os.RemoveAll(layerNodeModules); err != nil {
			return fmt.Errorf("failed to clear layer node_modules: %w", err)
		}
		if err := fs.Move(appNodeModules, layerNodeModules); err != nil {
			return fmt.Errorf("failed to move node_modules into layer: %w", err)
		}
	}

	return nil
}
