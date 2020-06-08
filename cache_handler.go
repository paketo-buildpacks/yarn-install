package yarninstall

type CacheHandler struct{}

func NewCacheHandler() CacheHandler {
	return CacheHandler{}
}

func (ch CacheHandler) Match(metadata map[string]interface{}, key, sha string) bool {
	value, ok := metadata[key].(string)
	return value == sha && ok
}
