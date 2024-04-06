package util

import "time"

type StopWatch struct {
	StartTime time.Time
	StopTime  time.Time
}

func NewStopWatch() StopWatch {
	return StopWatch{StartTime: time.Now()}
}

// Start starts (or restarts) the stopwatch, clearing the stop time
func (w *StopWatch) Start() {
	w.StartTime = time.Now()
	w.StopTime = time.Time{}
}

// Stop stops the stopwatch and returns the amount of time it's been running
func (w *StopWatch) Stop() time.Duration {
	w.StopTime = time.Now()
	if w.StartTime.IsZero() {
		w.StartTime = w.StopTime
	}

	return w.Elapsed()
}

// Elapsed returns either the duration since the watch was started
// (if it hasn't been stopped yet),
// or the duration between start and stop (if it is stopped)
func (w StopWatch) Elapsed() time.Duration {
	if w.StartTime.IsZero() {
		return 0
	}
	if w.StopTime.IsZero() {
		return time.Since(w.StartTime)
	}
	return w.StopTime.Sub(w.StartTime)
}
