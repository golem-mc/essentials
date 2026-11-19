package chat

import (
	"strings"

	"github.com/golem-mc/essentials/core"
)

// shoutKey is the session key of a user's shout mode.
const shoutKey = "chat.toggleshout"

// isToggleShout reports whether the user is in shout mode: the saved state
// with chat.persist-shout, else this session's, starting at
// chat.shout-default. Essentials: User.isToggleShout
func isToggleShout(u *core.User) bool {
	c := u.Ess().Config
	if c.Bool("chat.persist-shout", false) {
		return u.Data.Shouting != nil && *u.Data.Shouting
	}
	if v, ok := core.SessionValue[bool](u, shoutKey); ok {
		return v
	}
	return c.Bool("chat.shout-default", false)
}

// setToggleShout sets shout mode, saving it with chat.persist-shout.
// Essentials: User.setToggleShout
func setToggleShout(u *core.User, on bool) {
	if u.Session == nil {
		u.Session = map[string]any{}
	}
	u.Session[shoutKey] = on
	if u.Ess().Config.Bool("chat.persist-shout", false) {
		u.Data.Shouting = &on
		u.Save()
	}
}

// matchToggle reads on/enable/1 or off/disable/0; nil for anything else.
// Essentials: EssentialsToggleCommand.matchToggleArgument
func matchToggle(arg string) *bool {
	t, f := true, false
	switch {
	case strings.EqualFold(arg, "on") || strings.HasPrefix(arg, "ena") || arg == "1":
		return &t
	case strings.EqualFold(arg, "off") || strings.HasPrefix(arg, "dis") || arg == "0":
		return &f
	}
	return nil
}

// Essentials: Commandtoggleshout
func (m *Chat) toggleshoutCmd() core.Cmd {
	const others = "essentials.toggleshout.others"
	toggle := func(ctx *core.Ctx, u *core.User, on *bool) {
		v := !isToggleShout(u)
		if on != nil {
			v = *on
		}
		setToggleShout(u, v)
		if v {
			u.Send("shoutEnabled")
		} else {
			u.Send("shoutDisabled")
		}
		if ctx.User == nil || ctx.User.ID != u.ID {
			if v {
				ctx.Send("shoutEnabledFor", u.DisplayName())
			} else {
				ctx.Send("shoutDisabledFor", u.DisplayName())
			}
		}
	}
	// Essentials: EssentialsToggleCommand.toggleOtherPlayers
	toggleOthers := func(ctx *core.Ctx) error {
		if len(ctx.Args) < 1 || len(strings.TrimSpace(ctx.Args[0])) < 2 {
			return core.ErrPlayerNotFound
		}
		skipHidden := ctx.User != nil && !ctx.User.HasPermission("essentials.vanish.interact")
		var on *bool
		if len(ctx.Args) > 1 {
			on = matchToggle(ctx.Args[1])
		}
		found := false
		for _, u := range matchPlayers(ctx.Ess, ctx.Args[0]) {
			if skipHidden && !ctx.User.CanSee(u) {
				continue
			}
			found = true
			toggle(ctx, u, on)
		}
		if !found {
			return core.ErrPlayerNotFound
		}
		return nil
	}
	return core.Cmd{
		Name: "toggleshout",
		Run: func(ctx *core.Ctx) error {
			if ctx.User == nil {
				return toggleOthers(ctx)
			}
			// Essentials: EssentialsToggleCommand.handleToggleWithArgs
			switch {
			case len(ctx.Args) == 1:
				on := matchToggle(ctx.Args[0])
				if on == nil && ctx.Has(others) {
					return toggleOthers(ctx)
				}
				toggle(ctx, ctx.User, on)
			case len(ctx.Args) == 2 && ctx.Has(others):
				return toggleOthers(ctx)
			default:
				toggle(ctx, ctx.User, nil)
			}
			return nil
		},
		Complete: func(ctx *core.Ctx) []string {
			canOthers := ctx.Has(others)
			switch {
			case len(ctx.Args) == 1 && canOthers:
				return ctx.CompletePlayers()
			case len(ctx.Args) == 1 || len(ctx.Args) == 2 && canOthers:
				return []string{"enable", "disable"}
			}
			return []string{}
		},
	}
}

// matchPlayers answers the online player named exactly name, else those
// whose names start with it. Bukkit: Server.matchPlayer
func matchPlayers(e *core.Essentials, name string) []*core.User {
	if p, ok := e.Server.PlayerNamed(name); ok {
		return []*core.User{e.Users.Of(p)}
	}
	var out []*core.User
	low := strings.ToLower(name)
	for _, u := range e.OnlineUsers() {
		if strings.HasPrefix(strings.ToLower(u.Name()), low) {
			out = append(out, u)
		}
	}
	return out
}
