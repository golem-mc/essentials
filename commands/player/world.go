package player

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

func registerWorld(e *core.Essentials) {
	e.RegisterCommand(timeCmd(), weatherCmd(), thunderCmd(), ptimeCmd(), pweatherCmd(), treeCmd(), bigtreeCmd(),
		lightningCmd())
}

var spaces = regexp.MustCompile(`\s+`)

// Essentials: Commandtime
func timeCmd() core.Cmd {
	// worlds answers the worlds a selector names, sorted by name: the
	// sender's world (all for the console) without one.
	worlds := func(ctx *core.Ctx, sel string, given bool) ([]plugin.World, error) {
		var out []plugin.World
		switch {
		case !given && ctx.Player != nil:
			out = []plugin.World{ctx.Player.World()}
		case !given:
			out = ctx.Ess.Server.Worlds()
		default:
			if w, ok := core.WorldByName(ctx.Ess.Server, sel); ok {
				out = []plugin.World{w}
			} else if sel == "*" || strings.EqualFold(sel, "all") {
				out = ctx.Ess.Server.Worlds()
			} else {
				return nil, core.Tlf("invalidWorld")
			}
		}
		out = slices.Clone(out)
		slices.SortFunc(out, func(a, b plugin.World) int { return strings.Compare(worldLabel(a), worldLabel(b)) })
		return out, nil
	}
	normalize := func(w plugin.World) string { return spaces.ReplaceAllString(strings.ToLower(worldLabel(w)), "_") }
	canUpdate := func(ctx *core.Ctx, w plugin.World) bool {
		return !ctx.Ess.Config.Bool("world-time-permissions", false) || ctx.Has("essentials.time.world.all") ||
			ctx.Has("essentials.time.world."+normalize(w))
	}
	parse := func(s string) (int64, error) {
		if core.IsInt(s) {
			s += "t"
		}
		t, ok := parseTicks(s)
		if !ok {
			return 0, core.ErrNotEnoughArgs
		}
		return t, nil
	}
	return core.Cmd{
		Name: "time",
		Run: func(ctx *core.Ctx) error {
			args := ctx.Args
			var tick int64
			var ws []plugin.World
			var err error
			add := false
			switch {
			case len(args) == 0:
				if ws, err = worlds(ctx, "", false); err != nil {
					return err
				}
				label := strings.ToLower(ctx.Label)
				if !strings.HasSuffix(label, "day") && !strings.HasSuffix(label, "night") {
					if len(ws) == 1 {
						ctx.SendText(ctx.Ess.Mini(formatTime(ctx.Ess, ws[0].DayTime())))
						return nil
					}
					for _, w := range ws {
						ctx.Send("timeWorldCurrent", worldLabel(w), core.Raw(formatTime(ctx.Ess, w.DayTime())))
					}
					return nil
				}
				tick, _ = parseTicks(strings.ReplaceAll(label, "e", ""))
			case len(args) == 1:
				if ws, err = worlds(ctx, "", false); err != nil {
					return err
				}
				if tick, err = parse(args[0]); err != nil {
					return err
				}
			case strings.EqualFold(args[0], "set") || strings.EqualFold(args[0], "add"):
				add = strings.EqualFold(args[0], "add")
				if tick, err = parse(args[1]); err != nil {
					return err
				}
				if ws, err = worlds(ctx, ctx.Arg(2), len(args) > 2); err != nil {
					return err
				}
			default:
				if tick, err = parse(args[0]); err != nil {
					return err
				}
				if ws, err = worlds(ctx, args[1], true); err != nil {
					return err
				}
			}
			if !ctx.Has("essentials.time.set") {
				return core.Tlf("timeSetPermission")
			}
			for _, w := range ws {
				if !canUpdate(ctx, w) {
					return core.Tlf("timeSetWorldPermission", worldLabel(ctx.Player.World()))
				}
			}
			var names []string
			for _, w := range ws {
				// The time players with a relative /ptime see, to keep it.
				kept := map[plugin.Player]int64{}
				for _, p := range w.Players() {
					if t, fixed, over := p.PlayerTime(); over && !fixed && t != w.DayTime() {
						kept[p] = t
					}
				}
				// Bukkit's World.setTime moves the clock forward to the
				// time of day it is given.
				t := w.DayTime()
				if add {
					w.SetDayTime(t + mod(tick, ticksPerDay))
				} else {
					w.SetDayTime(t + mod(tick-t, ticksPerDay))
				}
				now := mod(w.DayTime(), ticksPerDay)
				for p, t := range kept {
					p.SetPlayerTime(t-now, true)
				}
				names = append(names, worldLabel(w))
			}
			key := "timeWorldSet"
			if add {
				key = "timeWorldAdd"
			}
			ctx.Send(key, formatTicks(tick), strings.Join(names, ", "))
			return nil
		},
		Complete: func(ctx *core.Ctx) []string {
			args := ctx.Args
			switch {
			case len(args) == 1 && ctx.Has("essentials.time.set"):
				return []string{"add", "set"}
			case len(args) == 2 && strings.EqualFold(args[0], "set"):
				return []string{"sunrise", "day", "morning", "noon", "afternoon", "sunset", "night", "midnight"}
			case len(args) == 2 && strings.EqualFold(args[0], "add"):
				return []string{"1000", "2000", "3000", "4000", "5000"}
			case len(args) == 3 && (strings.EqualFold(args[0], "set") || strings.EqualFold(args[0], "add")):
				var out []string
				for _, w := range ctx.Ess.Server.Worlds() {
					if ctx.Has("essentials.time.world." + normalize(w)) {
						out = append(out, worldLabel(w))
					}
				}
				if ctx.Has("essentials.time.world.all") {
					out = append(out, "*")
				}
				return out
			}
			return nil
		},
	}
}

