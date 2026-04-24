package domain

import "time"

var bangkokTZ *time.Location

func init() {
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	bangkokTZ = loc
}

func BangkokTZ() *time.Location {
	return bangkokTZ
}

func BangkokMidnight(t time.Time) time.Time {
	bt := t.In(bangkokTZ)
	return time.Date(bt.Year(), bt.Month(), bt.Day(), 0, 0, 0, 0, bangkokTZ)
}

func IsWeekend(t time.Time) bool {
	wd := t.In(bangkokTZ).Weekday()
	return wd == time.Saturday || wd == time.Sunday
}

func NextBusinessDay(t time.Time) time.Time {
	d := BangkokMidnight(t).AddDate(0, 0, 1)
	for IsWeekend(d) {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

func AddBusinessDays(t time.Time, n int) time.Time {
	d := BangkokMidnight(t)
	for i := 0; i < n; i++ {
		d = NextBusinessDay(d)
	}
	return d
}
