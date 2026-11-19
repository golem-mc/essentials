package protect_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/golem-mc/essentials/internal/esstest"
	_ "github.com/golem-mc/essentials/protect"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

func init() { esstest.RegisterCore() }

// TestWeather checks that weather.storm keeps rain away and weather.thunder
// thunder, each on its own flag, as Bukkit's two events.
func TestWeather(t *testing.T) {
	env := esstest.Start(t)
	w := env.Ess.Server.Overworld()
	env.Ess.Config.Set("protect.disable.weather.storm", true)
	env.Console("weather rain")
	if got := w.Weather(); got != plugin.WeatherClear {
		t.Fatalf("weather = %v with storms disabled", got)
	}
	env.Ess.Config.Set("protect.disable.weather.storm", false)
	env.Ess.Config.Set("protect.disable.weather.thunder", true)
	env.Console("weather thunder")
	if got := w.Weather(); got != plugin.WeatherRain {
		t.Fatalf("weather = %v with thunder disabled, want rain", got)
	}
	env.Ess.Config.Set("protect.disable.weather.thunder", false)
	env.Console("weather thunder")
	if got := w.Weather(); got != plugin.WeatherThunder {
		t.Fatalf("weather = %v with thunder allowed", got)
	}
}

// count answers how many entities of type name are in the world.
func count(w plugin.World, name string) int {
	n := 0
	for e := range w.Entities() {
		if e.Type().Name() == "minecraft:"+name {
			n++
		}
	}
	return n
}

// TestPreventSpawn checks that protect.prevent.spawn.<mob> discards a
// summoned mob and leaves the others.
func TestPreventSpawn(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Bob", false)
	w := env.Ess.Server.Overworld()
	env.Ess.Config.Set("protect.prevent.spawn.zombie", true)
	env.Console("summon zombie 5 64 5")
	env.Console("summon pig 5 64 5")
	env.Drain(c)
	if n := count(w, "zombie"); n != 0 {
		t.Fatalf("%d zombies spawned with prevent.spawn.zombie", n)
	}
	if n := count(w, "pig"); n != 1 {
		t.Fatalf("%d pigs, want 1", n)
	}
}

// targetOf answers the target of the first mob of type name.
func targetOf(t *testing.T, w plugin.World, name string) (plugin.LivingEntity, bool) {
	t.Helper()
	for e := range w.Entities() {
		if e.Type().Name() == "minecraft:"+name {
			return e.(plugin.Mob).Target()
		}
	}
	t.Fatalf("no %s", name)
	return nil, false
}

// TestEntityTarget checks that prevent.entitytarget keeps a zombie off a
// player, unless they have the bypass.
func TestEntityTarget(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Bob", false)
	w := env.Ess.Server.Overworld()
	env.Ess.Config.Set("protect.prevent.entitytarget", true)
	env.Console("summon zombie 3 64 0")
	env.Messages(c, 30)
	if _, ok := targetOf(t, w, "zombie"); ok {
		t.Fatal("the zombie targets Bob with prevent.entitytarget")
	}
	env.Ess.Config.Set("protect.prevent.entitytarget", false)
	env.Messages(c, 30)
	if tgt, ok := targetOf(t, w, "zombie"); !ok || tgt.UUID() != env.Player("Bob").UUID() {
		t.Fatal("the zombie does not target Bob once allowed")
	}
}

