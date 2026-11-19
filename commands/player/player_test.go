package player_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/golem-mc/essentials/commands/player"
	"github.com/golem-mc/essentials/internal/esstest"
	"github.com/golem-mc/golem/nbt"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

func init() { esstest.RegisterCore(player.Register) }

// TestFly toggles flight for oneself and for another player.
func TestFly(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Join("Bob", false)
	env.Expect(env.Run(a, "fly"), "Set fly mode enabled for Alice")
	if !env.Player("Alice").AllowFlight() || !env.User("Alice").Data.FlyMode {
		t.Fatal("fly did not enable flight")
	}
	env.Expect(env.Run(a, "fly off"), "Set fly mode disabled")
	if env.Player("Alice").AllowFlight() {
		t.Fatal("fly off left flight on")
	}
	env.Expect(env.Run(a, "fly Bob on"), "for Bob")
	if !env.Player("Bob").AllowFlight() {
		t.Fatal("fly Bob did not enable Bob's flight")
	}
	env.Expect(env.Console("fly Bob"), "Set fly mode disabled for Bob")
}

// health reads a player's health through vanilla's /data.
func health(env *esstest.Env, name string) string {
	out := strings.Join(env.Console("data get entity "+name+" Health"), " ")
	return out[strings.LastIndex(out, ":")+1:]
}

// TestGod turns god mode on and checks that damage is blocked.
func TestGod(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Expect(env.Run(a, "god"), "God mode enabled")
	if !env.User("Alice").Data.GodMode {
		t.Fatal("god did not set god mode")
	}
	before := health(env, "Alice")
	env.Console("damage Alice 5")
	env.Drain(a)
	if after := health(env, "Alice"); after != before {
		t.Fatalf("god mode player took damage: %s -> %s", before, after)
	}
	env.Expect(env.Run(a, "god"), "God mode disabled")
	env.Console("damage Alice 5")
	env.Drain(a)
	if after := health(env, "Alice"); after == before {
		t.Fatalf("damage did nothing after god was turned off: %s", after)
	}
}

// TestHealFeed heals and feeds a hungry, hurt player.
func TestHealFeed(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	p := env.Player("Alice")
	p.SetFood(5)
	env.Expect(env.Run(a, "feed"), "Your appetite was sated.")
	if p.Food() != 20 {
		t.Fatalf("food after feed = %d", p.Food())
	}
	env.User("Alice").Data.Timestamps.LastHeal = 0
	p.SetFood(5)
	env.Console("damage Alice 5")
	if p.Health() >= p.MaxHealth() {
		t.Fatalf("health after damage = %v", p.Health())
	}
	env.Expect(env.Run(a, "heal"), "You have been healed.")
	if p.Food() != 20 || p.Health() != p.MaxHealth() {
		t.Fatalf("after heal: food %d, health %v of %v", p.Food(), p.Health(), p.MaxHealth())
	}
	// An operator holds essentials.heal.cooldown.bypass, so a second heal
	// at once is allowed, as in Essentials.
	env.Expect(env.Run(a, "heal"), "You have been healed.")
}

// TestGamemode sets modes by name, alias label and number, and refuses a
// mode without its permission.
func TestGamemode(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	b := env.Join("Bob", false)
	env.Expect(env.Run(a, "gamemode creative"), "Set game mode creative for Alice")
	if env.Player("Alice").GameMode() != plugin.Creative {
		t.Fatal("gamemode creative did not apply")
	}
	env.Expect(env.Run(a, "gmc"), "Set game mode creative")
	env.Expect(env.Run(a, "gms"), "Set game mode survival")
	env.Expect(env.Run(a, "gma Bob"), "for Bob")
	if env.Player("Bob").GameMode() != plugin.Adventure {
		t.Fatal("gma Bob did not apply")
	}
	env.Expect(env.Console("gmsp Bob"), "spectator")
	env.Run(b, "gmc")
	if env.Player("Bob").GameMode() == plugin.Creative {
		t.Fatal("a player without essentials.gamemode changed mode")
	}
}

