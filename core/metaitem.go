package core

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// enchantmentAliases are Essentials' enchantment names, to vanilla keys.
// Essentials: Enchantments.ENCHANTMENTS and ALIASENCHANTMENTS
var enchantmentAliases = map[string]string{
	"ad": "power", "af": "flame", "ai": "infinity", "ak": "punch", "alldamage": "sharpness", "alldmg": "sharpness",
	"aquaaffinity": "aqua_affinity", "ardmg": "bane_of_arthropods", "arrowdamage": "power", "arrowkb": "punch",
	"arrowknockback": "punch", "arrowpower": "power", "arrowpunch": "punch", "arthropod": "bane_of_arthropods",
	"baneofarthropod": "bane_of_arthropods", "baneofarthropods": "bane_of_arthropods", "bind": "binding_curse",
	"bindcurse": "binding_curse", "binding": "binding_curse", "bindingcurse": "binding_curse",
	"blastprotect": "blast_protection", "blastprotection": "blast_protection", "blockslootbonus": "fortune",
	"bprotect": "blast_protection", "bprotection": "blast_protection", "breach": "breach", "breath": "respiration",
	"breathing": "respiration", "burst": "wind_burst", "chaneling": "channeling", "chanelling": "channeling",
	"channel": "channeling", "channeling": "channeling", "channelling": "channeling", "cutspeed": "efficiency",
	"d": "unbreaking", "dal": "sharpness", "dar": "bane_of_arthropods", "density": "density", "depth": "depth_strider",
	"depthstrider": "depth_strider", "digspeed": "efficiency", "ds": "efficiency", "du": "smite", "dura": "unbreaking",
	"durability": "unbreaking", "eff": "efficiency", "efficiency": "efficiency", "explosionprotection": "blast_protection",
	"explosionsprotection": "blast_protection", "expprot": "blast_protection", "fa": "fire_aspect",
	"fallprot": "feather_falling", "fallprotection": "feather_falling", "fastcharge": "quick_charge",
	"fastdraw": "quick_charge", "featherfall": "feather_falling", "featherfalling": "feather_falling",
	"fire": "fire_aspect", "firearrow": "flame", "fireaspect": "fire_aspect", "fireprot": "fire_protection",
	"fireprotect": "fire_protection", "fireprotection": "fire_protection", "flame": "flame", "flamearrow": "flame",
	"flameprot": "fire_protection", "flameprotect": "fire_protection", "flameprotection": "fire_protection",
	"fort": "fortune", "fortune": "fortune", "frost": "frost_walker", "frostwalker": "frost_walker",
	"highcrit": "thorns", "highercrit": "thorns", "impale": "impaling", "impaling": "impaling", "infarrows": "infinity",
	"infinite": "infinity", "infinitearrows": "infinity", "infinity": "infinity", "k": "knockback", "kb": "knockback",
	"kback": "knockback", "knockback": "knockback", "launch": "riptide", "lbb": "fortune", "lbm": "looting",
	"looting": "looting", "loyal": "loyalty", "loyalty": "loyalty", "luck": "luck_of_the_sea",
	"luckofsea": "luck_of_the_sea", "luckofseas": "luck_of_the_sea", "lunge": "lunge", "lure": "lure",
	"meleefire": "fire_aspect", "meleeflame": "fire_aspect", "mending": "mending", "minespeed": "efficiency",
	"mobloot": "looting", "mobslootbonus": "looting", "multishot": "multishot", "o": "respiration",
	"oceandamage": "impaling", "oceandmg": "impaling", "oxygen": "respiration", "p": "protection",
	"pe": "blast_protection", "pf": "fire_protection", "pfa": "feather_falling", "pierce": "piercing",
	"piercing": "piercing", "pp": "projectile_protection", "power": "power", "prot": "protection",
	"protect": "protection", "protection": "protection", "punch": "punch", "quickcharge": "quick_charge",
	"quickdraw": "quick_charge", "rapidcharge": "quick_charge", "rapiddraw": "quick_charge", "repair": "mending",
	"respiration": "respiration", "riptide": "riptide", "rod": "lure", "rodlure": "lure", "rp": "fire_aspect",
	"rs": "riptide", "sharp": "sharpness", "sharpness": "sharpness", "silk": "silk_touch", "silktouch": "silk_touch",
	"smite": "smite", "softtouch": "silk_touch", "soulspeed": "soul_speed", "soul_speed": "soul_speed",
	"sweep": "sweeping_edge", "sweeping": "sweeping_edge", "sweepingedge": "sweeping_edge", "swiftsneak": "swift_sneak",
	"thorn": "thorns", "thorns": "thorns", "triplecrossbow": "multishot", "tripleshot": "multishot",
	"unbreaking": "unbreaking", "undeaddamage": "smite", "vanish": "vanishing_curse", "vanishcurse": "vanishing_curse",
	"vanishing": "vanishing_curse", "vanishingcurse": "vanishing_curse", "waterworker": "aqua_affinity",
	"waterwalk": "depth_strider", "waterwalker": "depth_strider", "windburst": "wind_burst",
}

