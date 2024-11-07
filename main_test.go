package simplecache_test

import (
	cache "github.com/kamludwinski2/simplecache"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

type TestStruct struct {
	Name string
	Age  int
}

func TestSetAndGet(t *testing.T) {
	c := cache.New[TestStruct]()
	c.Set("item1", TestStruct{Name: "Alice", Age: 30})
	c.Set("item2", TestStruct{Name: "Bob", Age: 25})

	val, exists := c.Get("item1")
	assert.True(t, exists)
	assert.Equal(t, TestStruct{Name: "Alice", Age: 30}, val)

	_, exists = c.Get("nonexistent")
	assert.False(t, exists)

	assert.Equal(t, 1, c.Metrics["hits"])
	assert.Equal(t, 1, c.Metrics["misses"])
}

func TestExpiration(t *testing.T) {
	c := cache.New[TestStruct]().WithInterval(500 * time.Millisecond)
	expiredItems := make([]string, 0)

	go c.Maintain()
	defer c.Stop()

	c.OnExpiry(func(key string, item cache.Item[TestStruct]) {
		expiredItems = append(expiredItems, key)
	})

	c.Set("item1", TestStruct{Name: "Alice", Age: 30}, time.Now().Add(1*time.Second))
	time.Sleep(2 * time.Second)

	_, exists := c.Get("item1")
	assert.False(t, exists)

	assert.Len(t, expiredItems, 1)
	assert.Equal(t, "item1", expiredItems[0])
}

func TestCreateUpdateDeleteMiddlewares(t *testing.T) {
	createdItems := make([]TestStruct, 0)
	updatedItems := make([]TestStruct, 0)
	deletedItems := make([]TestStruct, 0)

	c := cache.New[TestStruct]().WithInterval(500 * time.Millisecond).
		Equals(func(a, b TestStruct) bool {
			return a.Name == b.Name && a.Age == b.Age
		}).
		OnCreate(func(items []TestStruct) {
			createdItems = append(createdItems, items...)
		}).
		OnUpdate(func(items []TestStruct) { updatedItems = append(updatedItems, items...) }).
		OnDelete(func(items []TestStruct) { deletedItems = append(deletedItems, items...) })

	go c.Maintain()
	defer c.Stop()

	c.Set("item1", TestStruct{Name: "Alice", Age: 30})
	time.Sleep(1 * time.Second)

	c.Set("item1", TestStruct{Name: "Alice", Age: 31})
	time.Sleep(1 * time.Second)

	c.Delete("item1")
	time.Sleep(1 * time.Second)

	assert.Len(t, createdItems, 1)
	assert.Equal(t, TestStruct{Name: "Alice", Age: 30}, createdItems[0])

	assert.Len(t, updatedItems, 1)
	assert.Equal(t, TestStruct{Name: "Alice", Age: 31}, updatedItems[0])

	assert.Len(t, deletedItems, 1)
	assert.Equal(t, TestStruct{Name: "Alice", Age: 31}, deletedItems[0])
}

func TestMetrics(t *testing.T) {
	c := cache.New[TestStruct]()
	c.Set("item1", TestStruct{Name: "Alice", Age: 30})
	c.Get("item1")
	c.Get("nonexistent")

	assert.Equal(t, 1, c.Metrics["hits"])
	assert.Equal(t, 1, c.Metrics["misses"])
	assert.Equal(t, 1, c.Metrics["items"])
}

func TestMemoryUsage(t *testing.T) {
	c := cache.New[TestStruct]()
	sizeOfItem := int(unsafe.Sizeof(cache.Item[TestStruct]{})) + int(unsafe.Sizeof(TestStruct{})) + int(unsafe.Sizeof(time.Time{}))

	c.Set("item1", TestStruct{Name: "Alice", Age: 30})
	assert.GreaterOrEqual(t, c.Metrics["memoryUsageBytes"], sizeOfItem)

	c.Delete("item1")
	assert.Equal(t, 0, c.Metrics["memoryUsageBytes"])
}