// TestExp sets, gives, takes and shows experience.
func TestExp(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Expect(env.Run(a, "exp set 100"), "Alice now has 100 exp")
	p := env.Player("Alice")
	if p.Level() != 7 {
		t.Fatalf("level at 100 exp = %d", p.Level())
	}
	env.Expect(env.Run(a, "exp give 5l"), "now has")
	if p.Level() != 12 {
		t.Fatalf("level after give 5l = %d", p.Level())
	}
	env.Expect(env.Console("exp take Alice 1000"), "Alice now has 0 exp")
	env.Expect(env.Run(a, "exp"), "Alice has 0 exp (level 0) and needs 7 more exp to level up.")
}

// TestVanish hides a player from others without essentials.vanish.see.
func TestVanish(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Join("Bob", false)
	env.Expect(env.Run(a, "vanish"), "You are now completely invisible")
	if !env.User("Alice").Vanished {
		t.Fatal("vanish did not vanish")
	}
	if env.Player("Bob").CanSee(env.Player("Alice")) {
		t.Fatal("Bob still sees the vanished Alice")
	}
	env.Expect(env.Run(a, "vanish off"), "disabled")
	if !env.Player("Bob").CanSee(env.Player("Alice")) {
		t.Fatal("Bob does not see Alice after unvanish")
	}
}

// countType counts the loaded entities of a vanilla type.
func countType(env *esstest.Env, name string) (n int, ents []plugin.Entity) {
	for ent := range env.Ess.Server.Overworld().Entities() {
		if ent.Type().Name() == "minecraft:"+name {
			n++
			ents = append(ents, ent)
		}
	}
	return n, ents
}

// TestSpawnmob spawns mobs with data, riders and the limit.
func TestSpawnmob(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Expect(env.Run(a, "spawnmob sheep:red 2 Alice"), "2 sheep spawned")
	n, sheep := countType(env, "sheep")
	if n != 2 {
		t.Fatalf("sheep = %d", n)
	}
	out := strings.Join(env.Console("data get entity "+sheep[0].UUID().String()+" Color"), " ")
	if !strings.Contains(out, "14b") {
		t.Fatalf("sheep colour: %s", out)
	}
	env.Expect(env.Run(a, "spawnmob pig,zombie 1 Alice"), "2 pigs spawned")
	_, pigs := countType(env, "pig")
	if len(pigs) != 1 || len(pigs[0].Passengers()) != 1 || pigs[0].Passengers()[0].Type().Name() != "minecraft:zombie" {
		t.Fatal("the zombie does not ride the pig")
	}
	env.Expect(env.Run(a, "spawnmob cow 50 Alice"), "Mob quantity limited")
	if n, _ := countType(env, "cow"); n != 10 {
		t.Fatalf("cows = %d, want the spawnmob-limit 10", n)
	}
	env.Expect(env.Run(a, "spawnmob nosuchmob 1 Alice"), "Invalid mob type.")
}

// TestTimeWeather sets the world's time and weather.
func TestTimeWeather(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Expect(env.Run(a, "time set noon"), "The time was set to 6000 ticks in: world")
	if d := env.Ess.Server.Overworld().DayTime() % 24000; d < 6000 || d > 6100 {
		t.Fatalf("daytime = %d", d)
	}
	env.Expect(env.Run(a, "night"), "14000 ticks")
	env.Expect(env.Run(a, "etime"), "20:00")
	env.Expect(env.Run(a, "eweather storm 60"), "storm")
	if env.Ess.Server.Overworld().Weather() == plugin.WeatherClear {
		t.Fatal("weather storm left it clear")
	}
	env.Expect(env.Console("eweather world sun"), "sun")
	if env.Ess.Server.Overworld().Weather() != plugin.WeatherClear {
		t.Fatal("weather sun left it raining")
	}
}

