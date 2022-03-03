package fakes

import (
	"sync"
)

type InstallProcess struct {
	ExecuteCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			WorkingDir       string
			ModulesLayerPath string
			Launch           bool
		}
		Returns struct {
			Error error
		}
		Stub func(string, string, bool) error
	}
	SetupModulesCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			WorkingDir              string
			CurrentModulesLayerPath string
			NextModulesLayerPath    string
		}
		Returns struct {
			String string
			Error  error
		}
		Stub func(string, string, string) (string, error)
	}
	ShouldRunCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			WorkingDir string
			Metadata   map[string]interface {
			}
		}
		Returns struct {
			Run bool
			Sha string
			Err error
		}
		Stub func(string, map[string]interface {
		}) (bool, string, error)
	}
}

func (f *InstallProcess) Execute(param1 string, param2 string, param3 bool) error {
	f.ExecuteCall.mutex.Lock()
	defer f.ExecuteCall.mutex.Unlock()
	f.ExecuteCall.CallCount++
	f.ExecuteCall.Receives.WorkingDir = param1
	f.ExecuteCall.Receives.ModulesLayerPath = param2
	f.ExecuteCall.Receives.Launch = param3
	if f.ExecuteCall.Stub != nil {
		return f.ExecuteCall.Stub(param1, param2, param3)
	}
	return f.ExecuteCall.Returns.Error
}
func (f *InstallProcess) SetupModules(param1 string, param2 string, param3 string) (string, error) {
	f.SetupModulesCall.mutex.Lock()
	defer f.SetupModulesCall.mutex.Unlock()
	f.SetupModulesCall.CallCount++
	f.SetupModulesCall.Receives.WorkingDir = param1
	f.SetupModulesCall.Receives.CurrentModulesLayerPath = param2
	f.SetupModulesCall.Receives.NextModulesLayerPath = param3
	if f.SetupModulesCall.Stub != nil {
		return f.SetupModulesCall.Stub(param1, param2, param3)
	}
	return f.SetupModulesCall.Returns.String, f.SetupModulesCall.Returns.Error
}
func (f *InstallProcess) ShouldRun(param1 string, param2 map[string]interface {
}) (bool, string, error) {
	f.ShouldRunCall.mutex.Lock()
	defer f.ShouldRunCall.mutex.Unlock()
	f.ShouldRunCall.CallCount++
	f.ShouldRunCall.Receives.WorkingDir = param1
	f.ShouldRunCall.Receives.Metadata = param2
	if f.ShouldRunCall.Stub != nil {
		return f.ShouldRunCall.Stub(param1, param2)
	}
	return f.ShouldRunCall.Returns.Run, f.ShouldRunCall.Returns.Sha, f.ShouldRunCall.Returns.Err
}
