package Timer

import (
	"math/rand"
	"time"
)

var MIN = 2 * 1000
var RANGE = 1 * 1000
var TIMESCALE = time.Millisecond

func GenRandom() time.Duration {
	return time.Duration(MIN+rand.Intn(RANGE)) * time.Second
}

func LeaderTimer() time.Duration {
	return time.Duration(MIN/2) * TIMESCALE
}
