package utils

import (
	"context"
	"sync/atomic"
)

func CollectChan[T any](ctx context.Context, n int, ch <-chan T) []T {
	items := make([]T, n)
	i := 0
	for i := 0; i < n; i++ {
		select {
		case item, open := <-ch:
			if !open {
				break
			}
			items[i] = item
		case <-ctx.Done():
			break
		}
	}
	return items[:i]
}

// WaitGroupChan creates a channel that closes when the provided sync.WaitGroup is done.
type WaitGroupChan struct {
	i         int
	x         int
	chAdd     chan wgAdd
	chWait    chan struct{}
	chCtxDone <-chan struct{}
	chStop    chan struct{}
	waitCalls uint32
}

type wgAdd struct {
	i   int
	err chan string
}

func NewWaitGroupChan(ctx context.Context) *WaitGroupChan {
	wg := &WaitGroupChan{
		chAdd:  make(chan wgAdd),
		chWait: make(chan struct{}),
		chStop: make(chan struct{}),
	}
	if ctx != nil {
		wg.chCtxDone = ctx.Done()
	}

	go func() {
		var done bool
		for {
			select {
			case <-wg.chCtxDone:
				if !done {
					close(wg.chWait)
				}
				return
			case <-wg.chStop:
				if !done {
					close(wg.chWait)
				}
				return
			case wgAdd := <-wg.chAdd:
				if done {
					wgAdd.err <- "WaitGroupChan already finished. Do you need to add a bounding wg.Add(1) and wg.Done()?"
					return
				}
				wg.i += wgAdd.i
				if wg.i < 0 {
					wgAdd.err <- "called Done() too many times"
					close(wg.chWait)
					return
				} else if wg.i == 0 {
					done = true
					close(wg.chWait)
				}
				wgAdd.err <- ""
			}
		}
	}()

	return wg
}

func (wg *WaitGroupChan) Close() {
	close(wg.chStop)
}

func (wg *WaitGroupChan) Add(i int) {
	if atomic.LoadUint32(&wg.waitCalls) > 0 {
		panic("cannot call Add() after Wait()")
	}
	ch := make(chan string)
	select {
	case <-wg.chCtxDone:
	case <-wg.chStop:
	case wg.chAdd <- wgAdd{i, ch}:
		err := <-ch
		if err != "" {
			panic(err)
		}
	}
}

func (wg *WaitGroupChan) Done() {
	ch := make(chan string)
	select {
	case <-wg.chCtxDone:
	case <-wg.chStop:
	case <-wg.chWait:
	case wg.chAdd <- wgAdd{-1, ch}:
		err := <-ch
		if err != "" {
			panic(err)
		}
	}
}

func (wg *WaitGroupChan) Wait() <-chan struct{} {
	atomic.StoreUint32(&wg.waitCalls, 1)
	return wg.chWait
}