// Enchantment resolves an enchantment name: an Essentials alias or a key.
// Essentials: Enchantments.getByName
func (e *Essentials) Enchantment(name string) (plugin.Enchantment, string, bool) {
	name = strings.ToLower(strings.TrimPrefix(name, "minecraft:"))
	key := name
	if k, ok := enchantmentAliases[strings.ReplaceAll(name, "_", "")]; ok {
		key = k
	}
	if ench, ok := e.Server.Registries().Enchantment(plugin.MinecraftKey(key)); ok {
		return ench, key, true
	}
	return 0, "", false
}

// EnchantmentNames answers the enchantment names for completion.
func (e *Essentials) EnchantmentNames() []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range enchantmentAliases {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// EnchantmentMaxLevel answers an enchantment's max level, 1 if unknown.
func EnchantmentMaxLevel(ench plugin.Enchantment) int {
	if def, ok := registry.EnchantmentEffectsByID(int32(ench)); ok && def.MaxLevel > 0 {
		return int(def.MaxLevel)
	}
	return 1
}

// EnchantmentFits reports whether ench may go on item: the enchantment's
// supported items, or any item for an enchanted book.
func EnchantmentFits(ench plugin.Enchantment, it registry.Item) bool {
	if it == registry.ItemEnchantedBook || it == registry.ItemBook {
		return true
	}
	def, ok := registry.EnchantmentEffectsByID(int32(ench))
	if !ok || def.SupportedItems == "" {
		return true
	}
	return it.InTag(registry.ItemTagOf(strings.TrimPrefix(def.SupportedItems, "#")))
}

// AddEnchantment adds ench at level to s (0 removes it), stored on an
// enchanted book; it refuses a level above the maximum or an item the
// enchantment does not fit unless allowUnsafe, as Bukkit's addEnchantment.
// Essentials: MetaItemStack.addEnchantment
func (e *Essentials) AddEnchantment(s *plugin.ItemStack, ench plugin.Enchantment, level int, allowUnsafe bool) error {
	if !allowUnsafe {
		key, _ := e.Server.Registries().KeyOf(ench)
		if level > EnchantmentMaxLevel(ench) {
			return fmt.Errorf("Enchantment %s: level %d is above the maximum %d", key.Path, level, EnchantmentMaxLevel(ench))
		}
		if !EnchantmentFits(ench, s.Item) {
			return fmt.Errorf("Enchantment %s: cannot be applied to this item", key.Path)
		}
	}
	m := &s.Patch.Enchantments
	if s.Item == registry.ItemEnchantedBook {
		m = &s.Patch.StoredEnchantments
	} else if s.Item == registry.ItemBook && level > 0 {
		s.Item = registry.ItemEnchantedBook
		m = &s.Patch.StoredEnchantments
	}
	if *m == nil {
		*m = map[plugin.Enchantment]int{}
	}
	if level == 0 {
		delete(*m, ench)
		return nil
	}
	(*m)[ench] = level
	return nil
}

// fireworkColors are Bukkit's DyeColor firework colours (RGB).
var fireworkColors = map[string]int32{
	"white": 0xF0F0F0, "orange": 0xEB8844, "magenta": 0xC354CD, "light_blue": 0x6689D3, "yellow": 0xDECF2A,
	"lime": 0x41CD34, "pink": 0xD88198, "gray": 0x434343, "light_gray": 0xABABAB, "cyan": 0x287697,
	"purple": 0x7B2FBE, "blue": 0x253192, "brown": 0x51301A, "green": 0x3B511A, "red": 0xB3312C, "black": 0x1E1B1B,
	"silver": 0xABABAB,
}

var fireworkShapes = map[string]string{
	"ball": "small_ball", "small_ball": "small_ball", "large": "large_ball", "ball_large": "large_ball",
	"large_ball": "large_ball", "star": "star", "creeper": "creeper", "burst": "burst",
}

// MetaItem builds an item's components from Essentials' meta words
// ("name:Big_Sword", "lore:a|b", "sharpness:5", "color:255,0,0",
// "player:Notch", "title:", "author:", "book:", "power:", firework and
// potion words). sender is checked for essentials.itemspawn.meta-<meta>;
// nil (a kit) may use everything. Essentials: MetaItemStack
type MetaItem struct {
	ess    *Essentials
	Stack  plugin.ItemStack
	sender *User
	// firework explosion being built
	fw       map[string]string
	fwColors []string
	fwFade   []string
	fwFx     []string
	fwShape  string
	fwExpl   []string
	fwPower  int
	// potion being built
	potEffect        string
	potPower, potDur int
	potSplash        bool
	potEffects       []string
	validFirework    bool
}

// NewMetaItem starts building meta on s for sender (nil for a kit).
func (e *Essentials) NewMetaItem(s plugin.ItemStack, sender *User) *MetaItem {
	return &MetaItem{ess: e, Stack: s, sender: sender, fwPower: -1}
}

func (m *MetaItem) perm(meta string, graceful, base bool) (bool, error) {
	prefix := "essentials."
	if base {
		prefix = "essentials.itemspawn.meta-"
	}
	if m.sender == nil || m.sender.HasPermission(prefix+meta) {
		return true, nil
	}
	if graceful {
		return false, nil
	}
	return false, Tlf("noMetaPerm", meta)
}

// Parse applies every word from args[from:]. A word starting with [ is the
// rest of the line as vanilla components (/give syntax).
// Essentials: parseStringMeta
func (m *MetaItem) Parse(args []string, from int, allowUnsafe bool) error {
	if from >= len(args) {
		return nil
	}
	if strings.HasPrefix(args[from], "[") || strings.HasPrefix(args[from], "{") {
		if _, err := m.perm("vanilla", false, true); err != nil {
			return err
		}
		if strings.HasPrefix(args[from], "{") {
			return Tlf("noMetaNbtKill")
		}
		key := strings.TrimPrefix(m.Stack.Item.Name(), "minecraft:")
		s, err := m.ess.Server.Items().Parse(key+strings.Join(args[from:], " "), m.Stack.Count)
		if err != nil {
			return err
		}
		m.Stack = s
		return nil
	}
	for _, w := range args[from:] {
		if err := m.Add(w, allowUnsafe); err != nil {
			return err
		}
	}
	return m.finish()
}

// Add applies one meta word. Essentials: addStringMeta
func (m *MetaItem) Add(word string, allowUnsafe bool) error {
	key, value, hasValue := strings.Cut(word, ":")
	lk := strings.ToLower(key)
	text := func(v string) plugin.Text {
		return m.ess.Mini(strings.ReplaceAll(v, "_", " ")).NotItalic()
	}
	s := &m.Stack
	switch {
	case hasValue && lk == "name":
		if ok, err := m.perm("name", false, true); !ok {
			return err
		}
		t := text(value)
		s.Patch.CustomName = &t
	case hasValue && (lk == "lore" || lk == "desc"):
		if ok, err := m.perm("lore", false, true); !ok {
			return err
		}
		s.Patch.Lore = nil
		for line := range strings.SplitSeq(value, "|") {
			s.Patch.Lore = append(s.Patch.Lore, text(line))
		}
	case lk == "custom-model-data" || lk == "cmd":
		if ok, err := m.perm("custom-model-data", false, true); !ok {
			return err
		}
		f, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return Tlf("invalidCustomModelData", value)
		}
		s.Patch.CustomModelData = &plugin.CustomModelData{Floats: []float32{float32(f)}}
	case lk == "unbreakable":
		if ok, err := m.perm("unbreakable", false, true); !ok {
			return err
		}
		v := !hasValue || value == "" || strings.EqualFold(value, "true")
		s.Patch.Unbreakable = &v
	case hasValue && (lk == "player" || lk == "owner"):
		if ok, err := m.perm("head", false, true); !ok {
			return err
		}
		if s.Item != registry.ItemPlayerHead {
			return Tlf("onlyPlayerSkulls")
		}
		s.Patch.Profile = &plugin.Profile{Name: value}
	case hasValue && lk == "book" && (s.Item == registry.ItemWritableBook || s.Item == registry.ItemWrittenBook):
		ok, _ := m.perm("book", true, true)
		ok2, _ := m.perm("chapter-"+strings.ToLower(value), true, true)
		if !ok && !ok2 {
			return Tlf("noChapterMeta")
		}
		pages, err := m.ess.BookChapter(value)
		if err != nil {
			return err
		}
		m.setBook("pages", pages)
	case hasValue && lk == "author" && s.Item == registry.ItemWrittenBook:
		if ok, err := m.perm("author", false, true); !ok {
			return err
		}
		m.setBookField("author", LegacyToMini(value))
	case hasValue && lk == "title" && s.Item == registry.ItemWrittenBook:
		if ok, err := m.perm("title", false, true); !ok {
			return err
		}
		m.setBookField("title", strings.ReplaceAll(value, "_", " "))
	case hasValue && lk == "power" && (s.Item == registry.ItemFireworkRocket):
		if ok, err := m.perm("firework-power", false, true); !ok {
			return err
		}
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 || n > 3 {
			return Tlf("invalidFireworkFormat", value, "power")
		}
		m.fwPower = n
	case hasValue && (lk == "color" || lk == "colour") && isLeather(s.Item):
		rgb, err := parseColor(value)
		if err != nil {
			return Tlf("leatherSyntax")
		}
		s.Patch.DyedColor = &rgb
	case s.Item == registry.ItemFireworkRocket || s.Item == registry.ItemFireworkStar:
		return m.addFirework(lk, value)
	case isPotion(s.Item):
		return m.addPotion(lk, value)
	default:
		ench, name, ok := m.ess.Enchantment(key)
		if !ok {
			return Tlf("unknownItemMeta", key)
		}
		if ok, err := m.perm("enchantments."+name, false, false); !ok {
			return err
		}
		level := -1
		if hasValue {
			if n, err := strconv.Atoi(value); err == nil {
				level = n
			}
		}
		if level < 0 || (!allowUnsafe && level > EnchantmentMaxLevel(ench)) {
			level = EnchantmentMaxLevel(ench)
		}
		return m.ess.AddEnchantment(s, ench, level, allowUnsafe)
	}
	return nil
}

