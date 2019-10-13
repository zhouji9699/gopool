package main

import (
	"fmt"
	"github.com/gopool"
	"sync"
	"time"
)


func main()  {
	defer gopool.Release()
	runs := 1000
	st := time.Now()
	var wg sync.WaitGroup
	fun := func() {
		fmt.Println("worker !")
		wg.Done()
	}
	for i := 0; i < runs; i++ {
		wg.Add(1)
		_ = gopool.Submit(func() {
			fun()
		})
	}
	wg.Wait()
	fmt.Println(time.Since(st))
	fmt.Printf("running goroutines: %d\n", gopool.Running())
}
