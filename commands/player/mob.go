package player

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/nbt"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// mob is a mob /spawnmob and /spawner know: its Essentials name, plural
// suffix and vanilla entity type. Essentials: Mob
type mob struct {
	name, suffix, key string
	typ               registry.EntityType
}

// mobTable lists Essentials' mobs: name, vanilla type, and the plural
// suffix when it is not "s" ("-" for none). Essentials: Mob
var mobTable = [][3]string{
	{"Chicken", "chicken"}, {"Cow", "cow"}, {"Creeper", "creeper"}, {"Ghast", "ghast"}, {"Giant", "giant"},
	{"Horse", "horse"}, {"Pig", "pig"}, {"PigZombie", "zombified_piglin"}, {"ZombifiedPiglin", "zombified_piglin"},
	{"Sheep", "sheep", "-"}, {"Skeleton", "skeleton"}, {"Slime", "slime"}, {"Spider", "spider"}, {"Squid", "squid"},
	{"Zombie", "zombie"}, {"Wolf", "wolf", "-"}, {"CaveSpider", "cave_spider"}, {"Enderman", "enderman", "-"},
	{"Silverfish", "silverfish", "-"}, {"EnderDragon", "ender_dragon"}, {"Villager", "villager"}, {"Blaze", "blaze"},
	{"MushroomCow", "mooshroom"}, {"MagmaCube", "magma_cube"}, {"Snowman", "snow_golem", "-"}, {"Ocelot", "ocelot"},
	{"IronGolem", "iron_golem"}, {"Wither", "wither"}, {"Bat", "bat"}, {"Witch", "witch"}, {"Boat", "oak_boat"},
	{"AcaciaBoat", "acacia_boat"}, {"DarkOakBoat", "dark_oak_boat"}, {"BirchBoat", "birch_boat"},
	{"JungleBoat", "jungle_boat"}, {"SpruceBoat", "spruce_boat"}, {"MangroveBoat", "mangrove_boat"},
	{"CherryBoat", "cherry_boat"}, {"BambooRaft", "bamboo_raft"}, {"PaleOakBoat", "pale_oak_boat"},
	{"PoplarBoat", "poplar_boat"}, {"Minecart", "minecart"}, {"ChestMinecart", "chest_minecart"},
	{"FurnaceMinecart", "furnace_minecart"}, {"TNTMinecart", "tnt_minecart"}, {"HopperMinecart", "hopper_minecart"},
	{"SpawnerMinecart", "spawner_minecart"}, {"EnderCrystal", "end_crystal"}, {"ExperienceOrb", "experience_orb"},
	{"ArmorStand", "armor_stand"}, {"Endermite", "endermite"}, {"Guardian", "guardian"},
	{"ElderGuardian", "elder_guardian"}, {"Rabbit", "rabbit"}, {"Shulker", "shulker"}, {"PolarBear", "polar_bear"},
	{"WitherSkeleton", "wither_skeleton"}, {"StraySkeleton", "stray"}, {"ZombieVillager", "zombie_villager"},
	{"SkeletonHorse", "skeleton_horse"}, {"ZombieHorse", "zombie_horse"}, {"Donkey", "donkey"}, {"Mule", "mule"},
	{"Evoker", "evoker"}, {"Vex", "vex"}, {"Vindicator", "vindicator"}, {"Llama", "llama"}, {"Husk", "husk"},
	{"Illusioner", "illusioner"}, {"Parrot", "parrot"}, {"Turtle", "turtle"}, {"Phantom", "phantom"},
	{"Cod", "cod", "-"}, {"Salmon", "salmon", "-"}, {"Pufferfish", "pufferfish", "-"},
	{"TropicalFish", "tropical_fish", "-"}, {"Drowned", "drowned"}, {"Dolphin", "dolphin"}, {"Cat", "cat"},
	{"Fox", "fox", "es"}, {"Panda", "panda"}, {"Pillager", "pillager"}, {"Ravager", "ravager"},
	{"TraderLlama", "trader_llama"}, {"WanderingTrader", "wandering_trader"}, {"Bee", "bee"}, {"Stray", "stray"},
	{"Hoglin", "hoglin"}, {"Piglin", "piglin"}, {"Strider", "strider"}, {"Zoglin", "zoglin"},
	{"PiglinBrute", "piglin_brute"}, {"Axolotl", "axolotl"}, {"Goat", "goat"}, {"GlowSquid", "glow_squid"},
	{"Allay", "allay"}, {"Frog", "frog"}, {"Tadpole", "tadpole"}, {"Warden", "warden"}, {"ChestBoat", "oak_chest_boat"},
	{"AcaciaChestBoat", "acacia_chest_boat"}, {"DarkOakChestBoat", "dark_oak_chest_boat"},
	{"BirchChestBoat", "birch_chest_boat"}, {"JungleChestBoat", "jungle_chest_boat"},
	{"SpruceChestBoat", "spruce_chest_boat"}, {"MangroveChestBoat", "mangrove_chest_boat"},
	{"CherryChestBoat", "cherry_chest_boat"}, {"BambooChestRaft", "bamboo_chest_raft"},
	{"PaleOakChestBoat", "pale_oak_chest_boat"}, {"PoplarChestBoat", "poplar_chest_boat"}, {"Camel", "camel"},
	{"Sniffer", "sniffer"}, {"Armadillo", "armadillo"}, {"Breeze", "breeze"}, {"Bogged", "bogged"},
	{"Creaking", "creaking"}, {"HappyGhast", "happy_ghast"}, {"CopperGolem", "copper_golem"},
	{"CamelHusk", "camel_husk"}, {"Nautilus", "nautilus"}, {"ZombieNautilus", "zombie_nautilus"},
	{"Parched", "parched"}, {"SulfurCube", "sulfur_cube"}, {"Cushion", "cushion"},
}