func isLeather(it registry.Item) bool {
	switch it {
	case registry.ItemLeatherHelmet, registry.ItemLeatherChestplate, registry.ItemLeatherLeggings, registry.ItemLeatherBoots,
		registry.ItemLeatherHorseArmor:
		return true
	}
	return false
}

func isPotion(it registry.Item) bool {
	return it == registry.ItemPotion || it == registry.ItemSplashPotion || it == registry.ItemLingeringPotion || it == registry.ItemTippedArrow
}

// parseColor reads "r,g,b", "#rrggbb" or a decimal RGB.
func parseColor(v string) (int32, error) {
	if strings.HasPrefix(v, "#") {
		n, err := strconv.ParseInt(v[1:], 16, 32)
		return int32(n), err
	}
	parts := strings.Split(v, ",")
	if len(parts) == 1 {
		n, err := strconv.ParseInt(v, 10, 32)
		return int32(n), err
	}
	if len(parts) != 3 {
		return 0, fmt.Errorf("bad colour")
	}
	var rgb int32
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 || n > 255 {
			return 0, fmt.Errorf("bad colour")
		}
		rgb = rgb<<8 | int32(n)
	}
	return rgb, nil
}

func (m *MetaItem) setBookField(field, value string) {
	if m.fw == nil {
		m.fw = map[string]string{}
	}
	m.fw["book."+field] = value
	m.writeBook()
}