// TestZombieDoorBreak checks prevent.zombie-door-break on the event a
// zombie breaking a door fires.
func TestZombieDoorBreak(t *testing.T) {
	env := esstest.Start(t)
	env.Join("Bob", false) // loads the chunks
	s := env.Ess.Server
	w := s.Overworld()
	zombie, _ := registry.EntityTypeByName("minecraft:zombie")
	z, err := w.Spawn(zombie, plugin.Location{World: w, Pos: plugin.Vec3{X: 3, Y: 64, Z: 0}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	door := w.Block(plugin.BlockPos{X: 4, Y: 64, Z: 0})
	fire := func() bool {
		ev := &plugin.EntityBreakDoorEvent{Entity: z, Block: door}
		s.Events().Fire(ev)
		return ev.Cancelled()
	}
	if fire() {
		t.Fatal("door break cancelled by default")
	}
	env.Ess.Config.Set("protect.prevent.zombie-door-break", true)
	if !fire() {
		t.Fatal("door break not cancelled with prevent.zombie-door-break")
	}
}

// TestCreeperExplosion checks that a creeper's blast above
// protect.creeper.max-height leaves the blocks alone, showing a harmless
// explosion in its place, and creeper-itemdamage the items.
func TestCreeperExplosion(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Bob", false)
	w := env.Ess.Server.Overworld()
	ground := plugin.BlockPos{X: 8, Y: 63, Z: 0}
	before := w.BlockState(ground)
	env.Ess.Config.Set("protect.creeper.max-height", ground.Y)
	env.Ess.Config.Set("protect.prevent.creeper-itemdamage", true)
	item := w.DropItem(plugin.Vec3{X: 8.5, Y: 64, Z: 1.5}, plugin.ItemStack{Item: registry.ItemDiamond, Count: 1}, false)
	env.Drain(c)
	c.Explosions(1)
	env.Console(fmt.Sprintf("summon creeper %d %d %d {ignited:1b,Fuse:1}", ground.X, ground.Y+1, ground.Z))
	booms := c.Explosions(20)
	if got := w.BlockState(ground); got != before {
		t.Fatalf("the creeper broke the ground: %v", got)
	}
	// The cancelled blast is not shown; the harmless one in its place is.
	if len(booms) != 1 || booms[0].Radius != 0 || booms[0].Blocks != 0 {
		t.Fatalf("explosions = %+v, want only the harmless one", booms)
	}
	if !item.IsValid() {
		t.Fatal("the creeper destroyed the item with creeper-itemdamage")
	}
}

// blastAt summons an exploder with a 1-tick fuse at pos (a /summon line's
// type and fuse tag) and lets it blow.
func blastAt(env *esstest.Env, c *esstest.Client, summon string, pos plugin.BlockPos, tag string) {
	env.Console(fmt.Sprintf("minecraft:summon %s %d %d %d %s", summon, pos.X, pos.Y, pos.Z, tag))
	env.Messages(c, 20)
}

// hurt reports whether p lost health, and heals them.
func hurt(p plugin.Player) bool {
	h := p.Health()
	p.SetHealth(p.MaxHealth())
	return h < p.MaxHealth()
}

// TestTNT checks tnt-explosion, and tnt-playerdamage and tnt-itemdamage,
// which read the TNT off the blast's damage source.
func TestTNT(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Bob", false)
	w := env.Ess.Server.Overworld()
	bob := env.Player("Bob")
	ground := plugin.BlockPos{X: 2, Y: 63, Z: 0}
	before := w.BlockState(ground)
	env.Ess.Config.Set("protect.prevent.tnt-explosion", true)
	env.Ess.Config.Set("protect.prevent.tnt-playerdamage", true)
	env.Ess.Config.Set("protect.prevent.tnt-itemdamage", true)
	item := w.DropItem(plugin.Vec3{X: 2.5, Y: 64, Z: 1.5}, plugin.ItemStack{Item: registry.ItemDiamond, Count: 1}, false)
	c.Explosions(1)
	blastAt(env, c, "tnt", plugin.BlockPos{X: ground.X, Y: ground.Y + 1, Z: ground.Z}, "{fuse:1}")
	if got := w.BlockState(ground); got != before {
		t.Fatalf("the TNT broke the ground: %v", got)
	}
	// A cancelled blast is not shown at all.
	if booms := c.Explosions(1); len(booms) != 0 {
		t.Fatalf("the cancelled TNT blast was shown: %+v", booms)
	}
	if hurt(bob) {
		t.Fatal("Bob took TNT damage with tnt-playerdamage")
	}
	if !item.IsValid() {
		t.Fatal("the TNT destroyed the item with tnt-itemdamage")
	}

	env.Ess.Config.Set("protect.prevent.tnt-playerdamage", false)
	env.Ess.Config.Set("protect.prevent.tnt-itemdamage", false)
	blastAt(env, c, "tnt", plugin.BlockPos{X: ground.X, Y: ground.Y + 1, Z: ground.Z}, "{fuse:1}")
	if !hurt(bob) {
		t.Fatal("Bob took no TNT damage without tnt-playerdamage")
	}
	if item.IsValid() {
		t.Fatal("the item survived the TNT without tnt-itemdamage")
	}
}

// TestCreeperPlayerDamage checks creeper-playerdamage, read off the blast's
// damage source.
func TestCreeperPlayerDamage(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Bob", false)
	bob := env.Player("Bob")
	at := plugin.BlockPos{X: 2, Y: 64, Z: 0}
	env.Ess.Config.Set("protect.prevent.creeper-playerdamage", true)
	blastAt(env, c, "creeper", at, "{ignited:1b,Fuse:1}")
	if hurt(bob) {
		t.Fatal("Bob took creeper damage with creeper-playerdamage")
	}
	env.Ess.Config.Set("protect.prevent.creeper-playerdamage", false)
	blastAt(env, c, "creeper", at, "{ignited:1b,Fuse:1}")
	if !hurt(bob) {
		t.Fatal("Bob took no creeper damage without creeper-playerdamage")
	}
}

// TestTNTMinecartPlayerDamage checks tnt-minecart-playerdamage, read off the
// blast's damage source.
func TestTNTMinecartPlayerDamage(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Bob", false)
	bob := env.Player("Bob")
	at := plugin.BlockPos{X: 2, Y: 64, Z: 0}
	env.Ess.Config.Set("protect.prevent.tnt-minecart-playerdamage", true)
	blastAt(env, c, "tnt_minecart", at, "{fuse:1}")
	if hurt(bob) {
		t.Fatal("Bob took TNT minecart damage with tnt-minecart-playerdamage")
	}
	env.Ess.Config.Set("protect.prevent.tnt-minecart-playerdamage", false)
	blastAt(env, c, "tnt_minecart", at, "{fuse:1}")
	if !hurt(bob) {
		t.Fatal("Bob took no TNT minecart damage without tnt-minecart-playerdamage")
	}
}

// pig summons a pig that stays put at pos.
func pig(t *testing.T, env *esstest.Env, c *esstest.Client, pos plugin.BlockPos) plugin.LivingEntity {
	t.Helper()
	env.Console(fmt.Sprintf("summon pig %d %d %d {NoAI:1b}", pos.X, pos.Y, pos.Z))
	env.Drain(c)
	for e := range env.Ess.Server.Overworld().Entities() {
		if e.Type() == registry.EntityTypePig {
			return e.(plugin.LivingEntity)
		}
	}
	t.Fatal("no pig")
	return nil
}

// TestMobBlastDamage checks that the creeper and TNT damage rules spare a
// mob too: Essentials cancels the damage whatever the target, and a mob hurt
// by a blast now knows the exploder. A creeper blast hurting a mob with a
// damage listener (Protect's) once froze the server.
func TestMobBlastDamage(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Bob", false)
	p := pig(t, env, c, plugin.BlockPos{X: 6, Y: 64, Z: 0})
	hurtPig := func() bool {
		h := p.Health()
		p.SetHealth(p.MaxHealth())
		return h < p.MaxHealth()
	}
	at := plugin.BlockPos{X: 7, Y: 64, Z: 0}
	for _, k := range []struct{ rule, summon, tag string }{
		{"creeper-playerdamage", "creeper", "{ignited:1b,Fuse:1}"},
		{"tnt-playerdamage", "tnt", "{fuse:1}"},
		{"tnt-minecart-playerdamage", "tnt_minecart", "{fuse:1}"},
	} {
		env.Ess.Config.Set("protect.prevent."+k.rule, true)
		blastAt(env, c, k.summon, at, k.tag)
		if !p.IsValid() || hurtPig() {
			t.Fatalf("the pig took %s damage with %s", k.summon, k.rule)
		}
		env.Ess.Config.Set("protect.prevent."+k.rule, false)
		blastAt(env, c, k.summon, at, k.tag)
		if !p.IsValid() {
			p = pig(t, env, c, plugin.BlockPos{X: 6, Y: 64, Z: 0})
		} else if !hurtPig() {
			t.Fatalf("the pig took no %s damage without %s", k.summon, k.rule)
		}
	}
}

// TestPreventSpawnArmorStand checks that prevent.spawn.armor_stand stops
// an armour stand, which is a CreatureSpawnEvent on Bukkit.
func TestPreventSpawnArmorStand(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Bob", false)
	w := env.Ess.Server.Overworld()
	env.Ess.Config.Set("protect.prevent.spawn.armor_stand", true)
	env.Console("summon armor_stand 3 64 3")
	env.Drain(c)
	if n := count(w, "armor_stand"); n != 0 {
		t.Fatalf("%d armour stands with prevent.spawn.armor_stand", n)
	}
	env.Ess.Config.Set("protect.prevent.spawn.armor_stand", false)
	env.Console("summon armor_stand 3 64 3")
	env.Drain(c)
	if n := count(w, "armor_stand"); n != 1 {
		t.Fatalf("%d armour stands without prevent.spawn.armor_stand, want 1", n)
	}
}

// TestPaintingBreak checks that tnt-explosion keeps a painting a TNT blast
// would break, by the blast's remover.
func TestPaintingBreak(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Bob", false)
	w := env.Ess.Server.Overworld()
	env.Console("fill 6 64 -1 6 66 1 stone")
	env.Console(`summon painting 5 65 0 {facing:1b,variant:"minecraft:kebab"}`)
	env.Drain(c)
	if count(w, "painting") != 1 {
		t.Fatal("no painting")
	}
	env.Ess.Config.Set("protect.prevent.tnt-explosion", true)
	blastAt(env, c, "tnt", plugin.BlockPos{X: 4, Y: 64, Z: 0}, "{fuse:1}")
	if count(w, "painting") != 1 {
		t.Fatal("the TNT broke the painting with tnt-explosion")
	}
	env.Ess.Config.Set("protect.prevent.tnt-explosion", false)
	blastAt(env, c, "tnt", plugin.BlockPos{X: 4, Y: 64, Z: 0}, "{fuse:1}")
	if count(w, "painting") != 0 {
		t.Fatal("the painting survived the TNT without tnt-explosion")
	}
}

// TestPreventSpawnLivingOnly checks that protect.prevent.spawn is
// CreatureSpawnEvent's: an item, which now fires EntitySpawnEvent too, is
// not stopped by prevent.spawn.item.
func TestPreventSpawnLivingOnly(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Bob", false)
	w := env.Ess.Server.Overworld()
	env.Ess.Config.Set("protect.prevent.spawn.item", true)
	spawned := 0
	plugin.Listen(env.Ess.Server, func(ev *plugin.EntitySpawnEvent) {
		if ev.Entity.Type() == registry.EntityTypeItem {
			spawned++
		}
	}, plugin.ListenOptions{Priority: plugin.Monitor})
	item := w.DropItem(plugin.Vec3{X: 3.5, Y: 64, Z: 3.5}, plugin.ItemStack{Item: registry.ItemStick, Count: 1}, false)
	env.Messages(c, 5)
	if spawned == 0 {
		t.Fatal("the item fired no EntitySpawnEvent")
	}
	if !item.IsValid() {
		t.Fatal("prevent.spawn.item stopped an item")
	}
}

// TestEntityTargetRetaliation checks that prevent.entitytarget keeps a wolf
// Bob hit from turning on him (Bukkit's TARGET_ATTACKED_ENTITY), unless he
// has the bypass.
func TestEntityTargetRetaliation(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Bob", false)
	w := env.Ess.Server.Overworld()
	bob := env.Player("Bob")
	var reasons []plugin.TargetReason
	plugin.Listen(env.Ess.Server, func(ev *plugin.EntityTargetEvent) {
		if ev.Target != nil {
			reasons = append(reasons, ev.Reason)
		}
	}, plugin.ListenOptions{Priority: plugin.Lowest})
	env.Console("summon wolf 3 64 0")
	env.Drain(c)
	var wolf plugin.Mob
	for e := range w.Entities() {
		if e.Type() == registry.EntityTypeWolf {
			wolf = e.(plugin.Mob)
		}
	}
	if wolf == nil {
		t.Fatal("no wolf")
	}
	hit := func() {
		c.Attack(wolf.ID())
		env.Messages(c, 10)
	}
	env.Ess.Config.Set("protect.prevent.entitytarget", true)
	hit()
	if !slices.Contains(reasons, plugin.TargetAttacked) {
		t.Fatalf("no TargetAttacked event; reasons %v", reasons)
	}
	if tgt, ok := wolf.Target(); ok {
		t.Fatalf("the wolf targets %v with prevent.entitytarget", tgt.UUID())
	}
	env.Ess.Config.Set("protect.prevent.entitytarget", false)
	hit()
	if tgt, ok := wolf.Target(); !ok || tgt.UUID() != bob.UUID() {
		t.Fatal("the wolf does not turn on Bob once allowed")
	}
}

// TestLavaItemDamage checks that lava-itemdamage keeps an item in lava.
func TestLavaItemDamage(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Bob", false)
	w := env.Ess.Server.Overworld()
	lava := plugin.BlockPos{X: 6, Y: 64, Z: 6}
	w.SetBlockState(lava, plugin.DefaultState(registry.BlockLava), plugin.UpdateClients)
	env.Ess.Config.Set("protect.prevent.lava-itemdamage", true)
	item := w.DropItem(plugin.Vec3{X: 6.5, Y: 64.2, Z: 6.5}, plugin.ItemStack{Item: registry.ItemStick, Count: 1}, false)
	env.Messages(c, 40)
	if !item.IsValid() {
		t.Fatal("the lava burnt the item with lava-itemdamage")
	}
	env.Ess.Config.Set("protect.prevent.lava-itemdamage", false)
	env.Messages(c, 40)
	if item.IsValid() {
		t.Fatal("the item survived the lava without lava-itemdamage")
	}
}
