package core

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
)

// WorldName answers the name Essentials shows and saves for a world: the
// Bukkit folder names for the vanilla dimensions (world, world_nether,
// world_the_end), else the key's path, or the whole key outside the
// minecraft namespace.
func WorldName(w plugin.World) string {
	if w == nil {
		return ""
	}
	k := w.Key()
	if k.Namespace == "minecraft" {
		switch k.Path {
		case "overworld":
			return "world"
		case "the_nether":
			return "world_nether"
		case "the_end":
			return "world_the_end"
		}
		return k.Path
	}
	return k.String()
}

// WorldByName finds a loaded world by WorldName, by its key, or by a
// dimension's short name (overworld, nether, end), ignoring case.
func WorldByName(s plugin.Server, name string) (plugin.World, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "nether", "the_nether":
		name = "world_nether"
	case "end", "the_end":
		name = "world_the_end"
	case "overworld", "normal":
		name = "world"
	}
	for _, w := range s.Worlds() {
		if strings.ToLower(WorldName(w)) == name || w.Key().String() == name || w.Key().Path == name {
			return w, true
		}
	}
	return nil, false
}

// WorldMatches reports whether w is the world name names.
func WorldMatches(w plugin.World, name string) bool {
	return w != nil && (strings.EqualFold(WorldName(w), name) || w.Key().String() == strings.ToLower(name))
}

// WorldNames answers the loaded worlds' names, for completion.
func WorldNames(s plugin.Server) []string {
	var out []string
	for _, w := range s.Worlds() {
		out = append(out, WorldName(w))
	}
	return out
}

var timePattern = regexp.MustCompile(`(?i)^(?:([0-9]+)\s*y[a-z]*[,\s]*)?(?:([0-9]+)\s*mo[a-z]*[,\s]*)?(?:([0-9]+)\s*w[a-z]*[,\s]*)?(?:([0-9]+)\s*d[a-z]*[,\s]*)?(?:([0-9]+)\s*h[a-z]*[,\s]*)?(?:([0-9]+)\s*m[a-z]*[,\s]*)?(?:([0-9]+)\s*(?:s[a-z]*)?)?`)

// ParseDateDiff reads a duration like "1d2h30m" and answers that time from
// now, into the future or the past; at most ten years ahead. illegalDate
// when nothing in s is a duration. Essentials: DateUtil.parseDateDiff
func ParseDateDiff(s string, future bool) (time.Time, error) {
	m := timePattern.FindStringSubmatch(strings.TrimSpace(s))
	found := false
	n := make([]int, 7)
	for i := 1; i < len(m); i++ {
		if m[i] != "" {
			found = true
			n[i-1], _ = strconv.Atoi(m[i])
		}
	}
	if !found {
		return time.Time{}, Tlf("illegalDate")
	}
	sign := 1
	if !future {
		sign = -1
	}
	t := time.Now().AddDate(sign*min(n[0], 100000), sign*n[1], sign*(n[2]*7+n[3]))
	t = t.Add(time.Duration(sign) * (time.Duration(n[4])*time.Hour + time.Duration(n[5])*time.Minute + time.Duration(n[6])*time.Second))
	if limit := time.Now().AddDate(10, 0, 0); t.After(limit) {
		return limit, nil
	}
	return t, nil
}

// unitNames are the message keys of the date units, singular then plural.
var unitNames = [][2]string{{"year", "years"}, {"month", "months"}, {"day", "days"}, {"hour", "hours"}, {"minute", "minutes"}, {"second", "seconds"}}

// FormatDateDiff answers the time between now and t in the three largest
// units, e.g. "2 days 3 hours 1 minute". Essentials: DateUtil.formatDateDiff
func FormatDateDiff(t time.Time) string {
	return formatDateDiff(time.Now(), t)
}

func formatDateDiff(from, to time.Time) string {
	tl := func(key string) string {
		if e, ok := Get(); ok {
			return e.TlString(key)
		}
		return key
	}
	if from.Equal(to) {
		return tl("now")
	}
	future := to.After(from)
	if future {
		to = to.Add(50 * time.Millisecond)
	} else {
		from, to = to.Add(-50*time.Millisecond), from
	}
	var parts []string
	cur := from
	steps := []func(time.Time, int) time.Time{
		func(t time.Time, n int) time.Time { return t.AddDate(n, 0, 0) },
		func(t time.Time, n int) time.Time { return t.AddDate(0, n, 0) },
		func(t time.Time, n int) time.Time { return t.AddDate(0, 0, n) },
		func(t time.Time, n int) time.Time { return t.Add(time.Duration(n) * time.Hour) },
		func(t time.Time, n int) time.Time { return t.Add(time.Duration(n) * time.Minute) },
		func(t time.Time, n int) time.Time { return t.Add(time.Duration(n) * time.Second) },
	}
	for i, step := range steps {
		if len(parts) > 2 {
			break
		}
		n := 0
		for !step(cur, n+1).After(to) {
			n++
			if n > 100000 {
				break
			}
		}
		if n > 0 {
			cur = step(cur, n)
			name := unitNames[i][0]
			if n > 1 {
				name = unitNames[i][1]
			}
			parts = append(parts, fmt.Sprintf("%d %s", n, tl(name)))
		}
	}
	if len(parts) == 0 {
		return tl("now")
	}
	return strings.Join(parts, " ")
}

// FormatDuration answers d as FormatDateDiff writes it.
func FormatDuration(d time.Duration) string {
	now := time.Now()
	return formatDateDiff(now, now.Add(d))
}

// FormatDouble answers f with at most two decimals, as Essentials'
// NumberUtil.formatDouble does.
func FormatDouble(f float64) string {
	s := strconv.FormatFloat(f, 'f', 2, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "-0" {
		return "0"
	}
	return s
}

// IsInt reports whether s is a whole number.
func IsInt(s string) bool { _, err := strconv.Atoi(s); return err == nil }

// IsPositiveInt reports whether s is a whole number above zero.
func IsPositiveInt(s string) bool { n, err := strconv.Atoi(s); return err == nil && n > 0 }

// LocationString answers a location for messages: world, x, y, z.
func LocationString(l plugin.Location) string {
	return fmt.Sprintf("%s, %d, %d, %d", WorldName(l.World), int(l.Pos.X), int(l.Pos.Y), int(l.Pos.Z))
}

// NotifyOfMail tells the user how many unread mails they have, at most once
// per notify-player-of-mail-cooldown. Essentials: User.notifyOfMail
func (e *Essentials) NotifyOfMail(u *User) {
	n := len(u.UnreadMail())
	if n == 0 {
		return
	}
	cd := e.Config.Seconds("notify-player-of-mail-cooldown", 60)
	if last, ok := SessionValue[time.Time](u, "core.mailNotified"); ok && time.Since(last) < cd {
		return
	}
	u.Session["core.mailNotified"] = time.Now()
	u.Send("youHaveNewMail", n)
}

// SendMail delivers a mail to to from from (nil for the console), expiring
// after expire (0 never). Essentials: MailServiceImpl.sendMail
func (e *Essentials) SendMail(to *User, from *User, message string, expire time.Duration) {
	m := MailMessage{Message: message, Timestamp: time.Now().UnixMilli(), SenderName: e.TlString("consoleName"), SenderUUID: "null"}
	if from != nil {
		m.SenderName, m.SenderUUID = from.Name(), from.ID.String()
	}
	if expire > 0 {
		m.Expire = time.Now().Add(expire).UnixMilli()
	}
	// Newest first; the recipient hears of it at their next command.
	to.Data.Mail = append([]MailMessage{m}, to.Data.Mail...)
	to.Save()
}