func (m *MetaItem) setBook(_ string, pages []string) {
	if m.fw == nil {
		m.fw = map[string]string{}
	}
	m.fw["book.pages"] = strings.Join(pages, "\x00")
	m.writeBook()
}

// writeBook writes the written or writable book component from the parts
// set so far.
func (m *MetaItem) writeBook() {
	var pages []string
	if p := m.fw["book.pages"]; p != "" {
		for pg := range strings.SplitSeq(p, "\x00") {
			pages = append(pages, pg)
		}
	}
	if m.Stack.Patch.Other == nil {
		m.Stack.Patch.Other = map[plugin.Key]string{}
	}
	if m.Stack.Item == registry.ItemWritableBook {
		var ps []string
		for _, pg := range pages {
			ps = append(ps, "{raw:"+snbtString(ParseMini(LegacyToMini(pg), m.ess.theme).PlainString())+"}")
		}
		m.Stack.Patch.Other[plugin.MinecraftKey("writable_book_content")] = "{pages:[" + strings.Join(ps, ",") + "]}"
		return
	}
	title := m.fw["book.title"]
	if title == "" {
		title = "Book"
	}
	author := m.fw["book.author"]
	if author == "" {
		author = "Server"
	}
	var ps []string
	for _, pg := range pages {
		ps = append(ps, "{raw:"+snbtString(ParseMini(LegacyToMini(pg), m.ess.theme).PlainString())+"}")
	}
	m.Stack.Patch.Other[plugin.MinecraftKey("written_book_content")] = fmt.Sprintf("{title:{raw:%s},author:%s,pages:[%s]}",
		snbtString(title), snbtString(ParseMini(author, m.ess.theme).PlainString()), strings.Join(ps, ","))
}

