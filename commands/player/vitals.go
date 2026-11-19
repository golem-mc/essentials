package player

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

func registerVitals(e *core.Essentials) {
	e.RegisterCommand(healCmd(), feedCmd(), gamemodeCmd(), speedCmd(), expCmd(), burnCmd(), killCmd(),
		suicideCmd(), restCmd(), iceCmd())
}

// healCooldown refuses a heal or feed within heal-cooldown of the last one,
// then records this one. Essentials: User.healCooldown
func healCooldown(u *core.User) error {
	now := time.Now()
	if last := u.Data.Timestamps.LastHeal; last > 0 {
		next := time.UnixMilli(last).Add(u.Ess().Settings.HealCooldown())
		if next.After(now) && !u.HasPermission("essentials.heal.cooldown.bypass") {
			return core.Tlf("timeBeforeHeal", core.FormatDateDiff(next))
		}
	}
	u.Data.Timestamps.LastHeal = now.UnixMilli()
	u.Save()
	return nil
}

// othersComplete completes players for the first argument with perm.
func othersComplete(perm string) func(ctx *core.Ctx) []string {
	return func(ctx *core.Ctx) []string {
		if len(ctx.Args) == 1 && ctx.Has(perm) {
			return loopPlayers(ctx)
		}
		return nil
	}
}

// Essentials: Commandheal
func healCmd() core.Cmd {
	heal := func(ctx *core.Ctx, u *core.User) error {
		p, ok := u.Player()
		if !ok {
			return nil
		}
		if p.Health() == 0 {
			return core.Tlf("healDead")
		}
		// Player.Heal would fire the event too, but a cancel must stop the
		// rest of the heal, so the event is fired here and the health set.
		ev := &plugin.EntityRegainHealthEvent{Entity: p, Amount: p.MaxHealth() - p.Health(), Reason: plugin.RegainPlugin}
		ctx.Ess.Server.Events().Fire(ev)
		if ev.Cancelled() {
			return nil
		}
		p.SetHealth(min(p.Health()+ev.Amount, p.MaxHealth()))
		p.SetFood(20)
		p.SetFireTicks(0)
		p.SetAir(300)
		u.Send("heal")
		if ctx.Ess.Config.Bool("remove-effects-on-heal", true) {
			p.ClearEffects()
		}
		if !isSender(ctx, u) {
			ctx.Send("healOther", u.DisplayName())
		}
		return nil
	}
	return core.Cmd{
		Name: "heal",
		Run: func(ctx *core.Ctx) error {
			if ctx.User == nil {
				if len(ctx.Args) < 1 {
					return core.ErrNotEnoughArgs
				}
				return loopOnline(ctx, true, true, ctx.Args[0], func(u *core.User) error { return heal(ctx, u) })
			}
			if !ctx.Has("essentials.heal.cooldown.bypass") {
				if err := healCooldown(ctx.User); err != nil {
					return err
				}
			}
			if len(ctx.Args) > 0 && ctx.Has("essentials.heal.others") {
				return loopOnline(ctx, true, true, ctx.Args[0], func(u *core.User) error { return heal(ctx, u) })
			}
			return heal(ctx, ctx.User)
		},
		Complete: othersComplete("essentials.heal.others"),
	}
}

// feedPlayer fills the food bar, unless a listener cancels the food level
// change: then it answers false. Essentials: Commandfeed.feedPlayer
func feedPlayer(e *core.Essentials, p plugin.Player) bool {
	ev := &plugin.FoodLevelChangeEvent{Player: p, Level: 30}
	e.Server.Events().Fire(ev)
	if ev.Cancelled() {
		return false
	}
	p.SetFood(min(ev.Level, 20))
	p.SetSaturation(10)
	p.SetExhaustion(0)
	return true
}

