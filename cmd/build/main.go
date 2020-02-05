package main

import (
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/packit"
	"github.com/cloudfoundry/packit/cargo"
	"github.com/cloudfoundry/packit/pexec"
	"github.com/cloudfoundry/yarn-cnb/yarn"
)

func main() {
	transport := cargo.NewTransport()
	executable := pexec.NewExecutable("yarn", lager.NewLogger("yarn"))
	dependencyInstaller := yarn.NewYarnDependencyInstaller(transport)
	installProcess := yarn.NewYarnInstallProcess(executable)

	packit.Build(yarn.Build(dependencyInstaller, installProcess))
}