// mobs are the mobs by lower-case name; a type this server lacks is left
// out.
var mobs = func() map[string]mob {
	m := map[string]mob{}
	for _, r := range mobTable {
		suffix := "s"
		switch r[2] {
		case "-":
			suffix = ""
		case "":
		default:
			suffix = r[2]
		}
		if t := entityType(r[1]); t >= 0 {
			m[strings.ToLower(r[0])] = mob{name: r[0], suffix: suffix, key: r[1], typ: t}
		}
	}
	return m
}()

// mobByName finds a mob by its Essentials name, ignoring case.
// Essentials: Mob.fromName
func mobByName(name string) (mob, bool) { m, ok := mobs[strings.ToLower(name)]; return m, ok }

// mobNames answers the mob names, sorted. Essentials: Mob.getMobList
func mobNames() []string {
	out := make([]string, 0, len(mobs))
	for n := range mobs {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}

func registerMobs(e *core.Essentials) { e.RegisterCommand(spawnmobCmd()) }

// mobParts splits "pig:saddle,zombie:baby" into the mob names.
// Essentials: SpawnMob.mobParts
func mobParts(s string) []string {
	var out []string
	for part := range strings.SplitSeq(s, ",") {
		name, _, _ := strings.Cut(part, ":")
		out = append(out, name)
	}
	return out
}

// mobDatas splits "pig:saddle,zombie" into the data of each part, nil
// for a part without a colon. Essentials: SpawnMob.mobData
func mobDatas(s string) []*string {
	var out []*string
	for part := range strings.SplitSeq(s, ",") {
		_, data, ok := strings.Cut(part, ":")
		if !ok {
			out = append(out, nil)
			continue
		}
		data, _, _ = strings.Cut(data, ":")
		out = append(out, &data)
	}
	return out
}

// Essentials: Commandspawnmob
func spawnmobCmd() core.Cmd {
	return core.Cmd{
		Name: "spawnmob",
		Run: func(ctx *core.Ctx) error {
			args := ctx.Args
			if ctx.User == nil {
				if len(args) < 3 {
					return usageWith(ctx, "mobsAvailable", strings.Join(mobNames(), ", "))
				}
				target, err := ctx.Ess.MatchUser(nil, args[2], true, false)
				if err != nil {
					return err
				}
				count, err := parseInt(args[1])
				if err != nil {
					return err
				}
				p, ok := target.Player()
				if !ok {
					return core.ErrPlayerNotFound
				}
				return spawnMobs(ctx, target, p.Location(), mobParts(args[0]), mobDatas(args[0]), count)
			}
			if len(args) == 0 {
				return usageWith(ctx, "mobsAvailable", strings.Join(mobNames(), ", "))
			}
			parts, datas := mobParts(args[0]), mobDatas(args[0])
			count := 1
			if len(args) >= 2 {
				n, err := parseInt(args[1])
				if err != nil {
					return err
				}
				count = n
			}
			if len(parts) > 1 && !ctx.Has("essentials.spawnmob.stack") {
				return core.Tlf("cannotStackMob")
			}
			if len(args) >= 3 {
				target, err := ctx.GetPlayer(2, false)
				if err != nil {
					return err
				}
				p, ok := target.Player()
				if !ok {
					return core.ErrPlayerNotFound
				}
				return spawnMobs(ctx, target, p.Location(), parts, datas, count)
			}
			return spawnMobs(ctx, ctx.User, targetLocation(ctx.Player, 300), parts, datas, count)
		},
		Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 {
				return mobNames()
			}
			return nil
		},
	}
}

