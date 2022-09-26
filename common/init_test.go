package common_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitCommon(t *testing.T) {
	suite := spec.New("common", spec.Report(report.Terminal{}))
	suite("CacheHandler", testCacheHandler)
	suite("PackageJSONParser", testPackageJSONParser)
	suite("YarnrcYmlParser", testYarnrcYmlParser)
	suite("ProjectPathParser", testProjectPathParser)
	suite("Symlinker", testSymlinker)
	suite.Run(t)
}
