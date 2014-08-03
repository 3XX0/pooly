package pooly

import (
	"errors"
	"time"
)

// Pooly default constants.
const (
	DefaultMaxConns   = 30
	DefaultMaxRetries = 3
	DefaultRetryDelay = 10 * time.Millisecond

	DefaultPrespawnConns        = 1
	DefaultMaxAttempts          = 3
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
