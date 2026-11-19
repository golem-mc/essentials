package teleport

import (
	"math/rand/v2"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

func (g *group) registerRandom() {
	g.e.RegisterCommand(
		core.Cmd{Name: "tpr", Run: g.runTpr, Complete: func(ctx *core.Ctx) []string {
			switch {
			case len(ctx.Args) == 1:
				var out []string
				for _, n := range g.rtp.locations() {
					if !g.rtp.perLocationPermission() || ctx.Has("essentials.tpr.location."+n) {
						out = append(out, n)
					}
				}
				return out
			case len(ctx.Args) == 2 && ctx.Has("essentials.tpr.others"):
				return ctx.CompletePlayers()
			}
			return nil
		}},
		core.Cmd{Name: "settpr", PlayerOnly: true, Run: g.runSettpr, Complete: func(ctx *core.Ctx) []string {
			switch len(ctx.Args) {
			case 1:
				return core.WorldNames(g.e.Server)
			case 2:
				return []string{"center", "minrange", "maxrange"}
			}
			return nil
		}},
	)
}

// randomTeleport is tpr.yml: named areas /tpr picks a random place in, with
// a cache of places found ahead. Essentials: RandomTeleport
type randomTeleport struct {
	e      *core.Essentials
	conf   *core.Config
	cached map[string][]plugin.Location
}

func newRandomTeleport(e *core.Essentials) *randomTeleport {
	r := &randomTeleport{e: e}
	r.reload()
	return r
}

func (r *randomTeleport) reload() {
	c, err := core.LoadConfig(r.e.DataPath("tpr.yml"), "tpr.yml")
	if err != nil {
		r.e.Log.Warn("cannot read tpr.yml", "err", err)
		c = core.NewMemoryConfig(nil)
	}
	r.conf = c
	r.cached = map[string][]plugin.Location{}
}

func (r *randomTeleport) save() {
	if err := r.conf.Save(); err != nil {
		r.e.Log.Warn("cannot save tpr.yml", "err", err)
	}
}

func key(name, k string) string { return "locations." + name + "." + k }

func (r *randomTeleport) has(name string) bool {
	_, ok := r.conf.Get("locations." + name)
	return ok
}

func (r *randomTeleport) locations() []string { return r.conf.Keys("locations") }

func (r *randomTeleport) defaultLocation() string {
	return r.conf.String("default-location", "{world}")
}

func (r *randomTeleport) perLocationPermission() bool {
	return r.conf.Bool("per-location-permission", false)
}

func (r *randomTeleport) findAttempts() int   { return r.conf.Int("find-attempts", 10) }
func (r *randomTeleport) cacheThreshold() int { return r.conf.Int("cache-threshold", 10) }

func (r *randomTeleport) excludedBiomes() map[string]bool {
	out := map[string]bool{}
	for _, b := range r.conf.StringList("excluded-biomes") {
		out[strings.ToLower(b)] = true
	}
	return out
}

// center answers the area's centre; unset, it becomes the overworld border's
// centre on the ground there.
func (r *randomTeleport) center(name string) plugin.Location {
	if s, ok := r.conf.Section(key(name, "center")); ok {
		l := core.LazyLocation{World: s.String("world", ""), WorldName: s.String("world-name", ""), X: s.Float("x", 0),
			Y: s.Float("y", 0), Z: s.Float("z", 0), Yaw: float32(s.Float("yaw", 0)), Pitch: float32(s.Float("pitch", 0))}
		if loc, ok := l.Resolve(r.e.Server); ok {
			return loc
		}
	}
	w := r.e.Server.Overworld()
	var cx, cz float64
	if b := w.Border(); b != nil {
		cx, cz = b.Center()
	}
	y := w.HighestBlockY(int(cx), int(cz), plugin.MotionBlocking) + 1
	loc := plugin.Location{World: w, Pos: plugin.Vec3{X: cx, Y: float64(y), Z: cz}}
	r.setCenter(name, loc)
	return loc
}

func (r *randomTeleport) setCenter(name string, loc plugin.Location) {
	l := core.LazyOf(loc)
	r.conf.Set(key(name, "center"), map[string]any{"world": l.World, "world-name": l.WorldName,
		"x": l.X, "y": l.Y, "z": l.Z, "yaw": float64(l.Yaw), "pitch": float64(l.Pitch)})
	r.save()
	delete(r.cached, name)
}

func (r *randomTeleport) minRange(name string) float64 {
	return r.conf.Float(key(name, "min-range"), 0)
}

func (r *randomTeleport) maxRange(name string) float64 {
	size := 60000000.0
	if b := r.center(name).World.Border(); b != nil {
		size = b.Size()
	}
	return r.conf.Float(key(name, "max-range"), size/2)
}

func (r *randomTeleport) setRange(name, k string, v float64) {
	r.conf.Set(key(name, k), v)
	r.save()
	delete(r.cached, name)
}

// randomLocation calls done with a random place in the area: a cached one,
// else one found now (the centre when every attempt fails), topping up the
// cache meanwhile. Essentials: RandomTeleport.getRandomLocation
func (g *group) randomLocation(name string, done func(plugin.Location)) {
	r := g.rtp
	if len(r.cached[name]) < r.cacheThreshold() {
		g.cacheLocations(name)
	}
	if c := r.cached[name]; len(c) > 0 {
		r.cached[name] = c[1:]
		done(c[0])
		return
	}
	g.attemptLocation(r.findAttempts(), r.center(name), r.minRange(name), r.maxRange(name), done)
}

// cacheLocations looks for find-attempts places on the next tick, keeping
// the valid ones. Essentials: RandomTeleport.cacheRandomLocations
func (g *group) cacheLocations(name string) {
	r := g.rtp
	g.e.Server.Scheduler().RunTask(func() {
		center, lo, hi := r.center(name), r.minRange(name), r.maxRange(name)
		for range r.findAttempts() {
			g.calculateLocation(center, lo, hi, func(l plugin.Location) {
				if g.validLocation(l) {
					r.cached[name] = append(r.cached[name], l)
				}
			})
		}
	})
}

// attemptLocation tries attempts random places, then gives the centre.
// Essentials: RandomTeleport.attemptRandomLocation
func (g *group) attemptLocation(attempts int, center plugin.Location, lo, hi float64, done func(plugin.Location)) {
	if attempts <= 0 {
		done(center)
		return
	}
	g.calculateLocation(center, lo, hi, func(l plugin.Location) {
		if g.validLocation(l) {
			done(l)
			return
		}
		g.attemptLocation(attempts-1, center, lo, hi, done)
	})
}

// calculateLocation picks a place lo to hi from center, evenly over the
// ring's square, and calls done with it on the ground once its chunk is
// loaded. Essentials: RandomTeleport.calculateRandomLocation
func (g *group) calculateLocation(center plugin.Location, lo, hi float64, done func(plugin.Location)) {
	rectX := rand.Float64()*(hi-lo) + lo
	rectZ := rand.Float64()*(hi+lo) - lo
	var ox, oz float64
	switch rand.IntN(4) {
	case 0:
		ox, oz = rectX, rectZ
	case 1:
		ox, oz = -rectZ, rectX
	case 2:
		ox, oz = -rectX, -rectZ
	default:
		ox, oz = rectZ, -rectX
	}
	w := center.World
	loc := plugin.Location{World: w, Pos: plugin.Vec3{X: center.Pos.X + ox, Y: float64(maxHeight(w)), Z: center.Pos.Z + oz},
		Yaw: 360*rand.Float32() - 180}
	w.LoadChunk(loc.Pos.Block().Chunk(), true, func(plugin.Chunk) {
		b := loc.Pos.Block()
		if w.Dimension() == plugin.MinecraftKey("the_nether") {
			loc.Pos.Y = float64(g.netherY(w, b.X, b.Z))
		} else {
			loc.Pos.Y = float64(w.HighestBlockY(b.X, b.Z, plugin.MotionBlocking) + 1)
		}
		done(loc)
	})
}

// netherY answers the lowest safe Y from 32 below the bedrock roof, or the
// world's bottom when there is none. Essentials: RandomTeleport.getNetherYAt
func (g *group) netherY(w plugin.World, x, z int) int {
	for y := 32; y < maxHeight(w); y++ {
		if state(w, x, y, z).Block() == bedrockBlock {
			break
		}
		if !g.unsafe(w, x, y, z) {
			return y
		}
	}
	return w.MinY()
}

// validLocation reports whether l is above the world's bottom and not in an
// excluded biome. Essentials: RandomTeleport.isValidRandomLocation
func (g *group) validLocation(l plugin.Location) bool {
	b := l.Pos.Block()
	if b.Y <= l.World.MinY() {
		return false
	}
	excluded := g.rtp.excludedBiomes()
	biome := l.World.Biome(b).Name()
	return !excluded[biome] && !excluded[strings.TrimPrefix(biome, "minecraft:")]
}

// Essentials: Commandtpr
func (g *group) runTpr(ctx *core.Ctx) error {
	r := g.rtp
	user := ctx.User
	if user == nil {
		if len(ctx.Args) < 2 {
			return core.ErrNotEnoughArgs
		}
		target, err := ctx.GetPlayer(1, false)
		if err != nil {
			return err
		}
		name := ctx.Args[0]
		if !r.has(name) {
			return core.Tlf("tprNotExist")
		}
		target.Send("tprSuccess")
		ctx.Send("tprOtherUser", target.DisplayName())
		sender := ctx.Sender
		g.randomLocation(name, func(loc plugin.Location) {
			if err := g.now(target, core.LocationTarget(loc), false, func() {
				target.Send("tprSuccessDone")
			}); err != nil {
				g.showError(sender, err)
			}
		})
		return nil
	}
	target := user
	var name string
	if len(ctx.Args) == 0 {
		name = strings.ReplaceAll(r.defaultLocation(), "{world}", core.WorldName(world(user)))
	} else {
		name = ctx.Args[0]
		if !r.has(name) {
			return core.Tlf("tprNotExist")
		}
		if r.perLocationPermission() && !user.HasPermission("essentials.tpr.location."+name) {
			return core.Tlf("tprNoPermission")
		}
		if len(ctx.Args) > 1 && user.HasPermission("essentials.tpr.others") {
			t, err := ctx.GetPlayer(1, false)
			if err != nil {
				return err
			}
			target = t
		}
	}
	target.Send("tprSuccess")
	if target.ID != user.ID {
		user.Send("tprOtherUser", target.DisplayName())
	}
	charge := ctx.Charge
	g.randomLocation(name, func(loc plugin.Location) {
		if err := g.teleport(target, target, core.LocationTarget(loc), charge, func() {
			target.Send("tprSuccessDone")
		}); err != nil {
			g.showErrorTo(user, err)
		}
	})
	return core.ErrNoCharge
}

// Essentials: Commandsettpr
func (g *group) runSettpr(ctx *core.Ctx) error {
	args := ctx.Args
	if len(args) < 2 {
		return core.ErrNotEnoughArgs
	}
	switch {
	case strings.EqualFold(args[1], "center"):
		g.rtp.setCenter(args[0], ctx.Player.Location())
		ctx.Send("settpr")
	case len(args) > 2:
		if k := strings.ToLower(args[1]); k == "minrange" || k == "maxrange" {
			v, err := parseDouble(args[2])
			if err != nil {
				return err
			}
			g.rtp.setRange(args[0], k[:3]+"-range", v)
		}
		ctx.Send("settprValue", strings.ToLower(args[1]), strings.ToLower(args[2]))
	default:
		return core.ErrNotEnoughArgs
	}
	return nil
}
