package main

import (
	"iter"
	"log"
	"maps"
	"sync"
)

type Cache[K comparable, V any] struct {
	store map[K]V
	lock  sync.RWMutex
}

func NewCache[K comparable, V any]() Cache[K, V] {
	return Cache[K, V]{
		store: make(map[K]V),
	}
}

func (c *Cache[_, _]) Clear() {
	clear(c.store)
}

func (c *Cache[K, V]) Store(key K, value V) {
	c.store[key] = value
	// log.Printf("CACHE %s = %s", key, value)
}

func (c *Cache[K, V]) Load(key K) (V, bool) {
	value, ok := c.store[key]
	if ok {
		// log.Printf("HIT %s = %s", key, value)
	} else {
		log.Printf("MISS %s", key)
	}
	return value, ok
}

func (c *Cache[_, _]) Len() int {
	return len(c.store)
}

func (c *Cache[K, _]) Entries() iter.Seq[K] {
	return maps.Keys(c.store)
}

func (c *Cache[_, _]) RLock() {
	c.lock.RLock()
}

func (c *Cache[_, _]) RUnlock() {
	c.lock.RUnlock()
}

func (c *Cache[_, _]) Lock() {
	c.lock.Lock()
}

func (c *Cache[_, _]) Unlock() {
	c.lock.Unlock()
}
