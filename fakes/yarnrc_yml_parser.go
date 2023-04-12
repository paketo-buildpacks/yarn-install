package fakes

import "sync"

type YarnrcYmlParser struct {
	ParseCall struct {
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

func (f *YarnrcYmlParser) Parse(param1 string) (string, error) {
	f.ParseCall.mutex.Lock()
	defer f.ParseCall.mutex.Unlock()
	f.ParseCall.CallCount++
	f.ParseCall.Receives.Path = param1
	if f.ParseCall.Stub != nil {
		return f.ParseCall.Stub(param1)
	}
	return f.ParseCall.Returns.NodeLinker, f.ParseCall.Returns.Err
}
