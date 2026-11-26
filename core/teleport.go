package core

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// Target is where a teleport goes: a fixed location or a player, read when
// the teleport happens (after the warmup). Essentials: ITarget
type Target interface {
	Location() (plugin.Location, bool)
}

// LocationTarget is a fixed location.
type LocationTarget plugin.Location

func (l LocationTarget) Location() (plugin.Location, bool) { return plugin.Location(l), l.World != nil }

// PlayerTarget is wherever the player is when the teleport happens.
type PlayerTarget struct{ User *User }

func (t PlayerTarget) Location() (plugin.Location, bool) {
	p, ok := t.User.Player()
	if !ok {
		return plugin.Location{}, false
	}
	return p.Location(), true
}

// pendingTeleport is a teleport waiting out its warmup.
type pendingTeleport struct {
	task       plugin.Task
	start      plugin.Vec3
	health     float32
	canMove    bool
	teleportee *User
}

// cooldown answers teleportCooldown-style errors while the user's teleport
// cooldown runs, unless they bypass it. Essentials: AsyncTeleport.cooldown
func (u *User) cooldown(check bool) error {
	cd := u.ess.Settings.TeleportCooldown()
	if cd <= 0 || u.HasPermission("essentials.teleport.cooldown.bypass") {
		return nil
	}
	last := time.UnixMilli(u.Data.Timestamps.LastTeleport)
	now := time.Now()
	if last.After(now) {
		u.Data.Timestamps.LastTeleport = now.UnixMilli()
		return nil
	}
	if next := last.Add(cd); next.After(now) && !u.HasPermission("essentials.teleport.cooldown.bypass") {
		return TlPlain("timeBeforeTeleport", FormatDateDiff(next))
	}
	if !check {
		u.Data.Timestamps.LastTeleport = now.UnixMilli()
		u.Save()
	}
	return nil
}

// Teleport teleports the user to target after the teleport-delay warmup,
// charging charge (may be nil) when it happens. then, if given, runs once
// the teleport has happened, which is after Teleport returns when there is a
// warmup (Essentials chains these on the teleport's future).
// Essentials: AsyncTeleport.teleport
func (u *User) Teleport(target Target, charge *Trade, cause plugin.TeleportCause, then ...func()) error {
	return u.teleportOther(u, target, charge, cause, then)
}

// TeleportOther teleports teleportee to target with the user's warmup and
// charge, as /tphere and /tpahere do; then as for Teleport.
// Essentials: AsyncTeleport.teleportPlayer
func (u *User) TeleportOther(teleportee *User, target Target, charge *Trade, cause plugin.TeleportCause, then ...func()) error {
	return u.teleportOther(teleportee, target, charge, cause, then)
}

func (u *User) teleportOther(teleportee *User, target Target, charge *Trade, cause plugin.TeleportCause, then []func()) error {
	done := func() {
		for _, f := range then {
			f()
		}
	}
	if err := charge.CheckAffordable(u); err != nil {
		return err
	}
	if err := u.cooldown(true); err != nil {
		return err
	}
	delay := u.ess.Settings.TeleportDelay()
	if delay <= 0 || u.HasPermission("essentials.teleport.timer.bypass") {
		if err := u.cooldown(false); err != nil {
			return err
		}
		if err := teleportee.TeleportNow(target, cause); err != nil {
			return err
		}
		if err := charge.Charge(u); err != nil {
			return err
		}
		done()
		return nil
	}
	u.cancelPending(false)
	p, ok := teleportee.Player()
	if !ok {
		return ErrPlayerNotFound
	}
	u.Send("dontMoveMessage", FormatDateDiff(time.Now().Add(delay)))
	pt := &pendingTeleport{start: p.Pos(), health: p.Health(), canMove: teleportee.HasPermission("essentials.teleport.timer.move"), teleportee: teleportee}
	deadline := time.Now().Add(delay)
	pt.task = u.ess.Server.Scheduler().RunTimer(10, 10, func() {
		p, ok := teleportee.Player()
		if !ok {
			u.cancelPending(false)
			return
		}
		moved := !pt.canMove && p.Pos().DistanceSq(pt.start) > 0.04
		if moved || p.Health() < pt.health {
			u.cancelPending(true)
			return
		}
		pt.health = p.Health()
		if time.Now().Before(deadline) {
			return
		}
		pt.task.Cancel()
		u.pending = nil
		if err := u.cooldown(false); err != nil {
			u.ess.ShowError(&Ctx{Ess: u.ess, Sender: p}, err)
			return
		}
		u.Send("teleportationCommencing")
		if err := teleportee.TeleportNow(target, cause); err != nil {
			u.ess.ShowError(&Ctx{Ess: u.ess, Sender: p}, err)
			return
		}
		if err := charge.Charge(u); err != nil {
			u.ess.ShowError(&Ctx{Ess: u.ess, Sender: p}, err)
			return
		}
		done()
	})
	u.pending = pt
	return nil
}

