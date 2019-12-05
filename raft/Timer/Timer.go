package Timer

import (
	"math/rand"
	"time"
	"fmt"
)

var MIN = 2 * 1000
var RANGE = 1 * 1000
var TIMESCALE = time.Millisecond

func GenRandom() time.Duration {
	t := time.Duration(MIN+rand.Intn(RANGE)) * TIMESCALE
	fmt.Println(t)
	return t
}

func LeaderTimer() time.Duration {
	return time.Duration(MIN/2) * TIMESCALE
}