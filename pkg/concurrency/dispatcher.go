package concurrency

import multierror "github.com/hashicorp/go-multierror"

// Dispatcher models two-way communication between 1 dispatcher and n=concurrency workers.
//
// It reads from input channel, calls dispatch, sends resulting items into dispatch channel,
// and waits for processed items from workers via input channel to continue the process.
//
// Each worker reads from dispatch channel, calls work on received items, and sends them back to the dispatcher.
// Dispatcher terminates when input channel is completely drained and no new items are generated from dispatch.
func Dispatcher[D any, O any](dispatch func(*Item[D, O]) ([]*Item[D, O], error), work func(*Item[D, O]) (O, error), items []*Item[D, O], concurrency int) error {
	ich := make(chan *Item[D, O])
	dch := make(chan *Item[D, O])

	for i := 0; i < concurrency; i++ {
		go dispatchWorker(work, dch, ich)
	}
	go func() {
		for _, i := range items {
			ich <- i
		}
	}()

	for pending := len(items); pending > 0; pending-- {
		i := <-ich
		if i.Err != nil {
			continue
		}
		out, err := dispatch(i)
		if err != nil {
			i.Err = err
			continue
		}
		pending += len(out)
		for _, o := range out {
			dch <- o
		}
	}
	close(ich)
	close(dch)

	var errs *multierror.Error
	for _, i := range items {
		if i.Err != nil {
			errs = multierror.Append(errs, i.Err)
		}
	}

	return errs.ErrorOrNil()
}

func dispatchWorker[D any, O any](f func(*Item[D, O]) (O, error), dch chan *Item[D, O], ich chan *Item[D, O]) {
	for i := range dch {
		i.Output, i.Err = f(i)
		go func(i *Item[D, O]) { ich <- i }(i)
	}
}
