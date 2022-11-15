package classic_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitClassic(t *testing.T) {
	suite := spec.New("Classic", spec.Report(report.Terminal{}))
	suite("ClassicBuild", testClassicBuild)
	suite("InstallProcess", testClassicInstallProcess)
	suite("PackageMgrConfigManager", testPackageManagerConfigurationManager)
	suite.Run(t)
}
