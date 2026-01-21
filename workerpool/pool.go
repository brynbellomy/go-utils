package workerpool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	butils "github.com/brynbellomy/go-utils"
)

type WorkerPool[T any] struct {
	numWorkers    int
	chWork        chan *Job[T]
	chStop        chan struct{}
	wgDone        *sync.WaitGroup
	activeWorkers int64
}

func NewWorkerPool[T any](numWorkers int) *WorkerPool[T] {
	return &WorkerPool[T]{
		numWorkers:    numWorkers,
		chWork:        make(chan *Job[T], numWorkers*2),
		chStop:        make(chan struct{}),
		wgDone:        &sync.WaitGroup{},
		activeWorkers: 0,
	}
}

type WorkFn[T any] func(ctx context.Context) (T, error)

type Result[T any] struct {
	Val T
	Err error
}

type Job[T any] struct {
	work       func(ctx context.Context) (T, error)
	attempt    int
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
	chResult   chan Result[T]
}

func NewJob[T any](
	maxRetries int,
	baseDelay, maxDelay time.Duration,
	work WorkFn[T],
) *Job[T] {
	return &Job[T]{
		work:       work,
		attempt:    0,
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
		maxDelay:   maxDelay,
		chResult:   make(chan Result[T], 1),
	}
}

func (j *Job[T]) shouldRetry() bool {
	if j.maxRetries < 0 || j.attempt < j.maxRetries {
		return true
	}
	return false
}

func NewJobWithDefaults[T any](work WorkFn[T]) *Job[T] {
	return NewJob(3, 100*time.Millisecond, 5*time.Second, work)
}

func NewSingleShotJob[T any](work WorkFn[T]) *Job[T] {
	return &Job[T]{
		work:       work,
		attempt:    0,
		maxRetries: 1,
		baseDelay:  0,
		maxDelay:   0,
		chResult:   make(chan Result[T], 1),
	}
}

type Batch[T any] struct {
	jobs                []*Job[T]
	maxRetries          int
	baseDelay, maxDelay time.Duration
}

func NewBatch[T any](size int, maxRetries int, baseDelay, maxDelay time.Duration) *Batch[T] {
	return &Batch[T]{
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
		maxDelay:   maxDelay,
	}
}

func (b *Batch[T]) Add(work WorkFn[T]) {
	job := NewJob(b.maxRetries, b.baseDelay, b.maxDelay, work)
	b.jobs = append(b.jobs, job)
}

func (wp *WorkerPool[T]) Start() {
	for range wp.numWorkers {
		wp.workerLoop()
	}
}

func (wp *WorkerPool[T]) Close() {
	close(wp.chStop)
	wp.wgDone.Wait()
}

func (wp *WorkerPool[T]) workerLoop() {
	wp.wgDone.Add(1)
	go func() {
		defer wp.wgDone.Done()

		ctx, cancel := butils.ContextFromChan(wp.chStop)
		defer cancel()
		for {
			select {
			case job, ok := <-wp.chWork:
				if !ok {
					return
				}
				// Track active worker count
				atomic.AddInt64(&wp.activeWorkers, 1)

				val, err := job.work(ctx)
				if err != nil && job.shouldRetry() {
					wp.retryJob(job)
				} else if err != nil {
					job.chResult <- Result[T]{Err: err}
				} else {
					job.chResult <- Result[T]{Val: val}
				}

				// Decrement active worker count
				atomic.AddInt64(&wp.activeWorkers, -1)
			case <-wp.chStop:
				return
			}
		}
	}()
}

func (wp *WorkerPool[T]) retryJob(job *Job[T]) {
	job.attempt++
	delay := job.baseDelay * (1 << (job.attempt - 1)) // exponential backoff
	delay = min(delay, job.maxDelay)

	wp.wgDone.Add(1)
	go func() {
		defer wp.wgDone.Done()

		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-wp.chStop:
			return
		case <-timer.C:
			wp.Submit(job)
		}
	}()
}

func (wp *WorkerPool[T]) Submit(job *Job[T]) <-chan Result[T] {
	select {
	case wp.chWork <- job:
	case <-wp.chStop:
	}
	return job.chResult
}

func (wp *WorkerPool[T]) SubmitBatch(batch *Batch[T]) {
	for _, job := range batch.jobs {
		wp.Submit(job)
	}
}

func (wp *WorkerPool[T]) CollectBatch(batch *Batch[T]) ([]T, []error) {
	vals := make([]T, 0, len(batch.jobs))
	errs := make([]error, 0, len(batch.jobs))
	for i, job := range batch.jobs {
		select {
		case res := <-job.chResult:
			if res.Err != nil {
				errs[i] = res.Err
			} else {
				vals[i] = res.Val
			}

		case <-wp.chStop:
			return nil, []error{errors.New("shutting down")}
		}
	}
	return vals, errs
}
