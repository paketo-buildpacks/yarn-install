package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/onsi/gomega/format"
	"github.com/paketo-buildpacks/occam"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
)

var settings struct {
	Extensions struct {
		UbiNodejsExtension struct {
			Online string
		}
	}
}

var (
	buildpackURI        string
	buildpackOfflineURI string
	nodeURI             string
	nodeOfflineURI      string
	yarnURI             string
	yarnOfflineURI      string
	buildPlanURI        string
	yarnList            string
	buildpackInfo       struct {
		Buildpack struct {
			ID   string
			Name string
		}
	}
)

func TestIntegration(t *testing.T) {
	var Expect = NewWithT(t).Expect
	format.MaxLength = 0

	var config struct {
		BuildPlan          string `json:"build-plan"`
		NodeEngine         string `json:"node-engine"`
		Yarn               string `json:"yarn"`
		UbiNodejsExtension string `json:"ubi-nodejs-extension"`
	}

	file, err := os.Open("./../integration.json")
	Expect(err).NotTo(HaveOccurred())

	Expect(json.NewDecoder(file).Decode(&config)).To(Succeed())

	root, err := filepath.Abs("./..")
	Expect(err).NotTo(HaveOccurred())

	file, err = os.Open("../buildpack.toml")
	Expect(err).NotTo(HaveOccurred())

	_, err = toml.NewDecoder(file).Decode(&buildpackInfo)
	Expect(err).NotTo(HaveOccurred())

	buildpackStore := occam.NewBuildpackStore()

	pack := occam.NewPack()

	builder, err := pack.Builder.Inspect.Execute()
	Expect(err).NotTo(HaveOccurred())

	if builder.BuilderName == "index.docker.io/paketocommunity/builder-ubi-buildpackless-base:latest" {
		settings.Extensions.UbiNodejsExtension.Online, err = buildpackStore.Get.
			Execute(config.UbiNodejsExtension)
		Expect(err).ToNot(HaveOccurred())
	}

	buildpackURI, err = buildpackStore.Get.
		WithVersion("1.2.3").
		Execute(root)
	Expect(err).ToNot(HaveOccurred())

	buildpackOfflineURI, err = buildpackStore.Get.
		WithOfflineDependencies().
		WithVersion("1.2.3").
		Execute(root)
	Expect(err).ToNot(HaveOccurred())

	nodeURI, err = buildpackStore.Get.Execute(config.NodeEngine)
	Expect(err).ToNot(HaveOccurred())

	nodeOfflineURI, err = buildpackStore.Get.
		WithOfflineDependencies().
		Execute(config.NodeEngine)
	Expect(err).ToNot(HaveOccurred())

	yarnURI, err = buildpackStore.Get.Execute(config.Yarn)
	Expect(err).ToNot(HaveOccurred())

	yarnOfflineURI, err = buildpackStore.Get.
		WithOfflineDependencies().
		Execute(config.Yarn)
	Expect(err).ToNot(HaveOccurred())

	buildPlanURI, err = buildpackStore.Get.
		Execute(config.BuildPlan)
	Expect(err).NotTo(HaveOccurred())

	yarnList = filepath.Join(root, "integration", "testdata", "yarn-list-buildpack")

	SetDefaultEventuallyTimeout(10 * time.Second)

	suite := spec.New("Integration", spec.Parallel(), spec.Report(report.Terminal{}))
	suite("Caching", testCaching)
	suite("DevDependenciesDuringBuild", testDevDependenciesDuringBuild)
	suite("Logging", testLogging)
	suite("ModuleBinaries", testModuleBinaries)
	suite("NoHoist", testNoHoist)
	suite("PreGyp", testPreGyp)
	suite("ProjectPathApp", testProjectPathApp)
	suite("ServiceBindings", testServiceBindings)
	suite("SimpleApp", testSimpleApp)
	suite("Vendored", testVendored)
	suite("Workspaces", testWorkspaces)
	suite.Run(t)
}
