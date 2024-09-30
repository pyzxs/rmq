package rmq

import (
	"sort"
	"sync"
	"time"
)

type keyItem struct {
	expiration int64
	key        string
}

type sortItems []*keyItem

func (its sortItems) Len() int {
	return len(its)
}

func (its sortItems) Less(i, j int) bool {
	return its[i].expiration < its[j].expiration
}

func (its sortItems) Swap(i, j int) {
	its[i], its[j] = its[j], its[i]
}

type item struct {
	value      interface{}
	expiration int64
}

type MemoryCache struct {
	items    map[string]*item
	maxItems int
	mu       sync.RWMutex
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		maxItems: MAX_CACHE_LEN,
		items:    make(map[string]*item),
	}
}

func (c *MemoryCache) Set(key string, value interface{}, expiration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var exp int64
	if expiration > 0 {
		exp = time.Now().Add(expiration).UnixNano()
	}
	c.items[key] = &item{
		value:      value,
		expiration: exp,
	}
}

func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.items[key]
	if !ok {
		return nil, false
	}
	if item.expiration > 0 && time.Now().UnixNano() > item.expiration {
		return nil, false
	}
	return item.value, true
}

// Delete 删除缓存
func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// StartGC 启动回收服务
func (c *MemoryCache) StartGC(interval time.Duration) {
	go func() {
		for {
			time.Sleep(interval)
			if len(c.items) == 0 {
				break
			}
			c.mu.Lock()
			c.gcMaxLen()
			c.gcExpire()
			c.mu.Unlock()
		}
	}()
}

// 回收超时缓存数据
func (c *MemoryCache) gcExpire() {
	now := time.Now().UnixNano()
	for key, item := range c.items {
		if item.expiration > 0 && now > item.expiration {
			delete(c.items, key)
		}
	}
}

// 回收超出最大限制的缓存数据
func (c *MemoryCache) gcMaxLen() {
	items := sortItems{}
	for k, item := range c.items {
		kts := &keyItem{key: k, expiration: item.expiration}
		items = append(items, kts)
	}
	sort.Sort(items)

	if len(items) > c.maxItems {
		delItems := items[:len(items)-c.maxItems]
		for _, item := range delItems {
			delete(c.items, item.key)
		}
	}
}