// spawnMobs spawns count stacks of the mobs parts names at the safe place
// near loc, each riding the one before, owned by target.
// Essentials: SpawnMob.spawnmob
func spawnMobs(ctx *core.Ctx, target *core.User, loc plugin.Location, parts []string, datas []*string, count int) error {
	e := ctx.Ess
	sloc, err := e.SafeDestination(loc)
	if err != nil {
		return err
	}
	for _, part := range parts {
		m, ok := mobByName(part)
		if !ok {
			return core.Tlf("invalidMob")
		}
		if e.Config.Bool("protect.prevent.spawn."+m.key, false) {
			return core.Tlf("disabledToSpawnMob")
		}
		if ctx.User != nil && !ctx.Has("essentials.spawnmob."+strings.ToLower(m.name)) {
			return core.Tlf("noPermToSpawnMob")
		}
	}
	limit := e.Config.Int("spawnmob-limit", 10)
	effective := limit / len(parts)
	if effective < 1 {
		effective = 1
		if len(parts) > limit {
			parts, datas = parts[:max(limit, 1)], datas[:max(limit, 1)]
		}
	}
	if count > effective {
		count = effective
		ctx.Send("mobSpawnLimit")
	}
	tp, _ := target.Player()
	for range count {
		var below plugin.Entity
		for i, part := range parts {
			m, _ := mobByName(part)
			ent, err := sloc.World.Spawn(m.typ, sloc, nil)
			if err != nil {
				return core.Tlf("unableToSpawnMob")
			}
			defaultMobData(e, m, ent)
			if datas[i] != nil {
				if err := changeMobData(ctx, m, ent, strings.ToLower(*datas[i]), tp); err != nil {
					return err
				}
			}
			if below != nil {
				below.AddPassenger(ent)
			}
			below = ent
		}
	}
	first, _ := mobByName(parts[0])
	total := count * len(parts)
	suffix := first.suffix
	if total == 1 {
		suffix = ""
	}
	ctx.SendText(plugin.Plain(fmt.Sprintf("%d %s%s %s", total, strings.ToLower(first.name), suffix, tl(ctx, "spawned"))))
	return nil
}

// stack answers one item of the vanilla name.
func stack(name string) plugin.ItemStack {
	it, ok := registry.ItemByName("minecraft:" + name)
	if !ok {
		return plugin.ItemStack{}
	}
	return plugin.NewStack(it, 1)
}

// holdWeapon puts an item in a mob's main hand with a 10% drop chance.
func holdWeapon(ent plugin.Entity, item string) {
	if l, ok := ent.(plugin.LivingEntity); ok {
		l.Equipment().Set(plugin.SlotMainHand, stack(item))
		l.Equipment().SetDropChance(plugin.SlotMainHand, 0.1)
	}
}

// defaultMobData gives a new mob what Essentials always gives it.
// Essentials: SpawnMob.defaultMobData
func defaultMobData(e *core.Essentials, m mob, ent plugin.Entity) {
	switch m.key {
	case "skeleton":
		holdWeapon(ent, "bow")
	case "zombified_piglin":
		holdWeapon(ent, "golden_sword")
	case "horse":
		if l, ok := ent.(plugin.LivingEntity); ok {
			if id, ok := e.Server.Registries().Attribute(plugin.MinecraftKey("jump_strength")); ok {
				if a, ok := l.Attribute(id); ok {
					a.SetBase(1.2)
				}
			}
		}
	}
}

// zombieKinds are Bukkit's Zombie subclasses.
var zombieKinds = []string{"zombie", "husk", "drowned", "zombie_villager", "zombified_piglin"}

func kindOf(ent plugin.Entity) string { return strings.TrimPrefix(ent.Type().Name(), "minecraft:") }

// setBaby makes a mob a baby or an adult; zombies are Ageable too.
func setBaby(ent plugin.Entity, baby bool) {
	if a, ok := ent.(plugin.Ageable); ok {
		a.SetBaby(baby)
	}
}

