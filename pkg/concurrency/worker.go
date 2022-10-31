package concurrency

import (
	"sync"

	multierror "github.com/hashicorp/go-multierror"
)

// Item is a unit of task executed by concurrent workers
type Item struct {
	Data   interface{}
	Output interface{}
	Err    error
}

// Execute creates n=`concurrency` workers to process `items` with `f` concurrently and returns the aggregated errors
func Execute(f func(*Item) (interface{}, error), items []*Item, concurrency int) error {
	ich := make(chan *Item, len(items))
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

func worker(f func(*Item) (interface{}, error), items chan *Item, wg *sync.WaitGroup) {
	defer wg.Done()

	for i := range items {
		i.Output, i.Err = f(i)
	}
}
