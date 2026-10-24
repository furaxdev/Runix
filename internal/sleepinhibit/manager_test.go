package sleepinhibit

import (
	"testing"
	"time"
)

func TestAcquireReleaseRoundTrip(t *testing.T) {
	fake := &Fake{}
	m := NewManager(fake)

	if err := m.Acquire("working"); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	held, _, reason := m.Status()
	if !held || reason != "working" {
		t.Errorf("Status = held=%v reason=%q, want held=true reason=working", held, reason)
	}
	if fake.ActiveCount() != 1 {
		t.Errorf("fake.ActiveCount() = %d, want 1", fake.ActiveCount())
	}

	m.Release()
	held, _, _ = m.Status()
	if held {
		t.Error("expected not held after Release")
	}
	if fake.ActiveCount() != 0 {
		t.Errorf("fake.ActiveCount() = %d, want 0 after release", fake.ActiveCount())
	}
}

func TestRefCountingHoldsUntilLastRelease(t *testing.T) {
	fake := &Fake{}
	m := NewManager(fake)

	m.Acquire("turn 1")
	m.Acquire("turn 2") // concurrent tool call in another turn
	if len(fake.Acquisitions) != 1 {
		t.Fatalf("expected the underlying Inhibitor to be acquired exactly once, got %d calls", len(fake.Acquisitions))
	}

	m.Release() // turn 1 done
	held, _, _ := m.Status()
	if !held {
		t.Error("expected still held: turn 2 hasn't released yet")
	}

	m.Release() // turn 2 done
	held, _, _ = m.Status()
	if held {
		t.Error("expected released once every acquirer has released")
	}
}

func TestForceReleaseDropsRegardlessOfRefCount(t *testing.T) {
	fake := &Fake{}
	m := NewManager(fake)
	m.Acquire("a")
	m.Acquire("b")
	m.Acquire("c")

	m.ForceRelease("stop button")
	held, _, _ := m.Status()
	if held {
		t.Error("expected ForceRelease to drop the hold regardless of ref count")
	}
	if fake.ActiveCount() != 0 {
		t.Errorf("fake.ActiveCount() = %d, want 0", fake.ActiveCount())
	}
}

func TestExtraReleaseIsHarmless(t *testing.T) {
	fake := &Fake{}
	m := NewManager(fake)
	m.Acquire("x")
	m.Release()
	m.Release() // one too many -- must not panic or go negative
	held, _, _ := m.Status()
	if held {
		t.Error("expected not held")
	}
}

func TestAcquireErrorIsPropagatedAndNeverClaimsHeld(t *testing.T) {
	fake := &Fake{AcquireErr: ErrUnsupported}
	m := NewManager(fake)
	if err := m.Acquire("x"); err != ErrUnsupported {
		t.Fatalf("Acquire error = %v, want ErrUnsupported", err)
	}
	held, _, _ := m.Status()
	if held {
		t.Error("Status must never report held=true when the underlying Acquire failed")
	}
}

func TestMaxHoldForceReleasesAndEmitsEvent(t *testing.T) {
	fake := &Fake{}
	m := NewManager(fake).WithMaxHold(30 * time.Millisecond)
	m.Acquire("long task")

	select {
	case e := <-m.Events:
		if e.Reason != "acquired" {
			t.Fatalf("first event = %+v, want acquired", e)
		}
	case <-time.After(time.Second):
		t.Fatal("expected an 'acquired' event")
	}

	select {
	case e := <-m.Events:
		if e.Held || e.Reason != "max-hold-exceeded" {
			t.Errorf("event = %+v, want {Held:false Reason:max-hold-exceeded}", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected a max-hold-exceeded release event")
	}

	held, _, _ := m.Status()
	if held {
		t.Error("expected released after max hold elapsed")
	}
}

func TestBatteryBelowThresholdReleasesAndPauses(t *testing.T) {
	fake := &Fake{}
	m := NewManager(fake)
	defer m.Close()
	m.WithBatteryCheck(func() (BatteryStatus, bool) {
		return BatteryStatus{Percent: 15, OnBattery: true}, true
	}, 10*time.Millisecond, 20)

	m.Acquire("working")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if held, _, _ := m.Status(); !held {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	held, _, _ := m.Status()
	if held {
		t.Error("expected the inhibit to be released once battery dropped below the threshold")
	}
}

func TestBatteryAboveThresholdDoesNotRelease(t *testing.T) {
	fake := &Fake{}
	m := NewManager(fake)
	defer m.Close()
	m.WithBatteryCheck(func() (BatteryStatus, bool) {
		return BatteryStatus{Percent: 80, OnBattery: true}, true
	}, 10*time.Millisecond, 20)

	m.Acquire("working")
	time.Sleep(60 * time.Millisecond) // let a few ticks pass
	held, _, _ := m.Status()
	if !held {
		t.Error("expected the inhibit to stay held when battery is above the threshold")
	}
}

func TestKeepAwakeOnBatterySkipsTheCheck(t *testing.T) {
	fake := &Fake{}
	m := NewManager(fake)
	defer m.Close()
	m.SetKeepAwakeOnBattery(true)
	m.WithBatteryCheck(func() (BatteryStatus, bool) {
		return BatteryStatus{Percent: 5, OnBattery: true}, true
	}, 10*time.Millisecond, 20)

	m.Acquire("working")
	time.Sleep(60 * time.Millisecond)
	held, _, _ := m.Status()
	if !held {
		t.Error("expected 'Keep awake on battery' to override the low-battery release")
	}
}

func TestOnACPowerIsNeverReleasedByBatteryCheck(t *testing.T) {
	fake := &Fake{}
	m := NewManager(fake)
	defer m.Close()
	m.WithBatteryCheck(func() (BatteryStatus, bool) {
		return BatteryStatus{Percent: 1, OnBattery: false}, true
	}, 10*time.Millisecond, 20)

	m.Acquire("working")
	time.Sleep(60 * time.Millisecond)
	held, _, _ := m.Status()
	if !held {
		t.Error("expected the inhibit to stay held while on AC power, regardless of percent")
	}
}
