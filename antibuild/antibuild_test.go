package antibuild_test

import (
	"fmt"
	"testing"

	_ "github.com/golem-mc/essentials/antibuild"
	"github.com/golem-mc/essentials/internal/esstest"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
	"github.com/golem-mc/golem/registry"
)

func init() { esstest.RegisterCore() }

// placeAt has the player place stone on top of the block at ground,
// reporting whether a block appeared above it.
func placeAt(env *esstest.Env, c *esstest.Client, ground plugin.BlockPos) bool {
	env.Player(c.Name()).Inventory().Set(plugin.SlotMainHand, plugin.NewStack(registry.ItemStone, 4))
	c.PlaceBlock(int32(ground.X), int32(ground.Y), int32(ground.Z), 1)
	c.Read(3)
	above := plugin.BlockPos{X: ground.X, Y: ground.Y + 1, Z: ground.Z}
	return env.Ess.Server.Overworld().Block(above).State().Block() == registry.BlockStone
}

// TestBuildDenied checks that a player without essentials.build cannot
// place a block and is told so, while an operator can.
func TestBuildDenied(t *testing.T) {
	env := esstest.Start(t)
	b := env.Join("Bob", false)
	// The click itself is an interaction, which disable.use refuses before
	// the place or break is tried, as in Essentials.
	env.Ess.Config.Set("protect.disable.use", false)
	at := env.Player("Bob").Location().Pos.Block()
	ground := plugin.BlockPos{X: at.X + 2, Y: at.Y - 1, Z: at.Z}

	if placeAt(env, b, ground) {
		t.Fatal("Bob placed a block")
	}
	env.Expect(env.Messages(b, 5), "You are not permitted to place stone here.")

	env.Console("op Bob")
	env.Drain(b)
	if !placeAt(env, b, ground) {
		t.Fatal("operator Bob could not place a block")
	}
}

// TestBreakDenied checks that a player without essentials.build cannot
// break a block, while an operator can.
func TestBreakDenied(t *testing.T) {
	env := esstest.Start(t, plugintest.WithGameMode("creative"))
	b := env.Join("Bob", false)
	// The click itself is an interaction, which disable.use refuses before
	// the place or break is tried, as in Essentials.
	env.Ess.Config.Set("protect.disable.use", false)
	w := env.Ess.Server.Overworld()
	at := env.Player("Bob").Location().Pos.Block()
	pos := plugin.BlockPos{X: at.X + 2, Y: at.Y, Z: at.Z}
	env.Console(fmt.Sprintf("setblock %d %d %d stone", pos.X, pos.Y, pos.Z))
	env.Drain(b)
	dig := func() bool {
		b.StartBreak(int32(pos.X), int32(pos.Y), int32(pos.Z))
		b.Read(3)
		return w.Block(pos).State().Block() != registry.BlockStone
	}
	if dig() {
		t.Fatal("Bob broke the block")
	}
	env.Expect(env.Messages(b, 5), "You are not permitted to break stone blocks here.")
	env.Console("op Bob")
	env.Drain(b)
	if !dig() {
		t.Fatal("operator Bob could not break the block")
	}
}

// find answers the first entity of type name in w.
func find(w plugin.World, name string) plugin.Entity {
	for e := range w.Entities() {
		if e.Type().Name() == "minecraft:"+name {
			return e
		}
	}
	return nil
}

// TestEntityBreakDenied checks that a player without essentials.build
// cannot knock the item out of an item frame, break an armour stand (which
// fires EntityDamageEvent since c0b90e608) or blow up an end crystal, and
// is told so, while an operator can.
func TestEntityBreakDenied(t *testing.T) {
	env := esstest.Start(t)
	b := env.Join("Bob", false)
	w := env.Ess.Server.Overworld()
	env.Ess.Config.Set("protect.disable.warn-on-build-disallow", true)
	env.Console("setblock 2 64 0 stone")
	env.Console(`summon item_frame 1 64 0 {Facing:4b,Item:{id:"minecraft:diamond",count:1}}`)
	env.Console("summon end_crystal 0 64 -2 {ShowBottom:0b}")
	env.Console("summon armor_stand 0 64 2")
	env.Drain(b)
	frame, crystal, stand := find(w, "item_frame"), find(w, "end_crystal"), find(w, "armor_stand")
	if frame == nil || crystal == nil || stand == nil {
		t.Fatal("the frame, crystal or stand did not spawn")
	}

	// hit attacks e as Bob and reports whether the blow was refused: Bob
	// was told so.
	hit := func(e plugin.Entity, want string) {
		t.Helper()
		b.Attack(e.ID())
		env.Expect(env.Messages(b, 10), want)
	}
	hit(frame, "You are not permitted to break ITEM_FRAME blocks here.")
	hit(crystal, "You are not permitted to break END_CRYSTAL blocks here.")
	hit(stand, "You are not permitted to break ARMOR_STAND blocks here.")
	hit(stand, "You are not permitted to break ARMOR_STAND blocks here.")
	if !crystal.IsValid() || !stand.IsValid() {
		t.Fatalf("Bob broke the crystal (%v) or the stand (%v)", !crystal.IsValid(), !stand.IsValid())
	}
	if f, ok := frame.(plugin.ItemFrame); ok && f.Item().Empty() {
		t.Fatal("Bob knocked the diamond out of the frame")
	}

	env.Console("op Bob")
	env.Drain(b)
	b.Attack(frame.ID())
	b.Attack(stand.ID())
	b.Attack(stand.ID())
	b.Read(5)
	if f, ok := frame.(plugin.ItemFrame); ok && !f.Item().Empty() {
		t.Fatal("operator Bob could not knock the diamond out of the frame")
	}
	if stand.IsValid() {
		t.Fatal("operator Bob could not break the armour stand")
	}
}