// snbtString quotes s as an SNBT string.
func snbtString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// addFirework handles a firework word. Essentials: addFireworkMeta
func (m *MetaItem) addFirework(key, value string) error {
	short := m.Stack.Item == registry.ItemFireworkRocket
	_ = short
	switch key {
	case "color", "colour", "c":
		if len(m.fwColors) > 0 && m.Stack.Item == registry.ItemFireworkRocket {
			m.flushExplosion()
		}
		m.fwColors = strings.Split(strings.ToLower(value), ",")
		m.validFirework = true
	case "shape", "type", "s", "t":
		sh, ok := fireworkShapes[strings.ToLower(value)]
		if !ok {
			return Tlf("invalidFireworkFormat", value, key)
		}
		m.fwShape = sh
	case "fade", "f":
		m.fwFade = strings.Split(strings.ToLower(value), ",")
	case "effect", "e":
		m.fwFx = strings.Split(strings.ToLower(value), ",")
	case "power", "p":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 || n > 3 {
			return Tlf("invalidFireworkFormat", value, key)
		}
		m.fwPower = n
	default:
		return Tlf("invalidFireworkFormat", value, key)
	}
	return nil
}

func colorList(names []string) (string, error) {
	var out []string
	for _, n := range names {
		c, ok := fireworkColors[strings.TrimSpace(n)]
		if !ok {
			return "", Tlf("invalidFireworkFormat", n, "color")
		}
		out = append(out, strconv.Itoa(int(c)))
	}
	return "[I;" + strings.Join(out, ",") + "]", nil
}

func (m *MetaItem) flushExplosion() error {
	if len(m.fwColors) == 0 {
		return nil
	}
	colors, err := colorList(m.fwColors)
	if err != nil {
		return err
	}
	shape := m.fwShape
	if shape == "" {
		shape = "small_ball"
	}
	e := fmt.Sprintf("{shape:%q,colors:%s", shape, colors)
	if len(m.fwFade) > 0 {
		fade, err := colorList(m.fwFade)
		if err != nil {
			return err
		}
		e += ",fade_colors:" + fade
	}
	for _, fx := range m.fwFx {
		switch strings.TrimSpace(fx) {
		case "trail":
			e += ",has_trail:1b"
		case "twinkle", "flicker":
			e += ",has_twinkle:1b"
		}
	}
	m.fwExpl = append(m.fwExpl, e+"}")
	m.fwColors, m.fwFade, m.fwFx, m.fwShape = nil, nil, nil, ""
	return nil
}

