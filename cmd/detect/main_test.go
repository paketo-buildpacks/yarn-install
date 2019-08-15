package main

import (
	"fmt"
	"github.com/cloudfoundry/libcfbuildpack/buildpackplan"
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/node-engine-cnb/node"
	"github.com/cloudfoundry/yarn-cnb/yarn"

	"github.com/cloudfoundry/libcfbuildpack/helper"

	"github.com/buildpack/libbuildpack/buildplan"
	"github.com/cloudfoundry/libcfbuildpack/detect"
	"github.com/cloudfoundry/libcfbuildpack/test"
	"github.com/cloudfoundry/yarn-cnb/modules"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitDetect(t *testing.T) {
	spec.Run(t, "Detect", testDetect, spec.Report(report.Terminal{}))
}

func testDetect(t *testing.T, when spec.G, it spec.S) {
	var factory *test.DetectFactory

	it.Before(func() {
		RegisterTestingT(t)
		factory = test.NewDetectFactory(t)
	})

	when("there is a yarn.lock and a package.json with a node version in engines", func() {
		const version string = "1.2.3"

		it.Before(func() {
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "yarn.lock"), 0666, "")).To(Succeed())

			packageJSONString := fmt.Sprintf(`{"engines": {"node" : "%s"}}`, version)
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "package.json"), 0666, packageJSONString)).To(Succeed())
		})

		it("should pass", func() {
			code, err := runDetect(factory.Detect)
			Expect(err).NotTo(HaveOccurred())

			Expect(code).To(Equal(detect.PassStatusCode))

			Expect(factory.Plans.Plan).To(Equal(buildplan.Plan{
				Requires:[]buildplan.Required{
					{
						Name: node.Dependency,
						Version:  version,
						Metadata: buildplan.Metadata{"build": true, "launch": true, buildpackplan.VersionSource: node.PackageJsonSource},
					},
					{
						Name: yarn.Dependency,
						Metadata: buildplan.Metadata{"launch": true},
					},
					{
						Name: modules.NodeModules,
						Metadata: buildplan.Metadata{"launch": true},
					},
				},
				Provides: []buildplan.Provided{
					{yarn.Dependency},
					{modules.NodeModules},
				},
			}))
		})
	})

	when("there is a yarn.lock and a package.json", func() {
		it.Before(func() {
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "yarn.lock"), 0666, "")).To(Succeed())
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "package.json"), 0666, "{}")).To(Succeed())
		})

		it("should pass with the default version of node", func() {
			code, err := runDetect(factory.Detect)
			Expect(err).NotTo(HaveOccurred())

			Expect(code).To(Equal(detect.PassStatusCode))

			Expect(factory.Plans.Plan).To(Equal(buildplan.Plan{
				Requires:[]buildplan.Required{
					{
						Name: node.Dependency,
						Version:  "",
						Metadata: buildplan.Metadata{"build": true, "launch": true, buildpackplan.VersionSource: node.PackageJsonSource},
					},
					{
						Name: yarn.Dependency,
						Metadata: buildplan.Metadata{"launch": true},
					},
					{
						Name: modules.NodeModules,
						Metadata: buildplan.Metadata{"launch": true},
					},
				},
				Provides: []buildplan.Provided{
					{yarn.Dependency},
					{modules.NodeModules},
				},
			}))
		})
	})

	when("there is a yarn.lock and no package.json", func() {
		it.Before(func() {
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "yarn.lock"), 0666, "")).To(Succeed())
		})

		it("should fail", func() {
			code, err := runDetect(factory.Detect)
			Expect(err).To(HaveOccurred())
			Expect(code).To(Equal(detect.FailStatusCode))
		})
	})

	when("there is no yarn.lock", func() {
		it("should fail", func() {
			code, err := runDetect(factory.Detect)
			Expect(err).To(HaveOccurred())
			Expect(code).To(Equal(detect.FailStatusCode))
		})
	})
}