// TestInteractEntityDenied checks that a player without essentials.build
// cannot put an item into an item frame or onto an armour stand (both now
// fire PlayerInteractEntityEvent), and is told so, while an operator can.
func TestInteractEntityDenied(t *testing.T) {
	env := esstest.Start(t)
	b := env.Join("Bob", false)
	w := env.Ess.Server.Overworld()
	env.Ess.Config.Set("protect.disable.warn-on-build-disallow", true)
	env.Console("setblock 2 64 0 stone")
	env.Console("summon item_frame 1 64 0 {Facing:4b}")
	env.Console("summon armor_stand 0 64 2")
	env.Drain(b)
	frame, stand := find(w, "item_frame"), find(w, "armor_stand")
	if frame == nil || stand == nil {
		t.Fatal("the frame or stand did not spawn")
	}
	// use right-clicks e holding a diamond helmet; on a stand, at its head.
	use := func(e plugin.Entity) {
		env.Player("Bob").Inventory().Set(plugin.SlotMainHand, plugin.NewStack(registry.ItemDiamondHelmet, 1))
		if e == stand {
			b.InteractAt(e.ID(), 0, 1.7, 0)
		} else {
			b.Interact(e.ID())
		}
	}
	use(frame)
	env.Expect(env.Messages(b, 10), "You are not permitted to place ITEM_FRAME here.")
	use(stand)
	env.Expect(env.Messages(b, 10), "You are not permitted to place ARMOR_STAND here.")
	if !frame.(plugin.ItemFrame).Item().Empty() {
		t.Fatal("Bob put the helmet into the frame")
	}

	env.Console("op Bob")
	env.Drain(b)
	use(frame)
	b.Read(5)
	if frame.(plugin.ItemFrame).Item().Empty() {
		t.Fatal("operator Bob could not put the helmet into the frame")
	}
}

// TestUseDenied checks that with disable.use, the right-click itself is
// refused for a player without essentials.build, as Essentials does.
func TestUseDenied(t *testing.T) {
	env := esstest.Start(t)
	b := env.Join("Bob", false)
	at := env.Player("Bob").Location().Pos.Block()
	if placeAt(env, b, plugin.BlockPos{X: at.X + 2, Y: at.Y - 1, Z: at.Z}) {
		t.Fatal("Bob placed a block")
	}
	env.Expect(env.Messages(b, 5), "You are not permitted to use STONE.")
}

// TestPaintingBlastDenied checks that a painting blown up by TNT a player
// without essentials.build lit stays, and the player is told so: Bukkit names
// the player who lit it as the remover, as golem now does. An operator's TNT
// breaks it.
func TestPaintingBlastDenied(t *testing.T) {
	env := esstest.Start(t)
	b := env.Join("Bob", false)
	w := env.Ess.Server.Overworld()
	env.Ess.Config.Set("protect.disable.use", false)
	env.Ess.Config.Set("protect.disable.warn-on-build-disallow", true)
	env.Console("effect give Bob resistance 120 4 true")
	env.Console("fill 6 64 -1 6 66 1 obsidian")
	env.Console(`summon painting 5 65 0 {facing:1b,variant:"minecraft:kebab"}`)
	env.Drain(b)
	if find(w, "painting") == nil {
		t.Fatal("no painting")
	}
	light := func() []string {
		t.Helper()
		env.Console("setblock 3 64 0 tnt")
		env.Player("Bob").Inventory().Set(plugin.SlotMainHand, plugin.NewStack(registry.ItemFlintAndSteel, 1))
		env.Drain(b)
		b.PlaceBlock(3, 64, 0, 4)
		msgs := env.Messages(b, 100)
		if w.Block(plugin.BlockPos{X: 3, Y: 64, Z: 0}).State().Block() == registry.BlockTnt {
			t.Fatal("Bob did not light the TNT")
		}
		return msgs
	}
	env.Expect(light(), "You are not permitted to break PAINTING blocks here.")
	if find(w, "painting") == nil {
		t.Fatal("Bob's TNT broke the painting")
	}
	env.Console("op Bob")
	light()
	if find(w, "painting") != nil {
		t.Fatal("operator Bob's TNT left the painting")
	}
}
