package timeutil

import "time"

type Clock interface {
	Now() time.Time
}

type realClock struct {
}

func NewClock() Clock {
	return realClock{}
}

func (realClock) Now() time.Time {
	return time.Now()
}
