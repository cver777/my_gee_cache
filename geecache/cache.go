package geecache

import (
	"geecache/lru"
	"sync"
)

type cache struct {
	mu 			sync.Mutex
	lru 		*lru.Cache
	cacheBytes  int64
}

func (c *cache) add(key string, value ByteView) {//同一个包可相互调用 比如ByteView，byteview是缓存值，一个私有静态数组，调用公有成员访问
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		c.lru = lru.New(c.cacheBytes, nil)//实例化lru
	}
	c.lru.Add(key, value)
}

func (c *cache) get(key string) (value ByteView, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		return
	}

	if v, ok := c.lru.Get(key); ok {
		return v.(ByteView), ok
	}
	return  
}