// TestSpeed sets fly and walk speed within the configured maximum.
func TestSpeed(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Expect(env.Run(a, "speed walk 10"), "Set walking speed to 10 for Alice")
	if s := env.Player("Alice").WalkSpeed(); s < 0.79 || s > 1.01 {
		t.Fatalf("walk speed = %v", s)
	}
	env.Expect(env.Run(a, "flyspeed 0.5"), "flying")
	if s := env.Player("Alice").FlySpeed(); s < 0.049 || s > 0.051 {
		t.Fatalf("fly speed = %v", s)
	}
}

// TestTreeRemove grows a tree where the player looks, then removes mobs.
func TestTreeRemove(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	p := env.Player("Alice")
	loc := p.Location()
	loc.Pitch = 90
	p.Teleport(loc, plugin.TeleportPlugin)
	env.Drain(a)
	msgs := env.Run(a, "tree tree")
	env.Expect(msgs, "Tree spawned.")
	if esstest.Has(msgs, "Placed feature") {
		t.Fatalf("the vanilla command's feedback leaked: %q", msgs)
	}
	env.Run(a, "spawnmob cow 3 Alice")
	env.Expect(env.Run(a, "remove animals"), "Removed 3 entities.")
}

// TestPtimePweather sets, shows and resets a player's own time and
// weather, and keeps a relative time across /time set.
func TestPtimePweather(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	p := env.Player("Alice")
	env.Expect(env.Run(a, "ptime get"), "Alice's time is normal")
	env.Expect(env.Run(a, "ptime @noon"), "Player time is fixed to 12:00")
	if ticks, fixed, over := p.PlayerTime(); !over || !fixed || ticks%24000 != 6000 {
		t.Fatalf("after ptime @noon: ticks %d fixed %v overridden %v", ticks, fixed, over)
	}
	env.Expect(env.Run(a, "ptime"), "Alice's time is fixed to 12:00")
	env.Expect(env.Run(a, "ptime midnight"), "Player time is set to 00:00 or 12:00 AM or 18000 ticks for: Alice")
	near := func(ticks, want int64) bool { d := (ticks - want + 24000) % 24000; return d < 100 || d > 23900 }
	if ticks, fixed, over := p.PlayerTime(); !over || fixed || !near(ticks%24000, 18000) {
		t.Fatalf("after ptime midnight: ticks %d fixed %v overridden %v", ticks, fixed, over)
	}
	env.Run(a, "etime set noon")
	if ticks, _, _ := p.PlayerTime(); !near(ticks%24000, 18000) {
		t.Fatalf("time set changed the relative player time to %d", ticks%24000)
	}
	env.Expect(env.Run(a, "ptime reset"), "Player time has been reset for: Alice")
	if _, _, over := p.PlayerTime(); over {
		t.Fatal("ptime reset left the time overridden")
	}

	env.Expect(env.Run(a, "pweather storm"), "Player weather is set to storm for: Alice")
	if raining, over := p.PlayerWeather(); !raining || !over {
		t.Fatalf("after pweather storm: raining %v overridden %v", raining, over)
	}
	env.Expect(env.Run(a, "pweather get"), "Alice's weather is downfall")
	env.Expect(env.Console("pweather sun Alice"), "Player weather is set to sun for: Alice")
	if raining, over := p.PlayerWeather(); raining || !over {
		t.Fatalf("after pweather sun: raining %v overridden %v", raining, over)
	}
	env.Expect(env.Run(a, "pweather reset"), "Player weather has been reset")
	env.Expect(env.Run(a, "pweather"), "Alice's weather is normal")
}

// TestBigtree grows a big tree where the player looks.
func TestBigtree(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	p := env.Player("Alice")
	loc := p.Location()
	loc.Pitch = 90
	p.Teleport(loc, plugin.TeleportPlugin)
	env.Drain(a)
	env.Expect(env.Run(a, "bigtree tree"), "Big tree spawned.")
	env.Expect(env.Run(a, "tree nosuchtree"), "/tree")
}

