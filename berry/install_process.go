package berry

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	return false, "", nil
}

func (ip BerryInstallProcess) SetupModules(workingDir, currentModulesLayerPath, nextModulesLayerPath, tempDir string) (string, error) {
	return "", nil
}

// The build process here relies on yarn install ... --frozen-lockfile note that
// even if we provide a node_modules directory we must run a 'yarn install' as
// this is the ONLY way to rebuild native extensions.
func (ip BerryInstallProcess) Execute(workingDir, modulesLayerPath string, launch bool) error {
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
