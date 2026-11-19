package items_test

import (
	"testing"

	"github.com/golem-mc/essentials/commands/items"
	"github.com/golem-mc/essentials/internal/esstest"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

func init() { esstest.RegisterCore(items.Register) }

// find answers the first stack of item in the player's storage.
func find(env *esstest.Env, name string, it registry.Item) (plugin.ItemStack, bool) {
	for _, s := range env.Player(name).Inventory().Contents() {
		if s.Item == it && !s.Empty() {
			return s, true
		}
	}
	return plugin.ItemStack{}, false
}

func count(env *esstest.Env, name string, it registry.Item) int {
	return env.Player(name).Inventory().Count(it)
}

// clicks records the window events of c's player, as the server fired them
// for c's real clicks and drags, with every listener's verdict in.
type clicks struct {
	env   *esstest.Env
	c     *esstest.Client
	click *plugin.InventoryClickEvent
	drag  *plugin.InventoryDragEvent
}

func recordClicks(env *esstest.Env, c *esstest.Client) *clicks {
	r := &clicks{env: env, c: c}
	id := c.Handle().UUID()
	monitor := plugin.ListenOptions{Priority: plugin.Monitor}
	plugin.Listen(env.Ess.Server, func(ev *plugin.InventoryClickEvent) {
		if ev.Player.UUID() == id {
			r.click = ev
		}
	}, monitor)
	plugin.Listen(env.Ess.Server, func(ev *plugin.InventoryDragEvent) {
		if ev.Player.UUID() == id {
			r.drag = ev
		}
	}, monitor)
	return r
}

// clickAt clicks slot of the open window (button and mode as
// plugintest.Player.Click) and reports whether the server cancelled it.
func (r *clicks) clickAt(slot int16, button int8, mode int32) bool {
	r.env.T.Helper()
	r.click = nil
	r.c.Click(slot, button, mode)
	r.env.Drain(r.c)
	if r.click == nil {
		r.env.T.Fatalf("no InventoryClickEvent for a click on slot %d", slot)
	}
	return r.click.Cancelled()
}

// dragOver drags the cursor over slots and reports whether the server
// cancelled the drag.
func (r *clicks) dragOver(slots ...int16) bool {
	r.env.T.Helper()
	r.drag = nil
	r.c.Drag(false, slots...)
	r.env.Drain(r.c)
	if r.drag == nil {
		r.env.T.Fatalf("no InventoryDragEvent for a drag over %v", slots)
	}
	return r.drag.Cancelled()
}

// TestItemAndGive spawns items with meta and amounts for oneself and others.
func TestItemAndGive(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Alice", true)
	env.Join("Bob", false)

	env.Expect(env.Run(c, "i diamond_sword 1 sharpness:5 name:Big_Sword"), "Giving 1 of diamond sword")
	s, ok := find(env, "Alice", registry.ItemDiamondSword)
	if !ok {
		t.Fatal("no sword given")
	}
	sharp, _, _ := env.Ess.Enchantment("sharpness")
	if s.Patch.Enchantments[sharp] != 5 {
		t.Fatalf("enchantments = %v", s.Patch.Enchantments)
	}
	if s.Patch.CustomName == nil || s.Patch.CustomName.PlainString() != "Big Sword" {
		t.Fatalf("name = %v", s.Patch.CustomName)
	}

	env.Expect(env.Run(c, "i stone"), "Giving 64 of stone")
	if n := count(env, "Alice", registry.ItemStone); n != 64 {
		t.Fatalf("stone = %d", n)
	}

	env.Expect(env.Console("egive Bob diamond 10"), "Giving 10 of diamond to Bob")
	if n := count(env, "Bob", registry.ItemDiamond); n != 10 {
		t.Fatalf("Bob's diamonds = %d", n)
	}
	env.Expect(env.Run(c, "i nosuchitem"), "Unknown item name")
}

// TestMoreAndRepair fills the held stack and repairs a damaged tool.
func TestMoreAndRepair(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Alice", true)
	inv := env.Player("Alice").Inventory()

	inv.Set(plugin.SlotMainHand, plugin.NewStack(registry.ItemCobblestone, 3))
	env.Run(c, "more")
	if got := inv.Get(plugin.SlotMainHand).Count; got != 64 {
		t.Fatalf("after /more count = %d", got)
	}
	inv.Set(plugin.SlotMainHand, plugin.NewStack(registry.ItemCobblestone, 3))
	env.Run(c, "more 5")
	if got := inv.Get(plugin.SlotMainHand).Count; got != 8 {
		t.Fatalf("after /more 5 count = %d", got)
	}

	// Ender pearls stack to 16; essentials.oversizedstacks (an operator's)
	// fills to oversized-stacksize, 64.
	inv.Set(plugin.SlotMainHand, plugin.NewStack(registry.ItemEnderPearl, 3))
	env.Run(c, "more")
	if got := inv.Get(plugin.SlotMainHand).Count; got != 64 {
		t.Fatalf("after oversized /more count = %d", got)
	}

	sword := plugin.NewStack(registry.ItemIronSword, 1)
	dmg := 100
	sword.Patch.Damage = &dmg
	inv.Set(plugin.SlotMainHand, sword)
	env.Expect(env.Run(c, "repair"), "successfully repaired your: iron sword")
	if d := inv.Get(plugin.SlotMainHand).Patch.Damage; d != nil && *d != 0 {
		t.Fatalf("damage after repair = %d", *d)
	}
	env.Expect(env.Run(c, "repair"), "cannot be repaired")
}

// TestKits saves the inventory as a kit and takes it again.
func TestKits(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Alice", true)
	env.Run(c, "i diamond_sword 1 sharpness:3 name:Kit_Sword")
	env.Run(c, "i bread 16")
	env.Expect(env.Run(c, "createkit starter 60"), "Created kit starter with 2 entries")
	env.Expect(env.Run(c, "showkit starter"), "minecraft:bread 16")

	env.Run(c, "clearinventory")
	env.Run(c, "clearinventory")
	if n := count(env, "Alice", registry.ItemBread); n != 0 {
		t.Fatalf("bread after clear = %d", n)
	}

	env.Expect(env.Run(c, "kit starter"), "Received kit starter")
	if n := count(env, "Alice", registry.ItemBread); n != 16 {
		t.Fatalf("bread from kit = %d", n)
	}
	s, ok := find(env, "Alice", registry.ItemDiamondSword)
	if !ok || s.Patch.CustomName == nil || s.Patch.CustomName.PlainString() != "Kit Sword" {
		t.Fatalf("kit sword = %+v", s)
	}
	env.Expect(env.Run(c, "kit"), "Starter")
	env.Expect(env.Console("essentials:kitreset starter Alice"), "Resetting kit starter cooldown for Alice")
	env.Expect(env.Run(c, "delkit starter"), "has been removed")
	env.Expect(env.Run(c, "kit starter"), "does not exist")
}

// TestPowertoolAndUnlimited binds a command to an item and toggles an
// unlimited item.
func TestPowertoolAndUnlimited(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Alice", true)
	env.Player("Alice").Inventory().Set(plugin.SlotMainHand, plugin.NewStack(registry.ItemStick, 1))
	env.Expect(env.Run(c, "powertool say hi"), "command assigned to stick")
	if got := env.User("Alice").Data.Powertools["stick"]; len(got) != 1 || got[0] != "say hi" {
		t.Fatalf("powertools = %v", env.User("Alice").Data.Powertools)
	}
	env.Expect(env.Run(c, "powertoollist"), "Item stick has the following commands: say hi")
	env.Expect(env.Run(c, "powertool"), "All commands removed from stick")

	env.Expect(env.Run(c, "unlimited glass"), "Giving unlimited amount of glass to Alice")
	env.Expect(env.Run(c, "unlimited list"), "glass")
	env.Expect(env.Run(c, "unlimited glass"), "Disabled unlimited placing of glass")
}

// TestInvsee opens another player's live inventory and ender chest, and
// keeps the modify permissions.
func TestInvsee(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Alice", true)
	b := env.Join("Bob", false)
	alice, bob := env.Player("Alice"), env.Player("Bob")
	bob.Inventory().SetItem(0, plugin.NewStack(registry.ItemDiamond, 5))
	env.Expect(env.Run(c, "invsee Alice"), "only view other players")
	env.Run(c, "invsee Bob")
	view, ok := alice.OpenView()
	if !ok {
		t.Fatal("no view open")
	}
	if h, ok := view.Top().Holder().(plugin.Player); !ok || h.UUID() != bob.UUID() || view.Top().Type() != plugin.InvPlayer {
		t.Fatalf("holder = %v, type %d", view.Top().Holder(), view.Top().Type())
	}
	if s := view.Top().Item(0); s.Item != registry.ItemDiamond || s.Count != 5 {
		t.Fatalf("invsee slot 0 = %+v", s)
	}
	bob.Inventory().SetItem(1, plugin.NewStack(registry.ItemEmerald, 2))
	env.Drain(c)
	if s := view.Top().Item(1); s.Item != registry.ItemEmerald {
		t.Fatalf("invsee slot 1 not refreshed: %+v", s)
	}
	if !env.User("Alice").InvSee {
		t.Fatal("InvSee not set")
	}
	r := recordClicks(env, c)
	click := func(slot int16) bool { return r.clickAt(slot, 0, 0) }
	if click(0) || alice.Cursor().Item != registry.ItemDiamond {
		t.Fatalf("a click with essentials.invsee.modify was cancelled: cursor %+v", alice.Cursor())
	}
	if click(0) || bob.Inventory().Item(0).Item != registry.ItemDiamond {
		t.Fatalf("putting the diamonds back: Bob's slot 0 = %+v", bob.Inventory().Item(0))
	}
	// Slot 39 of the window is Bob's helmet: no direct hat for Alice.
	alice.SetCursor(plugin.NewStack(registry.ItemStone, 1))
	if click(39) || !alice.Inventory().Get(plugin.SlotHead).Empty() || bob.Inventory().Get(plugin.SlotHead).Item != registry.ItemStone {
		t.Fatalf("a click on the window's slot 39 was a direct hat: Alice's head %+v, Bob's %+v",
			alice.Inventory().Get(plugin.SlotHead), bob.Inventory().Get(plugin.SlotHead))
	}
	if click(39) || alice.Cursor().Item != registry.ItemStone {
		t.Fatalf("taking the stone back: cursor %+v", alice.Cursor())
	}
	alice.SetCursor(plugin.ItemStack{})
	plugin.Provide[plugin.PermissionProvider](env.Ess.Server, perms{"essentials.invsee.preventmodify": true}, 0)
	if !click(0) || bob.Inventory().Item(0).Item != registry.ItemDiamond {
		t.Fatal("a click on a preventmodify inventory was not cancelled")
	}

	bob.EnderChest().SetItem(3, plugin.NewStack(registry.ItemGoldIngot, 7))
	env.Run(c, "enderchest Bob")
	if env.User("Alice").InvSee || !env.User("Alice").EnderSee {
		t.Fatal("flags not moved from invsee to enderchest")
	}
	view, _ = alice.OpenView()
	if s := view.Top().Item(3); s.Item != registry.ItemGoldIngot || s.Count != 7 || view.Top().Type() != plugin.InvEnderChest {
		t.Fatalf("ender chest slot 3 = %+v (type %d)", s, view.Top().Type())
	}

	bob.Inventory().Set(plugin.SlotHead, plugin.NewStack(registry.ItemIronHelmet, 1))
	bob.Inventory().Set(plugin.SlotOffHand, plugin.NewStack(registry.ItemShield, 1))
	env.Run(c, "invsee Bob equip")
	view, _ = alice.OpenView()
	if s := view.Top().Item(3); s.Item != registry.ItemIronHelmet || view.Top().Size() != 9 {
		t.Fatalf("equip slot 3 = %+v (size %d)", s, view.Top().Size())
	}
	if s := view.Top().Item(4); s.Item != registry.ItemShield {
		t.Fatalf("equip slot 4 = %+v, want the off hand", s)
	}
	if !click(3) {
		t.Fatal("a click in the equip view was not cancelled")
	}
	alice.CloseInventory()
	if env.User("Alice").InvSee {
		t.Fatal("InvSee kept after close")
	}

	// The server closes the window, with its close event, when the owner
	// leaves; that event clears InvSee.
	closes := 0
	plugin.Listen(env.Ess.Server, func(ev *plugin.InventoryCloseEvent) {
		if ev.Player.UUID() == alice.UUID() {
			closes++
		}
	})
	env.Run(c, "invsee Bob")
	b.Disconnect()
	env.Drain(c)
	if _, open := alice.OpenView(); open || env.User("Alice").InvSee || closes != 1 {
		t.Fatalf("after the owner left: open %v, InvSee %v, %d close events", open, env.User("Alice").InvSee, closes)
	}
}

type perms map[string]bool

func (p perms) Has(_ plugin.CommandSender, node string) (has, decided bool) {
	has, decided = p[node]
	return has, decided
}

// TestWorkstations opens each workstation window.
func TestWorkstations(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Alice", true)
	alice := env.Player("Alice")
	for cmd, kind := range map[string]plugin.InventoryType{
		"workbench": plugin.InvCrafting, "anvil": plugin.InvAnvil, "cartographytable": plugin.InvCartography,
		"grindstone": plugin.InvGrindstone, "loom": plugin.InvLoom, "smithingtable": plugin.InvSmithing,
		"stonecutter": plugin.InvStonecutter,
	} {
		env.Run(c, cmd)
		v, ok := alice.OpenView()
		if !ok || v.Top().Type() != kind {
			t.Fatalf("/%s opened %v", cmd, v)
		}
	}
}

// TestRecipe shows a recipe in a crafting table that works out the result,
// keeps it read-only and hands none of it out on closing.
func TestRecipe(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Alice", true)
	alice := env.Player("Alice")
	env.Expect(env.Run(c, "recipe stick"), "Recipe for stick")
	v, ok := alice.OpenView()
	if !ok || !env.User("Alice").RecipeSee {
		t.Fatal("no recipe window")
	}
	top := v.Top()
	if top.Type() != plugin.InvCrafting || top.Size() != 10 {
		t.Fatalf("recipe window: type %d, size %d", top.Type(), top.Size())
	}
	// The pattern is a column of two planks: grid slots 1 and 4.
	for _, i := range []int{1, 4} {
		if s := top.Item(i); s.Empty() || s.Count != 1 {
			t.Fatalf("grid slot %d = %+v", i, s)
		}
	}
	if s := top.Item(0); s.Item != registry.ItemStick || s.Count != 4 {
		t.Fatalf("result = %+v, want 4 sticks", s)
	}
	r := recordClicks(env, c)
	for _, mode := range []int32{0, 1} { // a left click and a shift-click
		for _, slot := range []int16{0, 1} {
			if !r.clickAt(slot, 0, mode) {
				t.Fatalf("click mode %d on slot %d of the recipe window was not cancelled", mode, slot)
			}
		}
	}
	if s := top.Item(0); s.Item != registry.ItemStick || s.Count != 4 {
		t.Fatalf("result after the clicks = %+v, want 4 sticks", s)
	}
	alice.SetCursor(plugin.NewStack(registry.ItemDirt, 2))
	if !r.dragOver(2, 3) || !top.Item(2).Empty() || !top.Item(3).Empty() {
		t.Fatalf("a drag into the recipe window was not cancelled: grid %+v %+v", top.Item(2), top.Item(3))
	}
	alice.SetCursor(plugin.ItemStack{})
	c.CloseWindow()
	env.Drain(c)
	if env.User("Alice").RecipeSee {
		t.Fatal("RecipeSee kept after close")
	}
	for _, s := range alice.Inventory().Contents() {
		if !s.Empty() {
			t.Fatalf("closing the recipe handed out %+v", s)
		}
	}

	// A /workbench afterwards is an ordinary table.
	env.Run(c, "workbench")
	v, _ = alice.OpenView()
	alice.SetCursor(plugin.NewStack(registry.ItemOakPlanks, 1))
	if r.clickAt(1, 0, 0) || v.Top().Item(1).Item != registry.ItemOakPlanks {
		t.Fatalf("a click in a /workbench table was cancelled: grid slot 1 = %+v", v.Top().Item(1))
	}
	c.CloseWindow()
}

// TestRecipeQuit leaves with a recipe open: the server closes the window
// with its close event, which clears RecipeSee and empties the table, so
// nothing shown is dropped.
func TestRecipeQuit(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Alice", true)
	alice := env.Player("Alice")
	w := alice.World()
	closes := 0
	var seeAtQuit bool
	plugin.Listen(env.Ess.Server, func(ev *plugin.InventoryCloseEvent) {
		if ev.Player.UUID() == alice.UUID() {
			closes++
		}
	})
	plugin.Listen(env.Ess.Server, func(ev *plugin.PlayerQuitEvent) {
		if ev.Player.UUID() == alice.UUID() {
			seeAtQuit = env.Ess.Users.Of(ev.Player).RecipeSee
		}
	})
	env.Run(c, "recipe stick")
	if _, ok := alice.OpenView(); !ok {
		t.Fatal("no recipe window")
	}
	c.Disconnect()
	env.Srv.RunTicks(3)
	if closes != 1 || seeAtQuit {
		t.Fatalf("leaving: %d close events, RecipeSee at quit %v", closes, seeAtQuit)
	}
	for en := range w.Entities() {
		if en.Type() == registry.EntityTypeItem {
			t.Fatalf("leaving with the recipe open dropped %v", en)
		}
	}
}

// TestEditsign edits the sign looked at, and a SignChangeEvent listener
// can veto the edit.
func TestEditsign(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Alice", true)
	env.Console("setblock 0 65 3 minecraft:oak_sign[rotation=8]")
	env.Console("minecraft:tp Alice 0.5 64 0.5 0 0")
	env.Drain(c)
	env.Expect(env.Run(c, "editsign set 1 Hello"), `Set line 1 to "Hello"`)
	w := env.Player("Alice").World()
	be, ok := w.BlockEntity(plugin.BlockPos{X: 0, Y: 65, Z: 3})
	sign, isSign := be.(plugin.Sign)
	if !ok || !isSign {
		t.Fatal("no sign")
	}
	if got := sign.Lines(true)[0].PlainString(); got != "Hello" {
		t.Fatalf("line 1 = %q", got)
	}
	plugin.Listen(env.Ess.Server, func(ev *plugin.SignChangeEvent) {
		if ev.Lines[1].PlainString() == "Nope" {
			ev.SetCancelled(true)
		}
	})
	env.Run(c, "editsign set 2 Nope")
	if got := sign.Lines(true)[1].PlainString(); got != "" {
		t.Fatalf("a vetoed edit wrote line 2 = %q", got)
	}
}

// TestHat puts the held item on the head, and refuses a type a provider
// sets in prevent-type; a click of the cursor on the helmet slot does it
// directly. (An operator has every unset prevent-type node, as in Bukkit,
// so stone is set false.)
func TestHat(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Alice", true)
	alice := env.Player("Alice")
	inv := alice.Inventory()
	inv.Set(plugin.SlotMainHand, plugin.NewStack(registry.ItemDirt, 1))
	env.Expect(env.Run(c, "hat"), "Enjoy your new hat")
	if inv.Get(plugin.SlotHead).Item != registry.ItemDirt {
		t.Fatal("no dirt hat")
	}
	plugin.Provide[plugin.PermissionProvider](env.Ess.Server, perms{"essentials.hat.prevent-type.glass": true, "essentials.hat.prevent-type.stone": false}, 0)
	inv.Set(plugin.SlotMainHand, plugin.NewStack(registry.ItemGlass, 1))
	env.Expect(env.Run(c, "hat"), "something to wear")
	// Essentials refuses anything with durability, tools and armour alike.
	for _, it := range []registry.Item{registry.ItemDiamondPickaxe, registry.ItemDiamondHelmet} {
		inv.Set(plugin.SlotMainHand, plugin.NewStack(it, 1))
		env.Expect(env.Run(c, "hat"), "cannot use this item as a hat")
	}

	// Direct hat: a stone clicked into the helmet slot (5 of the player's
	// own window) swaps with the dirt hat.
	inv.Set(plugin.SlotMainHand, plugin.ItemStack{})
	alice.SetCursor(plugin.NewStack(registry.ItemStone, 1))
	r := recordClicks(env, c)
	if !r.clickAt(5, 0, 0) || inv.Get(plugin.SlotHead).Item != registry.ItemStone || alice.Cursor().Item != registry.ItemDirt {
		t.Fatalf("direct hat: cancelled %v, head %v, cursor %v", r.click.Cancelled(), inv.Get(plugin.SlotHead), alice.Cursor())
	}
}

// TestSkull gives the head of an online player with their profile.
func TestSkull(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Alice", true)
	env.Join("Bob", false)
	env.Expect(env.Run(c, "skull Bob"), "skull of Bob")
	s, ok := find(env, "Alice", registry.ItemPlayerHead)
	if !ok || s.Patch.Profile == nil || s.Patch.Profile.Name != "Bob" || s.Patch.Profile.ID != env.Player("Bob").UUID() {
		t.Fatalf("skull = %+v", s.Patch.Profile)
	}
}

// TestClearAndCondense clears given items and packs ingots into blocks.
func TestClearAndCondense(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Alice", true)
	inv := env.Player("Alice").Inventory()
	inv.SetItem(0, plugin.NewStack(registry.ItemIronIngot, 20))
	env.Expect(env.Run(c, "condense"), "Converted all items into blocks")
	if n, b := count(env, "Alice", registry.ItemIronIngot), count(env, "Alice", registry.ItemIronBlock); n != 2 || b != 2 {
		t.Fatalf("after condense: %d ingots, %d blocks", n, b)
	}
	// With clearinventory.others the first word is a player, as in Essentials.
	env.Expect(env.Run(c, "ci Alice iron_block 1"), "Removed 1 of iron_block from Alice")
	if b := count(env, "Alice", registry.ItemIronBlock); b != 1 {
		t.Fatalf("blocks after clearing 1 = %d", b)
	}
}

// TestOversizedGive gives an operator (who holds essentials.oversizedstacks)
// ender pearls in stacks of oversized-stacksize, not of 16.
func TestOversizedGive(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Run(a, "give Alice ender_pearl 64")
	inv := env.Player("Alice").Inventory()
	if got := inv.Item(0); got.Item != registry.ItemEnderPearl || got.Count != 64 {
		t.Fatalf("slot 0 = %v x%d, want ender_pearl x64", got.Item, got.Count)
	}
	if !inv.Item(1).Empty() {
		t.Fatalf("slot 1 = %v x%d, want empty", inv.Item(1).Item, inv.Item(1).Count)
	}
}

// TestFireworkFire launches a rocket that flies for its flight duration
// instead of bursting at once.
func TestFireworkFire(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Alice", true)
	alice := env.Player("Alice")
	rocket, err := env.Ess.Server.Items().Parse("firework_rocket[fireworks={flight_duration:3}]", 1)
	if err != nil {
		t.Fatal(err)
	}
	alice.Inventory().Set(plugin.SlotMainHand, rocket)
	rockets := func() int {
		n := 0
		for en := range alice.World().Entities() {
			if en.Type() == registry.EntityTypeFireworkRocket {
				n++
			}
		}
		return n
	}
	env.Run(c, "firework fire")
	// Flight 3 lives 40 to 51 ticks (Bukkit: 10*(power+1)+rand(6)+rand(7)).
	env.Srv.RunTicks(10)
	if n := rockets(); n != 1 {
		t.Fatalf("%d rockets in flight about 15 ticks after the launch, want 1", n)
	}
	env.Srv.RunTicks(50)
	if n := rockets(); n != 0 {
		t.Fatalf("%d rockets still in flight 65 ticks after the launch", n)
	}
}

// TestUnlimitedEgg throws a real egg: when it breaks, a player with
// unlimited eggs gets it back. Essentials: EssentialsPlayerListener.onPlayerEggThrow
func TestUnlimitedEgg(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Alice", true)
	alice := env.Player("Alice")
	thrown := 0
	plugin.Listen(env.Ess.Server, func(ev *plugin.PlayerEggThrowEvent) {
		if ev.Player.UUID() == alice.UUID() {
			thrown++
		}
	})
	// throw throws one of 4 held eggs and answers how many are left once
	// it has broken on the ground.
	throw := func() int {
		t.Helper()
		alice.Inventory().Set(plugin.SlotMainHand, plugin.NewStack(registry.ItemEgg, 4))
		before := thrown
		c.UseItem()
		for range 10 {
			if env.Srv.RunTicks(5); thrown > before {
				break
			}
		}
		if thrown != before+1 {
			t.Fatalf("PlayerEggThrowEvent fired %d times for one throw", thrown-before)
		}
		env.Srv.RunTicks(2)
		return count(env, "Alice", registry.ItemEgg)
	}
	if n := throw(); n != 3 {
		t.Fatalf("eggs after a throw without unlimited = %d, want 3", n)
	}
	env.Expect(env.Run(c, "unlimited egg"), "Giving unlimited amount of egg")
	if n := throw(); n != 4 {
		t.Fatalf("eggs after a throw with unlimited = %d, want 4", n)
	}
}