// TestBurnKill sets a player on fire, kills a god mode player from the
// console, and lets a player kill themselves.
func TestBurnKill(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Join("Bob", false)
	env.Expect(env.Run(a, "burn Bob 5"), "You set Bob on fire for 5 seconds.")
	if f := env.Player("Bob").FireTicks(); f < 90 || f > 100 {
		t.Fatalf("Bob's fire ticks = %d", f)
	}
	env.User("Bob").Data.GodMode = true
	env.Expect(env.Console("ekill Bob"), "Killed Bob.")
	if h := env.Player("Bob").Health(); h != 0 {
		t.Fatalf("Bob's health after kill = %v", h)
	}
	env.Expect(env.Run(a, "suicide"), "Goodbye cruel world...")
	if h := env.Player("Alice").Health(); h != 0 {
		t.Fatalf("Alice's health after suicide = %v", h)
	}
}

// TestEventsCancel checks that /heal, /feed, /kill and /break fire the
// events Essentials fires, and stop when a listener cancels them.
func TestEventsCancel(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	b := env.Join("Bob", false)
	s := env.Ess.Server
	plugin.Listen(s, func(ev *plugin.EntityRegainHealthEvent) { ev.SetCancelled(true) })
	plugin.Listen(s, func(ev *plugin.FoodLevelChangeEvent) { ev.SetCancelled(true) })
	plugin.Listen(s, func(ev *plugin.BlockBreakEvent) { ev.SetCancelled(true) })
	plugin.Listen(s, func(ev *plugin.EntityDamageEvent) {
		if p, ok := ev.Entity.(plugin.Player); ok && p.Name() == "Bob" {
			ev.SetCancelled(true)
		}
	})
	p := env.Player("Alice")
	env.Console("damage Alice 5")
	env.Sync(func() { p.SetFood(5) })
	env.Drain(a)
	if msgs := env.Run(a, "heal"); esstest.Has(msgs, "healed") {
		t.Fatalf("a cancelled heal still healed: %q", msgs)
	}
	if p.Health() >= p.MaxHealth() || p.Food() != 5 {
		t.Fatalf("after cancelled heal: health %v food %d", p.Health(), p.Food())
	}
	if msgs := env.Run(a, "feed"); esstest.Has(msgs, "sated") || p.Food() != 5 {
		t.Fatalf("a cancelled feed fed: %q, food %d", msgs, p.Food())
	}
	// Alice is an operator, so essentials.kill.force kills through the cancel.
	env.Expect(env.Run(a, "kill Bob"), "Killed Bob.")
	if h := env.Player("Bob").Health(); h != 0 {
		t.Fatalf("Bob's health after a forced kill = %v", h)
	}
	env.Drain(b)

	loc := p.Location()
	loc.Pitch = 90
	p.Teleport(loc, plugin.TeleportPlugin)
	env.Drain(a)
	below := p.Location().Pos.Block()
	below.Y--
	before := p.World().BlockState(below)
	env.Run(a, "break")
	if after := p.World().BlockState(below); after != before {
		t.Fatalf("a cancelled break broke the block: %v -> %v", before, after)
	}
}

// TestFireballArrow marks /fireball's arrows so they cannot be picked up.
func TestFireballArrow(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Run(a, "fireball arrow")
	_, arrows := countType(env, "arrow")
	if len(arrows) != 1 {
		t.Fatalf("arrows = %d", len(arrows))
	}
	if v, ok := plugin.GetData[bool](arrows[0].Data(), plugin.Key{Namespace: "essentials", Path: "ess_fireball_proj"}); !ok || !v {
		t.Fatal("the arrow is not marked as a /fireball projectile")
	}
	// SetShooter(player) makes the arrow pickupable as vanilla's setOwner
	// does, so only the listener keeps it from being picked up.
	if ar, ok := arrows[0].(plugin.Arrow); !ok || ar.Pickup() != plugin.PickupAllowed {
		t.Fatal("the /fireball arrow's pickup is not allowed")
	}
}

