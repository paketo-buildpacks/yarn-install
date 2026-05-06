package yarninstall_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitYarn(t *testing.T) {
	suite := spec.New("yarn", spec.Report(report.Terminal{}))
	suite("Build", testBuild)
	suite("BerryInstallProcess", testBerryInstallProcess)
	suite("CacheHandler", testCacheHandler)
	suite("Detect", testDetect)
	suite("InstallProcess", testInstallProcess)
	suite("PackageManagerConfigurationManager", testPackageManagerConfigurationManager)
	suite("SwitchingInstallProcess", testSwitchingInstallProcess)
	suite("Symlinker", testSymlinker)
	suite.Run(t)
}
