package fakes

import "sync"

type InstallProcess struct {
	ExecuteCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			WorkingDir       string
			ModulesLayerPath string
		}
		Returns struct {
			Error error
		}
		Stub func(string, string) error
	}
	ShouldRunCall struct {
		sync.Mutex
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

func (f *InstallProcess) Execute(param1 string, param2 string) error {
	f.ExecuteCall.Lock()
	defer f.ExecuteCall.Unlock()
	f.ExecuteCall.CallCount++
	f.ExecuteCall.Receives.WorkingDir = param1
	f.ExecuteCall.Receives.ModulesLayerPath = param2
	if f.ExecuteCall.Stub != nil {
		return f.ExecuteCall.Stub(param1, param2)
	}
	return f.ExecuteCall.Returns.Error
}
func (f *InstallProcess) ShouldRun(param1 string, param2 map[string]interface {
}) (bool, string, error) {
	f.ShouldRunCall.Lock()
	defer f.ShouldRunCall.Unlock()
	f.ShouldRunCall.CallCount++
	f.ShouldRunCall.Receives.WorkingDir = param1
	f.ShouldRunCall.Receives.Metadata = param2
	if f.ShouldRunCall.Stub != nil {
		return f.ShouldRunCall.Stub(param1, param2)
	}
	return f.ShouldRunCall.Returns.Run, f.ShouldRunCall.Returns.Sha, f.ShouldRunCall.Returns.Err
}
