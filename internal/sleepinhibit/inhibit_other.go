//go:build !linux && !darwin && !windows

package sleepinhibit

// otherInhibitor is the fallback for any OS without a real
// implementation: it refuses outright (ErrUnsupported) rather than
// silently returning a no-op success, so a caller never shows "Keeping
// this PC awake" when nothing is actually holding the system awake --
// the honest-limits requirement in docs/RUNYX_CODE.md section 8.
type otherInhibitor struct{}

func newPlatformInhibitor() Inhibitor {
	return &otherInhibitor{}
}

func (otherInhibitor) Acquire(reason string) (func(), error) {
	return nil, ErrUnsupported
}
