package fakes

import "sync"

type CacheMatcher struct {
	MatchCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Metadata map[string]interface {
			}
			Key string
			Sha string
		}
		Returns struct {
			Bool bool
		}
		Stub func(map[string]interface {
		}, string, string) bool
	}
}

func (f *CacheMatcher) Match(param1 map[string]interface {
}, param2 string, param3 string) bool {
	f.MatchCall.Lock()
	defer f.MatchCall.Unlock()
	f.MatchCall.CallCount++
	f.MatchCall.Receives.Metadata = param1
	f.MatchCall.Receives.Key = param2
	f.MatchCall.Receives.Sha = param3
	if f.MatchCall.Stub != nil {
		return f.MatchCall.Stub(param1, param2, param3)
	}
	return f.MatchCall.Returns.Bool
}
