package yarn

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/cloudfoundry/packit"
)

const (
	PlanDependencyYarn = "yarn"
	PlanDependencyNode = "node"
)

type BuildPlanMetadata struct {
	VersionSource string `toml:"version-source"`
	Build         bool   `toml:"build"`
	Launch        bool   `toml:"launch"`
}

//go:generate faux --interface VersionParser --output fakes/version_parser.go
type VersionParser interface {
	ParseVersion(path string) (version string, err error)
}

func Detect(parser VersionParser) packit.DetectFunc {
	return func(context packit.DetectContext) (packit.DetectResult, error) {
		_, err := os.Stat(filepath.Join(context.WorkingDir, "yarn.lock"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return packit.DetectResult{}, packit.Fail
			}

			return packit.DetectResult{}, err
		}

		nodeVersion, err := parser.ParseVersion(filepath.Join(context.WorkingDir, "package.json"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return packit.DetectResult{}, packit.Fail
			}

			return packit.DetectResult{}, err
		}

		nodeRequirement := packit.BuildPlanRequirement{
			Name: PlanDependencyNode,
			Metadata: BuildPlanMetadata{
				Build:  true,
				Launch: true,
			},
		}

		if nodeVersion != "" {
			nodeRequirement.Version = nodeVersion
			nodeRequirement.Metadata = BuildPlanMetadata{
				VersionSource: "package.json",
				Build:         true,
				Launch:        true,
			}
		}

		return packit.DetectResult{
			Plan: packit.BuildPlan{
				Provides: []packit.BuildPlanProvision{
					{Name: PlanDependencyYarn},
				},
				Requires: []packit.BuildPlanRequirement{
					{Name: PlanDependencyYarn},
					nodeRequirement,
				},
			},
		}, nil
	}
}
