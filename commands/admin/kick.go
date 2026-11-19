package admin

import (
	"strings"

	"github.com/golem-mc/essentials/core"
)

func registerKicks(e *core.Essentials) {
	e.RegisterCommand(
		core.Cmd{Name: "kick", Run: runKick, Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 {
				return ctx.CompletePlayers()
			}
			return nil
		}},
		core.Cmd{Name: "kickall", Run: runKickAll},
	)
}

// kickReason answers the reason typed from args[from] on, or kickDefault.
func kickReason(ctx *core.Ctx, from int) string {
	r := ctx.Ess.TlString("kickDefault")
	if len(ctx.Args) > from {
		r = ctx.FinalArg(from)
	}
	return reasonText(r)
}

// Essentials: Commandkick
func runKick(ctx *core.Ctx) error {
	if len(ctx.Args) < 1 {
		return core.ErrNotEnoughArgs
	}
	e := ctx.Ess
	target, err := e.MatchUser(nil, ctx.Args[0], true, false)
	if err != nil {
		return err
	}
	if u := ctx.User; u != nil {
		if target.Vanished && !u.HasPermission("essentials.vanish.interact") && !u.CanSee(target) {
			return core.ErrPlayerNotFound
		}
		if target.HasPermission("essentials.kick.exempt") {
			return core.Tlf("kickExempt")
		}
	}
	reason := kickReason(ctx, 1)
	if p, ok := target.Player(); ok {
		p.Kick(kickText(e, reason))
	}
	e.Broadcast(nil, "essentials.kick.notify", "playerKicked", ctx.SenderName(), target.Name(), reason)
	return nil
}

// Essentials: Commandkickall
func runKickAll(ctx *core.Ctx) error {
	reason := kickText(ctx.Ess, kickReason(ctx, 0))
	for _, p := range ctx.Ess.Server.Players() {
		if ctx.Player != nil && strings.EqualFold(p.Name(), ctx.Player.Name()) {
			continue
		}
		if !p.HasPermission("essentials.kickall.exempt") {
			p.Kick(reason)
		}
	}
	ctx.Send("kickedAll")
	return nil
}