// changeMobData applies a part's data ("baby", "red", "saddle"...) to a
// new mob. Essentials: SpawnMob.changeMobData
func changeMobData(ctx *core.Ctx, m mob, ent plugin.Entity, data string, target plugin.Player) error {
	e := ctx.Ess
	if data == "" {
		var help []string
		for _, d := range possibleData(ent, true) {
			help = append(help, strings.Join(d.suggestions, ", "))
		}
		ctx.Send("mobDataList", strings.Join(help, ", "))
	}
	kind := kindOf(ent)
	isZombie := slices.Contains(zombieKinds, kind)
	setBaby(ent, false)
	if (isZombie || kind == "skeleton") && (strings.Contains(data, "armor") || strings.Contains(data, "armour")) {
		if l, ok := ent.(plugin.LivingEntity); ok {
			eq := l.Equipment()
			set := func(material string) {
				eq.Set(plugin.SlotFeet, stack(material+"_boots"))
				eq.Set(plugin.SlotLegs, stack(material+"_leggings"))
				eq.Set(plugin.SlotChest, stack(material+"_chestplate"))
				eq.Set(plugin.SlotHead, stack(material+"_helmet"))
			}
			switch {
			case strings.Contains(data, "noarmor") || strings.Contains(data, "noarmour"):
				for _, s := range []plugin.EquipmentSlot{plugin.SlotMainHand, plugin.SlotOffHand, plugin.SlotFeet,
					plugin.SlotLegs, plugin.SlotChest, plugin.SlotHead} {
					eq.Set(s, plugin.ItemStack{})
				}
			case strings.Contains(data, "netherite"):
				set("netherite")
			case strings.Contains(data, "copper"):
				set("copper")
			case strings.Contains(data, "diamond"):
				set("diamond")
			case strings.Contains(data, "gold"):
				set("golden")
			case strings.Contains(data, "leather"):
				set("leather")
			case strings.Contains(data, "iron"):
				set("iron")
			}
			for _, s := range []plugin.EquipmentSlot{plugin.SlotFeet, plugin.SlotLegs, plugin.SlotChest, plugin.SlotHead} {
				eq.SetDropChance(s, 0)
			}
		}
	}
	for {
		d, ok := dataFor(ent, data)
		if !ok {
			return nil
		}
		matched, err := d.set(e, ent, target, data)
		if err != nil {
			return err
		}
		if matched == "" {
			matched = d.nick
		}
		if matched == "" {
			return nil
		}
		next := strings.ReplaceAll(data, matched, "")
		if next == data {
			return nil
		}
		data = next
	}
}

// mobDatum is a word /spawnmob's data takes for some mobs. Essentials: MobData
type mobDatum struct {
	nick        string
	suggestions []string
	applies     func(ent plugin.Entity) bool
	public      bool
	// set applies the datum, answering the text it used up when that is not
	// the nickname.
	set func(e *core.Essentials, ent plugin.Entity, target plugin.Player, raw string) (string, error)
}

// possibleData answers the data that apply to ent, one per nickname in the
// table's order, a later entry replacing an earlier one.
// Essentials: MobData.getPossibleData
func possibleData(ent plugin.Entity, publicOnly bool) []mobDatum {
	var out []mobDatum
	for _, d := range mobDataTable {
		if publicOnly && !d.public || !d.applies(ent) {
			continue
		}
		if i := slices.IndexFunc(out, func(o mobDatum) bool { return o.nick == d.nick }); i >= 0 {
			out[i] = d
		} else {
			out = append(out, d)
		}
	}
	return out
}

// dataFor finds the datum whose suggestion data contains; a numeric
// suggestion ("<1-100>") takes a number. Essentials: MobData.fromData
func dataFor(ent plugin.Entity, data string) (mobDatum, bool) {
	if data == "" {
		return mobDatum{}, false
	}
	for _, d := range possibleData(ent, false) {
		for _, s := range d.suggestions {
			if strings.Contains(data, s) || strings.HasPrefix(s, "<") && core.IsInt(data) {
				return d, true
			}
		}
	}
	return mobDatum{}, false
}

// Type predicates for the data table.
func ofType(names ...string) func(plugin.Entity) bool {
	return func(ent plugin.Entity) bool { return slices.Contains(names, kindOf(ent)) }
}

func isAgeable(ent plugin.Entity) bool { _, ok := ent.(plugin.Ageable); return ok }

// isTameable matches Bukkit's Tameable (the horse kinds too, not the fox).
func isTameable(ent plugin.Entity) bool { _, ok := ent.(plugin.Tameable); return ok }

