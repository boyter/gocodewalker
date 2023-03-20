package gitignore_test

import (
	"testing"

	"github.com/denormal/go-gitignore"
)

func TestCache(t *testing.T) {
	// populate the cache with the defined tests
	_cache := gitignore.NewCache()
	for _k, _v := range _CACHETEST {
		_cache.Set(_k, _v)
	}

	// attempt to retrieve the values from the cache
	//		- if a GitIgnore instance is returned, ensure it is the correct
	//		  instance, and not some other instance
	for _k, _v := range _CACHETEST {
		_found := _cache.Get(_k)
		if _found != _v {
			t.Errorf("cache Get() mismatch; expected %v, got %v",
				_v, _found,
			)
		}
	}

	// ensure unknown cache keys return nil
	for _, _k := range _CACHEUNKNOWN {
		_found := _cache.Get(_k)
		if _found != nil {
			t.Errorf("cache.Get() unexpected return for key %q; "+
				"expected nil, got %v",
				_k, _found,
			)
		}
	}

	// ensure we can update the cache
	_ignore := null()
	for _k, _ := range _CACHETEST {
		_cache.Set(_k, _ignore)
	}
	for _k, _ := range _CACHETEST {
		_found := _cache.Get(_k)
		if _found != _ignore {
			t.Errorf("cache Get() mismatch; expected %v, got %v",
				_ignore, _found,
			)
		}
	}
} // TestCache()