// cancelPending cancels a warmup; notify tells the user and teleportee.
// Essentials: AsyncTimedTeleport.cancelTimer
func (u *User) cancelPending(notify bool) {
	if u.pending == nil {
		return
	}
	u.pending.task.Cancel()
	if notify {
		u.Send("pendingTeleportCancelled")
		if t := u.pending.teleportee; t != nil && t.ID != u.ID {
			t.Send("pendingTeleportCancelled")
		}
	}
	u.pending = nil
}

// CancelTeleport cancels the user's pending warmup.
func (u *User) CancelTeleport(notify bool) { u.cancelPending(notify) }

// TeleportNow teleports the user at once, with no warmup, cooldown or charge:
// the safe destination when teleport-safety is on, and /back set to where
// they were. Essentials: AsyncTeleport.nowAsync
func (u *User) TeleportNow(target Target, cause plugin.TeleportCause) error {
	p, ok := u.Player()
	if !ok {
		return ErrPlayerNotFound
	}
	to, ok := target.Location()
	if !ok {
		return Tlf("destinationNotSet")
	}
	if err := u.ess.CheckWorldPermission(u, to.World); err != nil {
		return err
	}
	u.cancelPending(false)
	from := p.Location()
	u.ess.HoldChunks(to)
	dest, err := u.ess.SafeDestinationFor(u, to)
	if err != nil {
		return err
	}
	if _, ok := p.Vehicle(); ok {
		p.Dismount()
	}
	if !u.ess.Settings.RegisterBackInListener() {
		u.SetLastLocation(from)
	}
	// false when a listener cancelled the teleport, which says why itself.
	if !p.Teleport(dest, cause) {
		return ErrQuiet
	}
	if inv := u.ess.Settings.TeleportInvulnerability(); inv > 0 {
		p.SetNoDamageTicks(int(inv * 20))
	}
	return nil
}

// Back teleports the user to their last location. Essentials:
// AsyncTeleport.back
func (u *User) Back(charge *Trade) error {
	loc, ok := u.LastLocation()
	if !ok {
		return Tlf("noLocationFound")
	}
	if err := u.Teleport(LocationTarget(loc), charge, plugin.TeleportCommand); err != nil {
		return err
	}
	return nil
}

// HoldChunks keeps the chunks around loc loaded for a second: a ticket loads
// them at once, so the safety check reads real blocks rather than the air an
// unloaded chunk reads as, and the teleport lands in a loaded chunk.
func (e *Essentials) HoldChunks(loc plugin.Location) {
	if loc.World == nil {
		return
	}
	c := loc.Pos.Block().Chunk()
	loc.World.AddTicket(c, 2)
	e.Server.Scheduler().RunLater(20, func() { loc.World.RemoveTicket(c, 2) })
}

// CheckWorldPermission refuses a world the user may not enter when
// world-teleport-permissions is on: essentials.worlds.<world>.
func (e *Essentials) CheckWorldPermission(u *User, w plugin.World) error {
	if w == nil || !e.Settings.WorldTeleportPermissions() {
		return nil
	}
	if p, ok := u.Player(); ok && p.World() == w {
		return nil
	}
	if !u.HasPermission("essentials.worlds." + WorldName(w)) {
		return Tlf("noPerm", "essentials.worlds."+WorldName(w))
	}
	return nil
}

// SafeDestinationFor answers where a teleport to loc lands: unchanged for a
// creative, spectator or god-mode user who can fly (unless
// force-safe-teleport-location), else the nearest safe spot when
// teleport-safety is on. Essentials: LocationUtil.getSafeDestination(user)
func (e *Essentials) SafeDestinationFor(u *User, loc plugin.Location) (plugin.Location, error) {
	if p, ok := u.Player(); ok && !e.Settings.AlwaysTeleportSafety() {
		gm := p.GameMode()
		if gm == plugin.Creative || gm == plugin.Spectator || u.IsGodMode() {
			if p.AllowFlight() && e.shouldFly(loc) {
				p.SetFlying(true)
			}
			if e.Settings.TeleportToCenter() {
				return RoundedDestination(loc), nil
			}
			return loc, nil
		}
	}
	if !e.Settings.TeleportSafety() || e.Settings.ForceDisableTeleportSafety() {
		if e.Settings.TeleportToCenter() {
			return RoundedDestination(loc), nil
		}
		return loc, nil
	}
	return e.SafeDestination(loc)
}

