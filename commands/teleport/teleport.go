// Package teleport holds EssentialsX's teleport commands: /back, homes,
// warps, /tp and its variants, teleport requests, random teleports and the
// position commands.
package teleport

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

// group is the package's state, shared by its commands.
type group struct {
	e   *core.Essentials
	rtp *randomTeleport
}

// Register registers the group's commands and listeners.
func Register(e *core.Essentials) {
	g := &group{e: e}
	g.rtp = newRandomTeleport(e)
	e.OnReload(g.rtp.reload)
	plugin.Listen(e.Server, g.onInteract)
	g.registerBack()
	g.registerHomes()
	g.registerWarps()
	g.registerTp()
	g.registerTpa()
	g.registerRandom()
	g.registerPosition()
}

// teleport teleports teleportee to to with owner's warmup, cooldown and
// charge (nil for none); then (may be nil) runs once the teleport happened,
// as Essentials chains it on the teleport's future.
// Essentials: AsyncTeleport.teleport, teleportPlayer
func (g *group) teleport(owner, teleportee *core.User, to core.Target, charge *core.Trade, then func()) error {
	var thens []func()
	if then != nil {
		thens = append(thens, then)
	}
	if owner.ID == teleportee.ID {
		return owner.Teleport(to, charge, plugin.TeleportCommand, thens...)
	}
	return owner.TeleportOther(teleportee, to, charge, plugin.TeleportCommand, thens...)
}

// now teleports u at once, after u's cooldown when cooldown is set; then
// (may be nil) runs if the teleport happened.
// Essentials: AsyncTeleport.now(Location)
func (g *group) now(u *core.User, to core.Target, cooldown bool, then func()) error {
	if cooldown {
		if err := g.cooldown(u); err != nil {
			return err
		}
	}
	if err := u.TeleportNow(to, plugin.TeleportCommand); err != nil {
		return err
	}
	if then != nil {
		then()
	}
	return nil
}

// nowTo teleports u to other at once, telling u teleporting with where
// other is then; then (may be nil) runs after. Essentials: AsyncTeleport.now(Player)
func (g *group) nowTo(u, other *core.User, then func()) error {
	to := core.PlayerTarget{User: other}
	return g.now(u, to, false, func() {
		loc, _ := to.Location()
		u.Send("teleporting", locArgs(loc)...)
		if then != nil {
			then()
		}
	})
}

// cooldown answers timeBeforeTeleport while u's teleport-cooldown runs, else
// starts it. Essentials: AsyncTeleport.cooldown
func (g *group) cooldown(u *core.User) error {
	cd := g.e.Settings.TeleportCooldown()
	if cd <= 0 || u.HasPermission("essentials.teleport.cooldown.bypass") {
		return nil
	}
	now := time.Now()
	last := time.UnixMilli(u.Data.Timestamps.LastTeleport)
	if last.After(now) {
		u.Data.Timestamps.LastTeleport = now.UnixMilli()
		return nil
	}
	if next := last.Add(cd); next.After(now) {
		return core.Tlf("timeBeforeTeleport", core.FormatDateDiff(next))
	}
	u.Data.Timestamps.LastTeleport = now.UnixMilli()
	u.Save()
	return nil
}

// showError shows err to sender as a command's error.
func (g *group) showError(sender plugin.CommandSender, err error) {
	g.e.ShowError(&core.Ctx{Ess: g.e, Sender: sender}, err)
}

// showErrorTo shows err to the online user u.
func (g *group) showErrorTo(u *core.User, err error) {
	if p, ok := u.Player(); ok {
		g.showError(p, err)
	}
}

// locArgs are a location's message arguments: world, block x, y, z.
func locArgs(l plugin.Location) []any {
	b := l.Pos.Block()
	return []any{core.WorldName(l.World), b.X, b.Y, b.Z}
}

// location answers the online user's location; zero when offline.
func location(u *core.User) plugin.Location {
	if p, ok := u.Player(); ok {
		return p.Location()
	}
	return plugin.Location{}
}

// world answers the online user's world; nil when offline.
func world(u *core.User) plugin.World { return location(u).World }

func sameWorld(a, b plugin.World) bool {
	return a != nil && b != nil && a.Key() == b.Key()
}

// checkWorld answers noPerm for essentials.worlds.<w> when the worlds a and
// b differ, world-teleport-permissions is on and u lacks it.
func (g *group) checkWorld(u *core.User, a, b, w plugin.World) error {
	if sameWorld(a, b) || !g.e.Settings.WorldTeleportPermissions() {
		return nil
	}
	node := "essentials.worlds." + core.WorldName(w)
	if !u.HasPermission(node) {
		return core.Tlf("noPerm", node)
	}
	return nil
}

// getWorld finds a world by index in the world list or by name.
// Essentials: Essentials.getWorld
func (g *group) getWorld(name string) (plugin.World, bool) {
	if n, err := strconv.Atoi(name); err == nil && n >= 0 && strings.Trim(name, "0123456789") == "" {
		if ws := g.e.Server.Worlds(); n < len(ws) {
			return ws[n], true
		}
	}
	return core.WorldByName(g.e.Server, name)
}

