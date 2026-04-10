package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type Counter struct {
	a     int
	mutex sync.Mutex
}

var a int64
var wg sync.WaitGroup

func main() {

	wg.Add(1)

	go func() {
		increment()
		wg.Done()
	}()

	wg.Wait()

	fmt.Println("Final value of a:", a)
}

func increment() {
	for i := 0; i < 100000000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done() // Signal completion when the goroutine finishes
			atomic.AddInt64(&a, 1)
		}()
	}
}

func (c *Counter) Increment() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.a++

}
