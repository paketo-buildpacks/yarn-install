package fakes

import (
	"sync"

	"github.com/cloudfoundry/yarn-cnb/yarn"
)

type DependencyInstaller struct {
	InstallCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Dependencies []yarn.BuildpackMetadataDependency
			CnbPath      string
			LayerPath    string
		}
		Returns struct {
			Error error
		}
		Stub func([]yarn.BuildpackMetadataDependency, string, string) error
	}
}

func (f *DependencyInstaller) Install(param1 []yarn.BuildpackMetadataDependency, param2 string, param3 string) error {
	f.InstallCall.Lock()
	defer f.InstallCall.Unlock()
	f.InstallCall.CallCount++
	f.InstallCall.Receives.Dependencies = param1
	f.InstallCall.Receives.CnbPath = param2
	f.InstallCall.Receives.LayerPath = param3
	if f.InstallCall.Stub != nil {
		return f.InstallCall.Stub(param1, param2, param3)
	}
	return f.InstallCall.Returns.Error
}
