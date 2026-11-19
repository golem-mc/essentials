package items

import (
	"slices"
	"strconv"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

func registerGive(e *core.Essentials) {
	e.RegisterCommand(
		core.Cmd{Name: "give", Run: runGive, Complete: func(ctx *core.Ctx) []string {
			switch len(ctx.Args) {
			case 1:
				return loopPlayers(ctx)
			case 2:
				return itemNames(ctx.Ess)
			case 3:
				return []string{"1", "64"}
			case 4:
				return []string{"0"}
			}
			return nil
		}},
		core.Cmd{Name: "item", PlayerOnly: true, Run: runItem, Complete: func(ctx *core.Ctx) []string {
			switch len(ctx.Args) {
			case 1:
				return itemNames(ctx.Ess)
			case 2:
				return []string{"1", "64"}
			case 3:
				return []string{"0"}
			}
			return nil
		}},
		core.Cmd{Name: "itemdb", Run: runItemdb, Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 {
				return itemNames(ctx.Ess)
			}
			return nil
		}},
		core.Cmd{Name: "more", PlayerOnly: true, Run: runMore, Complete: func(*core.Ctx) []string { return nil }},
	)
}

// spawnAmount sets the stack's count from the amount argument, else
// default-stack-size, else oversized-stacksize with
// essentials.oversizedstacks; a word that is no number shows the usage.
func spawnAmount(ctx *core.Ctx, s *plugin.ItemStack, arg string, hasArg bool) error {
	e := ctx.Ess
	if hasArg {
		n, err := strconv.Atoi(arg)
		if err != nil {
			return core.ErrNotEnoughArgs
		}
		if n > 0 {
			s.Count = n
			return nil
		}
	}
	switch {
	case e.Settings.DefaultStackSize() > 0:
		s.Count = e.Settings.DefaultStackSize()
	case e.Settings.OversizedStackSize() > 0 && ctx.Has("essentials.oversizedstacks"):
		s.Count = e.Settings.OversizedStackSize()
	}
	return nil
}

// spawnStack resolves an item name to the stack /give and /item hand out,
// at the item's max stack size. Essentials: ItemDb.get
func spawnStack(ctx *core.Ctx, name string) (plugin.ItemStack, error) {
	e := ctx.Ess
	s, err := e.Items.Get(name)
	if err != nil {
		return s, err
	}
	s.Count = max(e.Server.Items().MaxStackSize(s), 1)
	if !canSpawnItem(e, ctx.User, s.Item) {
		return s, core.Tlf("cantSpawnItem", flatName(s.Item))
	}
	return s, nil
}

func allowUnsafe(ctx *core.Ctx) bool {
	return ctx.Ess.Settings.UnsafeEnchantments() && ctx.Has("essentials.enchantments.allowunsafe")
}

// Essentials: Commandgive
func runGive(ctx *core.Ctx) error {
	e := ctx.Ess
	if len(ctx.Args) < 2 {
		return core.ErrNotEnoughArgs
	}
	s, err := spawnStack(ctx, ctx.Args[1])
	if err != nil {
		return err
	}
	if err := spawnAmount(ctx, &s, ctx.Arg(2), len(ctx.Args) > 2); err != nil {
		return err
	}
	if s.Item == registry.ItemAir {
		return core.Tlf("unableToSpawnItem", flatName(s.Item))
	}
	if len(ctx.Args) > 3 {
		start := 3
		if core.IsInt(ctx.Args[3]) {
			start = 4
		}
		if len(ctx.Args) > start {
			m := e.NewMetaItem(s, ctx.User)
			if err := m.Parse(ctx.Args, start, allowUnsafe(ctx)); err != nil {
				return err
			}
			s = m.Stack
		}
	}
	if s.Empty() {
		return core.Tlf("cantSpawnItem", "Air")
	}
	name := spacedName(s.Item)
	drop := e.Settings.DropItemsIfFull()
	targets, err := onlineTargets(ctx, ctx.Args[0])
	if err != nil {
		return err
	}
	for _, u := range targets {
		p, ok := u.Player()
		if !ok {
			continue
		}
		ctx.Send("giveSpawn", s.Count, name, u.DisplayName())
		for _, left := range e.GiveItems(p, false, s) {
			if drop {
				p.World().DropItem(p.Pos(), left, true)
			} else {
				ctx.Send("giveSpawnFailure", left.Count, name, u.DisplayName())
			}
		}
	}
	return nil
}

