package timeline

import "strings"

var knownHeroNameIndexKeys = [...]string{
	"m_pEntity.m_nameStringTableIndex",
	"m_pEntity.m_nameStringableIndex",
}

type heroNameIndexReader struct {
	key     string
	scanned bool
}

func newHeroNameIndexReader() *heroNameIndexReader { return &heroNameIndexReader{} }

func (r *heroNameIndexReader) read(get func(string) any, allFields func() map[string]any) (int32, bool) {
	if r.key != "" {
		idx, ok := get(r.key).(int32)
		return idx, ok
	}
	for _, key := range knownHeroNameIndexKeys {
		if idx, ok := get(key).(int32); ok {
			r.key = key
			return idx, true
		}
	}
	if r.scanned || allFields == nil {
		return 0, false
	}
	r.scanned = true
	for key, value := range allFields() {
		idx, ok := value.(int32)
		if !ok || !isHeroNameIndexKey(key) {
			continue
		}
		r.key = key
		return idx, true
	}
	return 0, false
}

func isHeroNameIndexKey(key string) bool {
	return strings.HasPrefix(key, "m_pEntity.") &&
		strings.HasSuffix(key, "Index") &&
		strings.Contains(strings.ToLower(key), "name")
}
