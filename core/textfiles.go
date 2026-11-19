package core

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
)

var startTime = time.Now()

// ReplaceKeywords fills Essentials' {KEYWORDS} in s for u (nil for the
// console). Essentials: KeywordReplacer
func (e *Essentials) ReplaceKeywords(s string, u *User) string {
	if !strings.Contains(s, "{") {
		return s
	}
	var p plugin.Player
	if u != nil {
		p, _ = u.Player()
	}
	val := map[string]func() string{
		"PLAYER":      func() string { return nameOr(u, func(u *User) string { return u.DisplayName() }) },
		"DISPLAYNAME": func() string { return nameOr(u, func(u *User) string { return u.DisplayName() }) },
		"USERNAME":    func() string { return nameOr(u, func(u *User) string { return u.Name() }) },
		"NICKNAME":    func() string { return nameOr(u, func(u *User) string { return u.Nick() }) },
		"PREFIX":      func() string { return "" },
		"SUFFIX":      func() string { return "" },
		"GROUP":       func() string { return "" },
		"BALANCE": func() string {
			if u == nil {
				return ""
			}
			return e.Economy.Format(u.Money())
		},
		"MAILS": func() string {
			if u == nil {
				return "0"
			}
			return strconv.Itoa(len(u.UnreadMail()))
		},
		"WORLD": func() string {
			if p == nil {
				return ""
			}
			return WorldName(p.World())
		},
		"WORLDNAME": func() string {
			if p == nil {
				return ""
			}
			return WorldName(p.World())
		},
		"ONLINE": func() string {
			n := 0
			for _, o := range e.OnlineUsers() {
				if u == nil || u.CanSee(o) {
					n++
				}
			}
			return strconv.Itoa(n)
		},
		"UNIQUE":     func() string { return strconv.Itoa(len(e.Users.All())) },
		"WORLDS":     func() string { return strings.Join(WorldNames(e.Server), ", ") },
		"PLAYERLIST": func() string { return strings.Join(e.PlayerNames(u), ", ") },
		"TIME":       func() string { return time.Now().Format("15:04:05") },
		"DATE":       func() string { return time.Now().Format("2006-01-02") },
		"WORLDTIME12": func() string {
			if p == nil {
				return ""
			}
			return FormatWorldTime(p.World().DayTime(), true)
		},
		"WORLDTIME24": func() string {
			if p == nil {
				return ""
			}
			return FormatWorldTime(p.World().DayTime(), false)
		},
		"WORLDDATE": func() string {
			if p == nil {
				return ""
			}
			return strconv.FormatInt(p.World().DayTime()/24000, 10)
		},
		"COORDS": func() string {
			if p == nil {
				return ""
			}
			return e.TlString("coordsKeyword", int(p.Pos().X), int(p.Pos().Y), int(p.Pos().Z))
		},
		"TPS":    func() string { return FormatDouble(e.Server.Ticks().TPS()[0]) },
		"UPTIME": func() string { return FormatDateDiff(startTime) },
		"IP": func() string {
			if p == nil {
				return ""
			}
			return hostOf(p)
		},
		"ADDRESS": func() string {
			if p == nil {
				return ""
			}
			return p.Address().String()
		},
		"PLUGINS": func() string {
			var names []string
			for _, i := range e.Server.Plugins() {
				if i.Enabled {
					names = append(names, i.Name)
				}
			}
			return strings.Join(names, ", ")
		},
		"VERSION": func() string {
			n, proto := e.Server.Version()
			return fmt.Sprintf("golem %s (%d), Go %s", n, proto, runtime.Version())
		},
		"PLAYTIME": func() string {
			if u == nil || u.Data.Timestamps.Login == 0 {
				return ""
			}
			return FormatDateDiff(time.UnixMilli(u.Data.Timestamps.Login))
		},
	}
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '{')
		if i < 0 {
			break
		}
		j := strings.IndexByte(s[i:], '}')
		if j < 0 {
			break
		}
		key := s[i+1 : i+j]
		if f, ok := val[strings.ToUpper(key)]; ok {
			b.WriteString(s[:i])
			b.WriteString(f())
		} else {
			b.WriteString(s[:i+j+1])
		}
		s = s[i+j+1:]
	}
	b.WriteString(s)
	return b.String()
}

func nameOr(u *User, f func(*User) string) string {
	if u == nil {
		if e, ok := Get(); ok {
			return e.TlString("consoleName")
		}
		return "Console"
	}
	return f(u)
}

func hostOf(p plugin.Player) string {
	a := p.Address().String()
	if i := strings.LastIndexByte(a, ':'); i >= 0 {
		a = a[:i]
	}
	return strings.Trim(a, "[]")
}