// parseDouble reads a number as Java's Double.parseDouble does, answering
// its NumberFormatException message.
func parseDouble(s string) (float64, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("For input string: \"%s\"", s)
	}
	return f, nil
}

// parseFloat reads a finite float. Essentials: FloatUtil.parseFloat
func parseFloat(s string) (float32, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 32)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("For input string: \"%s\"", s)
	}
	return float32(f), nil
}

// coord reads a coordinate: a number, or ~ and an offset from base.
func coord(arg string, base float64) (float64, error) {
	if !strings.HasPrefix(arg, "~") {
		return parseDouble(arg)
	}
	if len(arg) == 1 {
		return base, nil
	}
	d, err := parseDouble(arg[1:])
	return base + d, err
}

// coords reads three coordinates relative to base.
func coords(args []string, base plugin.Vec3) (plugin.Vec3, error) {
	x, err := coord(args[0], base.X)
	if err != nil {
		return plugin.Vec3{}, err
	}
	y, err := coord(args[1], base.Y)
	if err != nil {
		return plugin.Vec3{}, err
	}
	z, err := coord(args[2], base.Z)
	if err != nil {
		return plugin.Vec3{}, err
	}
	return plugin.Vec3{X: x, Y: y, Z: z}, nil
}

// outOfBounds reports whether a coordinate is beyond 30000000.
func outOfBounds(v plugin.Vec3) bool {
	const m = 30000000
	return v.X > m || v.Y > m || v.Z > m || v.X < -m || v.Y < -m || v.Z < -m
}

// invalidLocation shows the usage and teleportInvalidLocation, as
// Essentials' NotEnoughArgumentsException with a message does.
func invalidLocation(ctx *core.Ctx) error {
	ctx.Ess.ShowUsage(ctx)
	ctx.Send("teleportInvalidLocation")
	return core.ErrQuiet
}

// javaSplit splits s on sep as Java's String.split does: trailing empty
// strings are dropped.
func javaSplit(s, sep string) []string {
	parts := strings.Split(s, sep)
	for len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// safeString answers s lower-cased with every character but a-z and 0-9
// replaced by _. Essentials: StringUtil.safeString
func safeString(s string) string {
	b := []byte(strings.ToLower(s))
	for i, c := range b {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			b[i] = '_'
		}
	}
	return string(b)
}

// joinList joins with ", ". Essentials: StringUtil.joinList
func joinList(s []string) string { return strings.Join(s, ", ") }

// rawTl answers the message key as MiniMessage in u's locale, for a list
// built of messages. Essentials: playerTl
func (g *group) rawTl(u *core.User, key string) string {
	loc := ""
	if p, ok := u.Player(); ok && g.e.Settings.PerPlayerLocale() {
		loc = p.Locale()
	}
	return g.e.I18n.Format(loc, key)
}

// matchToggle reads on/off words. Essentials: EssentialsToggleCommand.matchToggleArgument
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

// toggleCmd is an EssentialsToggleCommand: toggle flips u's setting (nil
// enabled flips it) and tells the sender.
type toggleCmd struct {
	others string
	toggle func(ctx *core.Ctx, u *core.User, enabled *bool)
}

// run is handleToggleWithArgs for a player and toggleOtherPlayers for the
// console.
func (t toggleCmd) run(ctx *core.Ctx) error {
	if ctx.User == nil {
		return t.toggleOthers(ctx)
	}
	switch {
	case len(ctx.Args) == 1:
		v := matchToggle(ctx.Args[0])
		if v == nil && ctx.Has(t.others) {
			return t.toggleOthers(ctx)
		}
		t.toggle(ctx, ctx.User, v)
	case len(ctx.Args) == 2 && ctx.Has(t.others):
		return t.toggleOthers(ctx)
	default:
		t.toggle(ctx, ctx.User, nil)
	}
	return nil
}

// toggleOthers toggles every online player whose name starts with args[0]
// (or is it). Essentials: toggleOtherPlayers
func (t toggleCmd) toggleOthers(ctx *core.Ctx) error {
	if len(ctx.Args) < 1 || len(strings.TrimSpace(ctx.Args[0])) < 2 {
		return core.ErrPlayerNotFound
	}
	var v *bool
	if len(ctx.Args) > 1 {
		v = matchToggle(ctx.Args[1])
	}
	found := false
	for _, u := range matchPlayers(ctx.Ess, ctx.Args[0]) {
		if ctx.User != nil && !ctx.User.CanSee(u) {
			continue
		}
		found = true
		t.toggle(ctx, u, v)
	}
	if !found {
		return core.ErrPlayerNotFound
	}
	return nil
}

func (t toggleCmd) complete(ctx *core.Ctx) []string {
	words := []string{"enable", "disable"}
	others := ctx.Has(t.others)
	switch {
	case len(ctx.Args) == 1 && others:
		return ctx.CompletePlayers()
	case len(ctx.Args) == 1:
		return words
	case len(ctx.Args) == 2 && others:
		return words
	}
	return nil
}

// matchPlayers answers the online player named name, else those whose names
// start with it. Bukkit: Server.matchPlayer
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

// isSender reports whether u is the player who sent the command.
func isSender(ctx *core.Ctx, u *core.User) bool { return ctx.User != nil && ctx.User.ID == u.ID }
