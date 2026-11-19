package items

import (
	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// seeKind is which of these commands' windows a view shows, told apart by
// its top inventory as Essentials does.
type seeKind uint8

const (
	seeNone  seeKind = iota
	seeInv           // a player's inventory (/invsee)
	seeEquip         // a 9-slot chest a player holds (/invsee <player> equip)
	seeEnder         // an ender chest (/enderchest)
	seeTable         // a crafting table (/recipe while RecipeSee)
)

// craftingTableSize is a crafting table's top: the result and the 3x3
// grid. A player's own 2x2 grid also reports InvCrafting, with 5 slots
// (Bukkit tells them apart as WORKBENCH and CRAFTING).
const craftingTableSize = 10

// seeKindOf answers what the view's top inventory is to these commands,
// and the top. Essentials: EssentialsPlayerListener's InventoryType
// switches
func seeKindOf(v plugin.InventoryView) (seeKind, plugin.Inventory) {
	if v == nil {
		return seeNone, nil
	}
	top := v.Top()
	switch top.Type() {
	case plugin.InvPlayer:
		return seeInv, top
	case plugin.InvEnderChest:
		return seeEnder, top
	case plugin.InvCrafting:
		if top.Size() == craftingTableSize {
			return seeTable, top
		}
	case plugin.InvChest:
		if _, ok := top.Holder().(plugin.Player); ok && top.Size() == 9 {
			return seeEquip, top
		}
	}
	return seeNone, top
}

// javaHandledTop reports whether Essentials' click listener takes a branch
// of its own for a view with this top (PLAYER, ENDER_CHEST, WORKBENCH, or a
// 9-slot CHEST), and so does not try the direct hat.
// Essentials: EssentialsPlayerListener.onInventoryClickEvent
func javaHandledTop(top plugin.Inventory) bool {
	switch top.Type() {
	case plugin.InvPlayer, plugin.InvEnderChest:
		return true
	case plugin.InvCrafting:
		return top.Size() == craftingTableSize
	case plugin.InvChest:
		return top.Size() == 9
	}
	return false
}

// readOnly reports whether a click or drag in a window of kind must be
// cancelled for p. Essentials: EssentialsPlayerListener.onInventoryClickEvent
func readOnly(e *core.Essentials, p plugin.Player, kind seeKind, top plugin.Inventory) bool {
	u := e.Users.Of(p)
	switch kind {
	case seeInv:
		owner, ok := top.Holder().(plugin.Player)
		if !ok || !u.InvSee {
			return false
		}
		_, online := e.Server.Player(owner.UUID())
		return !p.HasPermission("essentials.invsee.modify") || owner.HasPermission("essentials.invsee.preventmodify") || !online
	case seeEnder:
		return u.EnderSee && !p.HasPermission("essentials.enderchest.modify")
	case seeTable:
		return u.RecipeSee
	case seeEquip:
		return u.InvSee
	}
	return false
}

// listenSeeing enforces the modify permissions of the /invsee and
// /enderchest windows and keeps /recipe's read-only. Essentials:
// EssentialsPlayerListener onInventoryClickEvent, onInventoryDragEvent,
// onInventoryCloseEvent. Essentials' onPlayerQuit also empties a recipe
// table, but the server closes a leaving player's window with its close
// event first, which already did.
func listenSeeing(e *core.Essentials) {
	low := plugin.ListenOptions{Priority: plugin.Lowest, IgnoreCancelled: true}
	plugin.Listen(e.Server, func(ev *plugin.InventoryClickEvent) {
		kind, top := seeKindOf(ev.View)
		if kind == seeEquip && ev.Click == plugin.ClickMiddle {
			return
		}
		if readOnly(e, ev.Player, kind, top) {
			ev.SetCancelled(true)
		}
	}, low)
	plugin.Listen(e.Server, func(ev *plugin.InventoryDragEvent) {
		kind, top := seeKindOf(ev.View)
		if kind == seeNone {
			return
		}
		// Essentials lets drags through; the port keeps a read-only window
		// read-only for them too.
		if readOnly(e, ev.Player, kind, top) {
			ev.SetCancelled(true)
			return
		}
		if kind != seeInv || !e.Users.Of(ev.Player).InvSee {
			return
		}
		// Bukkit: Inventories.isBottomInventorySlot, the armour, off hand
		// and the viewer's own slots.
		for slot := range ev.Slots {
			if slot > 35 {
				ev.SetCancelled(true)
				return
			}
		}
	}, low)
	plugin.Listen(e.Server, func(ev *plugin.InventoryCloseEvent) {
		closeSeeing(e, ev.Player, ev.View)
	}, plugin.ListenOptions{Priority: plugin.Monitor})
}

// closeSeeing clears the viewer's flag for the window; a recipe's crafting
// grid is emptied so the shown ingredients are not handed to the viewer.
// Essentials: EssentialsPlayerListener.onInventoryCloseEvent
func closeSeeing(e *core.Essentials, p plugin.Player, v plugin.InventoryView) {
	u := e.Users.Of(p)
	switch kind, top := seeKindOf(v); kind {
	case seeInv, seeEquip:
		u.InvSee = false
	case seeEnder:
		u.EnderSee = false
	case seeTable:
		if u.RecipeSee {
			u.RecipeSee = false
			top.Clear()
		}
	}
}

// Essentials: Commandinvsee
func runInvsee(ctx *core.Ctx) error {
	e := ctx.Ess
	if len(ctx.Args) < 1 {
		return core.ErrNotEnoughArgs
	}
	target, err := ctx.GetPlayer(0, false)
	if err != nil {
		return err
	}
	if target == ctx.User {
		ctx.Send("invseeNoSelf")
		return core.ErrNoCharge
	}
	tp, ok := target.Player()
	if !ok {
		return core.ErrPlayerNotFound
	}
	inv := plugin.Inventory(tp.Inventory())
	if len(ctx.Args) > 1 && ctx.Has("essentials.invsee.equip") {
		// A copy, as in Essentials: the armour (boots first) and the off hand.
		inv = e.Server.CreateInventory(plugin.InvChest, 1, e.TlFor(ctx.Sender, "equipped"), tp)
		owner := tp.Inventory()
		for i := range 4 {
			inv.SetItem(i, owner.Item(slotFeet+i))
		}
		inv.SetItem(4, owner.Item(slotOffhand))
	}
	ctx.Player.CloseInventory()
	if ctx.Player.OpenInventory(inv) != nil {
		ctx.User.InvSee = true
	}
	return nil
}

// Essentials: Commandenderchest
func runEnderchest(ctx *core.Ctx) error {
	target := ctx.User
	if len(ctx.Args) > 0 && ctx.Has("essentials.enderchest.others") {
		u, err := ctx.GetPlayer(0, false)
		if err != nil {
			return err
		}
		target = u
	}
	tp, ok := target.Player()
	if !ok {
		return core.ErrPlayerNotFound
	}
	ctx.Player.CloseInventory()
	if ctx.Player.OpenInventory(tp.EnderChest()) != nil {
		ctx.User.EnderSee = target != ctx.User
	}
	return nil
}

// recipeWindow shows a recipe's grid in a crafting table the viewer cannot
// change; the table works out the result, and closing it empties the grid.
// Essentials: Commandrecipe shapedRecipe/shapelessRecipe with showWindow,
// openWorkbench
func recipeWindow(ctx *core.Ctx, grid [3][3]registry.Item) {
	e, p := ctx.Ess, ctx.Player
	p.CloseInventory()
	ctx.User.RecipeSee = true
	table := e.Server.CreateInventory(plugin.InvCrafting, 0, plugin.Translate("container.crafting"), nil)
	var v plugin.InventoryView
	if table != nil {
		v = p.OpenInventory(table)
	}
	// The items must not land in another window, from which they could be
	// taken.
	kind, top := seeKindOf(v)
	if kind != seeTable {
		ctx.User.RecipeSee = false
		return
	}
	for j := range 3 {
		for k := range 3 {
			if it := grid[j][k]; it != registry.ItemAir {
				top.SetItem(1+j*3+k, plugin.NewStack(it, 1))
			}
		}
	}
}
