package spawn

import (
	"math/rand/v2"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// tryRandomTeleport teleports the user to a random location of tpr.yml's
// location name, once one is found, and reports whether name is a location.
// Essentials: EssentialsSpawnPlayerListener.tryRandomTeleport
func (m *Spawn) tryRandomTeleport(u *core.User, name string) bool {
	r, ok := loadRandom(m.ess)
	if !ok || !r.has(name) {
		return false
	}
	r.location(name, func(loc plugin.Location) {
		if err := u.TeleportNow(core.LocationTarget(loc), plugin.TeleportPlugin); err != nil && u.Online() {
			m.ess.Log.Warn("random spawn failed", "player", u.Name(), "err", err)
		}
	})
	return true
}

// randomTP reads the locations of tpr.yml, the file /settpr writes.
// Essentials keeps this in the core (RandomTeleport); the port's core has
// none yet, so the spawn module reads the file itself and does not cache
// locations. Essentials: RandomTeleport
type randomTP struct {
	e    *core.Essentials
	conf *core.Config
}

func loadRandom(e *core.Essentials) (*randomTP, bool) {
	c, err := core.LoadConfig(e.DataPath("tpr.yml"), "tpr.yml")
	if err != nil {
		e.Log.Warn("cannot read tpr.yml", "err", err)
		return nil, false
	}
	return &randomTP{e: e, conf: c}, true
}

// Essentials: RandomTeleport.hasLocation
func (r *randomTP) has(name string) bool {
	return name != "" && r.conf.IsSet("locations."+name)
}

func (r *randomTP) key(name, k string) string { return "locations." + name + "." + k }

// center answers the location's center, else the overworld border's
// center on the surface. Essentials: RandomTeleport.getCenter
func (r *randomTP) center(name string) plugin.Location {
	if m := r.conf.Map(r.key(name, "center")); m != nil {
		l := core.LazyLocation{
			World: str(m["world"]), WorldName: str(m["world-name"]),
			X: num(m["x"]), Y: num(m["y"]), Z: num(m["z"]), Yaw: float32(num(m["yaw"])), Pitch: float32(num(m["pitch"])),
		}
		if loc, ok := l.Resolve(r.e.Server); ok {
			return loc
		}
	}
	w := r.e.Server.Overworld()
	x, z := w.Border().Center()
	y := w.HighestBlockY(int(x), int(z), plugin.MotionBlocking) + 1
	return plugin.Location{World: w, Pos: plugin.Vec3{X: x, Y: float64(y), Z: z}}
}

// location finds a random location and calls done with it on the tick
// goroutine: up to find-attempts tries, then the center.
// Essentials: RandomTeleport.getRandomLocation
func (r *randomTP) location(name string, done func(plugin.Location)) {
	center := r.center(name)
	minR := r.conf.Float(r.key(name, "min-range"), 0)
	maxR := r.conf.Float(r.key(name, "max-range"), center.World.Border().Size()/2)
	excluded := map[string]bool{}
	for _, b := range r.conf.StringList("excluded-biomes") {
		excluded[strings.ToLower(b)] = true
	}
	var attempt func(n int)
	attempt = func(n int) {
		if n <= 0 {
			done(center)
			return
		}
		r.calculate(center, minR, maxR, func(loc plugin.Location, ok bool) {
			if ok && r.valid(loc, excluded) {
				done(loc)
				return
			}
			attempt(n - 1)
		})
	}
	attempt(r.conf.Int("find-attempts", 10))
}

// calculate picks an evenly spread point between the ranges around center
// and finds its surface once its chunk is loaded.
// Essentials: RandomTeleport.calculateRandomLocation
func (r *randomTP) calculate(center plugin.Location, minR, maxR float64, done func(plugin.Location, bool)) {
	rectX := rand.Float64()*(maxR-minR) + minR
	rectZ := rand.Float64()*(maxR+minR) - minR
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
	loc := plugin.Location{World: w, Pos: plugin.Vec3{X: center.Pos.X + ox, Z: center.Pos.Z + oz}, Yaw: 360*rand.Float32() - 180}
	pos := loc.Pos.Block()
	w.LoadChunk(pos.Chunk(), true, func(c plugin.Chunk) {
		if c == nil {
			done(loc, false)
			return
		}
		if w.Dimension().Path == "the_nether" {
			y, ok := netherY(w, pos.X, pos.Z)
			loc.Pos.Y = float64(y)
			done(loc, ok)
			return
		}
		loc.Pos.Y = float64(w.HighestBlockY(pos.X, pos.Z, plugin.MotionBlocking) + 1)
		done(loc, true)
	})
}

// netherY answers the lowest open spot under the bedrock roof from y 32:
// air for the feet and head over a solid block that is not lava.
// Essentials: RandomTeleport.getNetherYAt
func netherY(w plugin.World, x, z int) (int, bool) {
	top := w.MinY() + w.Height()
	for y := 32; y < top; y++ {
		at := func(dy int) registry.Block { return w.BlockState(plugin.BlockPos{X: x, Y: y + dy, Z: z}).Block() }
		if at(0) == registry.BlockBedrock {
			break
		}
		below := at(-1)
		if isAir(at(0)) && isAir(at(1)) && !isAir(below) && below != registry.BlockLava && below != registry.BlockMagmaBlock {
			return y, true
		}
	}
	return 0, false
}

func isAir(b registry.Block) bool {
	return b == registry.BlockAir || b == registry.BlockCaveAir || b == registry.BlockVoidAir
}

// valid refuses a location below the world or in an excluded biome.
// Essentials: RandomTeleport.isValidRandomLocation
func (r *randomTP) valid(loc plugin.Location, excluded map[string]bool) bool {
	if loc.Pos.Block().Y <= loc.World.MinY() {
		return false
	}
	name := loc.World.Biome(loc.Pos.Block()).Name()
	return !excluded[name] && !excluded[strings.TrimPrefix(name, "minecraft:")]
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func num(v any) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case float64:
		return n
	}
	return 0
}