// setStorm starts or stops rain for seconds (0 for vanilla's random
// length), leaving thunder alone. Bukkit: World.setStorm and
// setWeatherDuration
func setStorm(w plugin.World, storm bool, seconds int) {
	w.SetStorm(storm, time.Duration(seconds)*time.Second)
}

// Essentials: Commandweather
func weatherCmd() core.Cmd {
	set := func(ctx *core.Ctx, w plugin.World, storm bool, dur string) error {
		if dur != "" {
			secs, err := parseInt(dur)
			if err != nil {
				return err
			}
			setStorm(w, storm, secs)
			key := "weatherSunFor"
			if storm {
				key = "weatherStormFor"
			}
			ctx.Send(key, worldLabel(w), dur)
			return nil
		}
		setStorm(w, storm, 0)
		key := "weatherSun"
		if storm {
			key = "weatherStorm"
		}
		ctx.Send(key, worldLabel(w))
		return nil
	}
	return core.Cmd{
		Name: "weather",
		Run: func(ctx *core.Ctx) error {
			args := ctx.Args
			if ctx.User == nil {
				if len(args) < 2 {
					return errors.New("When running from console, usage is: /" + ctx.Label + " <world> <storm/sun> [duration]")
				}
				w, ok := core.WorldByName(ctx.Ess.Server, args[0])
				if !ok {
					return core.Tlf("weatherInvalidWorld", args[0])
				}
				return set(ctx, w, strings.EqualFold(args[1], "storm"), ctx.Arg(2))
			}
			var storm bool
			if len(args) == 0 {
				label := strings.ToLower(ctx.Label)
				switch {
				case strings.HasSuffix(label, "sun"):
				case strings.HasSuffix(label, "storm") || strings.HasSuffix(label, "rain"):
					storm = true
				default:
					return core.ErrNotEnoughArgs
				}
			} else {
				storm = strings.EqualFold(args[0], "storm")
			}
			return set(ctx, ctx.Player.World(), storm, ctx.Arg(1))
		},
		Complete: func(ctx *core.Ctx) []string {
			n := len(ctx.Args)
			if ctx.User == nil {
				if n == 1 {
					return core.WorldNames(ctx.Ess.Server)
				}
				n--
			}
			switch n {
			case 1:
				return []string{"storm", "sun"}
			case 2:
				return []string{"1", "60", "600", "3600", "86400"}
			}
			return nil
		},
	}
}

