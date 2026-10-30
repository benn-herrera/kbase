package model

import "fmt"

// Thinking is how much a chat call may reason before it answers. The zero
// value is ThinkingOff.
type Thinking int

const (
	ThinkingOff Thinking = iota
	ThinkingLow
	ThinkingMedium
	ThinkingHigh
)

// String is the level's name: off, low, medium or high.
func (t Thinking) String() string {
	switch t {
	case ThinkingOff:
		return "off"
	case ThinkingLow:
		return "low"
	case ThinkingMedium:
		return "medium"
	case ThinkingHigh:
		return "high"
	}
	return fmt.Sprintf("Thinking(%d)", int(t))
}

// DefaultTemperature is the temperature a chat call carries where its role
// states none: greedy decode, kbase's default rather than the server's.
const DefaultTemperature = 0.0

// Sampling is a chat call's temperature and thinking level as configured, a
// nil field being kbase's default: temperature DefaultTemperature, thinking
// off.
type Sampling struct {
	Temperature *float64
	Thinking    *Thinking
}

// Resolved is s with every nil field filled with kbase's default, so neither
// field of the result is nil. Both are fresh storage: the result shares
// nothing with s.
func (s Sampling) Resolved() Sampling {
	temperature, thinking := DefaultTemperature, ThinkingOff
	if s.Temperature != nil {
		temperature = *s.Temperature
	}
	if s.Thinking != nil {
		thinking = *s.Thinking
	}
	return Sampling{Temperature: &temperature, Thinking: &thinking}
}

// SamplingOf is the effective sampling c asks for on every chat call,
// resolved. It is a copy, so a caller writing through it changes nothing c sends.
func SamplingOf(c Client) Sampling {
	return c.sampling().Resolved()
}
