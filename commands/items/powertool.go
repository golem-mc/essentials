package items

import (
	"fmt"
	"slices"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

func registerPowertools(e *core.Essentials) {
	e.RegisterCommand(
		core.Cmd{Name: "powertool", Run: runPowertool, Complete: completePowertool},
		core.Cmd{Name: "powertoollist", PlayerOnly: true, Run: func(ctx *core.Ctx) error {
			tools := ctx.User.Data.Powertools
			if len(tools) == 0 {
				ctx.Send("noPowerTools")
				return nil
			}
			for _, item := range slices.Sorted(mapKeys(tools)) {
				ctx.Send("powerToolList", joinList(tools[item]), strings.ReplaceAll(strings.ToLower(item), "_", " "))
			}
			return nil
		}, Complete: func(*core.Ctx) []string { return nil }},
		core.Cmd{Name: "powertooltoggle", PlayerOnly: true, Run: func(ctx *core.Ctx) error {
			u := ctx.User
			if len(u.Data.Powertools) == 0 {
				ctx.Send("noPowerTools")
				return nil
			}
			u.Data.PowertoolsEnabled = !u.Data.PowertoolsEnabled
			u.Save()
			if u.Data.PowertoolsEnabled {
				ctx.Send("powerToolsEnabled")
			} else {
				ctx.Send("powerToolsDisabled")
			}
			return nil
		}, Complete: func(*core.Ctx) []string { return nil }},
		core.Cmd{Name: "unlimited", PlayerOnly: true, Run: runUnlimited, Complete: func(ctx *core.Ctx) []string {
			switch len(ctx.Args) {
			case 1:
				return append([]string{"list", "clear"}, itemNames(ctx.Ess)...)
			case 2:
				if ctx.Has("essentials.unlimited.others") {
					return ctx.CompletePlayers()
				}
			}
			return nil
		}},
	)
	listenPowertools(e)
	listenUnlimited(e)
}

func mapKeys[V any](m map[string]V) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// Essentials: Commandpowertool
func runPowertool(ctx *core.Ctx) error {
	e := ctx.Ess
	if ctx.User != nil {
		_, s := heldSlot(ctx.Player)
		return powertool(ctx, ctx.User, s, ctx.FinalArg(0))
	}
	if len(ctx.Args) < 3 {
		return fmt.Errorf("When running from console, usage is: /%s <player> <itemid> <command>", ctx.Label)
	}
	u, err := e.MatchUser(nil, ctx.Args[0], true, true)
	if err != nil {
		return err
	}
	s, err := e.Items.Get(ctx.Args[1])
	if err != nil {
		return err
	}
	return powertool(ctx, u, s, ctx.FinalArg(2))
}

func powertool(ctx *core.Ctx, u *core.User, s plugin.ItemStack, command string) error {
	if strings.EqualFold(command, "d:") {
		u.Data.Powertools = nil
		u.Save()
		ctx.Send("powerToolClearAll")
		return nil
	}
	if s.Empty() {
		return core.Tlf("powerToolAir")
	}
	itemName := spacedName(s.Item)
	key := typeName(s.Item)
	tools := slices.Clone(u.Data.Powertools[key])
	if command != "" {
		if strings.EqualFold(command, "l:") {
			if len(tools) == 0 {
				return core.Tlf("powerToolListEmpty", itemName)
			}
			ctx.Send("powerToolList", joinList(tools), itemName)
			return core.ErrNoCharge
		}
		if after, ok := strings.CutPrefix(command, "r:"); ok {
			command = after
			i := slices.Index(tools, command)
			if i < 0 {
				return core.Tlf("powerToolNoSuchCommandAssigned", command, itemName)
			}
			tools = slices.Delete(tools, i, i+1)
			ctx.Send("powerToolRemove", command, itemName)
		} else {
			if after, ok := strings.CutPrefix(command, "a:"); ok {
				if !ctx.Has("essentials.powertool.append") {
					return core.Tlf("noPerm", "essentials.powertool.append")
				}
				command = after
				if slices.Contains(tools, command) {
					return core.Tlf("powerToolAlreadySet", command, itemName)
				}
			} else {
				tools = nil
			}
			tools = append(tools, command)
			ctx.Send("powerToolAttach", joinList(tools), itemName)
		}
	} else {
		tools = nil
		ctx.Send("powerToolRemoveAll", itemName)
	}
	if !u.Data.PowertoolsEnabled {
		u.Data.PowertoolsEnabled = true
		u.Send("powerToolsEnabled")
	}
	if len(tools) == 0 {
		delete(u.Data.Powertools, key)
	} else {
		if u.Data.Powertools == nil {
			u.Data.Powertools = map[string][]string{}
		}
		u.Data.Powertools[key] = tools
	}
	u.Save()
	return nil
}

func completePowertool(ctx *core.Ctx) []string {
	e := ctx.Ess
	first := 0
	if ctx.User == nil {
		switch len(ctx.Args) {
		case 1:
			return ctx.CompletePlayers()
		case 2:
			return itemNames(e)
		}
		first = 2
	}
	if len(ctx.Args) != first+1 {
		return nil
	}
	opts := []string{"d:", "c:", "l:"}
	if ctx.Has("essentials.powertool.append") {
		for _, c := range commandNames(e) {
			opts = append(opts, "a:"+c)
		}
	}
	var u *core.User
	var s plugin.ItemStack
	if ctx.User != nil {
		u = ctx.User
		_, s = heldSlot(ctx.Player)
	} else {
		var err error
		if u, err = e.MatchUser(nil, ctx.Args[0], true, true); err != nil {
			return opts
		}
		if s, err = e.Items.Get(ctx.Args[1]); err != nil {
			return opts
		}
	}
	if !s.Empty() {
		for _, t := range u.Data.Powertools[typeName(s.Item)] {
			opts = append(opts, "r:"+t)
		}
	}
	return opts
}

// listenPowertools runs the commands bound to the item a player
// left-clicks with, and those with {player} on the player they hit.
// Essentials: EssentialsPlayerListener.usePowertools,
// EssentialsEntityListener.onPlayerVsPlayerPowertool
func listenPowertools(e *core.Essentials) {
	plugin.Listen(e.Server, func(ev *plugin.PlayerInteractEvent) {
		if ev.Action != plugin.LeftClickAir && ev.Action != plugin.LeftClickBlock || ev.Item.Empty() {
			return
		}
		u := e.Users.Of(ev.Player)
		if len(u.Data.Powertools) > 0 && u.Data.PowertoolsEnabled && usePowertools(e, u, ev.Player, ev.Item.Item) {
			ev.SetCancelled(true)
		}
	})
	plugin.Listen(e.Server, func(ev *plugin.PlayerAttackEvent) {
		defender, ok := ev.Target.(plugin.Player)
		if !ok {
			return
		}
		u := e.Users.Of(ev.Player)
		if !u.Data.PowertoolsEnabled {
			return
		}
		_, held := heldSlot(ev.Player)
		if held.Empty() {
			return
		}
		for _, tmpl := range u.Data.Powertools[typeName(held.Item)] {
			command := strings.ReplaceAll(tmpl, "{player}", defender.Name())
			if command == "" || command == tmpl {
				continue
			}
			runLater(e, ev.Player, command)
			ev.SetCancelled(true)
			return
		}
	}, plugin.ListenOptions{Priority: plugin.Low, IgnoreCancelled: true})
}

func usePowertools(e *core.Essentials, u *core.User, p plugin.Player, it registry.Item) bool {
	used := false
	for _, command := range u.Data.Powertools[typeName(it)] {
		switch {
		case strings.Contains(command, "{player}"):
		case strings.HasPrefix(command, "c:"):
			used = true
			p.Chat(command[2:])
		default:
			used = true
			runLater(e, p, command)
		}
	}
	return used
}

// runLater runs a powertool command as the player on the next tick.
func runLater(e *core.Essentials, p plugin.Player, command string) {
	e.Server.Scheduler().RunTask(func() {
		if !p.IsValid() {
			return
		}
		p.Chat("/" + command)
		e.Log.Info(fmt.Sprintf("[PT] %s issued server command: /%s", p.Name(), command))
	})
}

// hasUnlimited reports whether the user has unlimited of the item; the
// data keeps Bukkit's Material names.
func hasUnlimited(u *core.User, it registry.Item) bool {
	return slices.ContainsFunc(u.Data.Unlimited, func(m string) bool { return strings.EqualFold(m, typeName(it)) })
}

func setUnlimited(u *core.User, it registry.Item, on bool) {
	u.Data.Unlimited = slices.DeleteFunc(slices.Clone(u.Data.Unlimited), func(m string) bool { return strings.EqualFold(m, typeName(it)) })
	if on {
		u.Data.Unlimited = append(u.Data.Unlimited, strings.ToUpper(typeName(it)))
	}
	u.Save()
}

// Essentials: Commandunlimited
func runUnlimited(ctx *core.Ctx) error {
	e := ctx.Ess
	if len(ctx.Args) == 0 {
		return core.ErrNotEnoughArgs
	}
	target := ctx.User
	if len(ctx.Args) > 1 && ctx.Has("essentials.unlimited.others") {
		u, err := ctx.GetPlayer(1, false)
		if err != nil {
			return err
		}
		target = u
	}
	switch strings.ToLower(ctx.Args[0]) {
	case "list":
		out := tlRaw(e, ctx.Sender, "unlimitedItems") + " "
		if len(target.Data.Unlimited) == 0 {
			out += tlRaw(e, ctx.Sender, "none")
		}
		var names []string
		for _, m := range target.Data.Unlimited {
			names = append(names, strings.ReplaceAll(strings.ToLower(m), "_", ""))
		}
		ctx.SendText(core.ParseMini(out+core.EscapeTags(strings.Join(names, ", ")), e.Theme()))
	case "clear":
		for _, m := range slices.Clone(target.Data.Unlimited) {
			if err := toggleUnlimited(ctx, target, m); err != nil {
				return err
			}
		}
	default:
		return toggleUnlimited(ctx, target, ctx.Args[0])
	}
	return nil
}

func toggleUnlimited(ctx *core.Ctx, target *core.User, item string) error {
	e := ctx.Ess
	s, err := e.Items.Get(item)
	if err != nil {
		return err
	}
	s.Count = min(e.Server.Items().MaxStackSize(s), 2)
	name := flatName(s.Item)
	bucket := (s.Item == registry.ItemWaterBucket || s.Item == registry.ItemLavaBucket) && ctx.Has("essentials.unlimited.item-bucket")
	if e.Settings.PermissionBasedItemSpawn() && !ctx.Has("essentials.unlimited.item-all") &&
		!ctx.Has("essentials.unlimited.item-"+name) && !bucket {
		return core.Tlf("unlimitedItemPermission", name)
	}
	key := "disableUnlimited"
	enable := false
	if !hasUnlimited(target, s.Item) {
		key, enable = "enableUnlimited", true
		if p, ok := target.Player(); ok && countSimilar(e, p, s) < s.Count {
			e.GiveItems(p, false, s)
		}
	}
	if target != ctx.User {
		ctx.Send(key, name, target.DisplayName())
	}
	target.Send(key, name, target.DisplayName())
	setUnlimited(target, s.Item, enable)
	return nil
}

// countSimilar counts the stacks similar to s in the 36 storage slots.
func countSimilar(e *core.Essentials, p plugin.Player, s plugin.ItemStack) int {
	n := 0
	for _, c := range p.Inventory().Contents() {
		if !c.Empty() && e.Server.Items().Similar(c, s) {
			n += c.Count
		}
	}
	return n
}

// listenUnlimited gives back a placed block, a thrown egg or an emptied
// bucket the player has unlimited of. Essentials:
// EssentialsBlockListener.onBlockPlace, EssentialsPlayerListener
// onPlayerEggThrow and onPlayerBucketEmpty
func listenUnlimited(e *core.Essentials) {
	plugin.Listen(e.Server, func(ev *plugin.BlockPlaceEvent) {
		u := e.Users.Of(ev.Player)
		s := ev.Item
		if s.Empty() || !hasUnlimited(u, s.Item) || ev.Player.GameMode() != plugin.Survival {
			return
		}
		p := ev.Player
		e.Server.Scheduler().RunTask(func() {
			if p.IsValid() {
				one := s
				one.Count = 1
				e.GiveItems(p, false, one)
			}
		})
	}, plugin.ListenOptions{Priority: plugin.Low, IgnoreCancelled: true})
	plugin.Listen(e.Server, func(ev *plugin.PlayerEggThrowEvent) {
		if hasUnlimited(e.Users.Of(ev.Player), registry.ItemEgg) {
			e.GiveItems(ev.Player, false, plugin.NewStack(registry.ItemEgg, 1))
		}
	}, plugin.ListenOptions{Priority: plugin.High})
	plugin.Listen(e.Server, func(ev *plugin.PlayerBucketEvent) {
		if ev.Fill || ev.Item.Empty() || !hasUnlimited(e.Users.Of(ev.Player), ev.Item.Item) {
			return
		}
		ev.Result = ev.Item
	}, plugin.ListenOptions{Priority: plugin.High, IgnoreCancelled: true})
}
