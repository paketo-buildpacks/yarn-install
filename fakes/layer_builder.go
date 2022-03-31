package fakes

import (
	"sync"

	packit "github.com/paketo-buildpacks/packit/v2"
)

type LayerBuilder struct {
	BuildCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Context                 packit.BuildContext
			CurrentModulesLayerPath string
			ProjectPath             string
		}
		Returns struct {
			Layer packit.Layer
			Error error
		}
		Stub func(packit.BuildContext, string, string) (packit.Layer, error)
	}
}

func (f *LayerBuilder) Build(param1 packit.BuildContext, param2 string, param3 string) (packit.Layer, error) {
	f.BuildCall.mutex.Lock()
	defer f.BuildCall.mutex.Unlock()
	f.BuildCall.CallCount++
	f.BuildCall.Receives.Context = param1
	f.BuildCall.Receives.CurrentModulesLayerPath = param2
	f.BuildCall.Receives.ProjectPath = param3
	if f.BuildCall.Stub != nil {
		return f.BuildCall.Stub(param1, param2, param3)
	}
	return f.BuildCall.Returns.Layer, f.BuildCall.Returns.Error
}
