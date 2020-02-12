package integration_test

import (
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/dagger"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testIntegration(t *testing.T, when spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	when("when the node_modules are NOT vendored", func() {
		it("should build a working OCI image for a simple app", func() {
			app, err := dagger.NewPack(filepath.Join("testdata", "simple_app"),
				dagger.RandomImage(),
				dagger.SetBuildpacks(nodeURI, yarnURI),
				dagger.SetVerbose(),
			).Build()
			Expect(err).ToNot(HaveOccurred())
			defer app.Destroy()

			Expect(app.Start()).To(Succeed())

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("Hello, World!"))
		})
	})

	when("when the node_modules are vendored", func() {
		it("should build a working OCI image for a simple app", func() {
			app, err := dagger.NewPack(filepath.Join("testdata", "vendored"),
				dagger.RandomImage(),
				dagger.SetBuildpacks(nodeCachedURI, yarnCachedURI),
				dagger.SetVerbose(),
				dagger.SetOffline(),
			).Build()
			Expect(err).ToNot(HaveOccurred())
			defer app.Destroy()

			Expect(app.Start()).To(Succeed())

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("Hello, World!"))
		})
	})

	when("using yarn workspaces", func() {
		when("when offline", func() {
			it("should correctly install node modules in respective workspaces", func() {
				app, err := dagger.NewPack(filepath.Join("testdata", "with_yarn_workspaces_offline"),
					dagger.RandomImage(),
					dagger.SetBuildpacks(nodeCachedURI, yarnCachedURI),
					dagger.SetVerbose(),
					dagger.SetOffline(),
				).Build()

				Expect(err).ToNot(HaveOccurred())
				defer app.Destroy()

				Expect(app.Start()).To(Succeed())

				body, _, err := app.HTTPGet("/")
				Expect(err).ToNot(HaveOccurred())
				Expect(body).To(ContainSubstring("Package A value 1"))
				Expect(body).To(ContainSubstring("Package A value 2"))
			})
		})

		when("online", func() {
			it("should correctly install node modules in respective workspaces", func() {
				app, err := dagger.NewPack(filepath.Join("testdata", "with_yarn_workspaces"),
					dagger.RandomImage(),
					dagger.SetBuildpacks(nodeCachedURI, yarnCachedURI),
					dagger.SetVerbose(),
				).Build()

				Expect(err).ToNot(HaveOccurred())
				defer app.Destroy()

				Expect(app.Start()).To(Succeed())

				body, _, err := app.HTTPGet("/")
				Expect(err).ToNot(HaveOccurred())
				Expect(body).To(ContainSubstring("Package A value 1"))
				Expect(body).To(ContainSubstring("Package A value 2"))
			})
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