// Essentials: Commandfeed
func feedCmd() core.Cmd {
	other := func(ctx *core.Ctx, u *core.User) error {
		if p, ok := u.Player(); ok && feedPlayer(ctx.Ess, p) {
			ctx.Send("feedOther", u.DisplayName())
		}
		return nil
	}
	return core.Cmd{
		Name: "feed",
		Run: func(ctx *core.Ctx) error {
			if ctx.User == nil {
				if len(ctx.Args) < 1 {
					return core.ErrNotEnoughArgs
				}
				return loopOnline(ctx, true, true, ctx.Args[0], func(u *core.User) error { return other(ctx, u) })
			}
			if !ctx.Has("essentials.feed.cooldown.bypass") {
				if err := healCooldown(ctx.User); err != nil {
					return err
				}
			}
			if len(ctx.Args) > 0 && ctx.Has("essentials.feed.others") {
				return loopOnline(ctx, true, true, ctx.Args[0], func(u *core.User) error { return other(ctx, u) })
			}
			if !feedPlayer(ctx.Ess, ctx.Player) {
				return core.ErrQuiet
			}
			ctx.Send("feed")
			return nil
		},
		Complete: othersComplete("essentials.feed.others"),
	}
}

var gameModeNames = [...]string{plugin.Survival: "survival", plugin.Creative: "creative",
	plugin.Adventure: "adventure", plugin.Spectator: "spectator"}

// matchGameMode reads a mode or a mode alias; nil for toggle, an error for
// anything else. Essentials: Commandgamemode.matchGameMode
func matchGameMode(s string) (*plugin.GameMode, error) {
	s = strings.ToLower(s)
	in := func(words ...string) bool {
		for _, w := range words {
			if s == w {
				return true
			}
		}
		return false
	}
	var m plugin.GameMode
	switch {
	case in("gmc", "egmc", "1", "c") || strings.Contains(s, "creat"):
		m = plugin.Creative
	case in("gms", "egms", "0", "s") || strings.Contains(s, "survi"):
		m = plugin.Survival
	case in("gma", "egma", "2", "a") || strings.Contains(s, "advent"):
		m = plugin.Adventure
	case in("gmsp", "egmsp", "3", "sp") || strings.Contains(s, "spec"):
		m = plugin.Spectator
	case in("gmt", "egmt", "t") || strings.Contains(s, "toggle") || strings.Contains(s, "cycle"):
		return nil, nil
	default:
		return nil, core.ErrNotEnoughArgs
	}
	return &m, nil
}

// Essentials: Commandgamemode
func gamemodeCmd() core.Cmd {
	prohibited := func(u *core.User, to plugin.GameMode) bool {
		return u != nil && !u.HasPermission("essentials.gamemode.all") && !u.HasPermission("essentials.gamemode."+gameModeNames[to])
	}
	set := func(ctx *core.Ctx, mode *plugin.GameMode, u *core.User) error {
		if mode == nil {
			return usageWith(ctx, "gameModeInvalid")
		}
		if prohibited(ctx.User, *mode) {
			ctx.Send("cantGamemode", strings.ToUpper(gameModeNames[*mode]))
			return nil
		}
		if p, ok := u.Player(); ok {
			p.SetGameMode(*mode)
		}
		ctx.Send("gameMode", tl(ctx, gameModeNames[*mode]), u.DisplayName())
		return nil
	}
	loop := func(ctx *core.Ctx, modeArg, target string) error {
		mode, err := matchGameMode(modeArg)
		if err != nil {
			return err
		}
		return loopOnline(ctx, false, true, target, func(u *core.User) error { return set(ctx, mode, u) })
	}
	options := []string{"creative", "survival", "adventure", "spectator", "toggle"}
	return core.Cmd{
		Name: "gamemode",
		Run: func(ctx *core.Ctx) error {
			args := ctx.Args
			if ctx.User == nil {
				switch len(args) {
				case 0:
					return core.ErrNotEnoughArgs
				case 1:
					return loop(ctx, ctx.Label, args[0])
				case 2:
					return loop(ctx, args[0], args[1])
				}
				return nil
			}
			var mode *plugin.GameMode
			var err error
			switch {
			case len(args) == 0:
				if mode, err = matchGameMode(ctx.Label); err != nil {
					return err
				}
			case len(args) > 1 && len(strings.TrimSpace(args[1])) > 2 && ctx.Has("essentials.gamemode.others"):
				return loop(ctx, args[0], args[1])
			default:
				if mode, err = matchGameMode(args[0]); err != nil {
					if ctx.Has("essentials.gamemode.others") {
						return loop(ctx, ctx.Label, args[0])
					}
					return err
				}
			}
			if mode == nil {
				m := plugin.Survival
				switch ctx.Player.GameMode() {
				case plugin.Survival:
					m = plugin.Creative
				case plugin.Creative:
					m = plugin.Adventure
				}
				mode = &m
			}
			if prohibited(ctx.User, *mode) {
				ctx.Send("cantGamemode", tl(ctx, gameModeNames[*mode]))
				return nil
			}
			ctx.Player.SetGameMode(*mode)
			ctx.Send("gameMode", tl(ctx, gameModeNames[ctx.Player.GameMode()]), ctx.User.DisplayName())
			return nil
		},
		Complete: func(ctx *core.Ctx) []string {
			_, err := matchGameMode(ctx.Label)
			direct := err == nil
			others := ctx.User == nil || ctx.Has("essentials.gamemode.others")
			switch {
			case len(ctx.Args) == 1 && direct && others:
				return loopPlayers(ctx)
			case len(ctx.Args) == 1:
				return options
			case len(ctx.Args) == 2 && others && !direct:
				return loopPlayers(ctx)
			}
			return nil
		},
	}
}