// Essentials: Commandthunder
func thunderCmd() core.Cmd {
	return core.Cmd{
		Name:       "thunder",
		PlayerOnly: true,
		Run: func(ctx *core.Ctx) error {
			if len(ctx.Args) == 0 {
				return core.ErrNotEnoughArgs
			}
			w := ctx.Player.World()
			on := strings.EqualFold(ctx.Args[0], "true")
			secs := 0
			if len(ctx.Args) > 1 {
				n, err := parseInt(ctx.Args[1])
				if err != nil {
					return err
				}
				secs = n
			}
			// Bukkit: setThundering (the thunder timer rolled anew), then
			// with a duration setThunderDuration; the rain is left alone.
			w.SetThundering(on, time.Duration(secs)*time.Second)
			if len(ctx.Args) == 1 {
				ctx.Send("thunder", enableDisable(ctx.Ess, ctx.Sender, on))
			} else {
				ctx.Send("thunderDuration", enableDisable(ctx.Ess, ctx.Sender, on), secs)
			}
			return nil
		},
		Complete: func(ctx *core.Ctx) []string {
			switch len(ctx.Args) {
			case 1:
				return []string{"true", "false"}
			case 2:
				return core.Durations
			}
			return nil
		},
	}
}

var getAliases = []string{"get", "list", "show", "display"}

// playerOverride is /ptime and /pweather: a time or weather a player sees
// instead of the world's. Essentials: Commandptime, Commandpweather
type playerOverride struct {
	name, players, othersKey string
	// show tells the sender u's time or weather.
	show func(ctx *core.Ctx, u *core.User)
	// set checks the value argument and answers what applies it to a
	// player and the message to send with the players' names.
	set   func(ctx *core.Ctx, v string) (apply func(*core.User), done func(names string), err error)
	first []string
}

func (o playerOverride) cmd() core.Cmd {
	return core.Cmd{
		Name: o.name,
		Run: func(ctx *core.Ctx) error {
			args := ctx.Args
			show := func(u *core.User) error {
				o.show(ctx, u)
				return nil
			}
			if len(args) == 0 || slices.Contains(getAliases, strings.ToLower(args[0])) {
				if len(args) > 1 {
					if args[1] == "*" || args[1] == "**" {
						ctx.Send(o.players)
					}
					return loopOnline(ctx, false, true, args[1], show)
				}
				if ctx.User != nil {
					return show(ctx.User)
				}
				if len(args) == 1 {
					return core.ErrNotEnoughArgs
				}
				if len(ctx.Ess.Server.Players()) > 1 {
					ctx.Send(o.players)
				}
				for _, u := range ctx.Ess.OnlineUsers() {
					_ = show(u)
				}
				return nil
			}
			self := "*"
			if ctx.Player != nil {
				self = ctx.Player.Name()
			}
			if len(args) > 1 && !ctx.Has("essentials."+o.name+".others") && !strings.EqualFold(args[1], self) {
				ctx.Send(o.othersKey)
				return nil
			}
			apply, done, err := o.set(ctx, args[0])
			if err != nil {
				return err
			}
			target := self
			if len(args) > 1 {
				target = args[1]
			}
			var names []string
			err = loopOnline(ctx, false, true, target, func(u *core.User) error {
				apply(u)
				names = append(names, u.Name())
				return nil
			})
			if err != nil {
				return err
			}
			done(strings.Join(names, ", "))
			return nil
		},
		Complete: func(ctx *core.Ctx) []string {
			switch {
			case len(ctx.Args) == 1:
				return o.first
			case len(ctx.Args) == 2 && (slices.Contains(getAliases, ctx.Args[0]) || ctx.User == nil ||
				ctx.Has("essentials."+o.name+".others")):
				return loopPlayers(ctx)
			}
			return nil
		},
	}
}

