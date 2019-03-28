package integration

import (
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/dagger"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
)

func TestIntegration(t *testing.T) {
	spec.Run(t, "Integration", testIntegration, spec.Report(report.Terminal{}))
}

func testIntegration(t *testing.T, when spec.G, it spec.S) {
	var (
		bp     string
		nodeBP string
	)

	it.Before(func() {
		RegisterTestingT(t)
		var err error
		err = dagger.BuildCFLinuxFS3()
		Expect(err).ToNot(HaveOccurred())

		bp, err = dagger.PackageBuildpack()
		Expect(err).ToNot(HaveOccurred())

		nodeBP, err = dagger.GetLatestBuildpack("nodejs-cnb")
		Expect(err).ToNot(HaveOccurred())
	})

	when("when the node_modules are vendored", func() {
		it("should build a working OCI image for a simple app", func() {
			app, err := dagger.PackBuild(filepath.Join("testdata", "simple_app_vendored"), nodeBP, bp)
			Expect(err).ToNot(HaveOccurred())
			defer app.Destroy()

			Expect(app.Start()).To(Succeed())

			_, _, err = app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	when("when the node_modules are not vendored", func() {
		it("should build a working OCI image for a simple app", func() {
			app, err := dagger.PackBuild(filepath.Join("testdata", "simple_app"), nodeBP, bp)
			Expect(err).ToNot(HaveOccurred())
			defer app.Destroy()

			Expect(app.Start()).To(Succeed())

			_, _, err = app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	when("using yarn workspaces", func() {
		it("should correctly install node modules in respective workspaces", func() {
			app, err := dagger.PackBuild(filepath.Join("testdata", "yarn_with_workspaces"), nodeBP, bp)
			Expect(err).ToNot(HaveOccurred())
			defer app.Destroy()

			Expect(app.Start()).To(Succeed())

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("Package A value 1"))
			Expect(body).To(ContainSubstring("Package A value 2"))
		})
	})
}
