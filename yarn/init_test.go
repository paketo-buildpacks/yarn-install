package yarn_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitYarn(t *testing.T) {
	suite := spec.New("yarn", spec.Report(report.Terminal{}))
	suite("Build", testBuild)
	suite("Buildpack", testBuildpack)
	suite("Detect", testDetect)
	suite("PackageJSONParser", testPackageJSONParser)
	suite("YarnDependencyInstaller", testYarnDependencyInstaller)
	suite("InstallProcess", testInstallProcess)
	suite.Run(t)
}