// Essentials: Commandptime
func ptimeCmd() core.Cmd {
	return playerOverride{
		name: "ptime", players: "pTimePlayers", othersKey: "pTimeOthersPermission",
		first: []string{"get", "reset", "sunrise", "day", "morning", "noon", "afternoon", "sunset", "night", "midnight"},
		// Essentials: Commandptime.getUserTime
		show: func(ctx *core.Ctx, u *core.User) {
			p, ok := u.Player()
			if !ok {
				return
			}
			t, fixed, over := p.PlayerTime()
			if !over || !fixed && t == p.World().DayTime() {
				ctx.Send("pTimeNormal", u.Name())
				return
			}
			key := "pTimeCurrent"
			if fixed {
				key = "pTimeCurrentFixed"
			}
			ctx.Send(key, u.Name(), core.Raw(formatTime(ctx.Ess, t)))
		},
		set: func(ctx *core.Ctx, v string) (func(*core.User), func(string), error) {
			fixed := strings.HasPrefix(v, "@")
			v = strings.TrimPrefix(v, "@")
			if meansReset(v) {
				return func(u *core.User) {
						if p, ok := u.Player(); ok {
							p.ResetPlayerTime()
						}
					}, func(names string) {
						ctx.Send("pTimeReset", names)
					}, nil
			}
			ticks, ok := parseTicks(v)
			if !ok {
				return nil, nil, core.ErrNotEnoughArgs
			}
			// Essentials: Commandptime.setUserTime
			apply := func(u *core.User) {
				p, ok := u.Player()
				if !ok {
					return
				}
				t, _, _ := p.PlayerTime()
				t = t - mod(t, ticksPerDay) + ticksPerDay + ticks
				if !fixed {
					t -= mod(p.World().DayTime(), ticksPerDay)
				}
				p.SetPlayerTime(t, !fixed)
			}
			key := "pTimeSet"
			if fixed {
				key = "pTimeSetFixed"
			}
			return apply, func(names string) { ctx.Send(key, core.Raw(formatTime(ctx.Ess, ticks)), names) }, nil
		},
	}.cmd()
}

// Essentials: Commandpweather
func pweatherCmd() core.Cmd {
	return playerOverride{
		name: "pweather", players: "pWeatherPlayers", othersKey: "pWeatherOthersPermission",
		first: []string{"get", "reset", "storm", "sun"},
		// Essentials: Commandpweather.getUserWeather
		show: func(ctx *core.Ctx, u *core.User) {
			p, ok := u.Player()
			if !ok {
				return
			}
			raining, over := p.PlayerWeather()
			switch {
			case !over:
				ctx.Send("pWeatherNormal", u.Name())
			case raining:
				ctx.Send("pWeatherCurrent", u.Name(), "downfall") // Bukkit: WeatherType.DOWNFALL
			default:
				ctx.Send("pWeatherCurrent", u.Name(), "clear")
			}
		},
		set: func(ctx *core.Ctx, v string) (func(*core.User), func(string), error) {
			v = strings.ToLower(v)
			var raining bool
			switch v {
			case "sun", "clear":
			case "storm", "thunder":
				raining = true
			case "reset":
				return func(u *core.User) {
						if p, ok := u.Player(); ok {
							p.ResetPlayerWeather()
						}
					}, func(names string) {
						ctx.Send("pWeatherReset", names)
					}, nil
			default:
				return nil, nil, usageWith(ctx, "pWeatherInvalidAlias")
			}
			return func(u *core.User) {
					if p, ok := u.Player(); ok {
						p.SetPlayerWeather(raining)
					}
				}, func(names string) {
					ctx.Send("pWeatherSet", v, names)
				}, nil
		},
	}.cmd()
}

// treeKind is a tree /tree grows.
type treeKind struct {
	name string
	typ  plugin.TreeType
}

// treeTypes are Bukkit's tree types, in its order, by the name /tree takes
// (the enum name without underscores).
var treeTypes = []treeKind{
	{"tree", plugin.TreeOak}, {"bigtree", plugin.TreeFancyOak}, {"redwood", plugin.TreeSpruce},
	{"tallredwood", plugin.TreePine}, {"birch", plugin.TreeBirch}, {"jungle", plugin.TreeMegaJungle},
	{"smalljungle", plugin.TreeJungle}, {"cocoatree", plugin.TreeCocoa}, {"junglebush", plugin.TreeJungleBush},
	{"redmushroom", plugin.TreeRedMushroom}, {"brownmushroom", plugin.TreeBrownMushroom},
	{"swamp", plugin.TreeSwamp}, {"acacia", plugin.TreeAcacia}, {"darkoak", plugin.TreeDarkOak},
	{"megaredwood", plugin.TreeMegaSpruce}, {"megapine", plugin.TreeMegaPine}, {"tallbirch", plugin.TreeTallBirch},
	{"chorusplant", plugin.TreeChorusPlant}, {"crimsonfungus", plugin.TreeCrimsonFungus},
	{"warpedfungus", plugin.TreeWarpedFungus}, {"azalea", plugin.TreeAzalea}, {"mangrove", plugin.TreeMangrove},
	{"tallmangrove", plugin.TreeTallMangrove}, {"cherry", plugin.TreeCherry}, {"paleoak", plugin.TreePaleOak},
	{"paleoakcreaking", plugin.TreePaleOakCreaking},
}

