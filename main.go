package simplecache

import (
	"sync"
	"time"
	"unsafe"
)

type TickMiddleware func()
type Middleware[T any] func([]T)
type ExpiryMiddleware[T any] func(string, Item[T])

type Item[T any] struct {
	Value   T
	Expires time.Time
}

type Cache[T any] struct {
	sync.RWMutex

	data        map[any]Item[T]
	prev        map[any]Item[T]
	interval    time.Duration
	compareFunc func(a, b T) bool

	stopChan chan struct{}
	updates  map[string][]T

	beforeTickMiddleware []TickMiddleware
	afterTickMiddleware  []TickMiddleware

	createMiddlewares []Middleware[T]
	updateMiddlewares []Middleware[T]
	deleteMiddlewares []Middleware[T]
	expiryMiddlewares []ExpiryMiddleware[T]

	Metrics map[string]int
}

func New[T any]() *Cache[T] {
	return &Cache[T]{
		data:     make(map[any]Item[T]),
		prev:     make(map[any]Item[T]),
		updates:  make(map[string][]T),
		stopChan: make(chan struct{}),
		Metrics: map[string]int{
			"hits":             0,
			"misses":           0,
			"items":            0,
			"memoryUsageBytes": 0,
		},
	}
}

func (c *Cache[T]) Equals(f func(a, b T) bool) *Cache[T] {
	c.compareFunc = f

	return c
}

func (c *Cache[T]) WithInterval(d time.Duration) *Cache[T] {
	c.interval = d

	return c
}

func (c *Cache[T]) OnCreate(m Middleware[T]) *Cache[T] {
	c.createMiddlewares = append(c.createMiddlewares, m)

	return c
}

func (c *Cache[T]) OnUpdate(m Middleware[T]) *Cache[T] {
	c.updateMiddlewares = append(c.updateMiddlewares, m)

	return c
}

func (c *Cache[T]) OnDelete(m Middleware[T]) *Cache[T] {
	c.deleteMiddlewares = append(c.deleteMiddlewares, m)

	return c
}

func (c *Cache[T]) OnExpiry(m ExpiryMiddleware[T]) *Cache[T] {
	c.expiryMiddlewares = append(c.expiryMiddlewares, m)

	return c
}

func (c *Cache[T]) OnBeforeTick(m TickMiddleware) *Cache[T] {
	c.beforeTickMiddleware = append(c.beforeTickMiddleware, m)

	return c
}

func (c *Cache[T]) OnAfterTick(m TickMiddleware) *Cache[T] {
	c.afterTickMiddleware = append(c.afterTickMiddleware, m)

	return c
}

func (c *Cache[T]) Set(key any, value T, expires ...time.Time) {
	c.Lock()
	defer c.Unlock()

	var expiration time.Time
	if len(expires) > 0 {
		expiration = expires[0]
	}

	item := Item[T]{
		Value:   value,
		Expires: expiration,
	}

	// Update memory usage, remove old item if exists
	existingItem, exists := c.data[key]
	if exists {
		c.updateMemoryUsage(existingItem, false)
	}

	c.updateMemoryUsage(item, true)

	c.data[key] = item
	c.Metrics["items"] = len(c.data)
}

func (c *Cache[T]) updateMemoryUsage(item Item[T], add bool) {
	size := int(unsafe.Sizeof(item)) + int(unsafe.Sizeof(item.Value)) + int(unsafe.Sizeof(item.Expires))

	if add {
		c.Metrics["memoryUsageBytes"] += size
	} else {
		c.Metrics["memoryUsageBytes"] -= size
	}
}

func (c *Cache[T]) Get(key any) (T, bool) {
	c.RLock()
	defer c.RUnlock()

	item, exists := c.data[key]
	if !exists || (!item.Expires.IsZero() && item.Expires.Before(time.Now())) {
		c.Metrics["misses"]++

		var zero T
		return zero, false
	}

	c.Metrics["hits"]++

	return item.Value, true
}

func (c *Cache[T]) GetAll() []T {
	c.RLock()
	defer c.RUnlock()

	res := make([]T, 0, len(c.data))
	for _, item := range c.data {
		if item.Expires.IsZero() || item.Expires.After(time.Now()) {
			res = append(res, item.Value)
		}
	}

	return res
}

func (c *Cache[T]) Delete(key any) {
	c.Lock()
	defer c.Unlock()

	item, exists := c.data[key]
	if exists {
		delete(c.data, key)

		c.updateMemoryUsage(item, false)
		c.Metrics["items"] = len(c.data)
	}
}

func (c *Cache[T]) DeleteAll() {
	c.Lock()
	defer c.Unlock()

	for k := range c.data {
		delete(c.data, k)
	}

	c.Metrics["memoryUsageBytes"] = 0
	c.Metrics["items"] = 0
}

func (c *Cache[T]) Maintain() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return

		case <-ticker.C:
			for _, m := range c.beforeTickMiddleware {
				m()
			}

			c.Lock()

			processedDeletions := make(map[any]struct{})

			// Remove expired items
			for key, item := range c.data {
				if !item.Expires.IsZero() && item.Expires.Before(time.Now()) {
					c.updates["deleted"] = append(c.updates["deleted"], item.Value)

					for _, m := range c.expiryMiddlewares {
						m(key.(string), item)
					}

					delete(c.data, key)
					c.updateMemoryUsage(item, false)
					c.Metrics["items"] = len(c.data)

					processedDeletions[key] = struct{}{}
				}
			}

			// Check for created or updated records
			for key, item := range c.data {
				prevItem, exists := c.prev[key]
				if !exists {
					c.updates["created"] = append(c.updates["created"], item.Value)
				} else if !c.compareFunc(item.Value, prevItem.Value) {
					c.updates["updated"] = append(c.updates["updated"], item.Value)
				}
			}

			// Check for deleted records excluding already processed
			for key, prevValue := range c.prev {
				if _, exists := c.data[key]; !exists {
					_, processed := processedDeletions[key]

					if !processed {
						c.updates["deleted"] = append(c.updates["deleted"], prevValue.Value)
					}
				}
			}

			c.prev = make(map[any]Item[T], len(c.data))
			for key, item := range c.data {
				c.prev[key] = item
			}

			c.Unlock()

			// Call middlewares for created, updated, and deleted records
			if len(c.updates["created"]) > 0 {
				for _, m := range c.createMiddlewares {
					m(c.updates["created"])
				}
			}

			if len(c.updates["updated"]) > 0 {
				for _, m := range c.updateMiddlewares {
					m(c.updates["updated"])
				}
			}

			if len(c.updates["deleted"]) > 0 {
				for _, m := range c.deleteMiddlewares {
					m(c.updates["deleted"])
				}
			}

			// Clear updates for the new tick
			c.updates["created"] = c.updates["created"][:0]
			c.updates["updated"] = c.updates["updated"][:0]
			c.updates["deleted"] = c.updates["deleted"][:0]

			for _, m := range c.afterTickMiddleware {
				m()
			}
		}
	}
}

func (c *Cache[T]) Stop() {
	c.stopChan <- struct{}{}
}