var (
	isColorable = ofType("sheep", "shulker")
	isZombie    = ofType(zombieKinds...)
	isRaider    = ofType("pillager", "vindicator", "evoker", "illusioner", "ravager", "witch")
	isSlime     = ofType("slime", "magma_cube")
)

// dyeColors are the dye colours in vanilla's order.
var dyeColors = []string{"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray", "light_gray",
	"cyan", "purple", "blue", "brown", "green", "red", "black"}

// mergeData merges fields into a mob's saved data, as /data merge entity
// does, for data the API has no typed setter for.
func mergeData(ent plugin.Entity, fields nbt.Compound) bool {
	m, ok := ent.(plugin.Mob)
	return ok && m.MergeData(fields)
}

// merge answers a setter that merges one field into the mob's data.
func merge(key string, v nbt.Tag) func(*core.Essentials, plugin.Entity, plugin.Player, string) (string, error) {
	return func(_ *core.Essentials, ent plugin.Entity, _ plugin.Player, _ string) (string, error) {
		mergeData(ent, nbt.Compound{key: v})
		return "", nil
	}
}

// variant answers a setter of a registry variant ("minecraft:warm").
func variant(name string) func(*core.Essentials, plugin.Entity, plugin.Player, string) (string, error) {
	return func(_ *core.Essentials, ent plugin.Entity, _ plugin.Player, _ string) (string, error) {
		if v, ok := ent.(plugin.VariantHolder); ok {
			v.SetVariant(plugin.MinecraftKey(name))
		} else { // the zombie nautilus
			mergeData(ent, nbt.Compound{"variant": nbt.String("minecraft:" + name)})
		}
		return "", nil
	}
}

// tameTo tames a mob to the target player. Bukkit: Tameable.setTamed,
// setOwner
func tameTo(ent plugin.Entity, target plugin.Player) {
	t, ok := ent.(plugin.Tameable)
	if !ok {
		return
	}
	t.SetTamed(true)
	if target != nil {
		t.SetOwner(target.UUID())
	}
}

// saddle tames a mob to the target and saddles it.
func saddle(tame bool) func(*core.Essentials, plugin.Entity, plugin.Player, string) (string, error) {
	return func(_ *core.Essentials, ent plugin.Entity, target plugin.Player, _ string) (string, error) {
		if tame {
			tameTo(ent, target)
		}
		if l, ok := ent.(plugin.LivingEntity); ok {
			l.Equipment().Set(plugin.SlotSaddle, stack("saddle"))
		}
		return "", nil
	}
}

// horseColor and horseStyle set a horse's colour and markings.
// Bukkit: Horse.setColor, setStyle
func horseColor(c plugin.HorseColor) func(*core.Essentials, plugin.Entity, plugin.Player, string) (string, error) {
	return func(_ *core.Essentials, ent plugin.Entity, _ plugin.Player, _ string) (string, error) {
		if h, ok := ent.(plugin.Horse); ok {
			h.SetColor(c)
		}
		return "", nil
	}
}

func horseStyle(m plugin.HorseMarkings) func(*core.Essentials, plugin.Entity, plugin.Player, string) (string, error) {
	return func(_ *core.Essentials, ent plugin.Entity, _ plugin.Player, _ string) (string, error) {
		if h, ok := ent.(plugin.Horse); ok {
			h.SetMarkings(m)
		}
		return "", nil
	}
}

// fishVariant changes bytes of a tropical fish's packed Variant: size and
// pattern (mask 0xffff), body colour (0xff0000), pattern colour
// (0xff000000).
func fishVariant(ent plugin.Entity, value, mask int) {
	m, ok := ent.(plugin.Mob)
	if !ok {
		return
	}
	v, _ := nbt.IntField(m.SavedData(), "Variant")
	m.MergeData(nbt.Compound{"Variant": nbt.Int(int(v)&^mask | value)})
}

func weapon(item string) func(*core.Essentials, plugin.Entity, plugin.Player, string) (string, error) {
	return func(_ *core.Essentials, ent plugin.Entity, _ plugin.Player, _ string) (string, error) {
		holdWeapon(ent, item)
		return "", nil
	}
}

func datum(nick string, applies func(plugin.Entity) bool, public bool,
	set func(*core.Essentials, plugin.Entity, plugin.Player, string) (string, error)) mobDatum {
	return mobDatum{nick: nick, suggestions: []string{nick}, applies: applies, public: public, set: set}
}

// mobDataTable is Essentials' MobData, in its order. Boat variants are left
// out: since 1.21.2 each wood is its own entity type, and Essentials skips
// them there too.
var mobDataTable = buildMobData()

