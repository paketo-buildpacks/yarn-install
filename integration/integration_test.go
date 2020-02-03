package integration

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/yarn-cnb/modules"

	"github.com/cloudfoundry/dagger"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
)

var (
	bpDir, yarnURI, nodeURI string
)

func TestIntegration(t *testing.T) {
	var err error
	Expect := NewWithT(t).Expect
	bpDir, err = dagger.FindBPRoot()
	Expect(err).NotTo(HaveOccurred())
	yarnURI, err = dagger.PackageBuildpack(bpDir)
	Expect(err).ToNot(HaveOccurred())
	defer dagger.DeleteBuildpack(yarnURI)

	nodeURI, err = dagger.GetLatestBuildpack("node-engine-cnb")
	Expect(err).ToNot(HaveOccurred())
	defer dagger.DeleteBuildpack(nodeURI)

	spec.Run(t, "Integration", testIntegration, spec.Report(report.Terminal{}))
}

func testIntegration(t *testing.T, when spec.G, it spec.S) {
	var Expect func(interface{}, ...interface{}) GomegaAssertion
	it.Before(func() {
		Expect = NewWithT(t).Expect
	})

	when("when the node_modules are NOT vendored", func() {
		it.Pend("should build a working OCI image for a simple app", func() {
			app, err := dagger.PackBuild(filepath.Join("testdata", "simple_app"), nodeURI, yarnURI)
			Expect(err).ToNot(HaveOccurred())
			defer app.Destroy()

			Expect(app.Start()).To(Succeed())

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("Hello, World!"))
		})
	})

	when("when the node_modules are vendored", func() {
		it.Pend("should build a working OCI image for a simple app", func() {
			app, err := dagger.PackBuild(filepath.Join("testdata", "vendored"), nodeURI, yarnURI)
			Expect(err).ToNot(HaveOccurred())
			defer app.Destroy()

			Expect(app.Start()).To(Succeed())

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("Hello, World!"))
		})
	})

	when("using yarn workspaces", func() {
		it.Pend("should correctly install node modules in respective workspaces", func() {
			app, err := dagger.PackBuild(filepath.Join("testdata", "with_yarn_workspaces"), nodeURI, yarnURI)
			Expect(err).ToNot(HaveOccurred())
			defer app.Destroy()

			Expect(app.Start()).To(Succeed())

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("Package A value 1"))
			Expect(body).To(ContainSubstring("Package A value 2"))
		})
	})

	when("the app is pushed twice", func() {
		it.Pend("does not reinstall node_modules when yarn.lock is not changed", func() {
			appDir := filepath.Join("testdata", "simple_app")
			app, err := dagger.PackBuild(appDir, nodeURI, yarnURI)
			Expect(err).ToNot(HaveOccurred())
			defer app.Destroy()

			Expect(app.BuildLogs()).To(MatchRegexp(fmt.Sprintf("%s .*: Contributing to layer", modules.DirMetadata)))

			// pack rebuild
			app, err = dagger.PackBuildNamedImage(app.ImageName, appDir, nodeURI, yarnURI)
			Expect(err).NotTo(HaveOccurred())

			Expect(app.BuildLogs()).To(MatchRegexp(fmt.Sprintf("%s .*: Reusing cached layer", modules.DirMetadata)))
			Expect(app.BuildLogs()).NotTo(MatchRegexp(fmt.Sprintf("%s .*: Contributing to layer", modules.DirMetadata)))

			Expect(app.Start()).To(Succeed())

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("Hello, World!"))
		})
	})

	when("the app uses pre-gyp", func() {
		it.Pend("should build a working OCI image for a simple app", func() {
			app, err := dagger.PackBuild(filepath.Join("testdata", "yarn_pre_gyp"), nodeURI, yarnURI)
			Expect(err).ToNot(HaveOccurred())
			defer app.Destroy()

			Expect(app.Start()).To(Succeed())

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("Hello, World!"))
		})
	})
}