// Essentials: Commanditem
func runItem(ctx *core.Ctx) error {
	e := ctx.Ess
	if len(ctx.Args) == 0 {
		return core.ErrNotEnoughArgs
	}
	s, err := spawnStack(ctx, ctx.Args[0])
	if err != nil {
		return err
	}
	if err := spawnAmount(ctx, &s, ctx.Arg(1), len(ctx.Args) > 1); err != nil {
		return err
	}
	if s.Item == registry.ItemAir {
		return core.Tlf("unableToSpawnItem", flatName(s.Item))
	}
	if len(ctx.Args) > 2 {
		m := e.NewMetaItem(s, ctx.User)
		if err := m.Parse(ctx.Args, 2, allowUnsafe(ctx)); err != nil {
			return err
		}
		s = m.Stack
	}
	if s.Empty() {
		return core.Tlf("cantSpawnItem", "Air")
	}
	ctx.Send("itemSpawn", s.Count, spacedName(s.Item))
	e.GiveItems(ctx.Player, false, s)
	return nil
}

// Essentials: Commanditemdb
func runItemdb(ctx *core.Ctx) error {
	e := ctx.Ess
	var s plugin.ItemStack
	held := false
	if len(ctx.Args) == 0 {
		if ctx.Player == nil {
			return core.ErrNotEnoughArgs
		}
		held = true
		_, s = heldSlot(ctx.Player)
	} else {
		var err error
		if s, err = e.Items.Get(ctx.Args[0]); err != nil {
			return err
		}
	}
	ctx.Send("itemType", strings.ToUpper(typeName(s.Item)), "none")
	if held && !s.Empty() {
		if uses := maxDurability(s); uses != 0 {
			ctx.Send("durability", strconv.Itoa(uses+1-damage(s)))
		}
	}
	names := append([]string(nil), e.Items.Names(s.Item)...)
	if len(names) == 0 {
		return nil
	}
	slices.Sort(names)
	if len(names) > 15 {
		names = names[:14]
	}
	ctx.Send("itemNames", joinList(names))
	return nil
}

// Essentials: Commandmore
func runMore(ctx *core.Ctx) error {
	e := ctx.Ess
	slot, s := heldSlot(ctx.Player)
	if s.Empty() {
		return core.Tlf("cantSpawnItem", "Air")
	}
	oversized := ctx.Has("essentials.oversizedstacks")
	limit := e.Server.Items().MaxStackSize(s)
	if oversized {
		limit = e.Settings.OversizedStackSize()
	}
	if s.Count >= limit {
		return core.Tlf("fullStack")
	}
	if !canSpawnItem(e, ctx.User, s.Item) {
		return core.Tlf("cantSpawnItem", flatName(s.Item))
	}
	n := limit
	if len(ctx.Args) >= 1 {
		if !core.IsPositiveInt(ctx.Args[0]) {
			return core.Tlf("nonZeroPosNumber")
		}
		add, _ := strconv.Atoi(ctx.Args[0])
		if int64(s.Count)+int64(add) > int64(limit) {
			key := "fullStackDefault"
			if oversized {
				key = "fullStackDefaultOversize"
			}
			ctx.Send(key, limit)
		} else {
			n = s.Count + add
		}
	}
	s.Count = n
	ctx.Player.Inventory().Set(slot, s)
	return nil
}