// TestSpawnmobData applies data through the typed handles, and tames a
// donkey to the target.
func TestSpawnmobData(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	one := func(line, kind string) plugin.Entity {
		t.Helper()
		env.Expect(env.Run(a, line), "spawned")
		_, ents := countType(env, kind)
		if len(ents) != 1 {
			t.Fatalf("%s: %d %s", line, len(ents), kind)
		}
		return ents[0]
	}
	if c := one("spawnmob cat:siamese 1 Alice", "cat").(plugin.VariantHolder); c.Variant().String() != "minecraft:siamese" {
		t.Fatalf("cat variant = %v", c.Variant())
	}
	h := one("spawnmob horse:blackpolka 1 Alice", "horse").(plugin.Horse)
	if h.Color() != plugin.HorseBlack || h.Markings() != plugin.MarkingsBlackDots {
		t.Fatalf("horse colour %v markings %v", h.Color(), h.Markings())
	}
	if c := one("spawnmob creeper:powered 1 Alice", "creeper").(plugin.Creeper); !c.Powered() {
		t.Fatal("creeper not powered")
	}
	if s := one("spawnmob slime:4 1 Alice", "slime").(plugin.Slime); s.Size() != 4 {
		t.Fatalf("slime size = %d", s.Size())
	}
	if z := one("spawnmob zombie:baby 1 Alice", "zombie").(plugin.Ageable); !z.Baby() {
		t.Fatal("zombie not a baby")
	}
	if s := one("spawnmob shulker:red 1 Alice", "shulker"); !strings.Contains(strings.Join(env.Console("data get entity "+s.UUID().String()+" Color"), " "), "14b") {
		t.Fatal("shulker colour not set")
	}
	d := one("spawnmob donkey:tamed 1 Alice", "donkey")
	out := strings.Join(env.Console("data get entity "+d.UUID().String()), " ")
	uuid := strings.Join(env.Console("data get entity Alice UUID"), " ")
	uuid = uuid[strings.Index(uuid, "[I;"):]
	if !strings.Contains(out, "Tame: 1b") || !strings.Contains(out, "Owner: "+uuid) {
		t.Fatalf("donkey not tamed to Alice (%s): %s", uuid, out)
	}
}

// TestKillCause checks the cause of the damage event /kill and /suicide
// fire (SUICIDE for oneself, CUSTOM for another: generic_kill and generic),
// and that /kill kills a god mode player with no further damage event.
func TestKillCause(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Join("Bob", false)
	s := env.Ess.Server
	var causes []string
	plugin.Listen(s, func(ev *plugin.EntityDamageEvent) {
		if _, ok := ev.Entity.(plugin.Player); ok {
			k, _ := s.Registries().KeyOf(ev.Source.Type)
			causes = append(causes, k.Path)
		}
	})
	env.User("Bob").Data.GodMode = true
	env.Expect(env.Run(a, "kill Bob"), "Killed Bob.")
	if h := env.Player("Bob").Health(); h != 0 {
		t.Fatalf("Bob's health after kill = %v", h)
	}
	env.Expect(env.Run(a, "suicide"), "Goodbye cruel world...")
	if want := []string{"generic", "generic_kill"}; !slices.Equal(causes, want) {
		t.Fatalf("damage causes = %q, want %q", causes, want)
	}
}

