package async

import "time"

// timeInterface allows mocking time in tests
type timeInterface interface {
	Now() time.Time
	Unix() int64
	After(d time.Duration) <-chan time.Time
}

// realTime implements timeInterface using actual time
type realTime struct{}

func (realTime) Now() time.Time {
	return time.Now()
}

func (realTime) Unix() int64 {
	return time.Now().Unix()
}

func (realTime) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}