// Essentials: Commandspeed
func speedCmd() core.Cmd {
	isFlyMode := func(s string) (bool, error) {
		switch {
		case strings.Contains(s, "fly") || strings.EqualFold(s, "f"):
			return true, nil
		case strings.Contains(s, "walk") || strings.Contains(s, "run") || strings.EqualFold(s, "w") || strings.EqualFold(s, "r"):
			return false, nil
		}
		return false, core.ErrNotEnoughArgs
	}
	moveSpeed := func(s string) (float64, error) {
		f, ok := parseFloat(s)
		if !ok {
			return 0, core.ErrNotEnoughArgs
		}
		return math.Max(0.0001, math.Min(f, 10)), nil
	}
	realSpeed := func(e *core.Essentials, speed float64, fly, bypass bool) float32 {
		def := 0.2
		maxSpeed := 1.0
		if fly {
			def = 0.1
		}
		if !bypass {
			maxSpeed = e.Settings.MaxWalkSpeed()
			if fly {
				maxSpeed = e.Settings.MaxFlySpeed()
			}
		}
		if speed < 1 {
			return float32(def * speed)
		}
		return float32((speed-1)/9*(maxSpeed-def) + def)
	}
	apply := func(ctx *core.Ctx, p plugin.Player, name string, fly, bypass bool, speed float64) {
		kind := "walking"
		if fly {
			kind = "flying"
			p.SetFlySpeed(realSpeed(ctx.Ess, speed, true, bypass))
		} else {
			p.SetWalkSpeed(realSpeed(ctx.Ess, speed, false, bypass))
		}
		ctx.Send("moveSpeed", tlRaw(ctx.Ess, ctx.Sender, kind), core.FormatDouble(speed), name)
	}
	others := func(ctx *core.Ctx, fly, bypass bool, speed float64, name string) error {
		found := false
		for _, p := range matchPlayers(ctx.Ess, name) {
			u := ctx.Ess.Users.Of(p)
			if hiddenFrom(ctx, u) {
				continue
			}
			found = true
			apply(ctx, p, u.DisplayName(), fly, bypass, speed)
		}
		if !found {
			return core.ErrPlayerNotFound
		}
		return nil
	}
	// flyPermCheck turns the mode to the one the user may set.
	flyPermCheck := func(u *core.User, fly bool) bool {
		canFly, canWalk := u.HasPermission("essentials.speed.fly"), u.HasPermission("essentials.speed.walk")
		if fly && canFly || !fly && canWalk || !canFly && !canWalk {
			return fly
		}
		return !canWalk
	}
	return core.Cmd{
		Name: "speed",
		Run: func(ctx *core.Ctx) error {
			args := ctx.Args
			if ctx.User == nil {
				if len(args) < 3 {
					return core.ErrNotEnoughArgs
				}
				fly, err := isFlyMode(args[0])
				if err != nil {
					return err
				}
				speed, err := moveSpeed(args[1])
				if err != nil {
					return err
				}
				return others(ctx, fly, true, speed, args[2])
			}
			if len(args) < 1 {
				return core.ErrNotEnoughArgs
			}
			bypass := ctx.Has("essentials.speed.bypass")
			var fly bool
			var speed float64
			var err error
			if len(args) == 1 {
				label := strings.ToLower(ctx.Label)
				inferred := ctx.Player.Flying()
				switch {
				case strings.Contains(label, "fly") || label == "fspeed" || label == "efspeed":
					inferred = true
				case strings.Contains(label, "walk") || label == "wspeed" || label == "ewspeed":
					inferred = false
				}
				fly = flyPermCheck(ctx.User, inferred)
				if speed, err = moveSpeed(args[0]); err != nil {
					return err
				}
			} else {
				mode, err := isFlyMode(args[0])
				if err != nil {
					return err
				}
				fly = flyPermCheck(ctx.User, mode)
				if speed, err = moveSpeed(args[1]); err != nil {
					return err
				}
				if len(args) > 2 && ctx.Has("essentials.speed.others") {
					if len(strings.TrimSpace(args[2])) < 2 {
						return core.ErrPlayerNotFound
					}
					return others(ctx, fly, bypass, speed, args[2])
				}
			}
			apply(ctx, ctx.Player, ctx.User.DisplayName(), fly, bypass, speed)
			return nil
		},
		Complete: func(ctx *core.Ctx) []string {
			switch {
			case len(ctx.Args) == 1:
				return []string{"walk", "fly", "1", "1.5", "1.75", "2"}
			case len(ctx.Args) == 2:
				return []string{"1", "1.5", "1.75", "2"}
			case len(ctx.Args) == 3 && ctx.Has("essentials.speed.others"):
				return ctx.CompletePlayers()
			}
			return nil
		},
	}
}

