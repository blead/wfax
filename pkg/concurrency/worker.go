package concurrency

import (
	"sync"

	multierror "github.com/hashicorp/go-multierror"
)

// Item is a unit of task executed by concurrent workers
type Item[D any, O any] struct {
	Data   D
	Output O
	Err    error
}

// Execute creates n=concurrency workers to process items with f concurrently and returns the aggregated errors
func Execute[D any, O any](f func(*Item[D, O]) (O, error), items []*Item[D, O], concurrency int) error {
	ich := make(chan *Item[D, O], len(items))
	wg := new(sync.WaitGroup)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker(f, ich, wg)
	}

	for _, i := range items {
		ich <- i
	}

	close(ich)
	wg.Wait()

	var errs error
	for _, i := range items {
		if i.Err != nil {
			errs = multierror.Append(errs, i.Err)
		}
	}

	return errs
}

func worker[D any, O any](f func(*Item[D, O]) (O, error), items chan *Item[D, O], wg *sync.WaitGroup) {
	defer wg.Done()

	for i := range items {
		i.Output, i.Err = f(i)
	}
}
