package hw04lrucache

import (
	"sync"
)

type Key string

type Cache interface {
	Set(key Key, value interface{}) bool
	Get(key Key) (interface{}, bool)
	Clear()
}

type cacheItem struct {
	key   Key
	value interface{}
}

type lruCache struct {
	mu       sync.Mutex
	capacity int
	queue    List
	items    map[Key]*ListItem
}

func NewCache(capacity int) Cache {
	if capacity < 0 {
		capacity = 0
	}
	return &lruCache{
		capacity: capacity,
		queue:    NewList(),
		items:    make(map[Key]*ListItem, capacity),
	}
}

func (c *lruCache) Set(key Key, value interface{}) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, ok := c.items[key]; ok {
		payload(item).value = value
		c.queue.MoveToFront(item)
		return true
	}

	if c.capacity == 0 {
		return false
	}

	if c.queue.Len() >= c.capacity {
		c.evict()
	}

	c.items[key] = c.queue.PushFront(&cacheItem{key: key, value: value})
	return false
}

func (c *lruCache) Get(key Key) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.queue.MoveToFront(item)
	return payload(item).value, true
}

func (c *lruCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.queue = NewList()
	c.items = make(map[Key]*ListItem, c.capacity)
}

func (c *lruCache) evict() {
	back := c.queue.Back()
	if back == nil {
		return
	}
	c.queue.Remove(back)
	delete(c.items, payload(back).key)
}

func payload(i *ListItem) *cacheItem {
	ci, _ := i.Value.(*cacheItem)
	return ci
}