func buildMobData() []mobDatum {
	baby := func(b bool) func(*core.Essentials, plugin.Entity, plugin.Player, string) (string, error) {
		return func(_ *core.Essentials, ent plugin.Entity, _ plugin.Player, _ string) (string, error) {
			setBaby(ent, b)
			return "", nil
		}
	}
	tamed := func(_ *core.Essentials, ent plugin.Entity, target plugin.Player, _ string) (string, error) {
		tameTo(ent, target)
		return "", nil
	}
	color := func(_ *core.Essentials, ent plugin.Entity, _ plugin.Player, raw string) (string, error) {
		c := strings.ToLower(raw)
		i := slices.Index(dyeColors, c)
		switch {
		case c == "random":
			i = rand.IntN(len(dyeColors))
		case c == "":
			return raw, nil
		case i < 0:
			return "", core.Tlf("sheepMalformedColor")
		}
		if sh, ok := ent.(plugin.Sheep); ok {
			sh.SetColor(plugin.DyeColor(i))
		} else { // a shulker
			mergeData(ent, nbt.Compound{"Color": nbt.Byte(i)})
		}
		return raw, nil
	}
	horse := ofType("horse")
	cat := ofType("cat")
	villager := ofType("villager")
	parrot := ofType("parrot")
	fish := ofType("tropical_fish")
	panda := ofType("panda")
	var d []mobDatum
	add := func(x ...mobDatum) { d = append(d, x...) }
	add(datum("baby", isAgeable, true, baby(true)), datum("adult", isAgeable, true, baby(false)),
		datum("kitten", cat, false, baby(true)), datum("piglet", ofType("pig"), false, baby(true)),
		datum("puppy", ofType("wolf"), false, baby(true)), datum("chick", ofType("chicken"), false, baby(true)),
		datum("colt", horse, false, baby(true)), datum("kitten", ofType("ocelot"), false, baby(true)),
		datum("lamb", ofType("sheep"), false, baby(true)), datum("calf", ofType("cow"), false, baby(true)),
		datum("child", villager, false, baby(true)),
		datum("tamed", isTameable, true, tamed), datum("tame", isTameable, false, tamed),
		datum("random", isColorable, true, color),
		mobDatum{nick: "", suggestions: dyeColors, applies: isColorable, public: true, set: color})
	for _, s := range []struct {
		nick   string
		style  plugin.HorseMarkings
		public bool
	}{{"polka", plugin.MarkingsBlackDots, true}, {"sooty", plugin.MarkingsBlackDots, false},
		{"blaze", plugin.MarkingsWhite, true}, {"socks", plugin.MarkingsWhite, false},
		{"leopard", plugin.MarkingsWhiteDots, true}, {"appaloosa", plugin.MarkingsWhiteDots, false},
		{"paint", plugin.MarkingsWhiteField, true}, {"milky", plugin.MarkingsWhiteField, false},
		{"splotchy", plugin.MarkingsWhiteField, false}} {
		add(datum(s.nick, horse, s.public, horseStyle(s.style)))
	}
	for _, c := range []struct {
		nick   string
		color  plugin.HorseColor
		public bool
	}{{"black", plugin.HorseBlack, true}, {"chestnut", plugin.HorseChestnut, true},
		{"liver", plugin.HorseChestnut, false}, {"creamy", plugin.HorseCreamy, true},
		{"flaxen", plugin.HorseCreamy, false}, {"gray", plugin.HorseGray, true}, {"dapple", plugin.HorseGray, false},
		{"buckskin", plugin.HorseDarkBrown, true}, {"darkbrown", plugin.HorseDarkBrown, false},
		{"dark", plugin.HorseDarkBrown, false}, {"dbrown", plugin.HorseDarkBrown, false},
		{"bay", plugin.HorseBrown, true}, {"brown", plugin.HorseBrown, false}} {
		add(datum(c.nick, horse, c.public, horseColor(c.color)))
	}
	add(datum("saddle", horse, true, saddle(true)))
	for _, a := range [][2]string{{"goldarmor", "golden"}, {"diamondarmor", "diamond"}, {"netheritearmor", "netherite"},
		{"armor", "iron"}} {
		item := a[1] + "_horse_armor"
		add(datum(a[0], horse, true, func(_ *core.Essentials, ent plugin.Entity, _ plugin.Player, _ string) (string, error) {
			if t, ok := ent.(plugin.Tameable); ok {
				t.SetTamed(true) // no owner
			}
			if l, ok := ent.(plugin.LivingEntity); ok {
				l.Equipment().Set(plugin.SlotBody, stack(item))
			}
			return "", nil
		}))
	}
	for _, c := range []struct {
		nick, variant string
		public        bool
	}{{"siamese", "siamese", true}, {"white", "white", false}, {"red", "red", true}, {"orange", "red", false},
		{"tabby", "tabby", true}, {"black", "all_black", true}, {"tuxedo", "black", true},
		{"britishshorthair", "british_shorthair", true}, {"calico", "calico", true}, {"persian", "persian", true},
		{"ragdoll", "ragdoll", true}, {"jellie", "jellie", true}, {"allblack", "all_black", true}} {
		add(datum(c.nick, cat, c.public, variant(c.variant)))
	}
	add(datum("baby", isZombie, true, baby(true)), datum("adult", isZombie, true, baby(false)))
	swords := [][3]string{{"netheritesword", "netherite_sword", "1"}, {"diamondsword", "diamond_sword", "1"},
		{"goldsword", "golden_sword", "1"}, {"ironsword", "iron_sword", "1"}, {"stonesword", "stone_sword", "-"},
		{"sword", "stone_sword", "1"}}
	for _, s := range swords {
		add(datum(s[0], isZombie, s[2] != "", weapon(s[1])))
	}
	for _, s := range swords {
		add(datum(s[0], ofType("skeleton"), s[2] != "", weapon(s[1])))
	}
	creeper := ofType("creeper")
	wolf := ofType("wolf")
	powered := func(_ *core.Essentials, ent plugin.Entity, _ plugin.Player, _ string) (string, error) {
		if c, ok := ent.(plugin.Creeper); ok {
			c.SetPowered(true)
		}
		return "", nil
	}
	add(datum("bow", ofType("skeleton"), true, weapon("bow")),
		datum("powered", creeper, true, powered), datum("electric", creeper, false, powered),
		datum("charged", creeper, false, powered),
		datum("saddle", ofType("pig"), true, saddle(false)),
		datum("angry", wolf, true, merge("AngerTime", nbt.Int(600))), datum("rabid", wolf, false, merge("AngerTime", nbt.Int(600))))
	for _, p := range []string{"none", "armorer", "butcher", "cartographer", "cleric", "farmer", "fisherman", "fletcher",
		"leatherworker", "librarian", "mason", "nitwit", "shepherd", "toolsmith", "weaponsmith"} {
		nick := p
		if p == "none" {
			nick = "villager"
		}
		key := plugin.MinecraftKey(p)
		add(datum(nick, villager, true, func(_ *core.Essentials, ent plugin.Entity, _ plugin.Player, _ string) (string, error) {
			if v, ok := ent.(plugin.Villager); ok {
				v.SetProfession(key)
			}
			return "", nil
		}))
	}
	for _, t := range []string{"desert", "jungle", "plains", "savanna", "snowy", "swamp", "taiga"} {
		key := plugin.MinecraftKey(t)
		add(datum(t, villager, true, func(_ *core.Essentials, ent plugin.Entity, _ plugin.Player, _ string) (string, error) {
			if v, ok := ent.(plugin.Villager); ok {
				v.SetVillagerType(key)
			}
			return "", nil
		}))
	}
	add(mobDatum{nick: "", suggestions: []string{"<1-100>"}, applies: isSlime, public: true,
		set: func(e *core.Essentials, ent plugin.Entity, _ plugin.Player, raw string) (string, error) {
			n, err := strconv.Atoi(raw)
			if err != nil {
				return "", core.Tlf("slimeMalformedSize")
			}
			if sl, ok := ent.(plugin.Slime); ok {
				sl.SetSize(n)
			}
			return raw, nil
		}},
		mobDatum{nick: "", suggestions: []string{"<1-2000000000>"}, applies: ofType("experience_orb"), public: true,
			set: func(_ *core.Essentials, ent plugin.Entity, _ plugin.Player, raw string) (string, error) {
				n, err := strconv.Atoi(raw)
				if err != nil {
					return "", core.Tlf("invalidNumber")
				}
				if o, ok := ent.(plugin.ExperienceOrb); ok {
					o.SetValue(n)
				}
				return raw, nil
			}})
	// vanilla's parrot variants: red_blue, blue, green, yellow_blue, gray
	for i, c := range []string{"red", "blue", "green", "cyan", "gray"} {
		add(datum(c, parrot, true, merge("Variant", nbt.Int(i))))
	}
	patterns := []string{"kob", "sunstreak", "snooper", "dasher", "brinely", "spotty", "flopper", "stripey", "glitter",
		"blockfish", "betty", "clayfish"}
	for i, p := range patterns {
		value := i/6 | (i%6)<<8
		add(datum(p, fish, true, func(_ *core.Essentials, ent plugin.Entity, _ plugin.Player, _ string) (string, error) {
			fishVariant(ent, value, 0xffff)
			return "", nil
		}))
	}
	mooshroom := ofType("mooshroom")
	add(datum("brown", mooshroom, true, merge("Type", nbt.String("brown"))), datum("red", mooshroom, true, merge("Type", nbt.String("red"))))
	genes := []string{"aggressive", "lazy", "worried", "playful", "brown", "weak"}
	for _, g := range genes {
		add(datum(g, panda, true, merge("MainGene", nbt.String(g))))
	}
	for _, g := range genes {
		add(datum(g+"_hidden", panda, true, merge("HiddenGene", nbt.String(g))))
	}
	for _, kind := range []string{"llama", "trader_llama"} {
		for i, c := range []string{"creamy", "white", "brown", "gray"} {
			add(datum(c, ofType(kind), true, merge("Variant", nbt.Int(i))))
		}
	}
	fox := ofType("fox")
	add(datum("red", fox, true, merge("Type", nbt.String("red"))), datum("snow", fox, true, merge("Type", nbt.String("snow"))))
	add(mobDatum{nick: "", suggestions: []string{"<1-100>"}, applies: ofType("phantom"), public: true,
		set: func(_ *core.Essentials, ent plugin.Entity, _ plugin.Player, raw string) (string, error) {
			n, err := strconv.Atoi(raw)
			if err != nil {
				return "", core.Tlf("slimeMalformedSize")
			}
			mergeData(ent, nbt.Compound{"size": nbt.Int(n)})
			return raw, nil
		}},
		datum("leader", isRaider, true, merge("PatrolLeader", nbt.Byte(1))))
	for _, layer := range []struct {
		nick, suffix string
		shift        int
	}{{"fish_body_color", "body", 16}, {"fish_pattern_color", "pattern", 24}} {
		var sugg []string
		for _, c := range dyeColors {
			sugg = append(sugg, c+layer.suffix)
		}
		shift := layer.shift
		add(mobDatum{nick: layer.nick, suggestions: sugg, applies: fish, public: true,
			set: func(_ *core.Essentials, ent plugin.Entity, _ plugin.Player, raw string) (string, error) {
				for i, s := range sugg {
					if strings.Contains(raw, s) {
						fishVariant(ent, i<<shift, 0xff<<shift)
						return s, nil
					}
				}
				return "", nil
			}})
	}
	axolotl := ofType("axolotl")
	for _, a := range []struct {
		nick   string
		v      int
		public bool
	}{{"lucy", 0, true}, {"leucistic", 0, false}, {"pink", 0, false}, {"wild", 1, true}, {"brown", 1, false},
		{"gold", 2, true}, {"cyan", 3, true}, {"blue", 4, true}} {
		add(datum(a.nick, axolotl, a.public, merge("Variant", nbt.Int(a.v))))
	}
	add(datum("screaming", ofType("goat"), true, merge("IsScreamingGoat", nbt.Byte(1))))
	for _, v := range []string{"temperate", "warm", "cold"} {
		add(datum(v, ofType("frog"), true, variant(v)))
	}
	add(datum("saddle", ofType("camel"), true, saddle(true)))
	for _, v := range []string{"pale", "spotted", "snowy", "black", "ashen", "rusty", "woods", "chestnut", "striped"} {
		add(datum(v, wolf, true, variant(v)))
	}
	for _, s := range []string{"small", "medium", "large"} {
		add(datum(s, ofType("salmon"), true, merge("type", nbt.String(s))))
	}
	for _, kind := range []string{"cow", "chicken", "pig"} {
		for _, v := range []string{"temperate", "warm", "cold"} {
			add(datum(v, ofType(kind), true, variant(v)))
		}
	}
	add(datum("saddle", ofType("camel_husk"), true, saddle(true)))
	for _, v := range []string{"temperate", "warm"} {
		add(datum(v, ofType("zombie_nautilus"), true, variant(v)))
	}
	add(datum("saddle", ofType("nautilus"), true, saddle(true)),
		datum("saddle", ofType("zombie_nautilus"), true, saddle(true)))
	return d
}
