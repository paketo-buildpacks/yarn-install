package yarninstall_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitYarn(t *testing.T) {
	suite := spec.New("yarn", spec.Report(report.Terminal{}))
	suite("Build", testBuild)
	suite("CacheHandler", testCacheHandler)
	suite("Detect", testDetect)
	suite("InstallProcess", testInstallProcess)
	suite("PackageJSONParser", testPackageJSONParser)
	suite("ProjectPathParser", testProjectPathParser)
	suite.Run(t)
}