var expSubs = []string{"set", "give", "take", "reset", "show"}

// Essentials: Commandexp
func expCmd() core.Cmd {
	// hasPerm and hasOther are ExpCommands.hasPermission and hasOtherPermission.
	hasPerm := func(ctx *core.Ctx, sub string) bool {
		return ctx.User == nil || sub == "show" || ctx.Has("essentials.exp."+sub)
	}
	hasOther := func(ctx *core.Ctx, sub string) bool {
		return ctx.User == nil || ctx.Has("essentials.exp."+sub+".others")
	}
	show := func(ctx *core.Ctx, u *core.User) {
		p, ok := u.Player()
		if !ok {
			return
		}
		l := p.Level()
		toNext := core.ExpToNextLevel(l)
		until := toNext - int(math.Round(float64(toNext)*float64(p.Progress())))
		ctx.Send("exp", u.DisplayName(), core.TotalExp(p), l, until)
	}
	setExp := func(ctx *core.Ctx, u *core.User, amountArg string, give bool) error {
		p, ok := u.Player()
		if !ok {
			return nil
		}
		s := strings.ToLower(amountArg)
		var amount int64
		if strings.Contains(s, "l") {
			s = strings.ReplaceAll(s, "l", "")
			level, err := parseInt(s)
			if err != nil {
				return err
			}
			if give {
				level += p.Level()
			}
			amount = int64(core.ExpAtLevel(max(level, 0)))
			core.SetTotalExperience(p, 0)
		} else {
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return &core.TlError{Key: "errorWithMessage", Args: []any{"For input string: \"" + s + "\""}, Plain: true}
			}
			if n > math.MaxInt32 || n < math.MinInt32 {
				return core.ErrNotEnoughArgs
			}
			amount = n
		}
		if give {
			amount += int64(core.TotalExp(p))
		}
		amount = max(0, min(amount, math.MaxInt32))
		core.SetTotalExperience(p, int(amount))
		ctx.Send("expSet", u.DisplayName(), amount)
		return nil
	}
	return core.Cmd{
		Name: "exp",
		Run: func(ctx *core.Ctx) error {
			args := ctx.Args
			if len(args) == 0 || len(args) < 2 && ctx.User == nil {
				if ctx.User == nil {
					return core.ErrNotEnoughArgs
				}
				show(ctx, ctx.User)
				return nil
			}
			sub := strings.ToLower(args[0])
			known := false
			for _, s := range expSubs {
				known = known || s == sub
			}
			if !known {
				return core.ErrNotEnoughArgs
			}
			if !hasPerm(ctx, sub) {
				ctx.Send("noAccessSubCommand", "/"+ctx.Label+" "+sub)
				return nil
			}
			each := func(target, amount string, give bool) error {
				return loopOnline(ctx, false, true, target, func(u *core.User) error { return setExp(ctx, u, amount, give) })
			}
			switch sub {
			case "set", "give":
				give := sub == "give"
				switch {
				case len(args) == 3 && hasOther(ctx, sub):
					return each(args[1], args[2], give)
				case len(args) == 2 && ctx.User != nil:
					return setExp(ctx, ctx.User, args[1], give)
				}
			case "take":
				switch {
				case len(args) == 3 && hasOther(ctx, sub):
					return each(args[1], "-"+args[2], true)
				case len(args) == 2 && ctx.User != nil:
					return setExp(ctx, ctx.User, "-"+args[1], true)
				}
			case "reset":
				switch {
				case len(args) == 2 && hasOther(ctx, sub):
					return each(args[1], "0", false)
				case ctx.User != nil:
					return setExp(ctx, ctx.User, "0", false)
				}
			case "show":
				switch {
				case len(args) == 2 && (ctx.User == nil || ctx.Has("essentials.exp.others")):
					u, err := ctx.GetPlayer(1, false)
					if err != nil {
						return err
					}
					show(ctx, u)
					return nil
				case ctx.User != nil:
					show(ctx, ctx.User)
					return nil
				}
			}
			return core.ErrNotEnoughArgs
		},
		Complete: func(ctx *core.Ctx) []string {
			args := ctx.Args
			levels := func(a string) []string {
				n := strings.ReplaceAll(strings.ToLower(a), "l", "")
				if core.IsInt(n) {
					return []string{n + "l"}
				}
				return nil
			}
			if ctx.User == nil {
				switch {
				case len(args) == 1:
					return expSubs
				case len(args) == 2 && (strings.EqualFold(args[0], "set") || strings.EqualFold(args[0], "give")):
					n := strings.ReplaceAll(strings.ToLower(args[1]), "l", "")
					if core.IsInt(n) {
						return []string{n, args[1] + "l"}
					}
					return nil
				case len(args) == 2:
					return loopPlayers(ctx)
				case len(args) == 3 && (strings.EqualFold(args[0], "set") || strings.EqualFold(args[0], "give")):
					return loopPlayers(ctx)
				}
				return nil
			}
			switch len(args) {
			case 1:
				out := []string{"show"}
				for _, s := range expSubs {
					if hasPerm(ctx, s) {
						out = append(out, s)
					}
				}
				return out
			case 2:
				sub := strings.ToLower(args[0])
				known := false
				for _, s := range expSubs {
					known = known || s == sub
				}
				if !known {
					return nil
				}
				if hasOther(ctx, sub) {
					return loopPlayers(ctx)
				}
				if hasPerm(ctx, sub) && sub != "show" && sub != "reset" {
					return levels(args[1])
				}
			case 3:
				if !strings.EqualFold(args[0], "show") && !strings.EqualFold(args[0], "reset") {
					return levels(args[2])
				}
			}
			return nil
		},
	}
}

