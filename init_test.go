package yarninstall_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitYarnInstall(t *testing.T) {
	suite := spec.New("yarn-install", spec.Report(report.Terminal{}))
	suite("Build", testBuild)
	suite("Detect", testDetect)
	suite.Run(t)
}
