package utils

import (
	"sync"
	"time"
)

func Debounce(f func(), delay time.Duration) func() {
	var mutex sync.Mutex
	var timer *time.Timer

	return func() {
		mutex.Lock()
		defer mutex.Unlock()

		if timer != nil {
			timer.Stop()
		}

		timer = time.AfterFunc(delay, f)
	}
}