// Essentials: Commandburn
func burnCmd() core.Cmd {
	return core.Cmd{
		Name: "burn",
		Run: func(ctx *core.Ctx) error {
			if len(ctx.Args) < 2 {
				return core.ErrNotEnoughArgs
			}
			u, err := ctx.GetPlayer(0, false)
			if err != nil {
				return err
			}
			secs, err := parseInt(ctx.Args[1])
			if err != nil {
				return err
			}
			if p, ok := u.Player(); ok {
				p.SetFireTicks(int(max(0, min(int64(secs)*20, math.MaxInt32))))
			}
			ctx.Send("burnMsg", u.DisplayName(), max(0, secs))
			return nil
		},
		Complete: func(ctx *core.Ctx) []string {
			switch len(ctx.Args) {
			case 1:
				return ctx.CompletePlayers()
			case 2:
				return []string{"1", "60", "600", "3600", "86400"}
			}
			return nil
		},
	}
}

// killDamage fires the damage event Essentials fires before /kill and
// /suicide set the health to zero, of Float.MAX_VALUE. Its cause is SUICIDE
// for oneself and CUSTOM for another; the API's cause is the damage type
// (generic_kill for SUICIDE, generic for CUSTOM), so it stands in for
// Essentials' MAGIC source. Essentials: ModernDamageEventProvider.callDamageEvent
func killDamage(e *core.Essentials, p plugin.Player, suicide bool) *plugin.EntityDamageEvent {
	cause := "generic"
	if suicide {
		cause = "generic_kill"
	}
	ev := &plugin.EntityDamageEvent{Entity: p, Amount: math.MaxFloat32, Final: math.MaxFloat32}
	if t, ok := e.Server.Registries().DamageType(plugin.MinecraftKey(cause)); ok {
		ev.Source = plugin.DamageSource{Type: t}
	}
	e.Server.Events().Fire(ev)
	return ev
}

