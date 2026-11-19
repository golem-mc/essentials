package player

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/golem-mc/essentials/core"
)

// Ticks are in-game time: 0 is 6:00, 6000 noon, 18000 midnight.
// Essentials: DescParseTickFormat
const (
	ticksAtMidnight = 18000
	ticksPerDay     = 24000
	ticksPerHour    = 1000
)

var tickNames = map[string]int64{
	"sunrise": 23000, "dawn": 23000,
	"daystart": 0, "day": 0,
	"morning": 1000,
	"midday":  6000, "noon": 6000,
	"afternoon": 9000,
	"sunset":    12000, "dusk": 12000, "sundown": 12000, "nightfall": 12000,
	"nightstart": 14000, "night": 14000,
	"midnight": 18000,
}

var (
	nonTimeChars = regexp.MustCompile(`[^A-Za-z0-9:]`)
	ticksForm    = regexp.MustCompile(`^[0-9]+ti?c?k?s?$`)
	form24       = regexp.MustCompile(`^[0-9]{2}[^0-9]?[0-9]{2}$`)
	form12       = regexp.MustCompile(`^[0-9]{1,2}([^0-9]?[0-9]{2})?(pm|am)$`)
	digits       = regexp.MustCompile(`[^0-9]`)
)

// parseTicks reads a time: ticks ("1000t"), 24-hour ("13:30"), 12-hour
// ("1:30pm") or a name ("noon"); false when it is none of them.
// Essentials: DescParseTickFormat.parse
func parseTicks(desc string) (int64, bool) {
	desc = nonTimeChars.ReplaceAllString(strings.ToLower(desc), "")
	if ticksForm.MatchString(desc) {
		n, err := strconv.ParseInt(digits.ReplaceAllString(desc, ""), 10, 64)
		return n % ticksPerDay, err == nil
	}
	if form24.MatchString(desc) {
		d := digits.ReplaceAllString(desc, "")
		if len(d) == 4 {
			h, _ := strconv.Atoi(d[:2])
			m, _ := strconv.Atoi(d[2:])
			return hoursMinutesToTicks(h, m), true
		}
	}
	if form12.MatchString(desc) {
		d := digits.ReplaceAllString(desc, "")
		var h, m int
		switch len(d) {
		case 4:
			h, _ = strconv.Atoi(d[:2])
			m, _ = strconv.Atoi(d[2:])
		case 3:
			h, _ = strconv.Atoi(d[:1])
			m, _ = strconv.Atoi(d[1:])
		case 2, 1:
			h, _ = strconv.Atoi(d)
		default:
			return 0, false
		}
		if strings.HasSuffix(desc, "pm") && h != 12 {
			h += 12
		}
		if strings.HasSuffix(desc, "am") && h == 12 {
			h -= 12
		}
		return hoursMinutesToTicks(h, m), true
	}
	n, ok := tickNames[desc]
	return n, ok
}

func hoursMinutesToTicks(h, m int) int64 {
	t := int64(ticksAtMidnight) + int64(h)*ticksPerHour + int64(float64(m)/60*ticksPerHour)
	return t % ticksPerDay
}

// meansReset reports whether s asks for the normal time or weather.
func meansReset(s string) bool { return s == "reset" || s == "normal" || s == "default" }

// clock answers the hour and minute ticks show. Essentials:
// DescParseTickFormat.ticksToDate
func clock(ticks int64) (hour, minute int) {
	ticks = ticks - ticksAtMidnight + ticksPerDay
	ticks -= ticks / ticksPerDay * ticksPerDay
	hours := ticks / ticksPerHour
	ticks -= hours * ticksPerHour
	mins := int64(math.Floor(float64(ticks) / (1000.0 / 60)))
	secs := int64(math.Floor((float64(ticks) - float64(mins)*(1000.0/60)) / (1000.0 / 3600)))
	total := (hours*3600 + mins*60 + secs + 1) % 86400
	return int(total / 3600), int(total % 3600 / 60)
}

// formatTicks answers ticks as "1000 ticks". Essentials: formatTicks
func formatTicks(ticks int64) string { return fmt.Sprintf("%d ticks", ticks%ticksPerDay) }

// formatTime answers ticks in timeFormat: 24-hour, 12-hour and ticks, as
// MiniMessage. Essentials: DescParseTickFormat.format
func formatTime(e *core.Essentials, ticks int64) string {
	h, m := clock(ticks)
	h12, ampm := h%12, "AM"
	if h12 == 0 {
		h12 = 12
	}
	if h >= 12 {
		ampm = "PM"
	}
	return e.I18n.Format("", "timeFormat", fmt.Sprintf("%02d:%02d", h, m), fmt.Sprintf("%d:%02d %s", h12, m, ampm), formatTicks(ticks))
}

// mod answers a modulo b, from 0 to b-1 for a negative a too.
func mod(a, b int64) int64 { return (a%b + b) % b }
