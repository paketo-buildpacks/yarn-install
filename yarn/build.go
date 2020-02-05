package yarn

import (
	"path/filepath"

	"github.com/cloudfoundry/packit"
)

//go:generate faux --interface DependencyInstaller --output fakes/dependency_installer.go
type DependencyInstaller interface {
	Install(dependencies []BuildpackMetadataDependency, cnbPath, layerPath string) error
}

//go:generate faux --interface InstallProcess --output fakes/install_process.go
type InstallProcess interface {
	Execute(workingDir, layerPath string) error
}

func Build(dependencyInstaller DependencyInstaller, installProcess InstallProcess) packit.BuildFunc {
	return func(context packit.BuildContext) (packit.BuildResult, error) {
		buildpack, err := ParseBuildpack(filepath.Join(context.CNBPath, "buildpack.toml"))
		if err != nil {
			return packit.BuildResult{}, err
		}

		yarnLayer, err := context.Layers.Get("yarn", packit.LaunchLayer)
		if err != nil {
			return packit.BuildResult{}, err
		}

		err = dependencyInstaller.Install(buildpack.Metadata.Dependencies, context.CNBPath, yarnLayer.Path)
		if err != nil {
			return packit.BuildResult{}, err
		}

		modulesLayer, err := context.Layers.Get("modules", packit.LaunchLayer)
		if err != nil {
			return packit.BuildResult{}, err
		}

		err = modulesLayer.Reset()
		if err != nil {
			return packit.BuildResult{}, err
		}

		err = installProcess.Execute(context.WorkingDir, modulesLayer.Path)
		if err != nil {
			return packit.BuildResult{}, err
		}

		return packit.BuildResult{
			Plan: context.Plan,
			Layers: []packit.Layer{
				yarnLayer,
				modulesLayer,
			},
			Processes: []packit.Process{
				{
					Type:    "web",
					Command: "yarn start",
				},
			},
		}, nil
	}
}
