package lru

import (
	"fmt"
	"testing"
	"time"
)

type String string

func (d String) Len() int {
	return len(d)
}

func TestCache(t *testing.T) {
	// 创建一个LRU缓存，最大容量100字节
	cache := New(100, func(key string, value Value) {
		fmt.Printf("Evicted key: %s\n", key)
	})

	// 添加缓存项
	cache.Add("key1", String("value1"), time.Now().Add(5*time.Minute))
	cache.Add("key2", String("value2"), time.Now().Add(5*time.Minute))
	cache.Add("key3", String("value3"), time.Now().Add(5*time.Minute))

	// 获取并检查缓存项是否存在
	if v, ok := cache.Get("key1"); !ok || string(v.(String)) != "value1" {
		t.Errorf("cache hit key1 failed")
	}

	// 测试 LRU 逻辑（超过容量后最久未使用的项被删除）
	cache.Add("key4", String("value4 that takes more space"), time.Now().Add(5*time.Minute))
	
	if _, ok := cache.Get("key2"); ok {
		t.Errorf("cache should have evicted key2 but didn't")
	}

	// 测试缓存过期
	time.Sleep(2 * time.Second)
	if _, ok := cache.Get("key1"); !ok {
		t.Errorf("cache miss for key1, but it should be present")
	}
}

func TestCacheExpiration(t *testing.T) {
	cache := New(100, nil)
	cache.Add("expiringKey", String("expiringValue"), time.Now().Add(1*time.Second))
	time.Sleep(2 * time.Second)
	if _, ok := cache.Get("expiringKey"); ok {
		t.Errorf("cache should have expired key but didn't")
	}
}
