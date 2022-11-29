package fakes

import (
	"sync"

	"github.com/paketo-buildpacks/packit/v2"
	yarninstall "github.com/paketo-buildpacks/yarn-install"
)

type BuildProcess struct {
	BuildCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Context        packit.BuildContext
			InstallProcess yarninstall.InstallProcess
			SbomGenerator  yarninstall.SBOMGenerator
			Symlinker      yarninstall.SymlinkManager
			EntryResolver  yarninstall.EntryResolver
			ProjectPath    string
			TempDir        string
		}
		Returns struct {
			BuildResult packit.BuildResult
			Error       error
		}
		Stub func(packit.BuildContext, yarninstall.InstallProcess, yarninstall.SBOMGenerator, yarninstall.SymlinkManager, yarninstall.EntryResolver, string, string) (packit.BuildResult, error)
	}
}

func (f *BuildProcess) Build(param1 packit.BuildContext, param2 yarninstall.InstallProcess, param3 yarninstall.SBOMGenerator, param4 yarninstall.SymlinkManager, param5 yarninstall.EntryResolver, param6 string, param7 string) (packit.BuildResult, error) {
	f.BuildCall.mutex.Lock()
	defer f.BuildCall.mutex.Unlock()
	f.BuildCall.CallCount++
	f.BuildCall.Receives.Context = param1
	f.BuildCall.Receives.InstallProcess = param2
	f.BuildCall.Receives.SbomGenerator = param3
	f.BuildCall.Receives.Symlinker = param4
	f.BuildCall.Receives.EntryResolver = param5
	f.BuildCall.Receives.ProjectPath = param6
	f.BuildCall.Receives.TempDir = param7
	if f.BuildCall.Stub != nil {
		return f.BuildCall.Stub(param1, param2, param3, param4, param5, param6, param7)
	}
	return f.BuildCall.Returns.BuildResult, f.BuildCall.Returns.Error
}
