package Timer

import "time"

var MIN       = 2*1000
var RANGE     = 1*1000
var TIMESCALE = time.Milisecond


func GenRandom() time.Duration {
	return (MIN + rand.Intn(RANGE)) * TIMESCALE
}


func LeaderTimer() time.Duration {
	return (MIN/2) * TIMESCALE
}