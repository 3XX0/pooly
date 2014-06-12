package pooly

import (
	"errors"
	"time"
)

// Pooly default variables.
var (
	DefaultDriver = NewNetDriver("tcp")
	DefaultBandit = NewRoundRobin()
)

// Pooly default constants.
const (
	DefaultConnsNum        = 10
	DefaultAttemptsNum     = 3
	DefaultRetryDelay      = 10 * time.Millisecond
	DefaultSeriesNum       = 60
	DefaultPrespawnConnNum = 1
	DefaultCloseDeadline   = 30 * time.Second
	DefaultDecayDuration   = 1 * time.Second
)

// Pooly global errors.
var (
	ErrInvalidArg      = errors.New("pooly: invalid argument")
	ErrPoolClosed      = errors.New("pooly: pool is closed")
	ErrOpTimeout       = errors.New("pooly: operation timed out")
	ErrNoHostAvailable = errors.New("pooly: no host available")
)
