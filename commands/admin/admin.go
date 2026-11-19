// Package admin holds EssentialsX's admin commands: bans, kicks, jails,
// /essentials, /gc, /backup and /ext.
package admin

import (
	"net"
	"regexp"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

// Register registers the group's commands and listeners.
func Register(e *core.Essentials) {
	registerBans(e)
	registerKicks(e)
	registerJails(e)
	registerEssentials(e)
	registerMisc(e)
	registerBackup(e)
}

// consoleName is Essentials' Console.NAME, the source a console ban is
// recorded with.
const consoleName = "Console"

// sourceName answers who a ban is recorded as made by: the player's display
// name, or Console.
func sourceName(ctx *core.Ctx) string {
	if ctx.User != nil {
		return ctx.User.DisplayName()
	}
	return consoleName
}

// isPlayerWithout reports whether a player sent the command and lacks node;
// the console has every node.
func isPlayerWithout(ctx *core.Ctx, node string) bool {
	return ctx.User != nil && !ctx.User.HasPermission(node)
}

var (
	legacyCode = regexp.MustCompile(`(?i)&([0-9a-fk-or])`)
	legacyHex  = regexp.MustCompile(`(?i)&#([0-9a-f]{6})`)
)

// replaceFormat turns the & codes (and &#rrggbb) a staff member typed into §
// codes, which messages show as colours. Essentials: FormatUtil.replaceFormat
func replaceFormat(s string) string {
	s = legacyHex.ReplaceAllStringFunc(s, func(m string) string { return "§#" + strings.ToLower(m[2:]) })
	return legacyCode.ReplaceAllStringFunc(s, func(m string) string { return "§" + strings.ToLower(m[1:]) })
}

// reasonText is a kick or ban reason as typed: \n and | break lines and &
// codes colour it. Essentials: the commands' reason handling
func reasonText(s string) string {
	return replaceFormat(strings.NewReplacer(`\n`, "\n", "|", "\n").Replace(s))
}

// kickText answers a reason holding § codes as a disconnect message.
func kickText(e *core.Essentials, reason string) plugin.Text {
	return e.Mini(core.EscapeTags(reason))
}

// timePattern is DateUtil's, unanchored: removeTimePattern drops its first
// match from a tempban's time-and-reason.
var timePattern = regexp.MustCompile(`(?i)(?:([0-9]+)\s*y[a-z]*[,\s]*)?(?:([0-9]+)\s*mo[a-z]*[,\s]*)?(?:([0-9]+)\s*w[a-z]*[,\s]*)?(?:([0-9]+)\s*d[a-z]*[,\s]*)?(?:([0-9]+)\s*h[a-z]*[,\s]*)?(?:([0-9]+)\s*m[a-z]*[,\s]*)?(?:([0-9]+)\s*(?:s[a-z]*)?)?`)

// removeTimePattern answers s without its leading duration.
// Essentials: DateUtil.removeTimePattern
func removeTimePattern(s string) string {
	loc := timePattern.FindStringIndex(s)
	if loc == nil {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(s[:loc[0]] + s[loc[1]:])
}

var ipPattern = regexp.MustCompile(`^([01]?\d\d?|2[0-4]\d|25[0-5])\.([01]?\d\d?|2[0-4]\d|25[0-5])\.([01]?\d\d?|2[0-4]\d|25[0-5])\.([01]?\d\d?|2[0-4]\d|25[0-5])$`)

// validIP reports whether s is an IPv4 address. Essentials: FormatUtil.validIP
func validIP(s string) bool { return ipPattern.MatchString(s) }

// hostOf answers a player's IP address.
func hostOf(p plugin.Player) string {
	a := p.Address()
	if a == nil {
		return ""
	}
	if tcp, ok := a.(*net.TCPAddr); ok {
		return tcp.IP.String()
	}
	h, _, err := net.SplitHostPort(a.String())
	if err != nil {
		return a.String()
	}
	return h
}

// ipOf answers the address args[0] names: an IP as typed, else the last
// login address of the player it names, else the word itself.
func ipOf(ctx *core.Ctx) (string, error) {
	arg := ctx.Arg(0)
	if validIP(arg) {
		return arg, nil
	}
	ip := arg
	if u, err := ctx.Ess.MatchUser(nil, arg, true, true); err == nil {
		ip = u.Data.IPAddress
		if p, ok := u.Player(); ok {
			ip = hostOf(p)
		}
	}
	if ip == "" {
		return "", core.ErrPlayerNotFound
	}
	return ip, nil
}

// loopOnlinePlayers runs fn for each online user term names: @s, * and **
// for everyone, else every player whose name contains it, else whose
// display name does. Essentials: EssentialsLoopCommand.loopOnlinePlayers
// (with multiple matches and wildcards)
func loopOnlinePlayers(ctx *core.Ctx, term string, fn func(*core.User)) error {
	if term == "" {
		return core.ErrPlayerNotFound
	}
	if ctx.User != nil && (term == "@s" || term == "@p") {
		fn(ctx.User)
		return nil
	}
	skipHidden := ctx.User != nil && !ctx.User.HasPermission("essentials.vanish.interact")
	hidden := func(u *core.User) bool { return skipHidden && !ctx.User.CanSee(u) }
	if term == "*" || term == "**" {
		for _, u := range ctx.Ess.OnlineUsers() {
			if !hidden(u) {
				fn(u)
			}
		}
		return nil
	}
	if len(strings.TrimSpace(term)) < 2 {
		return core.ErrPlayerNotFound
	}
	low := strings.ToLower(term)
	var matched []*core.User
	if p, ok := ctx.Ess.Server.PlayerNamed(term); ok {
		matched = []*core.User{ctx.Ess.Users.Of(p)}
	} else {
		for _, u := range ctx.Ess.OnlineUsers() {
			if strings.Contains(strings.ToLower(u.Name()), low) {
				matched = append(matched, u)
			}
		}
	}
	if len(matched) == 0 {
		for _, u := range ctx.Ess.OnlineUsers() {
			if strings.Contains(strings.ToLower(core.StripLegacy(u.DisplayName())), low) {
				matched = append(matched, u)
			}
		}
	}
	found := false
	for _, u := range matched {
		if hidden(u) {
			continue
		}
		found = true
		fn(u)
	}
	if !found {
		return core.ErrPlayerNotFound
	}
	return nil
}

// loopPlayerNames completes player names and the wildcards.
// Essentials: EssentialsLoopCommand.getPlayers
func loopPlayerNames(ctx *core.Ctx) []string {
	return append(ctx.CompletePlayers(), "**", "*")
}