// TestSpawnmobTame tames horse kinds through Tameable: to the target with
// tamed, without an owner with horse armour; the fox is not tameable.
func TestSpawnmobTame(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Expect(env.Run(a, "spawnmob donkey:tamed 1 Alice"), "spawned")
	_, ds := countType(env, "donkey")
	d, ok := ds[0].(plugin.Tameable)
	if !ok {
		t.Fatal("a donkey is not Tameable")
	}
	if owner, owned := d.Owner(); !d.Tamed() || !owned || owner != env.Player("Alice").UUID() {
		t.Fatalf("donkey tamed %v owner %v (%v), want Alice", d.Tamed(), owner, owned)
	}
	env.Expect(env.Run(a, "spawnmob horse:diamondarmor 1 Alice"), "spawned")
	_, hs := countType(env, "horse")
	h := hs[0].(plugin.Tameable)
	if _, owned := h.Owner(); !h.Tamed() || owned {
		t.Fatalf("armoured horse tamed %v owned %v, want tamed without an owner", h.Tamed(), owned)
	}
	if body := h.Equipment().Get(plugin.SlotBody); body.Item != itemType(t, env, "diamond_horse_armor") {
		t.Fatalf("horse body slot = %v", body)
	}
	env.Expect(env.Run(a, "spawnmob camel:saddle 1 Alice"), "spawned")
	_, cs := countType(env, "camel")
	if owner, owned := cs[0].(plugin.Tameable).Owner(); !owned || owner != env.Player("Alice").UUID() {
		t.Fatal("a saddled camel is not tamed to Alice")
	}
	env.Expect(env.Run(a, "spawnmob fox:tamed 1 Alice"), "spawned")
	if _, fs := countType(env, "fox"); len(fs) != 1 {
		t.Fatalf("foxes = %d", len(fs))
	} else if _, ok := fs[0].(plugin.Tameable); ok {
		t.Fatal("the fox is Tameable (Bukkit's is not)")
	}
}

// itemType answers a vanilla item's type.
func itemType(t *testing.T, env *esstest.Env, name string) registry.Item {
	t.Helper()
	it, ok := env.Ess.Server.Registries().Item(plugin.MinecraftKey(name))
	if !ok {
		t.Fatalf("no item %s", name)
	}
	return it
}

// TestSpawnmobMergeData sets data with no typed setter through Mob.MergeData
// and reads it back with Mob.SavedData.
func TestSpawnmobMergeData(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	saved := func(line, kind string) nbt.Compound {
		t.Helper()
		env.Expect(env.Run(a, line), "spawned")
		_, ents := countType(env, kind)
		if len(ents) != 1 {
			t.Fatalf("%s: %d %s", line, len(ents), kind)
		}
		return ents[0].(plugin.Mob).SavedData()
	}
	if c, _ := nbt.ByteField(saved("spawnmob shulker:red 1 Alice", "shulker"), "Color"); c != 14 {
		t.Fatalf("shulker Color = %d", c)
	}
	if v, _ := nbt.IntField(saved("spawnmob tropicalfish:betty-redbody-limepattern 1 Alice", "tropical_fish"),
		"Variant"); v != 1|4<<8|14<<16|5<<24 {
		t.Fatalf("tropical fish Variant = %#x", v)
	}
	if g := saved("spawnmob panda:lazy 1 Alice", "panda")["MainGene"]; g != nbt.String("lazy") {
		t.Fatalf("panda MainGene = %v", g)
	}
	if l, _ := nbt.ByteField(saved("spawnmob pillager:leader 1 Alice", "pillager"), "PatrolLeader"); l != 1 {
		t.Fatal("pillager not a patrol leader")
	}
	if v, _ := nbt.IntField(saved("spawnmob axolotl:gold 1 Alice", "axolotl"), "Variant"); v != 2 {
		t.Fatalf("axolotl Variant = %d", v)
	}
}

// TestKittycannon launches a baby cat tamed without an owner, as
// Essentials' setTamed(true) does.
func TestKittycannon(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Run(a, "kittycannon")
	_, cats := countType(env, "cat")
	if len(cats) != 1 {
		t.Fatalf("cats = %d", len(cats))
	}
	c := cats[0].(plugin.Tameable)
	if _, owned := c.Owner(); !c.Tamed() || owned || !c.Baby() {
		t.Fatalf("cat tamed %v owned %v baby %v", c.Tamed(), owned, c.Baby())
	}
}

