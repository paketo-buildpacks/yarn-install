package fakes

import (
	"sync"
)

type SymlinkManager struct {
	LinkCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Oldname string
			Newname string
		}
		Returns struct {
			Error error
		}
		Stub func(string, string) error
	}
	UnlinkCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Path string
		}
		Returns struct {
			Error error
		}
		Stub func(string) error
	}
}

func (f *SymlinkManager) Link(param1 string, param2 string) error {
	f.LinkCall.mutex.Lock()
	defer f.LinkCall.mutex.Unlock()
	f.LinkCall.CallCount++
	f.LinkCall.Receives.Oldname = param1
	f.LinkCall.Receives.Newname = param2
	if f.LinkCall.Stub != nil {
		return f.LinkCall.Stub(param1, param2)
	}
	return f.LinkCall.Returns.Error
}
func (f *SymlinkManager) Unlink(param1 string) error {
	f.UnlinkCall.mutex.Lock()
	defer f.UnlinkCall.mutex.Unlock()
	f.UnlinkCall.CallCount++
	f.UnlinkCall.Receives.Path = param1
	if f.UnlinkCall.Stub != nil {
		return f.UnlinkCall.Stub(param1)
	}
	return f.UnlinkCall.Returns.Error
}
