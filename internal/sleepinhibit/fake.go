package sleepinhibit

import "sync"

// Fake is a test double for Inhibitor: records every Acquire call and
// whether its release func has been called, with no OS interaction.
// Exported (not test-only) since other packages wiring up
// Runyx Code's daemon need it for their own tests too.
type Fake struct {
	mu           sync.Mutex
	AcquireErr   error // if set, Acquire returns this error instead of succeeding
	Acquisitions []FakeAcquisition
}

type FakeAcquisition struct {
	Reason   string
	Released bool
}

func (f *Fake) Acquire(reason string) (func(), error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.AcquireErr != nil {
		return nil, f.AcquireErr
	}
	idx := len(f.Acquisitions)
	f.Acquisitions = append(f.Acquisitions, FakeAcquisition{Reason: reason})
	released := false
	return func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		if released {
			return
		}
		released = true
		f.Acquisitions[idx].Released = true
	}, nil
}

// ActiveCount returns how many acquisitions have not yet been released.
func (f *Fake) ActiveCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, a := range f.Acquisitions {
		if !a.Released {
			n++
		}
	}
	return n
}
