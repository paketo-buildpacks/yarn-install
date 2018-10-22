package detect

import (
	"fmt"
	"path/filepath"

	libbuildpackV3 "github.com/buildpack/libbuildpack"
	"github.com/cloudfoundry/libbuildpack"
	"github.com/cloudfoundry/yarn-cnb/package_json"
)

const NodeDependency = "node"
const YarnDependency = "yarn"

func UpdateBuildPlan(libDetect *libbuildpackV3.Detect) error {
	yarnLockPath := filepath.Join(libDetect.Application.Root, "yarn.lock")
	if exists, err := libbuildpack.FileExists(yarnLockPath); err != nil {
		return fmt.Errorf("error checking filepath %s", yarnLockPath)
	} else if !exists {
		return fmt.Errorf("no yarn.lock found at %s", yarnLockPath)
	}

	packageJSONPath := filepath.Join(libDetect.Application.Root, "package.json")
	if exists, err := libbuildpack.FileExists(packageJSONPath); err != nil {
		return fmt.Errorf("error checking filepath %s", packageJSONPath)
	} else if !exists {
		return fmt.Errorf("no package.json found at %s", packageJSONPath)
	}

	pkgJSON, err := package_json.LoadPackageJSON(packageJSONPath, libDetect.Logger)
	if err != nil {
		return err
	}

	libDetect.BuildPlan[NodeDependency] = libbuildpackV3.BuildPlanDependency{
		Version: pkgJSON.Engines.Node,
		Metadata: libbuildpackV3.BuildPlanDependencyMetadata{
			"build":  true,
			"launch": true,
		},
	}

	libDetect.BuildPlan[YarnDependency] = libbuildpackV3.BuildPlanDependency{
		Version: pkgJSON.Engines.Yarn,
		Metadata: libbuildpackV3.BuildPlanDependencyMetadata{
			"launch": true,
		},
	}

	return nil
}
