package converter

import (
	"time"
)

const (
	microSecond = 1000000
	hours       = 9
	minutes     = 60
	seconds     = 60
	hourOneDay  = 24
)

var JPLoc = time.FixedZone("Asia/Tokyo", hours*minutes*seconds)
