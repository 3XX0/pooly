package pooly

import (
	"errors"
	"time"
)

// Pooly default constants.
const (
	DefaultMaxConns    = 30
	DefaultConnRetries = 3
	DefaultRetryDelay  = 10 * time.Millisecond

	DefaultPrespawnConns        = 1
	DefaultGetAttempts          = 3
	DefaultCloseDeadline        = 30 * time.Second
	DefaultDecayDuration        = 1 * time.Minute
	DefaultMemoizeScoreDuration = 100 * time.Millisecond
)

// Pooly global errors.
var (
	ErrInvalidArg      = errors.New("pooly: invalid argument")
	ErrPoolClosed      = errors.New("pooly: pool is closed")
	ErrOpTimeout       = errors.New("pooly: operation timed out")
	ErrNoHostAvailable = errors.New("pooly: no host available")
)

// statsd sample rate in percentage
var sampleRate float32 = 1.0

// SetStatsdSampleRate sets the metrics sampling rate.
// It takes a float between 0 and 1 which defines the percentage of the time
// where pooly will send its service metrics to statsd.
func SetStatsdSampleRate(r float32) {
	sampleRate = r
}
