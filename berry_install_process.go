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
	defer f.Close()

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

	nodeEnv := os.Getenv("NODE_ENV")

	file, err := os.CreateTemp("", "berry-node-env-*")
	if err != nil {
		return true, "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close temp file: %w", closeErr)
		}
	}()

	if _, writeErr := file.WriteString(nodeEnv); writeErr != nil {
		return true, "", fmt.Errorf("failed to write temp file: %w", writeErr)
	}

	pathsToSum := []string{
		filepath.Join(workingDir, "yarn.lock"),
		filepath.Join(workingDir, "package.json"),
		file.Name(),
	}
	for _, optional := range []string{".yarnrc.yml", ".pnp.cjs", "pnp.loader.mjs"} {
		p := filepath.Join(workingDir, optional)
		if _, statErr := os.Stat(p); statErr == nil {
			pathsToSum = append(pathsToSum, p)
		}
	}
	sum, err := ip.summer.Sum(pathsToSum...)
	if err != nil {
		return true, "", fmt.Errorf("unable to sum config files: %w", err)
	}

	prevSHA, ok := metadata["cache_sha"].(string)
	if (ok && sum != prevSHA) || !ok {
		return true, sum, nil
	}

	return false, "", nil
}

func (ip BerryInstallProcess) SetupModules(workingDir, currentModulesLayerPath, nextModulesLayerPath string) (string, error) {
	if currentModulesLayerPath != "" {
		err := fs.Copy(filepath.Join(currentModulesLayerPath, "node_modules"), filepath.Join(nextModulesLayerPath, "node_modules"))
		if err != nil {
			return "", fmt.Errorf("failed to copy node_modules directory: %w", err)
		}
	}
	return nextModulesLayerPath, nil
}

// Execute runs `yarn install --immutable` for Yarn Berry.
//
// If the app declares yarnPath in .yarnrc.yml pointing to a committed .cjs
// binary, that binary is invoked via `node <yarnPath>` — giving the app full
// control over the Berry version. Otherwise the buildpack-delivered Berry
// (on PATH as `yarn`) is used with YARN_IGNORE_PATH=1 to prevent any stale
// yarnPath from interfering.
func (ip BerryInstallProcess) Execute(workingDir, modulesLayerPath string, launch bool) error {
	environment := os.Environ()

	// Redirect Berry's install-state cache into the layer so it survives across
	// builds. The app is not expected to commit .yarn/install-state.gz.
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

	// Move node_modules from working directory into the layer so the layer can
	// be cached and reused across builds.
	srcNodeModules := filepath.Join(workingDir, "node_modules")
	dstNodeModules := filepath.Join(modulesLayerPath, "node_modules")
	if info, statErr := os.Lstat(srcNodeModules); statErr == nil && info.IsDir() {
		if err := fs.Move(srcNodeModules, dstNodeModules); err != nil {
			return fmt.Errorf("failed to move node_modules into layer: %w", err)
		}
	}

	return nil
}
