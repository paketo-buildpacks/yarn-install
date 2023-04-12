package yarninstall

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/paketo-buildpacks/packit/v2"
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

//go:generate faux --interface YarnrcYmlParser --output fakes/yarnrc_yml_parser.go
type YarnrcYmlParser interface {
	Parse(path string) (nodeLinker string, err error)
}

//go:generate faux --interface PathParser --output fakes/path_parser.go
type PathParser interface {
	Get(path string) (projectPath string, err error)
}

func Detect(projectPathParser PathParser, versionParser VersionParser, yarnrcYmlParser YarnrcYmlParser) packit.DetectFunc {
	return func(context packit.DetectContext) (packit.DetectResult, error) {
		projectPath, err := projectPathParser.Get(context.WorkingDir)
		if err != nil {
			return packit.DetectResult{}, err
		}

		hasYarnrcYml := true
		linker, err := yarnrcYmlParser.Parse(filepath.Join(projectPath, ".yarnrc.yml"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				hasYarnrcYml = false
			} else {
				return packit.DetectResult{}, err
			}
		}

		_, err = os.Stat(filepath.Join(projectPath, "yarn.lock"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if !hasYarnrcYml {
					return packit.DetectResult{}, packit.Fail
				}
			} else {
				return packit.DetectResult{}, err
			}
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

		var provides []packit.BuildPlanProvision

		if !hasYarnrcYml || linker == "node-modules" || linker == "pnpm" {
			provides = append(provides, packit.BuildPlanProvision{Name: PlanDependencyNodeModules})
		} else {
			provides = append(provides, packit.BuildPlanProvision{Name: PlanDependencyYarnPkgs})
		}

		return packit.DetectResult{
			Plan: packit.BuildPlan{
				Provides: provides,
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
