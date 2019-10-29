package Timer

import "time"

var RANGE     = 100
var TIMESCALE = time.Second


func GenRandom() time.Duration {
	return rand.Intn(RANGE) * TIMESCALE
}