// generateTree grows a tree above the block the player looks at.
// Bukkit: World.generateTree
func generateTree(ctx *core.Ctx, typ plugin.TreeType, failKey string) (bool, error) {
	loc := targetLocation(ctx.Player, ctx.Ess.Config.Int("tree-command-range-limit", 300))
	pos := loc.Pos.Block()
	pos.Y++
	w := loc.World
	if solid(w.BlockState(pos)) {
		return false, core.Tlf(failKey)
	}
	return w.GenerateTree(typ, pos), nil
}

// Essentials: Commandtree
func treeCmd() core.Cmd {
	return core.Cmd{
		Name:       "tree",
		PlayerOnly: true,
		Run: func(ctx *core.Ctx) error {
			if len(ctx.Args) < 1 {
				return core.ErrNotEnoughArgs
			}
			name := ctx.Args[0]
			if strings.EqualFold(name, "jungle") {
				name = "smalljungle"
			}
			i := slices.IndexFunc(treeTypes, func(t treeKind) bool { return strings.EqualFold(t.name, name) })
			if i < 0 {
				return core.ErrNotEnoughArgs
			}
			ok, err := generateTree(ctx, treeTypes[i].typ, "treeFailure")
			if err != nil {
				return err
			}
			if ok {
				ctx.Send("treeSpawned")
			} else {
				ctx.Send("treeFailure")
			}
			return nil
		},
		Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) != 1 {
				return nil
			}
			var out []string
			for _, t := range treeTypes {
				out = append(out, t.name)
			}
			return out
		},
	}
}

// Essentials: Commandbigtree
func bigtreeCmd() core.Cmd {
	kinds := map[string]plugin.TreeType{"redwood": plugin.TreePine, "tree": plugin.TreeFancyOak,
		"jungle": plugin.TreeMegaJungle, "darkoak": plugin.TreeDarkOak}
	return core.Cmd{
		Name:       "bigtree",
		PlayerOnly: true,
		Run: func(ctx *core.Ctx) error {
			typ, ok := kinds[strings.ToLower(ctx.Arg(0))]
			if !ok {
				return core.ErrNotEnoughArgs
			}
			placed, err := generateTree(ctx, typ, "bigTreeFailure")
			if err != nil {
				return err
			}
			if !placed {
				return core.Tlf("bigTreeFailure")
			}
			ctx.Send("bigTreeSuccess")
			return nil
		},
		Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 {
				return []string{"redwood", "tree", "jungle", "darkoak"}
			}
			return nil
		},
	}
}

// Essentials: Commandlightning
func lightningCmd() core.Cmd {
	return core.Cmd{
		Name: "lightning",
		Run: func(ctx *core.Ctx) error {
			if len(ctx.Args) == 0 || !ctx.Has("essentials.lightning.others") {
				if ctx.Player == nil {
					return core.ErrNotEnoughArgs
				}
				b := targetBlockExact(ctx.Player, 600)
				b.World.StrikeLightning(plugin.Vec3{X: float64(b.Pos.X), Y: float64(b.Pos.Y), Z: float64(b.Pos.Z)}, false)
				return nil
			}
			power := 5
			if len(ctx.Args) > 1 {
				if n, err := parseInt(ctx.Args[1]); err == nil {
					power = n
				}
			}
			return loopOnline(ctx, false, true, ctx.Args[0], func(u *core.User) error {
				p, ok := u.Player()
				if !ok {
					return nil
				}
				ctx.Send("lightningUse", u.DisplayName())
				p.World().StrikeLightning(p.Location().Pos, true)
				if !u.Data.GodMode {
					damage(ctx.Ess, p, float32(power), "lightning_bolt")
				}
				if ctx.Ess.Config.Bool("warn-on-smite", true) {
					u.Send("lightningSmited")
				}
				return nil
			})
		},
		Complete: func(ctx *core.Ctx) []string {
			if ctx.User != nil && !ctx.Has("essentials.lightning.others") {
				return nil
			}
			switch len(ctx.Args) {
			case 1:
				return loopPlayers(ctx)
			case 2:
				return []string{"5"}
			}
			return nil
		},
	}
}
