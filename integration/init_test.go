package integration_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/paketo-buildpacks/occam"
	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
)

var (
	yarnURI       string
	yarnCachedURI string
	nodeURI       string
	nodeCachedURI string
	buildpackInfo struct {
		Buildpack struct {
			ID   string
			Name string
		}
	}
)

func TestIntegration(t *testing.T) {
	var Expect = NewWithT(t).Expect

	var config struct {
		NodeEngine string `json:"node-engine"`
	}

	file, err := os.Open("./../integration.json")
	Expect(err).NotTo(HaveOccurred())

	Expect(json.NewDecoder(file).Decode(&config)).To(Succeed())

	root, err := filepath.Abs("./..")
	Expect(err).NotTo(HaveOccurred())

	file, err = os.Open("../buildpack.toml")
	Expect(err).NotTo(HaveOccurred())

	_, err = toml.DecodeReader(file, &buildpackInfo)
	Expect(err).NotTo(HaveOccurred())

	buildpackStore := occam.NewBuildpackStore()

	version, err := GetGitVersion()
	Expect(err).ToNot(HaveOccurred())

	yarnURI, err = buildpackStore.Get.
		WithVersion(version).
		Execute(root)
	Expect(err).ToNot(HaveOccurred())

	yarnCachedURI, err = buildpackStore.Get.
		WithOfflineDependencies().
		WithVersion(version).
		Execute(root)
	Expect(err).ToNot(HaveOccurred())

	nodeURI, err = buildpackStore.Get.Execute(config.NodeEngine)
	Expect(err).ToNot(HaveOccurred())

	nodeCachedURI, err = buildpackStore.Get.
		WithOfflineDependencies().
		Execute(config.NodeEngine)
	Expect(err).ToNot(HaveOccurred())

	SetDefaultEventuallyTimeout(10 * time.Second)

	suite := spec.New("Integration", spec.Parallel(), spec.Report(report.Terminal{}))
	suite("Caching", testCaching)
	suite("Logging", testLogging)
	suite("ModuleBinaries", testModuleBinaries)
	suite("PreGyp", testPreGyp)
	suite("SimpleApp", testSimpleApp)
	suite("Vendored", testVendored)
	suite("Workspaces", testWorkspaces)
	suite("NoHoist", testNoHoist)
	suite.Run(t)
}

func ContainerLogs(id string) func() string {
	docker := occam.NewDocker()

	return func() string {
		logs, _ := docker.Container.Logs.Execute(id)
		return logs.String()
	}
}

func GetGitVersion() (string, error) {
	gitExec := pexec.NewExecutable("git")
	revListOut := bytes.NewBuffer(nil)

	err := gitExec.Execute(pexec.Execution{
		Args:   []string{"rev-list", "--tags", "--max-count=1"},
		Stdout: revListOut,
	})

	if revListOut.String() == "" {
		return "0.0.0", nil
	}

	if err != nil {
		return "", err
	}

	stdout := bytes.NewBuffer(nil)
	err = gitExec.Execute(pexec.Execution{
		Args:   []string{"describe", "--tags", strings.TrimSpace(revListOut.String())},
		Stdout: stdout,
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(strings.TrimPrefix(stdout.String(), "v")), nil
}
