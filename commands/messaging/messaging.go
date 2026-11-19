// Package messaging holds EssentialsX's messaging and player information
// commands: private messages and replies, mail, ignores, social spy, /me,
// /helpop, broadcasts, mutes, AFK, nicknames, /list, /near, /seen, /whois,
// /playtime, and the text file commands (/motd, /rules, /info, /customtext,
// /help).
package messaging

import (
	"strings"
	"time"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

// group is the package's state for one Essentials instance.
type group struct {
	e *core.Essentials
	// console is the console's side of private messages.
	console replyState
	// mailCount and mailSince are the mails sent this minute, server wide.
	// Essentials: Commandmail.mailsPerMinute
	mailCount int
	mailSince time.Time
}

// Register registers the group's commands and listeners.
func Register(e *core.Essentials) {
	g := &group{e: e}
	g.registerMsg()
	g.registerMail()
	g.registerChat()
	g.registerPlayers()
	g.registerText()
}

// locale answers the locale messages to s are written in.
func (g *group) locale(s plugin.CommandSender) string {
	if p, ok := s.(plugin.Player); ok && g.e.Settings.PerPlayerLocale() {
		return p.Locale()
	}
	return ""
}

// tl answers the message key for s as MiniMessage, for a parsed argument.
// Essentials: CommandSource.tl
func (g *group) tl(s plugin.CommandSender, key string, args ...any) core.Raw {
	return core.Raw(g.e.I18n.Format(g.locale(s), key, args...))
}

// trueFalse answers true or false in s's language. Essentials:
// CommonPlaceholders.trueFalse
func (g *group) trueFalse(s plugin.CommandSender, b bool) core.Raw {
	if b {
		return g.tl(s, "true")
	}
	return g.tl(s, "false")
}

// enableDisable answers enabled or disabled in s's language. Essentials:
// CommonPlaceholders.enableDisable
func (g *group) enableDisable(s plugin.CommandSender, b bool) core.Raw {
	if b {
		return g.tl(s, "enabled")
	}
	return g.tl(s, "disabled")
}

// sender answers the user's command sender, or nil offline.
func sender(u *core.User) plugin.CommandSender {
	if p, ok := u.Player(); ok {
		return p
	}
	return nil
}

// isHidden reports whether u is hidden from everyone: vanished or hidden.
// Essentials: User.isHidden
func isHidden(u *core.User) bool { return u.Hidden || u.Vanished }

// hiddenFrom reports whether viewer cannot see u. Essentials: isHiddenFrom
func hiddenFrom(u *core.User, viewer plugin.Player) bool {
	p, ok := u.Player()
	if !ok || viewer == nil {
		return true
	}
	return !viewer.CanSee(p)
}

// canInteractVanished reports whether u may act on vanished players.
func canInteractVanished(u *core.User) bool { return u.HasPermission("essentials.vanish.interact") }

// skipHidden reports whether the sender of ctx should not see u, as the
// loops over online players decide.
func skipHidden(ctx *core.Ctx, u *core.User) bool {
	if ctx.User == nil || canInteractVanished(ctx.User) {
		return false
	}
	return hiddenFrom(u, ctx.Player)
}

// getPlayer finds the user term names for the sender: hidden players only
// for a sender who may interact with them. Essentials: getPlayer(server,
// sender, args, pos, getOffline)
func (g *group) getPlayer(ctx *core.Ctx, term string, offline bool) (*core.User, error) {
	if term == "" {
		return nil, core.ErrPlayerNotFound
	}
	if ctx.User != nil {
		return g.e.MatchUser(ctx.User, term, canInteractVanished(ctx.User), offline)
	}
	return g.e.MatchUser(nil, term, true, offline)
}

// matchPlayers answers the online players whose name is term, or else
// starts with it. Bukkit: Server.matchPlayer
func (g *group) matchPlayers(term string) []*core.User {
	if p, ok := g.e.Server.PlayerNamed(term); ok {
		return []*core.User{g.e.Users.Of(p)}
	}
	low := strings.ToLower(term)
	var out []*core.User
	for _, u := range g.e.OnlineUsers() {
		if strings.HasPrefix(strings.ToLower(u.Name()), low) {
			out = append(out, u)
		}
	}
	return out
}

// loopOnline runs fn for the online players term names: @s, the wildcards
// * and ** when allowed, every partial match with multiple, else one
// player. Essentials: EssentialsLoopCommand.loopOnlinePlayersConsumer
func (g *group) loopOnline(ctx *core.Ctx, multiple, wildcards bool, term string, fn func(*core.User) error) error {
	if term == "" {
		return core.ErrPlayerNotFound
	}
	if ctx.User != nil && (term == "@s" || term == "@p") {
		return fn(ctx.User)
	}
	if wildcards && (term == "*" || term == "**") {
		for _, u := range g.e.OnlineUsers() {
			if skipHidden(ctx, u) {
				continue
			}
			if err := fn(u); err != nil {
				return err
			}
		}
		return nil
	}
	if multiple {
		if len(strings.TrimSpace(term)) < 2 {
			return core.ErrPlayerNotFound
		}
		found := false
		matched := g.matchPlayers(term)
		if len(matched) == 0 {
			low := strings.ToLower(term)
			for _, u := range g.e.OnlineUsers() {
				if !skipHidden(ctx, u) && strings.Contains(strings.ToLower(stripFormat(u.DisplayName())), low) {
					found = true
					if err := fn(u); err != nil {
						return err
					}
				}
			}
		}
		for _, u := range matched {
			if skipHidden(ctx, u) {
				continue
			}
			found = true
			if err := fn(u); err != nil {
				return err
			}
		}
		if !found {
			return core.ErrPlayerNotFound
		}
		return nil
	}
	u, err := g.getPlayer(ctx, term, false)
	if err != nil {
		return err
	}
	return fn(u)
}

// loopOffline is loopOnline over saved users as well: a UUID, ** for every
// user, * for the online ones, else one player, online or not.
// Essentials: EssentialsLoopCommand.loopOfflinePlayersConsumer
func (g *group) loopOffline(ctx *core.Ctx, wildcards bool, term string, fn func(*core.User) error) error {
	if term == "" {
		return core.ErrPlayerNotFound
	}
	if ctx.User != nil && (term == "@s" || term == "@p") {
		return fn(ctx.User)
	}
	if id, err := parseUUID(term); err == nil {
		if !g.e.Users.Known(id) {
			return core.ErrPlayerNotFound
		}
		return fn(g.e.Users.Get(id))
	}
	switch {
	case wildcards && term == "**":
		for _, id := range g.e.Users.All() {
			if err := fn(g.e.Users.Get(id)); err != nil {
				return err
			}
		}
		return nil
	case wildcards && term == "*":
		for _, u := range g.e.OnlineUsers() {
			if skipHidden(ctx, u) {
				continue
			}
			if err := fn(u); err != nil {
				return err
			}
		}
		return nil
	}
	u, err := g.e.MatchUser(nil, term, true, true)
	if err != nil {
		return err
	}
	return fn(u)
}

// loopCompletions are the wildcards loop commands complete besides names.
var loopCompletions = []string{"**", "*"}

// toggle is an EssentialsToggleCommand: on, off or flip a setting for the
// sender or, with the others permission, for matched players.
type toggle struct {
	g      *group
	others string
	set    func(ctx *core.Ctx, u *core.User, enabled *bool)
}

// matchToggle reads on/enable/1 or off/disable/0; nil for anything else.
// Essentials: matchToggleArgument
func matchToggle(arg string) *bool {
	switch {
	case strings.EqualFold(arg, "on") || strings.HasPrefix(arg, "ena") || arg == "1":
		return new(true)
	case strings.EqualFold(arg, "off") || strings.HasPrefix(arg, "dis") || arg == "0":
		return new(false)
	}
	return nil
}

func (t toggle) run(ctx *core.Ctx) error {
	if ctx.User == nil {
		return t.toggleOthers(ctx)
	}
	switch {
	case len(ctx.Args) == 1:
		on := matchToggle(ctx.Args[0])
		if on == nil && ctx.Has(t.others) {
			return t.toggleOthers(ctx)
		}
		t.set(ctx, ctx.User, on)
	case len(ctx.Args) == 2 && ctx.Has(t.others):
		return t.toggleOthers(ctx)
	default:
		t.set(ctx, ctx.User, nil)
	}
	return nil
}

// toggleOthers toggles every player args[0] matches. Essentials:
// toggleOtherPlayers
func (t toggle) toggleOthers(ctx *core.Ctx) error {
	if len(ctx.Args) < 1 || len(strings.TrimSpace(ctx.Args[0])) < 2 {
		return core.ErrPlayerNotFound
	}
	found := false
	for _, u := range t.g.matchPlayers(ctx.Args[0]) {
		if skipHidden(ctx, u) {
			continue
		}
		found = true
		var on *bool
		if len(ctx.Args) > 1 {
			on = matchToggle(ctx.Args[1])
		}
		t.set(ctx, u, on)
	}
	if !found {
		return core.ErrPlayerNotFound
	}
	return nil
}

func (t toggle) complete(ctx *core.Ctx) []string {
	n := len(ctx.Args)
	if ctx.User != nil && !ctx.Has(t.others) {
		if n == 1 {
			return []string{"enable", "disable"}
		}
		return nil
	}
	switch n {
	case 1:
		return ctx.CompletePlayers()
	case 2:
		return []string{"enable", "disable"}
	}
	return nil
}

// isSelf reports whether the sender of ctx is u.
func isSelf(ctx *core.Ctx, u *core.User) bool { return ctx.User != nil && ctx.User.ID == u.ID }

// muteError answers the error a muted user gets for talking, as /me, /mail
// and /afk raise it.
func muteError(u *core.User) error {
	reason := u.Data.MuteReason
	if u.Data.Timestamps.Mute <= 0 {
		if reason != "" {
			return core.Tlf("voiceSilencedReason", reason)
		}
		return core.Tlf("voiceSilenced")
	}
	diff := core.FormatDateDiff(time.UnixMilli(u.Data.Timestamps.Mute))
	if reason != "" {
		return core.Tlf("voiceSilencedReasonTime", diff, reason)
	}
	return core.Tlf("voiceSilencedTime", diff)
}

// broadcastIn sends the message key to the online users in world, with
// the keywords of each recipient filled in to the arguments. Essentials:
// broadcastTl with a world filter and parseKeywords, as /broadcastworld
// calls it; core's Broadcast covers the other broadcasts.
func (g *group) broadcastIn(world plugin.World, key string, args ...any) {
	for _, u := range g.e.OnlineUsers() {
		if p, ok := u.Player(); !ok || p.World() != world {
			continue
		}
		a := make([]any, len(args))
		for i, v := range args {
			switch s := v.(type) {
			case core.Raw:
				a[i] = core.Raw(g.e.ReplaceKeywords(string(s), u))
			case string:
				a[i] = g.e.ReplaceKeywords(s, u)
			default:
				a[i] = v
			}
		}
		u.Send(key, a...)
	}
}
