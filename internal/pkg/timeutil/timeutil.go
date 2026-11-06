package timeutil

import "time"

func WIB() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil || loc == nil {
		return time.FixedZone("WIB", 7*60*60)
	}
	return loc
}

func NowWIB() time.Time {
	return time.Now().In(WIB())
}

func WithinMinutes(now time.Time, startMin, endMin int) bool {
	nowMin := now.Hour()*60 + now.Minute()
	if startMin <= endMin {
		return nowMin >= startMin && nowMin <= endMin
	}
	return nowMin >= startMin || nowMin <= endMin
}
