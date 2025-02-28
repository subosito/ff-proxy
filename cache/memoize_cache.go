package cache

import (
	"crypto/md5" //#nosec G501
	"fmt"
	"reflect"
	"time"

	jsoniter "github.com/json-iterator/go"
	gocache "github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

type memoizeMetrics interface {
	// cacheMissInc increments a counter whenever the raw bytes don't exist in the memoize cache
	cacheMissInc()

	// cacheHitInc increments a counter whenever we've found the raw bytes in the memoize cache
	cacheHitInc()

	// cacheMarshalInc increments a counter each time we marshal an object and store it in the memoize cache
	cacheMarshalInc()

	// cacheHitWithUnmarshalInc increments a counter whenever we've found the raw bytes in the memoize cache but have
	// still had to perform an unmarshal. This shouldn't happen but this counter will let us know if it is occuring
	cacheHitWithUnmarshalInc()
}

type internalCache interface {
	Get(key string) (interface{}, bool)
	Set(key string, v interface{}, d time.Duration)
	Delete(key string)
}

type memoizeCache struct {
	Cache
	metrics memoizeMetrics
}

// NewMemoizeCache creates a memoize cache
func NewMemoizeCache(rc redis.UniversalClient, defaultExpiration, cleanupInterval time.Duration, metrics memoizeMetrics) Cache {
	mc := memoizeCache{}
	c := gocache.New(defaultExpiration, cleanupInterval)

	if metrics == nil {
		metrics = noOpMetrics{}
	}
	mc.metrics = metrics

	mc.Cache = NewKeyValCache(rc,
		WithTTL(0),
		WithMarshalFunc(mc.makeMarshalFunc(c)),
		WithUnmarshalFunc(mc.makeUnmarshalFunc(c)),
	)

	return mc
}

func (m memoizeCache) makeMarshalFunc(ffCache internalCache) func(interface{}) ([]byte, error) {
	return func(i interface{}) ([]byte, error) {
		data, err := jsoniter.Marshal(i)
		if err != nil {
			return nil, err
		}

		/* #nosec */
		hasher := md5.New()
		hasher.Write(data)
		hash := hasher.Sum(nil)
		ffCache.Set(string(hash), i, gocache.DefaultExpiration)
		m.metrics.cacheMarshalInc()
		return data, nil
	}
}

func (m memoizeCache) makeUnmarshalFunc(ffCache internalCache) func([]byte, interface{}) error {
	return func(bytes []byte, i interface{}) error {

		/* #nosec */
		hasher := md5.New()
		hasher.Write(bytes)
		hash := hasher.Sum(nil)
		if resp, ok := ffCache.Get(string(hash)); ok {
			val := reflect.ValueOf(i)
			if val.Kind() != reflect.Ptr {
				m.metrics.cacheHitWithUnmarshalInc()
				return jsoniter.Unmarshal(bytes, &i)
			}

			// We got a hit for the bytes in our memoize cache so can return them
			// and don't have to perform any unmarshaling.
			m.metrics.cacheHitInc()
			respValue := reflect.ValueOf(resp)
			if respValue.Kind() == reflect.Ptr {
				val.Elem().Set(respValue.Elem())
			} else {
				val.Elem().Set(respValue)
			}

			return nil
		}

		// The raw bytes weren't in the memoize cache so we increment our cache
		// readMiss counters and have to perform a full unmarshal
		m.metrics.cacheMissInc()
		err := jsoniter.Unmarshal(bytes, &i)
		if err != nil {
			return err
		}

		// Because we didn't find these bytes in our local cache.
		// save them for next time.
		ffCache.Set(string(hash), i, gocache.DefaultExpiration)
		return nil
	}
}

// MemoizeMetrics implements the memoizeMetrics interface
type MemoizeMetrics struct {
	cacheMarshal     prometheus.Counter
	hitWithUnmarshal prometheus.Counter

	miss prometheus.Counter
	hit  prometheus.Counter
}

// NewMemoizeMetrics creates a MemoizeMetrics struct that records prometheus metrics that tracks activity in the
// memoize cache
func NewMemoizeMetrics(label string, reg *prometheus.Registry) MemoizeMetrics {
	m := MemoizeMetrics{
		miss: prometheus.NewCounter(prometheus.CounterOpts{
			Name: fmt.Sprintf("ff_%s_memoize_cache_miss", label),
			Help: "Tracks the number of misses we get performing lookups in our memoize cache. When we get a readMiss we have to perform a full unmarshal",
		}),
		hit: prometheus.NewCounter(prometheus.CounterOpts{
			Name: fmt.Sprintf("ff_%s_memoize_cache_hit", label),
			Help: "Tracks the number of hits we get performing lookups in our memoize cache. When we get a hit we can return raw bytes and avoid performing any unmarshaling",
		}),

		cacheMarshal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: fmt.Sprintf("ff_%s_memoize_cache_write_marshal", label),
			Help: "Tracks the number of times the memoize cache marshals an object to bytes. This happens every time we set a value in the cache",
		}),

		hitWithUnmarshal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: fmt.Sprintf("ff_%s_memoize_cache_hit_with_unmarshal", label),
			Help: "Tracks the number of hits we get performing lookups in our memoize cache but we've still had to perform a full unmarshal",
		}),
	}

	reg.MustRegister(
		m.cacheMarshal,
		m.hitWithUnmarshal,
		m.miss,
		m.hit,
	)

	return m
}

func (m MemoizeMetrics) cacheMarshalInc() {
	m.cacheMarshal.Inc()
}

func (m MemoizeMetrics) cacheMissInc() {
	m.miss.Inc()
}

func (m MemoizeMetrics) cacheHitWithUnmarshalInc() {
	m.hitWithUnmarshal.Inc()
}

func (m MemoizeMetrics) cacheHitInc() {
	m.hit.Inc()
}

type noOpMetrics struct{}

func (n noOpMetrics) cacheMarshalInc() {}

func (n noOpMetrics) cacheMissInc() {}

func (n noOpMetrics) cacheHitWithUnmarshalInc() {}

func (n noOpMetrics) cacheHitInc() {}
