package sync

import (
	"sync"
)

// Once is an object that will perform exactly one action.
type Once struct {
	done uint32
	m    sync.Mutex
}

// Do calls function once, if it return error do not lock call
func (o *Once) Do(f func() error) error {
	o.m.Lock()
	defer o.m.Unlock()

	var err error

	if o.done == 0 {
		err = f()
		if err != nil {
			o.done = 0
		} else {
			o.done = 1
		}
	}

	return err
}

// Reset reset once
func (o *Once) Reset() {
	o.m.Lock()
	defer o.m.Unlock()

	o.done = 0
}
