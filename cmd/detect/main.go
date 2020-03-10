package main

import (
	"github.com/cloudfoundry/packit"
	"github.com/cloudfoundry/yarn-install-cnb/yarn"
)

func main() {
	packageJSONParser := yarn.NewPackageJSONParser()

	packit.Detect(yarn.Detect(packageJSONParser))
}
