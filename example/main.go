package main

import (
	"fmt"
	"os"
	"time"

	cache "github.com/kamludwinski2/simplecache"
)

type TestStruct struct {
	Name string
	Age  int
}

func NewTestStruct(name string, age int) TestStruct {
	return TestStruct{name, age}
}

func main() {
	fmt.Println("Hello World")

	onCreateFunc := func(records []TestStruct) {
		fmt.Println("created records: ")
		fmt.Println(records)
	}

	onUpdateFunc := func(records []TestStruct) {
		fmt.Println("updated records: ")
		fmt.Println(records)
	}

	onDeleteFunc := func(records []TestStruct) {
		fmt.Println("deleted records: ")
		fmt.Println(records)
	}

	onExpiryFunc := func(key string, item cache.Item[TestStruct]) {
		fmt.Printf("expired record key: %s\n", key)
		fmt.Println(item)
	}

	eqFunc := func(a, b TestStruct) bool {
		return a.Name == b.Name && a.Age == b.Age
	}

	c := cache.New[TestStruct]().
		Equals(eqFunc).                // equality check, used to determine updates
		WithInterval(1 * time.Second). // ticker interval to check for cache changes
		OnBeforeTick(func() {
			fmt.Println("before tick")
		}). // debug message before tick
		OnAfterTick(func() {
			fmt.Println("after tick")
		}).                     // debug message after tick
		OnCreate(onCreateFunc). // create middleware
		OnUpdate(onUpdateFunc). // update middleware
		OnDelete(onDeleteFunc). // delete middleware
		OnExpiry(onExpiryFunc)  // expiry middleware

	go c.Maintain() // start maintaining cache (ticker)

	time.Sleep(2 * time.Second) // initial delay to perform some "ticks"

	s1 := NewTestStruct("S1", 1)
	s2 := NewTestStruct("S2", 2)

	c.Set(s1.Name, s1)                                // set a non expiring item
	c.Set(s2.Name, s2, time.Now().Add(3*time.Second)) // Expires in 3 seconds

	time.Sleep(5 * time.Second) // simulate delay

	fmt.Println(c.Metrics)
	s3 := NewTestStruct("S3", 3)
	c.Set(s3.Name, s3)
	c.Get(s1.Name)  // get an item to "hit"
	c.Get("random") // get a non-existent item to "miss"

	go func() {
		time.Sleep(5 * time.Second)
		c.Stop() // stop maintaining cache
	}()

	time.Sleep(3 * time.Second)

	fmt.Println(c.Metrics)
	c.Delete(s1.Name) // delete existing item

	fmt.Println(c.Metrics)

	time.Sleep(10 * time.Second) // artificial delay
	fmt.Println("finished")

	os.Exit(0) // not needed, but I like to ensure no memory leaks. os.Exit() also ends any goroutines
}
