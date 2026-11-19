package player

import (
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

// toggleCmd is a command that switches a state on, off or over, for the
// sender or, with the others permission, for matched players.
// Essentials: EssentialsToggleCommand
type toggleCmd struct {
	name, others string
	// toggle sets the state; nil switches it over.
	toggle func(ctx *core.Ctx, u *core.User, on *bool)
}

func (t toggleCmd) cmd() core.Cmd {
	return core.Cmd{
		Name: t.name,
		Run: func(ctx *core.Ctx) error {
			if ctx.User == nil {
				return t.toggleOthers(ctx)
			}
			switch {
			case len(ctx.Args) == 1:
				on := matchToggle(ctx.Args[0])
				if on == nil && ctx.Has(t.others) {
					return t.toggleOthers(ctx)
				}
				t.toggle(ctx, ctx.User, on)
			case len(ctx.Args) == 2 && ctx.Has(t.others):
				return t.toggleOthers(ctx)
			default:
				t.toggle(ctx, ctx.User, nil)
			}
			return nil
		},
		Complete: func(ctx *core.Ctx) []string {
			switch {
			case len(ctx.Args) == 1 && (ctx.User == nil || ctx.Has(t.others)):
				return ctx.CompletePlayers()
			case len(ctx.Args) == 1 || len(ctx.Args) == 2 && (ctx.User == nil || ctx.Has(t.others)):
				return []string{"enable", "disable"}
			}
			return nil
		},
	}
}

// toggleOthers toggles every player the first argument matches.
// Essentials: EssentialsToggleCommand.toggleOtherPlayers
func (t toggleCmd) toggleOthers(ctx *core.Ctx) error {
	if len(ctx.Args) < 1 || len(strings.TrimSpace(ctx.Args[0])) < 2 {
		return core.ErrPlayerNotFound
	}
	found := false
	for _, p := range matchPlayers(ctx.Ess, ctx.Args[0]) {
		u := ctx.Ess.Users.Of(p)
		if hiddenFrom(ctx, u) {
			continue
		}
		found = true
		var on *bool
		if len(ctx.Args) > 1 {
			on = matchToggle(ctx.Args[1])
		}
		t.toggle(ctx, u, on)
	}
	if !found {
		return core.ErrPlayerNotFound
	}
	return nil
}

// isSender reports whether u sent the command.
func isSender(ctx *core.Ctx, u *core.User) bool { return ctx.User != nil && ctx.User.ID == u.ID }

func registerToggles(e *core.Essentials) {
	e.RegisterCommand(
		// Essentials: Commandfly
		toggleCmd{name: "fly", others: "essentials.fly.others", toggle: func(ctx *core.Ctx, u *core.User, on *bool) {
			p, ok := u.Player()
			if !ok {
				return
			}
			enabled := !p.AllowFlight()
			if on != nil {
				enabled = *on
			}
			p.SetFallDistance(0)
			p.SetAllowFlight(enabled)
			u.Data.FlyMode = enabled
			u.Save()
			if !p.AllowFlight() {
				p.SetFlying(false)
			}
			u.Send("flyMode", enableDisable(ctx.Ess, p, enabled), u.DisplayName())
			if !isSender(ctx, u) {
				ctx.Send("flyMode", enableDisable(ctx.Ess, p, enabled), u.DisplayName())
			}
		}}.cmd(),
		// Essentials: Commandgod
		toggleCmd{name: "god", others: "essentials.god.others", toggle: func(ctx *core.Ctx, u *core.User, on *bool) {
			p, ok := u.Player()
			if !ok {
				return
			}
			enabled := !u.Data.GodMode
			if on != nil {
				enabled = *on
			}
			u.Data.GodMode = enabled
			u.Save()
			if enabled && p.Health() != 0 {
				p.SetHealth(p.MaxHealth())
				p.SetFood(20)
			}
			u.Send("godMode", enableDisable(ctx.Ess, p, enabled))
			if !isSender(ctx, u) {
				ctx.Send("godMode", enableDisable(ctx.Ess, p, enabled))
			}
		}}.cmd(),
		// Essentials: Commandvanish
		toggleCmd{name: "vanish", others: "essentials.vanish.others", toggle: func(ctx *core.Ctx, u *core.User, on *bool) {
			p, ok := u.Player()
			if !ok {
				return
			}
			enabled := !u.Vanished
			if on != nil {
				enabled = *on
			}
			u.SetVanished(enabled)
			u.Send("vanish", u.DisplayName(), enableDisable(ctx.Ess, p, enabled))
			if enabled {
				u.Send("vanished")
			}
			if !isSender(ctx, u) {
				ctx.Send("vanish", u.DisplayName(), enableDisable(ctx.Ess, p, enabled))
			}
		}}.cmd(),
	)

	// Keeps a flying player flying across a game mode change.
	// Essentials: EssentialsPlayerListener.onPlayerGameModeChange
	plugin.Listen(e.Server, func(ev *plugin.PlayerGameModeChangeEvent) {
		if !e.Config.Bool("gamemode-change-preserve-flying", false) {
			return
		}
		p := ev.Player
		if !p.HasPermission("essentials.fly") || !p.Flying() || !p.AllowFlight() {
			return
		}
		e.Server.Scheduler().RunLater(1, func() {
			if p.IsValid() {
				p.SetAllowFlight(true)
				p.SetFlying(true)
			}
		})
	}, plugin.ListenOptions{Priority: plugin.Monitor, IgnoreCancelled: true})
}
