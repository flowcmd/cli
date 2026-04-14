package types

import "time"

type Workflow struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Steps       []Step `yaml:"steps"`
}

type Retry struct {
	Attempts int           `yaml:"attempts"`
	Delay    time.Duration `yaml:"delay"`
}

type Step struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Run         string `yaml:"run"`
	When        string `yaml:"when,omitempty"`
	Parallel    bool   `yaml:"parallel,omitempty"`
	Retry       *Retry `yaml:"retry,omitempty"`
}

type StepState string

const (
	StatePending   StepState = "pending"
	StateRunning   StepState = "running"
	StateCompleted StepState = "completed"
	StateFailed    StepState = "failed"
	StateSkipped   StepState = "skipped"
)

type StepResult struct {
	Name     string
	State    StepState
	Output   string
	Error    string
	ExitCode int
	Started  time.Time
	Finished time.Time
	Attempts int
}

func (r *StepResult) Duration() time.Duration {
	if r.Started.IsZero() {
		return 0
	}
	end := r.Finished
	if end.IsZero() {
		end = time.Now()
	}
	return end.Sub(r.Started)
}
