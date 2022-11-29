package fakes

import "sync"

type YarnrcYmlParser struct {
	ParseLinkerCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Path string
		}
		Returns struct {
			NodeLinker string
			Err        error
		}
		Stub func(string) (string, error)
	}
}

func (f *YarnrcYmlParser) ParseLinker(param1 string) (string, error) {
	f.ParseLinkerCall.mutex.Lock()
	defer f.ParseLinkerCall.mutex.Unlock()
	f.ParseLinkerCall.CallCount++
	f.ParseLinkerCall.Receives.Path = param1
	if f.ParseLinkerCall.Stub != nil {
		return f.ParseLinkerCall.Stub(param1)
	}
	return f.ParseLinkerCall.Returns.NodeLinker, f.ParseLinkerCall.Returns.Err
}
