package integration_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloudfoundry/dagger"
	"github.com/cloudfoundry/packit/pexec"
	"github.com/paketo-buildpacks/occam"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
)

var (
	bpDir         string
	yarnURI       string
	yarnCachedURI string
	nodeURI       string
	nodeCachedURI string
)

func TestIntegration(t *testing.T) {
	var (
		Expect = NewWithT(t).Expect
		err    error
	)

	bpDir, err = dagger.FindBPRoot()
	Expect(err).NotTo(HaveOccurred())

	yarnURI, err = dagger.PackageBuildpack(bpDir)
	Expect(err).ToNot(HaveOccurred())

	yarnCachedURI, _, err = dagger.PackageCachedBuildpack(bpDir)
	Expect(err).ToNot(HaveOccurred())

	nodeURI, err = dagger.GetLatestCommunityBuildpack("paketo-buildpacks", "node-engine")
	Expect(err).ToNot(HaveOccurred())

	nodeRepo, err := dagger.GetLatestUnpackagedCommunityBuildpack("paketo-buildpacks", "node-engine")
	Expect(err).ToNot(HaveOccurred())

	nodeCachedURI, _, err = dagger.PackageCachedBuildpack(nodeRepo)
	Expect(err).ToNot(HaveOccurred())

	// HACK: we need to fix dagger and the package.sh scripts so that this isn't required
	yarnURI = fmt.Sprintf("%s.tgz", yarnURI)
	yarnCachedURI = fmt.Sprintf("%s.tgz", yarnCachedURI)
	nodeCachedURI = fmt.Sprintf("%s.tgz", nodeCachedURI)

	defer dagger.DeleteBuildpack(yarnURI)
	defer dagger.DeleteBuildpack(yarnCachedURI)
	defer dagger.DeleteBuildpack(nodeURI)
	defer os.RemoveAll(nodeRepo)
	defer dagger.DeleteBuildpack(nodeCachedURI)

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

	dagger.SyncParallelOutput(func() { suite.Run(t) })
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
