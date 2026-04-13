package types

import (
	"testing"
	"time"
)

func TestDuration(t *testing.T) {
	r := &StepResult{}
	if d := r.Duration(); d != 0 {
		t.Errorf("zero start should give zero duration, got %v", d)
	}

	r.Started = time.Now().Add(-50 * time.Millisecond)
	if d := r.Duration(); d <= 0 {
		t.Errorf("running step should report positive duration, got %v", d)
	}

	r.Finished = r.Started.Add(40 * time.Millisecond)
	got := r.Duration()
	if got != 40*time.Millisecond {
		t.Errorf("finished duration mismatch: got %v want 40ms", got)
	}
}