// RoundedDestination answers loc at the middle of its block.
// Essentials: LocationUtil.getRoundedDestination
func RoundedDestination(loc plugin.Location) plugin.Location {
	b := loc.Pos.Block()
	loc.Pos = plugin.Vec3{X: float64(b.X) + .5, Y: math.Round(loc.Pos.Y), Z: float64(b.Z) + .5}
	return loc
}

// safeRadius and safeVolume are the cube searched for a safe block, nearest
// first. Essentials: LocationUtil.RADIUS, VOLUME
const safeRadius = 3

var safeVolume = func() []plugin.BlockPos {
	var out []plugin.BlockPos
	for x := -safeRadius; x <= safeRadius; x++ {
		for y := -safeRadius; y <= safeRadius; y++ {
			for z := -safeRadius; z <= safeRadius; z++ {
				out = append(out, plugin.BlockPos{X: x, Y: y, Z: z})
			}
		}
	}
	slices.SortStableFunc(out, func(a, b plugin.BlockPos) int {
		return (a.X*a.X + a.Y*a.Y + a.Z*a.Z) - (b.X*b.X + b.Y*b.Y + b.Z*b.Z)
	})
	return out
}()

// blockSets are the block tables the safety check reads, built once.
var (
	damagingBlocks = registry.BlocksOf(func(b registry.Block) bool {
		switch strings.TrimPrefix(b.Name(), "minecraft:") {
		case "cactus", "campfire", "soul_campfire", "fire", "soul_fire", "magma_block", "sweet_berry_bush",
			"wither_rose", "pointed_dripstone", "powder_snow":
			return true
		}
		return strings.HasSuffix(b.Name(), "_bed")
	})
	lavaBlock, _   = registry.BlockByName("minecraft:lava")
	waterBlock, _  = registry.BlockByName("minecraft:water")
	portalBlock, _ = registry.BlockByName("minecraft:nether_portal")
)

// hollow reports whether a player can stand inside s: no collision and not a
// liquid (water only with is-water-safe). Barriers, farmland and paths are
// solid. Essentials: HOLLOW_MATERIALS
func (e *Essentials) hollow(s plugin.BlockState) bool {
	b := s.Block()
	if b == lavaBlock {
		return false
	}
	if b == waterBlock {
		return e.Settings.WaterSafe()
	}
	return registry.CollisionHeight(uint16(s)) == 0
}

func (e *Essentials) aboveAir(w plugin.World, x, y, z, maxY int) bool {
	return y > maxY || e.hollow(w.BlockState(plugin.BlockPos{X: x, Y: y - 1, Z: z}))
}

func (e *Essentials) damaging(w plugin.World, x, y, z int) bool {
	below := w.BlockState(plugin.BlockPos{X: x, Y: y - 1, Z: z}).Block()
	if damagingBlocks[below] || below == lavaBlock {
		return true
	}
	at := w.BlockState(plugin.BlockPos{X: x, Y: y, Z: z})
	if at.Block() == portalBlock {
		return true
	}
	return !e.hollow(at) || !e.hollow(w.BlockState(plugin.BlockPos{X: x, Y: y + 1, Z: z}))
}

func (e *Essentials) unsafe(w plugin.World, x, y, z, maxY int) bool {
	return e.damaging(w, x, y, z) || e.aboveAir(w, x, y, z, maxY)
}

