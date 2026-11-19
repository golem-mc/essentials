// Package player holds EssentialsX's player commands: the toggles (fly, god,
// vanish), vitals (heal, feed, exp, speed, gamemode...), world time and
// weather, mob spawning and the fun commands.
package player

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// Register registers the group's commands and listeners.
func Register(e *core.Essentials) {
	registerToggles(e)
	registerVitals(e)
	registerWorld(e)
	registerFun(e)
	registerMobs(e)
}

// tl answers the message key in the sender's locale as plain text, for an
// argument Essentials passes as a string. Essentials: CommandSource.tl
func tl(ctx *core.Ctx, key string, args ...any) string {
	return ctx.Ess.TlFor(ctx.Sender, key, args...).PlainString()
}

// tlRaw answers the message key in the sender's locale as a parsed
// argument. Essentials: AdventureUtil.parsed(source.tl(key))
func tlRaw(e *core.Essentials, sender plugin.CommandSender, key string, args ...any) core.Raw {
	loc := ""
	if p, ok := sender.(plugin.Player); ok && e.Settings.PerPlayerLocale() {
		loc = p.Locale()
	}
	return core.Raw(e.I18n.Format(loc, key, args...))
}

// enableDisable answers "enabled" or "disabled" for the sender.
// Essentials: CommonPlaceholders.enableDisable
func enableDisable(e *core.Essentials, sender plugin.CommandSender, on bool) core.Raw {
	if on {
		return tlRaw(e, sender, "enabled")
	}
	return tlRaw(e, sender, "disabled")
}

// usageWith shows the command's usage followed by the message key, as
// Essentials does for a NotEnoughArgumentsException with a message.
func usageWith(ctx *core.Ctx, key string, args ...any) error {
	ctx.Ess.ShowUsage(ctx)
	ctx.Send(key, args...)
	return core.ErrQuiet
}

// parseInt reads an int as Java's Integer.parseInt, failing with its
// message.
func parseInt(s string) (int, error) {
	n, err := strconv.ParseInt(strings.TrimPrefix(s, "+"), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("For input string: \"%s\"", s)
	}
	return int(n), nil
}

// parseFloat reads a finite float as Essentials' FloatUtil.parseFloat.
func parseFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil && !math.IsNaN(f) && !math.IsInf(f, 0)
}

// matchToggle reads on/off words; nil for anything else.
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

// matchPlayers answers the online players whose name contains term, or only
// the one whose name is term. Bukkit: Server.matchPlayer
func matchPlayers(e *core.Essentials, term string) []plugin.Player {
	low := strings.ToLower(term)
	var out []plugin.Player
	for _, p := range e.Server.Players() {
		if strings.EqualFold(p.Name(), term) {
			return []plugin.Player{p}
		}
		if strings.Contains(strings.ToLower(p.Name()), low) {
			out = append(out, p)
		}
	}
	return out
}

// hiddenFrom reports whether the sender should skip u: the sender is a
// player who cannot see u and may not interact with vanished players.
// Essentials: skipHidden && isHidden(player) && isHiddenFrom(player)
func hiddenFrom(ctx *core.Ctx, u *core.User) bool {
	return ctx.User != nil && !ctx.User.HasPermission("essentials.vanish.interact") && !ctx.User.CanSee(u)
}

// loopOnline runs fn for each online user term names: @s, * or ** with
// wildcards, every name or display name containing it with multiple, else
// the one player it matches. fn's first error stops the loop.
// Essentials: EssentialsLoopCommand.loopOnlinePlayersConsumer
func loopOnline(ctx *core.Ctx, multiple, wildcards bool, term string, fn func(*core.User) error) error {
	if term == "" {
		return core.ErrPlayerNotFound
	}
	if ctx.User != nil && (term == "@s" || term == "@p") {
		return fn(ctx.User)
	}
	e := ctx.Ess
	switch {
	case wildcards && (term == "*" || term == "**"):
		for _, u := range e.OnlineUsers() {
			if hiddenFrom(ctx, u) {
				continue
			}
			if err := fn(u); err != nil {
				return err
			}
		}
		return nil
	case multiple:
		if len(strings.TrimSpace(term)) < 2 {
			return core.ErrPlayerNotFound
		}
		found := false
		matched := matchPlayers(e, term)
		if len(matched) == 0 {
			low := strings.ToLower(term)
			for _, u := range e.OnlineUsers() {
				if hiddenFrom(ctx, u) || !strings.Contains(strings.ToLower(core.StripLegacy(u.DisplayName())), low) {
					continue
				}
				found = true
				if err := fn(u); err != nil {
					return err
				}
			}
		} else {
			for _, p := range matched {
				u := e.Users.Of(p)
				if hiddenFrom(ctx, u) {
					continue
				}
				found = true
				if err := fn(u); err != nil {
					return err
				}
			}
		}
		if !found {
			return core.ErrPlayerNotFound
		}
		return nil
	}
	u, err := e.MatchUser(ctx.User, term, false, false)
	if err != nil {
		return err
	}
	return fn(u)
}

// loopPlayers completes player names and the wildcards.
// Essentials: EssentialsLoopCommand.getPlayers
func loopPlayers(ctx *core.Ctx) []string { return append(ctx.CompletePlayers(), "**", "*") }

// isAir reports whether s is air, cave air or void air.
func isAir(s plugin.BlockState) bool {
	b := s.Block()
	return b == airBlock || b == caveAirBlock || b == voidAirBlock
}

// transparent reports whether a sight line passes s: no collision and not
// lava. Essentials: LocationUtil.TRANSPARENT_MATERIALS
func transparent(s plugin.BlockState) bool {
	return s.Block() != lavaBlock && registry.CollisionHeight(uint16(s)) == 0
}

// solid reports whether s blocks movement. Bukkit: Material.isSolid
func solid(s plugin.BlockState) bool { return registry.CollisionHeight(uint16(s)) > 0 }

var (
	airBlock, _     = registry.BlockByName("minecraft:air")
	caveAirBlock, _ = registry.BlockByName("minecraft:cave_air")
	voidAirBlock, _ = registry.BlockByName("minecraft:void_air")
	lavaBlock, _    = registry.BlockByName("minecraft:lava")
	bedrockBlock, _ = registry.BlockByName("minecraft:bedrock")
	spawnerBlock, _ = registry.BlockByName("minecraft:spawner")
)

// eyeLocation answers where p's eyes are, facing where p looks.
// Bukkit: LivingEntity.getEyeLocation
func eyeLocation(p plugin.Player) plugin.Location {
	l := p.Location()
	l.Pos = p.EyePos()
	return l
}

// targetLocation answers the block p looks at within max blocks, seeing
// through transparent blocks. Essentials: LocationUtil.getTarget
func targetLocation(p plugin.Player, max int) plugin.Location {
	b := p.TargetBlockPassing(max, transparent)
	if b.World == nil { // no line of sight at all (eyes outside the world)
		b = plugin.Block{World: p.World(), Pos: p.Pos().Block()}
	}
	return plugin.Location{World: b.World, Pos: plugin.Vec3{X: float64(b.Pos.X), Y: float64(b.Pos.Y), Z: float64(b.Pos.Z)}}
}

// targetBlockExact answers the block whose shape p looks at within max
// blocks, passing fluids, else the first that is not air.
// Essentials: User.getTargetBlock
func targetBlockExact(p plugin.Player, max int) plugin.Block {
	if b, ok := p.TargetBlock(max); ok {
		return b
	}
	return p.TargetBlockPassing(max, nil)
}

// worldLabel names a world in messages. Bukkit: World.getName
func worldLabel(w plugin.World) string { return core.WorldName(w) }
