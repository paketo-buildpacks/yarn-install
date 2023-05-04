package yarninstall

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/fs"
)

type BuildPlanMetadata struct {
	Version       string `toml:"version"`
	VersionSource string `toml:"version-source"`
	Build         bool   `toml:"build"`
}

//go:generate faux --interface VersionParser --output fakes/version_parser.go
type VersionParser interface {
	ParseVersion(path string) (version string, err error)
}

//go:generate faux --interface PathParser --output fakes/path_parser.go
type PathParser interface {
	Get(path string) (projectPath string, err error)
}

func Detect(projectPathParser PathParser, versionParser VersionParser) packit.DetectFunc {
	return func(context packit.DetectContext) (packit.DetectResult, error) {
		projectPath, err := projectPathParser.Get(context.WorkingDir)
		if err != nil {
			return packit.DetectResult{}, err
		}

		exists, err := fs.Exists(filepath.Join(projectPath, "yarn.lock"))
		if err != nil {
			return packit.DetectResult{}, err
		}

		if !exists {
			return packit.DetectResult{}, packit.Fail.WithMessage("no 'yarn.lock' file found in the project path %s", projectPath)
		}

		nodeVersion, err := versionParser.ParseVersion(filepath.Join(projectPath, "package.json"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return packit.DetectResult{}, packit.Fail
			}

			return packit.DetectResult{}, err
		}

		nodeRequirement := packit.BuildPlanRequirement{
			Name: PlanDependencyNode,
			Metadata: BuildPlanMetadata{
				Build: true,
			},
		}

		if nodeVersion != "" {
			nodeRequirement.Metadata = BuildPlanMetadata{
				Version:       nodeVersion,
				VersionSource: "package.json",
				Build:         true,
			}
		}

		return packit.DetectResult{
			Plan: packit.BuildPlan{
				Provides: []packit.BuildPlanProvision{
					{Name: PlanDependencyNodeModules},
				},
				Requires: []packit.BuildPlanRequirement{
					nodeRequirement,
					{
						Name: PlanDependencyYarn,
						Metadata: BuildPlanMetadata{
							Build: true,
						},
					},
				},
			},
		}, nil
	}
}