// Essentials: Commandkill
func killCmd() core.Cmd {
	return core.Cmd{
		Name: "kill",
		Run: func(ctx *core.Ctx) error {
			if len(ctx.Args) == 0 {
				return core.ErrNotEnoughArgs
			}
			force := ctx.Has("essentials.kill.force")
			return loopOnline(ctx, true, true, ctx.Args[0], func(u *core.User) error {
				p, ok := u.Player()
				if !ok {
					return nil
				}
				if ctx.User != nil && u.HasPermission("essentials.kill.exempt") && !force {
					return core.Tlf("killExempt", u.DisplayName())
				}
				self := ctx.Player != nil && ctx.Player.Name() == p.Name()
				if killDamage(ctx.Ess, p, self).Cancelled() && ctx.User != nil && !force {
					return nil
				}
				p.SetHealth(0) // no damage event, as Bukkit's setHealth(0)
				ctx.Send("kill", u.DisplayName())
				return nil
			})
		},
		Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 {
				return loopPlayers(ctx)
			}
			return nil
		},
	}
}

// Essentials: Commandsuicide
func suicideCmd() core.Cmd {
	return core.Cmd{
		Name:       "suicide",
		PlayerOnly: true,
		Run: func(ctx *core.Ctx) error {
			killDamage(ctx.Ess, ctx.Player, true) // fired for listeners; a cancel changes nothing
			ctx.Player.SetHealth(0)
			ctx.Send("suicideMessage")
			ctx.User.UpdateDisplayName()
			ctx.Ess.Broadcast(ctx.User, "", "suicideSuccess", ctx.User.DisplayName())
			return nil
		},
		Complete: func(*core.Ctx) []string { return nil },
	}
}

// selfOrOthers runs a command that acts on the sender, or with perm on the
// players the first argument matches. Essentials: Commandrest, Commandice
func selfOrOthers(name, perm, otherKey string, act func(ctx *core.Ctx, u *core.User, p plugin.Player)) core.Cmd {
	return core.Cmd{
		Name: name,
		Run: func(ctx *core.Ctx) error {
			if len(ctx.Args) == 0 && ctx.User == nil {
				return core.ErrNotEnoughArgs
			}
			if len(ctx.Args) > 0 && ctx.Has(perm) {
				return loopOnline(ctx, false, true, ctx.Args[0], func(u *core.User) error {
					if p, ok := u.Player(); ok {
						act(ctx, u, p)
						ctx.Send(otherKey, u.DisplayName())
					}
					return nil
				})
			}
			act(ctx, ctx.User, ctx.Player)
			return nil
		},
		Complete: othersComplete(perm),
	}
}

// Essentials: Commandrest
func restCmd() core.Cmd {
	return selfOrOthers("rest", "essentials.rest.others", "restOther", func(ctx *core.Ctx, u *core.User, p plugin.Player) {
		if s, ok := ctx.Ess.Server.Registries().Statistic(plugin.MinecraftKey("time_since_rest")); ok {
			p.SetStatistic(s, 0)
		}
		u.Send("rest")
	})
}

// maxFreezeTicks is the ticks in powder snow that freeze an entity fully.
// vanilla: Entity.getTicksRequiredToFreeze
const maxFreezeTicks = 140

// Essentials: Commandice
func iceCmd() core.Cmd {
	return selfOrOthers("ice", "essentials.ice.others", "iceOther", func(_ *core.Ctx, u *core.User, p plugin.Player) {
		p.SetFrozenTicks(maxFreezeTicks)
		u.Send("ice")
	})
}