// TestTreeKinds grows tree types the API only gained in c0b90e608
// (Bukkit's COCOA_TREE, TALL_BIRCH, PALE_OAK) through World.GenerateTree.
func TestTreeKinds(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	p := env.Player("Alice")
	start := p.Location()
	start.Pitch = 90
	for i, k := range []struct{ name, log string }{{"cocoatree", "jungle_log"}, {"tallbirch", "birch_log"},
		{"paleoak", "pale_oak_log"}} {
		loc := start
		loc.Pos.X += float64(20 * (i + 1))
		p.Teleport(loc, plugin.TeleportPlugin)
		env.Drain(a)
		at := p.Location().Pos.Block()
		env.Expect(env.Run(a, "tree "+k.name), "Tree spawned.")
		if got, _ := env.Ess.Server.Registries().KeyOf(p.World().BlockState(at).Block()); got.Path != k.log {
			t.Fatalf("tree %s: block at the feet = %v, want %s", k.name, got, k.log)
		}
	}
}

// TestThunder flips only the thunder flag and timer, leaving the rain's
// flag and timer alone (Bukkit: setThundering, setThunderDuration).
func TestThunder(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	w := env.Ess.Server.Overworld()
	env.Expect(env.Run(a, "thunder true"), "enabled")
	if w.Weather() != plugin.WeatherThunder {
		t.Fatalf("weather after thunder true = %v", w.Weather())
	}
	env.Expect(env.Run(a, "thunder false"), "disabled")
	if w.Weather() != plugin.WeatherClear {
		t.Fatalf("thunder in a clear world started rain: weather %v", w.Weather())
	}
	env.Run(a, "eweather storm 100")
	env.Expect(env.Run(a, "thunder true 60"), "60")
	if w.Weather() != plugin.WeatherThunder {
		t.Fatalf("weather after thunder true 60 = %v", w.Weather())
	}
	near := func(d, want time.Duration) bool { return d <= want && d > want-2*time.Second }
	if d := w.ThunderDuration(); !near(d, 60*time.Second) {
		t.Fatalf("thunder duration = %v, want about 60s", d)
	}
	if d := w.RainDuration(); !near(d, 100*time.Second) {
		t.Fatalf("thunder changed the rain duration to %v (want about 100s)", d)
	}
	env.Expect(env.Run(a, "thunder false"), "disabled")
	if w.Weather() != plugin.WeatherRain {
		t.Fatalf("weather after thunder false = %v, want rain", w.Weather())
	}
	if d := w.RainDuration(); !near(d, 100*time.Second) {
		t.Fatalf("thunder false changed the rain duration to %v (want about 100s)", d)
	}

	// /weather is setStorm: rain only, the timer set, thunder left alone.
	env.Run(a, "thunder true 60")
	env.Expect(env.Run(a, "eweather sun 100"), "sun")
	if w.Weather() == plugin.WeatherRain || w.Weather() == plugin.WeatherThunder && w.ThunderDuration() < 50*time.Second {
		t.Fatalf("weather sun: weather %v, thunder left %v", w.Weather(), w.ThunderDuration())
	}
	if d := w.RainDuration(); !near(d, 100*time.Second) {
		t.Fatalf("weather sun 100: clear for %v, want about 100s", d)
	}
	env.Expect(env.Run(a, "eweather storm 30"), "storm")
	if w.Weather() != plugin.WeatherThunder {
		t.Fatalf("weather storm under thunder = %v, want thunder kept", w.Weather())
	}

	// A vanilla forced clear ends when thunder is set .
	env.Console("minecraft:weather clear 100")
	env.Expect(env.Run(a, "thunder true 60"), "60")
	env.Srv.RunTicks(2)
	if d := w.ThunderDuration(); !near(d, 60*time.Second) {
		t.Fatalf("thunder after a forced clear lasts %v, want about 60s", d)
	}
	env.Expect(env.Run(a, "eweather storm 30"), "storm")
	env.Srv.RunTicks(2)
	if w.Weather() != plugin.WeatherThunder {
		t.Fatalf("storm and thunder after a forced clear = %v", w.Weather())
	}
}

// TestPrefixedOverride runs the overriding commands as essentials:<name>
// with arguments: Essentials' command runs, not vanilla's.
func TestPrefixedOverride(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Expect(env.Run(a, "essentials:gamemode creative"), "Set game mode creative for Alice")
	env.Expect(env.Run(a, "essentials:time set noon"), "The time was set to 6000 ticks in: world")
}