// addPotion handles a potion word. Essentials: addPotionMeta
func (m *MetaItem) addPotion(key, value string) error {
	switch key {
	case "effect", "e":
		if ok, err := m.perm("potions."+strings.ToLower(value), true, false); !ok {
			if err == nil {
				err = Tlf("invalidPotionMeta", value)
			}
			return err
		}
		if _, ok := m.ess.Server.Registries().Effect(plugin.MinecraftKey(strings.ToLower(value))); !ok {
			return Tlf("invalidPotionMeta", value)
		}
		if m.potEffect != "" {
			m.flushPotion()
		}
		m.potEffect = strings.ToLower(value)
	case "power", "p":
		n, err := strconv.Atoi(value)
		if err != nil {
			return Tlf("invalidPotionMeta", value)
		}
		m.potPower = max(n-1, 0)
	case "amplifier", "a":
		n, err := strconv.Atoi(value)
		if err != nil {
			return Tlf("invalidPotionMeta", value)
		}
		m.potPower = n
	case "duration", "d":
		n, err := strconv.Atoi(value)
		if err != nil {
			return Tlf("invalidPotionMeta", value)
		}
		m.potDur = n * 20
	case "splash", "s":
		m.potSplash = strings.EqualFold(value, "true") || value == ""
	default:
		return Tlf("invalidPotionMeta", value)
	}
	return nil
}

func (m *MetaItem) flushPotion() {
	if m.potEffect == "" {
		return
	}
	dur := m.potDur
	if dur == 0 {
		dur = 120 * 20
	}
	m.potEffects = append(m.potEffects, fmt.Sprintf("{id:\"minecraft:%s\",amplifier:%db,duration:%d}", m.potEffect, m.potPower, dur))
	m.potEffect, m.potPower, m.potDur = "", 0, 0
}

// finish writes the firework and potion components built from the words.
func (m *MetaItem) finish() error {
	if m.Stack.Item == registry.ItemFireworkRocket || m.Stack.Item == registry.ItemFireworkStar {
		if m.validFirework {
			if ok, err := m.perm("firework", true, true); !ok {
				if err == nil {
					err = Tlf("noMetaFirework")
				}
				return err
			}
		}
		if err := m.flushExplosion(); err != nil {
			return err
		}
		if m.Stack.Patch.Other == nil {
			m.Stack.Patch.Other = map[plugin.Key]string{}
		}
		if m.Stack.Item == registry.ItemFireworkStar {
			if len(m.fwExpl) > 0 {
				m.Stack.Patch.Other[plugin.MinecraftKey("firework_explosion")] = m.fwExpl[0]
			}
		} else if len(m.fwExpl) > 0 || m.fwPower >= 0 {
			if len(m.fwExpl) > 1 {
				if ok, _ := m.perm("firework-multiple", true, true); !ok {
					return Tlf("multipleCharges")
				}
			}
			p := max(m.fwPower, 1)
			m.Stack.Patch.Other[plugin.MinecraftKey("fireworks")] = fmt.Sprintf("{flight_duration:%db,explosions:[%s]}", p, strings.Join(m.fwExpl, ","))
		}
	}
	if isPotion(m.Stack.Item) {
		m.flushPotion()
		if m.potSplash && m.Stack.Item == registry.ItemPotion {
			m.Stack.Item = registry.ItemSplashPotion
		}
		if len(m.potEffects) > 0 {
			if m.Stack.Patch.Other == nil {
				m.Stack.Patch.Other = map[plugin.Key]string{}
			}
			m.Stack.Patch.Other[plugin.MinecraftKey("potion_contents")] = "{custom_effects:[" + strings.Join(m.potEffects, ",") + "]}"
		}
	}
	return nil
}

// BookChapter answers a chapter of book.txt as pages: the lines after
// "#name" up to the next "#", split into pages of 14 lines.
// Essentials: BookInput + BookPager
func (e *Essentials) BookChapter(name string) ([]string, error) {
	b, err := os.ReadFile(e.DataPath("book.txt"))
	if err != nil {
		b = Resource("book.txt")
	}
	var lines []string
	in := false
	for line := range strings.SplitSeq(strings.ReplaceAll(string(b), "\r", ""), "\n") {
		if strings.HasPrefix(line, "#") {
			if in {
				break
			}
			in = strings.EqualFold(strings.TrimSpace(line[1:]), name)
			continue
		}
		if in {
			lines = append(lines, line)
		}
	}
	if !in && len(lines) == 0 {
		return nil, Tlf("bookLocked")
	}
	var pages []string
	for i := 0; i < len(lines); i += 14 {
		pages = append(pages, strings.Join(lines[i:min(i+14, len(lines))], "\n"))
	}
	return pages, nil
}
