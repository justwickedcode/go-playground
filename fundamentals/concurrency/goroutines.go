package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// GOROUTINES

func runGoroutines() {
	var wg sync.WaitGroup

	for i := range 3 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			fmt.Println("goroutine", n)
		}(i)
	}

	wg.Wait()
}

// WAITGROUP Go() style

func runWaitGroupGo() {
	var wg sync.WaitGroup

	for i := range 3 {
		wg.Go(func() {
			fmt.Println("wg.Go goroutine", i)
		})
	}

	wg.Wait()
}

// CHANNELS

func runChannels() {
	ch := make(chan string, 3)

	ch <- "one"
	ch <- "two"
	ch <- "three"
	close(ch)

	for msg := range ch {
		fmt.Println(msg)
	}
}

// CHANNELS WITH GOROUTINES

type uploadResult struct {
	fileName string
	duration time.Duration
	timedOut bool
}

func uploadFile(fileName string, ch chan uploadResult) {
	t := time.Now()
	time.Sleep(2 * time.Second)
	ch <- uploadResult{fileName, time.Since(t), false}
}

func runConcurrentUploads() {
	files := []string{"photo.jpg", "video.mp4", "doc.pdf", "music.mp3", "archive.zip"}
	ch := make(chan uploadResult, len(files))

	for _, file := range files {
		go func(f string) {
			innerCh := make(chan uploadResult, 1)
			go uploadFile(f, innerCh)
			select {
			case r := <-innerCh:
				ch <- r
			case <-time.After(3 * time.Second):
				ch <- uploadResult{f, 3 * time.Second, true}
			}
		}(file)
	}

	for range len(files) {
		r := <-ch
		fmt.Printf("%s duration=%v timedOut=%v\n", r.fileName, r.duration, r.timedOut)
	}
}

// SELECT

func runSelect() {
	ch1 := make(chan string)
	ch2 := make(chan string)

	go func() {
		time.Sleep(1 * time.Second)
		ch1 <- "hello from ch1"
	}()

	go func() {
		time.Sleep(2 * time.Second)
		ch2 <- "hello from ch2"
	}()

	timeout := time.After(500 * time.Millisecond)

	for range 3 {
		select {
		case msg := <-ch1:
			fmt.Println(msg)
		case msg := <-ch2:
			fmt.Println(msg)
		case <-timeout:
			fmt.Println("timed out")
		}
	}
}

// MUTEX

type Account struct {
	balance int
	mu      sync.Mutex
}

func (a *Account) Deposit(amount int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.balance += amount
}

func (a *Account) Withdraw(amount int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.balance < amount {
		return false
	}
	a.balance -= amount
	return true
}

func runMutex() {
	acc := &Account{balance: 1000}
	var wg sync.WaitGroup

	for range 5 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			acc.Deposit(n)
		}(100)
	}

	for range 5 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			acc.Withdraw(n)
		}(80)
	}

	wg.Wait()
	fmt.Println("final balance:", acc.balance)
}

// WORKER POOL

func worker(id int, jobs <-chan int, results chan<- int) {
	for j := range jobs {
		results <- j * j
	}
}

func runWorkerPool() {
	jobs := make(chan int, 100)
	results := make(chan int, 100)
	var wg sync.WaitGroup
	var wgCollector sync.WaitGroup

	for id := range 3 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			worker(id, jobs, results)
		}(id)
	}

	wgCollector.Add(1)
	go func() {
		defer wgCollector.Done()
		for r := range results {
			fmt.Println("result:", r)
		}
	}()

	for i := range 10 {
		jobs <- i
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	wgCollector.Wait()
}

// CONTEXT

func workerWithContext(ctx context.Context, id int, jobs <-chan int, results chan<- int) {
	for {
		select {
		case <-ctx.Done():
			return
		case j, ok := <-jobs:
			if !ok {
				return
			}
			results <- j * j
		}
	}
}

func runContext() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var completed int64
	jobs := make(chan int, 100)
	results := make(chan int, 100)
	var wg sync.WaitGroup
	var wgCollector sync.WaitGroup

	for id := range 3 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			workerWithContext(ctx, id, jobs, results)
		}(id)
	}

	wgCollector.Add(1)
	go func() {
		defer wgCollector.Done()
		for range results {
			atomic.AddInt64(&completed, 1)
		}
	}()

outer:
	for i := range 10000 {
		select {
		case jobs <- i:
		case <-ctx.Done():
			fmt.Println("timeout while sending jobs")
			break outer
		}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	wgCollector.Wait()
	fmt.Printf("completed %d / 10000\n", atomic.LoadInt64(&completed))
}

func main() {
	fmt.Println("goroutines")
	runGoroutines()

	fmt.Println("waitgroup go")
	runWaitGroupGo()

	fmt.Println("channels")
	runChannels()

	fmt.Println("concurrent uploads")
	runConcurrentUploads()

	fmt.Println("select")
	runSelect()

	fmt.Println("mutex")
	runMutex()

	fmt.Println("worker pool")
	runWorkerPool()

	fmt.Println("context")
	runContext()
}