// HostOf answers the player's IP address without the port.
func HostOf(p plugin.Player) string { return hostOf(p) }

// FormatWorldTime answers a world's day time as 12 or 24 hour clock time;
// tick 0 is 6:00. Essentials: DescParseTickFormat
func FormatWorldTime(ticks int64, twelve bool) string {
	t := (ticks + 6000) % 24000
	hours := t / 1000
	minutes := (t % 1000) * 60 / 1000
	if !twelve {
		return fmt.Sprintf("%02d:%02d", hours, minutes)
	}
	suffix := "AM"
	if hours >= 12 {
		suffix = "PM"
	}
	h := hours % 12
	if h == 0 {
		h = 12
	}
	return fmt.Sprintf("%d:%02d %s", h, minutes, suffix)
}

// TextLines answers a text file's lines from the data directory, with the
// keywords filled in for u: motd.txt, rules.txt, info.txt and so on. A
// per-player file (motd_<name>.txt or <file>_<group>) is not looked for.
func (e *Essentials) TextLines(file string, u *User) []string {
	b, err := os.ReadFile(e.DataPath(file))
	if err != nil {
		return nil
	}
	var out []string
	for line := range strings.SplitSeq(strings.ReplaceAll(string(b), "\r", ""), "\n") {
		if strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "##") {
			out = append(out, line)
			continue
		}
		out = append(out, e.ReplaceKeywords(line, u))
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// ShowText shows lines one page (9 lines) or chapter at a time, as /info,
// /rules and /motd do; commandName names the command for its headers (""
// for none) and onePage shows up to 20 lines without headers.
// Essentials: TextPager.showPage
func (e *Essentials) ShowText(sender plugin.CommandSender, lines []string, pageStr, chapterPageStr, commandName string, onePage bool) {
	send := func(line string) { sender.SendMessage(e.Mini(line)) }
	chapters := []string{}
	bookmarks := map[string]int{}
	for i, l := range lines {
		if strings.HasPrefix(l, "#") && len(l) > 1 {
			name := strings.TrimSpace(l[1:])
			chapters = append(chapters, name)
			bookmarks[strings.ToLower(name)] = i
		}
	}
	if pageStr == "" || IsInt(pageStr) {
		if len(lines) > 0 && strings.HasPrefix(lines[0], "#") {
			if onePage {
				return
			}
			e.Send(sender, "infoChapter")
			sender.SendMessage(plugin.Plain(strings.Join(chapters, ", ")))
			return
		}
		page := 1
		if n, err := strconv.Atoi(pageStr); err == nil && n > 1 {
			page = n
		}
		start := (page - 1) * 9
		if onePage {
			start = 0
		}
		end := 0
		for end < len(lines) && !strings.HasPrefix(lines[end], "#") {
			end++
		}
		pages := (end + 8) / 9
		if page > pages && end > 0 {
			e.Send(sender, "infoUnknownChapter")
			return
		}
		if !onePage && commandName != "" {
			e.Send(sender, "infoPages", page, pages, capitalTitle(commandName))
		}
		limit := 9
		if onePage {
			limit = 20
		}
		for i := start; i < end && i < start+limit; i++ {
			send(lines[i])
		}
		if !onePage && page < pages && commandName != "" {
			e.Send(sender, "readNextPage", commandName, page+1)
		}
		return
	}
	chapterPage := 0
	if n, err := strconv.Atoi(chapterPageStr); err == nil && n > 1 {
		chapterPage = n - 1
	}
	at, ok := bookmarks[strings.ToLower(pageStr)]
	if !ok {
		e.Send(sender, "infoUnknownChapter")
		return
	}
	cs := at + 1
	ce := cs
	for ce < len(lines) && !strings.HasPrefix(lines[ce], "#") {
		ce++
	}
	start := cs + chapterPage*9
	if onePage {
		start = cs
	}
	page, pages := chapterPage+1, (ce-cs+8)/9
	if !onePage && commandName != "" {
		e.Send(sender, "infoChapterPages", capitalTitle(commandName)+": "+pageStr, page, pages)
	}
	limit := 9
	if onePage {
		limit = 20
	}
	for i := start; i < ce && i < start+limit; i++ {
		send(lines[i])
	}
	if !onePage && page < pages && commandName != "" {
		e.Send(sender, "readNextPage", commandName, pageStr+" "+strconv.Itoa(page+1))
	}
}

// capitalTitle capitalises a command name for a page header: "info" is
// "Info", "help foo" is "Help: foo". Essentials: I18n.capitalCase
func capitalTitle(name string) string {
	first, rest, ok := strings.Cut(name, " ")
	t := strings.ToUpper(first[:1]) + first[1:]
	if ok {
		return t + ": " + rest
	}
	return t
}
