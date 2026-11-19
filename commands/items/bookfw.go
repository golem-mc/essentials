package items

import (
	"maps"
	"math"
	"strconv"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/nbt"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// component reads a component kept as SNBT in the stack's Other.
func component(s plugin.ItemStack, name string) (nbt.Compound, bool) {
	raw, ok := s.Patch.Other[plugin.MinecraftKey(name)]
	if !ok || raw == "" {
		return nil, false
	}
	tag, _, err := nbt.ParseValueTag(raw)
	if err != nil {
		return nil, false
	}
	c, ok := tag.(nbt.Compound)
	if !ok {
		if ct, isCT := tag.(nbt.CompoundTag); isCT {
			return ct.Entries(), true
		}
	}
	return c, ok
}

// setComponent writes a component as SNBT; nil removes it.
func setComponent(s *plugin.ItemStack, name string, c nbt.Tag) {
	other := maps.Clone(s.Patch.Other)
	if other == nil {
		other = map[plugin.Key]string{}
	}
	if c == nil {
		delete(other, plugin.MinecraftKey(name))
	} else {
		other[plugin.MinecraftKey(name)] = nbt.Stringify(c)
	}
	s.Patch.Other = other
}

// filterable reads a filterable string: "x" or {raw:"x"}.
func filterable(t nbt.Tag) string {
	switch v := t.(type) {
	case nbt.String:
		return string(v)
	case nbt.Compound:
		return filterable(v["raw"])
	case nbt.CompoundTag:
		return filterable(v.Entries()["raw"])
	}
	return ""
}

// componentText answers a text component's plain text.
func componentText(t nbt.Tag) string {
	switch v := t.(type) {
	case nbt.String:
		return string(v)
	case nbt.CompoundTag:
		return componentText(v.Entries())
	case nbt.Compound:
		var b strings.Builder
		if s, ok := v["text"].(nbt.String); ok {
			b.WriteString(string(s))
		}
		if extra, ok := v["extra"].(nbt.List); ok {
			for _, e := range extra.Items {
				b.WriteString(componentText(e))
			}
		}
		return b.String()
	case nbt.List:
		var b strings.Builder
		for _, e := range v.Items {
			b.WriteString(componentText(e))
		}
		return b.String()
	}
	return ""
}

func list(t nbt.Tag) []nbt.Tag {
	if l, ok := t.(nbt.List); ok {
		return l.Items
	}
	return nil
}

func compoundList(items []nbt.Tag) nbt.List {
	return nbt.List{Elem: nbt.TypeCompound, Items: items}
}

// Essentials: Commandbook
func runBook(ctx *core.Ctx) error {
	e := ctx.Ess
	inv := ctx.Player.Inventory()
	s := inv.Get(plugin.SlotMainHand)
	player := ctx.User.Name()
	switch s.Item {
	case registry.ItemWrittenBook:
		book, _ := component(s, "written_book_content")
		if book == nil {
			book = nbt.Compound{}
		}
		isAuthor := strings.EqualFold(filterable(book["author"]), player)
		mayEdit := isAuthor || ctx.Has("essentials.book.others")
		switch {
		case len(ctx.Args) > 1 && strings.EqualFold(ctx.Args[0], "author"):
			if !ctx.Has("essentials.book.author") || !mayEdit {
				return core.Tlf("denyChangeAuthor")
			}
			author := strings.TrimSpace(formatString(e, ctx.User, "essentials.book.author", ctx.FinalArg(1)))
			book["author"] = nbt.String(author)
			setComponent(&s, "written_book_content", book)
			inv.Set(plugin.SlotMainHand, s)
			ctx.Send("bookAuthorSet", author)
		case len(ctx.Args) > 1 && strings.EqualFold(ctx.Args[0], "title"):
			if !ctx.Has("essentials.book.title") || !mayEdit {
				return core.Tlf("denyChangeTitle")
			}
			title := strings.TrimSpace(formatString(e, ctx.User, "essentials.book.title", ctx.FinalArg(1)))
			book["title"] = nbt.Compound{"raw": nbt.String(title)}
			setComponent(&s, "written_book_content", book)
			inv.Set(plugin.SlotMainHand, s)
			ctx.Send("bookTitleSet", title)
		default:
			if !mayEdit {
				return core.Tlf("denyBookEdit")
			}
			var pages []nbt.Tag
			for _, pg := range list(book["pages"]) {
				var raw nbt.Tag = pg
				if c, ok := pg.(nbt.Compound); ok && c["raw"] != nil {
					raw = c["raw"]
				}
				pages = append(pages, nbt.Compound{"raw": nbt.String(componentText(raw))})
			}
			out := plugin.NewStack(registry.ItemWritableBook, s.Count)
			setComponent(&out, "writable_book_content", nbt.Compound{"pages": compoundList(pages)})
			inv.Set(plugin.SlotMainHand, out)
			ctx.Send("editBookContents")
		}
	case registry.ItemWritableBook:
		book, _ := component(s, "writable_book_content")
		author := ""
		if !ctx.Has("essentials.book.author") {
			author = player
		}
		var pages []nbt.Tag
		for _, pg := range list(book["pages"]) {
			pages = append(pages, nbt.Compound{"raw": nbt.String(filterable(pg))})
		}
		out := plugin.NewStack(registry.ItemWrittenBook, s.Count)
		setComponent(&out, "written_book_content", nbt.Compound{"title": nbt.Compound{"raw": nbt.String("")},
			"author": nbt.String(author), "pages": compoundList(pages)})
		inv.Set(plugin.SlotMainHand, out)
		ctx.Send("bookLocked")
	default:
		return core.Tlf("holdBook")
	}
	return nil
}

// dyeColors are Bukkit's DyeColor firework colours. Essentials:
// MetaItemStack.colorMap
var dyeColors = map[string]int32{
	"WHITE": 0xF0F0F0, "ORANGE": 0xEB8844, "MAGENTA": 0xC354CD, "LIGHT_BLUE": 0x6689D3, "YELLOW": 0xDECF2A,
	"LIME": 0x41CD34, "PINK": 0xD88198, "GRAY": 0x434343, "LIGHT_GRAY": 0xABABAB, "CYAN": 0x287697,
	"PURPLE": 0x7B2FBE, "BLUE": 0x253192, "BROWN": 0x51301A, "GREEN": 0x3B511A, "RED": 0xB3312C, "BLACK": 0x1E1B1B,
}

var dyeNames = []string{"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray", "light_gray",
	"cyan", "purple", "blue", "brown", "green", "red", "black"}

// fireworkShapes are FireworkEffect.Type's names, as vanilla's shapes.
var fireworkShapes = map[string]string{
	"BALL": "small_ball", "BALL_LARGE": "large_ball", "STAR": "star", "BURST": "burst", "CREEPER": "creeper",
}

// explosion is a firework effect being built. Bukkit: FireworkEffect.Builder
type explosion struct {
	colors, fade   []int32
	shape          string
	trail, flicker bool
}

func (x *explosion) tag() nbt.Compound {
	shape := x.shape
	if shape == "" {
		shape = "small_ball"
	}
	c := nbt.Compound{"shape": nbt.String(shape), "colors": nbt.IntArray(x.colors)}
	if len(x.fade) > 0 {
		c["fade_colors"] = nbt.IntArray(x.fade)
	}
	if x.trail {
		c["has_trail"] = nbt.Byte(1)
	}
	if x.flicker {
		c["has_twinkle"] = nbt.Byte(1)
	}
	return c
}

func fireworkColors(value, key string) ([]int32, error) {
	var out []int32
	for c := range strings.SplitSeq(value, ",") {
		if rgb, ok := dyeColors[strings.ToUpper(c)]; ok {
			out = append(out, rgb)
		} else if len(c) == 7 && c[0] == '#' && isHexString(c[1:]) {
			n, _ := strconv.ParseInt(c[1:], 16, 32)
			out = append(out, int32(n))
		} else {
			return nil, core.Tlf("invalidFireworkFormat", value, key)
		}
	}
	return out, nil
}

// fireworkMeta builds firework effects from meta words onto a rocket.
// Essentials: MetaItemStack.addFireworkMeta
type fireworkMeta struct {
	ctx   *core.Ctx
	stack *plugin.ItemStack
	cur   explosion
	valid bool
}

func (f *fireworkMeta) explosions() (nbt.Compound, []nbt.Tag) {
	fw, _ := component(*f.stack, "fireworks")
	if fw == nil {
		fw = nbt.Compound{}
	}
	return fw, list(fw["explosions"])
}

// flush adds the effect built so far to the rocket.
func (f *fireworkMeta) flush() error {
	fw, expl := f.explosions()
	expl = append(expl, f.cur.tag())
	if len(expl) > 1 && !f.ctx.Has("essentials.itemspawn.meta-firework-multiple") {
		return core.Tlf("multipleCharges")
	}
	fw["explosions"] = compoundList(expl)
	setComponent(f.stack, "fireworks", fw)
	f.cur = explosion{}
	return nil
}

func (f *fireworkMeta) add(word string) error {
	key, value, ok := strings.Cut(word, ":")
	if !ok {
		return nil
	}
	switch strings.ToLower(key) {
	case "color", "colour", "c":
		if f.valid {
			if !f.ctx.Has("essentials.itemspawn.meta-firework") {
				return core.Tlf("noMetaFirework")
			}
			if err := f.flush(); err != nil {
				return err
			}
		}
		colors, err := fireworkColors(value, key)
		if err != nil {
			return err
		}
		f.valid = true
		f.cur.colors = append(f.cur.colors, colors...)
	case "shape", "type", "s", "t":
		v := strings.ToUpper(value)
		if v == "LARGE" {
			v = "BALL_LARGE"
		}
		shape, ok := fireworkShapes[v]
		if !ok {
			return core.Tlf("invalidFireworkFormat", value, key)
		}
		f.cur.shape = shape
	case "fade", "f":
		colors, err := fireworkColors(value, key)
		if err != nil {
			return err
		}
		f.cur.fade = append(f.cur.fade, colors...)
	case "effect", "e":
		for fx := range strings.SplitSeq(value, ",") {
			switch strings.ToLower(fx) {
			case "twinkle":
				f.cur.flicker = true
			case "trail":
				f.cur.trail = true
			default:
				return core.Tlf("invalidFireworkFormat", value, key)
			}
		}
	}
	return nil
}

// Essentials: Commandfirework
func runFirework(ctx *core.Ctx) error {
	e := ctx.Ess
	if len(ctx.Args) == 0 {
		return core.ErrNotEnoughArgs
	}
	slot, s := heldSlot(ctx.Player)
	if s.Item != registry.ItemFireworkRocket {
		return core.Tlf("holdFirework")
	}
	inv := ctx.Player.Inventory()
	arg := strings.ToLower(ctx.Args[0])
	switch {
	case arg == "clear":
		if fw, ok := component(s, "fireworks"); ok {
			delete(fw, "explosions")
			setComponent(&s, "fireworks", fw)
		}
		inv.Set(slot, s)
		ctx.Send("fireworkEffectsCleared")
	case len(ctx.Args) > 1 && (arg == "power" || arg == "p"):
		power, err := strconv.Atoi(ctx.Args[1])
		if err != nil || power < 0 {
			return core.Tlf("invalidFireworkFormat", ctx.Args[1], ctx.Args[0])
		}
		fw, _ := component(s, "fireworks")
		if fw == nil {
			fw = nbt.Compound{}
		}
		fw["flight_duration"] = nbt.Byte(min(power, 4))
		setComponent(&s, "fireworks", fw)
		inv.Set(slot, s)
	case (arg == "fire" || arg == "f") && ctx.Has("essentials.firework.fire"):
		amount, direction := 1, false
		if len(ctx.Args) > 1 {
			if n, err := strconv.Atoi(ctx.Args[1]); err == nil {
				amount = n
				if limit := e.Config.Int("spawnmob-limit", 10); amount > limit {
					amount = limit
					ctx.Send("mobSpawnLimit")
				}
			} else {
				direction = true
			}
		}
		for range amount {
			launchFirework(ctx, s, direction)
		}
	default:
		f := &fireworkMeta{ctx: ctx, stack: &s}
		for _, a := range ctx.Args {
			if err := f.add(a); err != nil {
				ctx.Send("fireworkSyntax")
				return err
			}
		}
		if !f.valid {
			ctx.Send("fireworkSyntax")
			return core.Tlf("fireworkColor")
		}
		if _, expl := f.explosions(); len(expl) > 0 && !ctx.Has("essentials.firework.multiple") {
			return core.Tlf("multipleCharges")
		}
		fw, expl := f.explosions()
		fw["explosions"] = compoundList(append(expl, f.cur.tag()))
		setComponent(&s, "fireworks", fw)
		inv.Set(slot, s)
	}
	return nil
}

// launchFirework spawns a rocket with the held stack's effects at the
// player, or flying where they look.
func launchFirework(ctx *core.Ctx, s plugin.ItemStack, direction bool) {
	p := ctx.Player
	rocket := s
	rocket.Count = 1
	fw, _ := component(s, "fireworks")
	if fw == nil {
		fw = nbt.Compound{}
	}
	var motion plugin.Vec3
	if direction {
		if d, ok := fw["flight_duration"].(nbt.Byte); ok && d > 1 {
			fw["flight_duration"] = nbt.Byte(1)
		}
		loc := p.Location()
		yaw, pitch := float64(loc.Yaw)*math.Pi/180, float64(loc.Pitch)*math.Pi/180
		const speed = 0.070
		motion = plugin.Vec3{X: -math.Sin(yaw) * math.Cos(pitch) * speed, Y: -math.Sin(pitch) * speed,
			Z: math.Cos(yaw) * math.Cos(pitch) * speed}
	}
	setComponent(&rocket, "fireworks", fw)
	_, err := p.World().Spawn(registry.EntityTypeFireworkRocket, p.Location(), func(en plugin.Entity) {
		if direction {
			en.SetVelocity(motion)
		}
		if f, ok := en.(plugin.Firework); ok {
			f.SetItem(rocket)
		}
	})
	if err != nil {
		ctx.Ess.Log.Warn("cannot launch firework", "err", err)
	}
}

func completeFirework(ctx *core.Ctx) []string {
	a := ctx.Args
	colorOpts := func(arg, prefix string) []string {
		if i := strings.LastIndex(arg, ","); i >= 0 {
			prefix = arg[:i+1]
		}
		var out []string
		for _, c := range dyeNames {
			out = append(out, prefix+c+",")
		}
		return out
	}
	switch {
	case len(a) == 1:
		if strings.HasPrefix(a[0], "color:") {
			return colorOpts(a[0], "color:")
		}
		out := []string{"clear", "power", "color:"}
		if ctx.Has("essentials.firework.fire") {
			out = append(out, "fire")
		}
		return out
	case len(a) == 2 && a[0] == "power":
		return []string{"1", "2", "3", "4"}
	case len(a) == 2 && a[0] == "fire":
		return []string{"1"}
	case len(a) == 2 && strings.HasPrefix(a[0], "color:"):
		arg := a[1]
		if !strings.HasPrefix(arg, "fade:") {
			arg = "fade:"
		}
		return colorOpts(arg, "fade:")
	case len(a) == 3 && strings.HasPrefix(a[0], "color:"):
		return []string{"shape:star", "shape:ball", "shape:large", "shape:creeper", "shape:burst"}
	case len(a) == 4 && strings.HasPrefix(a[0], "color:"):
		return []string{"effect:trail", "effect:twinkle", "effect:trail,twinkle", "effect:twinkle,trail"}
	}
	return nil
}

func isPotionItem(it registry.Item) bool {
	return it == registry.ItemPotion || it == registry.ItemSplashPotion || it == registry.ItemLingeringPotion ||
		it == registry.ItemTippedArrow
}

// effectNames answers the status effects' names.
func effectNames() []string {
	out := make([]string, 0, len(registry.MobEffects))
	for _, n := range registry.MobEffects {
		out = append(out, strings.TrimPrefix(n, "minecraft:"))
	}
	return out
}

// potionMeta builds a custom effect from meta words onto a potion.
// Essentials: MetaItemStack.addPotionMeta
type potionMeta struct {
	ctx                       *core.Ctx
	stack                     *plugin.ItemStack
	effect                    string
	power, duration           int
	validEffect, validDur     bool
	validPower, splash, added bool
}

func (m *potionMeta) add(word string) error {
	key, value, ok := strings.Cut(word, ":")
	if !ok {
		return nil
	}
	switch strings.ToLower(key) {
	case "effect", "e":
		name := strings.ToLower(strings.TrimPrefix(value, "minecraft:"))
		if _, ok := m.ctx.Ess.Server.Registries().Effect(plugin.MinecraftKey(name)); !ok {
			return core.Tlf("invalidPotionMeta", value)
		}
		if !m.ctx.Has("essentials.potions." + name) {
			return core.Tlf("noPotionEffectPerm", name)
		}
		m.effect, m.validEffect = name, true
	case "power", "p":
		n, err := strconv.Atoi(value)
		if err != nil {
			return core.Tlf("invalidPotionMeta", value)
		}
		if n > 0 && n < 4 {
			n--
		}
		m.power, m.validPower = n, true
	case "amplifier", "a":
		n, err := strconv.Atoi(value)
		if err != nil {
			return core.Tlf("invalidPotionMeta", value)
		}
		m.power, m.validPower = n, true
	case "duration", "d":
		n, err := strconv.Atoi(value)
		if err != nil {
			return core.Tlf("invalidPotionMeta", value)
		}
		if n != -1 {
			n *= 20
		}
		m.duration, m.validDur = n, true
	case "splash", "s":
		m.splash = strings.EqualFold(value, "true")
	}
	if !m.validEffect || !m.validDur || !m.validPower {
		return nil
	}
	pc, _ := component(*m.stack, "potion_contents")
	if pc == nil {
		pc = nbt.Compound{}
	}
	id := "minecraft:" + m.effect
	var effects []nbt.Tag
	for _, t := range list(pc["custom_effects"]) {
		if c, ok := t.(nbt.Compound); ok {
			if cid, _ := c["id"].(nbt.String); string(cid) == id {
				continue
			}
		}
		effects = append(effects, t)
	}
	if len(effects) > 1 && !m.ctx.Has("essentials.potions.multiple") {
		return core.Tlf("multiplePotionEffects")
	}
	effects = append(effects, nbt.Compound{"id": nbt.String(id), "amplifier": nbt.Byte(m.power), "duration": nbt.Int(m.duration)})
	pc["custom_effects"] = compoundList(effects)
	setComponent(m.stack, "potion_contents", pc)
	switch {
	case m.splash && m.stack.Item == registry.ItemPotion:
		m.stack.Item = registry.ItemSplashPotion
	case !m.splash && m.stack.Item == registry.ItemSplashPotion:
		m.stack.Item = registry.ItemPotion
	}
	*m = potionMeta{ctx: m.ctx, stack: m.stack, power: 1, duration: 120, added: true}
	return nil
}

// Essentials: Commandpotion
func runPotion(ctx *core.Ctx) error {
	slot, s := heldSlot(ctx.Player)
	if len(ctx.Args) == 0 {
		var names []string
		for _, n := range effectNames() {
			if ctx.Has("essentials.potions." + n) {
				names = append(names, n)
			}
		}
		return usageWith(ctx, "potions", joinList(names))
	}
	if !isPotionItem(s.Item) {
		return core.Tlf("holdPotion")
	}
	inv := ctx.Player.Inventory()
	switch {
	case strings.EqualFold(ctx.Args[0], "clear"):
		if pc, ok := component(s, "potion_contents"); ok {
			delete(pc, "custom_effects")
			setComponent(&s, "potion_contents", pc)
		}
		inv.Set(slot, s)
	case strings.EqualFold(ctx.Args[0], "apply") && ctx.Has("essentials.potion.apply"):
		pc, _ := component(s, "potion_contents")
		for _, t := range list(pc["custom_effects"]) {
			c, ok := t.(nbt.Compound)
			if !ok {
				continue
			}
			id, _ := c["id"].(nbt.String)
			key, err := plugin.ParseKey(string(id))
			if err != nil {
				continue
			}
			eff, ok := ctx.Ess.Server.Registries().Effect(key)
			if !ok {
				continue
			}
			amp, _ := c["amplifier"].(nbt.Byte)
			dur, _ := c["duration"].(nbt.Int)
			ctx.Player.AddEffect(plugin.EffectInstance{Effect: eff, Amplifier: int(amp), Duration: int(dur), Particles: true, Icon: true})
		}
	case len(ctx.Args) < 3:
		return core.ErrNotEnoughArgs
	default:
		m := &potionMeta{ctx: ctx, stack: &s, power: 1, duration: 120}
		for _, a := range ctx.Args {
			if err := m.add(a); err != nil {
				return err
			}
		}
		if !m.added {
			ctx.Send("invalidPotion")
			return core.ErrNotEnoughArgs
		}
		inv.Set(slot, s)
	}
	return nil
}

func completePotion(ctx *core.Ctx) []string {
	a := ctx.Args
	switch {
	case len(a) == 1:
		out := []string{"clear"}
		if ctx.Has("essentials.potion.apply") {
			out = append(out, "apply")
		}
		for _, n := range effectNames() {
			if ctx.Has("essentials.potions." + n) {
				out = append(out, "effect:"+n)
			}
		}
		return out
	case len(a) == 2 && strings.HasPrefix(a[0], "effect:"):
		return []string{"power:1", "power:2", "power:3", "power:4", "amplifier:0", "amplifier:1", "amplifier:2", "amplifier:3"}
	case len(a) == 3 && strings.HasPrefix(a[0], "effect:"):
		var out []string
		for _, d := range []string{"1", "60", "600", "3600", "86400"} {
			out = append(out, "duration:"+d)
		}
		return out
	case len(a) == 4 && strings.HasPrefix(a[0], "effect:"):
		return []string{"splash:true", "splash:false"}
	}
	return nil
}
