package berry_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitBerry(t *testing.T) {
	suite := spec.New("Berry", spec.Report(report.Terminal{}))
	suite("BerryBuild", testBerryBuild)
	suite("InstallProcess", testBerryInstallProcess)
	suite.Run(t)
}
