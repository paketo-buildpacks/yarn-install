package yarn_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/cloudfoundry/yarn-cnb/yarn"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuildpack(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		path string
	)

	it.Before(func() {
		file, err := ioutil.TempFile("", "buildpack.toml")
		Expect(err).NotTo(HaveOccurred())
		defer file.Close()

		_, err = file.WriteString(`api = "0.2"

[buildpack]
  id = "org.cloudfoundry.yarn"
  name = "Yarn Buildpack"
  version = "1.2.3"

[metadata]
  include_files = ["bin/build", "bin/detect", "buildpack.toml"]
  pre_package = "./scripts/build.sh"
  [metadata.default_versions]
    yarn = "1.*"

  [[metadata.dependencies]]
    id = "yarn"
    name = "Yarn"
    sha256 = "c03f83a4faad738482ccb557aa36587f6dbfb5c88d2ae3542b081e623dc3e86e"
    source = "https://github.com/yarnpkg/yarn/releases/download/v1.21.0/yarn-v1.21.0.tar.gz"
    source_sha256 = "dd17d4e5bc560aa28140038a31fa50603ef76b710fee44e5ec5efbea7ad24c61"
    stacks = ["io.buildpacks.stacks.bionic", "org.cloudfoundry.stacks.cflinuxfs3"]
    uri = "https://buildpacks.cloudfoundry.org/dependencies/yarn/yarn-1.21.0-any-stack-c03f83a4.tgz"
    version = "1.21.0"

  [[metadata.dependencies]]
    id = "yarn"
    name = "Yarn"
    sha256 = "fd04cba1d0061c05ad6bf76af88ee8eae67dd899015479b39f15ccd626eb2ddd"
    source = "https://github.com/yarnpkg/yarn/releases/download/v1.21.1/yarn-v1.21.1.tar.gz"
    source_sha256 = "d1d9f4a0f16f5ed484e814afeb98f39b82d4728c6c8beaafb5abc99c02db6674"
    stacks = ["io.buildpacks.stacks.bionic", "org.cloudfoundry.stacks.cflinuxfs3"]
    uri = "https://buildpacks.cloudfoundry.org/dependencies/yarn/yarn-1.21.1-any-stack-fd04cba1.tgz"
    version = "1.21.1"

[[stacks]]
  id = "org.cloudfoundry.stacks.cflinuxfs3"

[[stacks]]
  id = "io.buildpacks.stacks.bionic"
`)
		Expect(err).NotTo(HaveOccurred())

		path = file.Name()
	})

	it.After(func() {
		Expect(os.Remove(path)).To(Succeed())
	})

	it("parses the buildpack.toml file", func() {
		buildpack, err := yarn.ParseBuildpack(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(buildpack).To(Equal(yarn.Buildpack{
			API: "0.2",
			Info: yarn.BuildpackInfo{
				ID:      "org.cloudfoundry.yarn",
				Name:    "Yarn Buildpack",
				Version: "1.2.3",
			},
			Metadata: yarn.BuildpackMetadata{
				IncludeFiles: []string{"bin/build", "bin/detect", "buildpack.toml"},
				PrePackage:   "./scripts/build.sh",
				DefaultVersions: yarn.BuildpackMetadataDefaultVersions{
					Yarn: "1.*",
				},
				Dependencies: []yarn.BuildpackMetadataDependency{
					{
						ID:           "yarn",
						Name:         "Yarn",
						SHA256:       "c03f83a4faad738482ccb557aa36587f6dbfb5c88d2ae3542b081e623dc3e86e",
						Source:       "https://github.com/yarnpkg/yarn/releases/download/v1.21.0/yarn-v1.21.0.tar.gz",
						SourceSHA256: "dd17d4e5bc560aa28140038a31fa50603ef76b710fee44e5ec5efbea7ad24c61",
						Stacks:       []string{"io.buildpacks.stacks.bionic", "org.cloudfoundry.stacks.cflinuxfs3"},
						URI:          "https://buildpacks.cloudfoundry.org/dependencies/yarn/yarn-1.21.0-any-stack-c03f83a4.tgz",
						Version:      "1.21.0",
					},
					{
						ID:           "yarn",
						Name:         "Yarn",
						SHA256:       "fd04cba1d0061c05ad6bf76af88ee8eae67dd899015479b39f15ccd626eb2ddd",
						Source:       "https://github.com/yarnpkg/yarn/releases/download/v1.21.1/yarn-v1.21.1.tar.gz",
						SourceSHA256: "d1d9f4a0f16f5ed484e814afeb98f39b82d4728c6c8beaafb5abc99c02db6674",
						Stacks:       []string{"io.buildpacks.stacks.bionic", "org.cloudfoundry.stacks.cflinuxfs3"},
						URI:          "https://buildpacks.cloudfoundry.org/dependencies/yarn/yarn-1.21.1-any-stack-fd04cba1.tgz",
						Version:      "1.21.1",
					},
				},
			},
			Stacks: []yarn.BuildpackStack{
				{ID: "org.cloudfoundry.stacks.cflinuxfs3"},
				{ID: "io.buildpacks.stacks.bionic"},
			},
		}))
	})

	context("failure cases", func() {
		context("when the buildpack.toml file does not exist", func() {
			it("returns an error", func() {
				_, err := yarn.ParseBuildpack("no-such-file.toml")
				Expect(err).To(MatchError(ContainSubstring("failed to parse buildpack.toml")))
				Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
			})
		})

		context("when the buildpack.toml is malformed", func() {
			it.Before(func() {
				err := ioutil.WriteFile(path, []byte("%%%"), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			it("returns an error", func() {
				_, err := yarn.ParseBuildpack(path)
				Expect(err).To(MatchError(ContainSubstring("failed to parse buildpack.toml")))
				Expect(err).To(MatchError(ContainSubstring("bare keys cannot contain '%'")))
			})
		})
	})
}
