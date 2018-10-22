package build

import (
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
	"testing"

	"github.com/buildpack/libbuildpack"
	. "github.com/onsi/gomega"
)

//go:generate mockgen -source=build.go -destination=mocks_test.go -package=build_test

func TestUnitBuild(t *testing.T) {
	RegisterTestingT(t)
	spec.Run(t, "Build", testBuild, spec.Report(report.Terminal{}))
}

func testBuild(t *testing.T, when spec.G, it spec.S) {
	//var (
	//	mockCtrl *gomock.Controller
	//	mockYarn  *MockModuleInstaller
	//	modules  build.Modules
	//	f        test.BuildFactory
	//	err      error
	//)
	//
	//it.Before(func() {
	//	f = test.NewBuildFactory(t)
	//
	//	mockCtrl = gomock.NewController(t)
	//	mockYarn = NewMockModuleInstaller(mockCtrl)
	//})
	//
	//it.After(func() {
	//	mockCtrl.Finish()
	//})
	//
	//when("NewModules", func() {
	//	it("returns true if a build plan exists", func() {
	//		f.AddBuildPlan(t, detect.YarnDependency, libbuildpack.BuildPlanDependency{})
	//
	//		_, ok, err := build.NewModules(f.Build, mockYarn)
	//		Expect(err).NotTo(HaveOccurred())
	//		Expect(ok).To(BeTrue())
	//	})
	//
	//	it("returns false if a build plan does not exist", func() {
	//		_, ok, err := build.NewModules(f.Build, mockYarn)
	//		Expect(err).NotTo(HaveOccurred())
	//		Expect(ok).To(BeFalse())
	//	})
	//})

	when("CreateLaunchMetadata", func() {
		it("returns launch metadata for running with yarn", func() {
			Expect(CreateLaunchMetadata()).To(Equal(libbuildpack.LaunchMetadata{
				Processes: libbuildpack.Processes{
					libbuildpack.Process{
						Type:    "web",
						Command: "yarn start",
					},
				},
			}))
		})
	})
}