// SafeDestination answers the nearest safe place to stand near loc: down to
// the ground, then the nearest safe block within 3, then up. The chunks
// around loc must be loaded. Essentials: LocationUtil.getSafeDestination
func (e *Essentials) SafeDestination(loc plugin.Location) (plugin.Location, error) {
	w := loc.World
	if w == nil {
		return loc, Tlf("destinationNotSet")
	}
	minY, maxY := w.MinY(), w.MinY()+w.Height()-1
	b := loc.Pos.Block()
	// Essentials: a search starting below the logical height (the nether
	// roof) never goes above it.
	if e.Settings.ConsiderWorldHeight() && b.Y < w.LogicalHeight() {
		maxY = min(maxY, w.LogicalHeight())
	}
	x, y, z := b.X, int(math.Round(loc.Pos.Y)), b.Z
	if border := w.Border(); border != nil && !border.Contains(plugin.Vec3{X: float64(x), Z: float64(z)}) {
		cx, cz := border.Center()
		r := border.Size()/2 - 1
		x = int(math.Max(cx-r, math.Min(cx+r, float64(x))))
		z = int(math.Max(cz-r, math.Min(cz+r, float64(z))))
	}
	ox, oy, oz := x, y, z
	for e.aboveAir(w, x, y, z, maxY) {
		y--
		if y < minY {
			y = oy
			break
		}
	}
	if e.unsafe(w, x, y, z, maxY) {
		if math.Round(loc.Pos.X) == float64(ox) {
			x--
		} else {
			x++
		}
		if math.Round(loc.Pos.Z) == float64(oz) {
			z--
		} else {
			z++
		}
	}
	for i := 0; e.unsafe(w, x, y, z, maxY); {
		i++
		if i >= len(safeVolume) {
			x, y, z = ox, clamp(oy+safeRadius, minY, maxY), oz
			break
		}
		v := safeVolume[i]
		x, y, z = ox+v.X, clamp(oy+v.Y, minY, maxY), oz+v.Z
	}
	for e.unsafe(w, x, y, z, maxY) {
		y++
		if y >= maxY {
			x++
			break
		}
	}
	for e.unsafe(w, x, y, z, maxY) {
		y--
		if y <= minY+1 {
			x++
			y = min(w.HighestBlockY(x, z, plugin.MotionBlocking)+1, maxY)
			if x-48 > ox {
				return loc, Tlf("holeInFloor")
			}
		}
	}
	return plugin.Location{World: w, Pos: plugin.Vec3{X: float64(x) + .5, Y: float64(y), Z: float64(z) + .5}, Yaw: loc.Yaw, Pitch: loc.Pitch}, nil
}

// shouldFly reports whether the ground below loc is more than a block away.
// Essentials: LocationUtil.shouldFly
func (e *Essentials) shouldFly(loc plugin.Location) bool {
	b := loc.Pos.Block()
	y := b.Y
	for y > loc.World.MinY() && e.hollow(loc.World.BlockState(plugin.BlockPos{X: b.X, Y: y, Z: b.Z})) {
		y--
	}
	return b.Y-y > 1 || y <= loc.World.MinY()
}

func clamp(v, lo, hi int) int { return max(lo, min(hi, v)) }

// TpRequest is a pending /tpa or /tpahere. Essentials: IUser.TpaRequest
type TpRequest struct {
	From     plugin.UUID
	Name     string
	Here     bool
	Location plugin.Location // the requester's location for /tpahere
	Time     time.Time
}

// AddTpRequest records a request to u from from, replacing an older one from
// the same player and dropping the oldest past tpa-max-requests.
func (u *User) AddTpRequest(from *User, here bool, loc plugin.Location) {
	u.Teleports = slices.DeleteFunc(u.Teleports, func(r *TpRequest) bool { return r.From == from.ID })
	u.Teleports = append([]*TpRequest{{From: from.ID, Name: from.Name(), Here: here, Location: loc, Time: time.Now()}}, u.Teleports...)
	if max := u.ess.Settings.TpaMaxRequests(); max > 0 && len(u.Teleports) > max {
		u.Teleports = u.Teleports[:max]
	}
}

// TpRequests answers the requests that have not timed out, newest first.
func (u *User) TpRequests() []*TpRequest {
	if d := u.ess.Settings.TpaAcceptCancellation(); d > 0 {
		u.Teleports = slices.DeleteFunc(u.Teleports, func(r *TpRequest) bool { return time.Since(r.Time) > d })
	}
	return u.Teleports
}

// TpRequestFrom answers the request from the user named name, or the newest
// when name is empty.
func (u *User) TpRequestFrom(name string) (*TpRequest, bool) {
	for _, r := range u.TpRequests() {
		if name == "" || strings.EqualFold(r.Name, name) {
			return r, true
		}
	}
	return nil, false
}

// RemoveTpRequest drops a request.
func (u *User) RemoveTpRequest(r *TpRequest) {
	u.Teleports = slices.DeleteFunc(u.Teleports, func(o *TpRequest) bool { return o == r })
}
