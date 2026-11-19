package core

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// itemEntry is an items.json entry: a material and the data that makes the
// stack (a potion, a spawner's mob).
type itemEntry struct {
	Material   string `json:"material"`
	PotionData *struct {
		Type     string `json:"type"`
		Upgraded bool   `json:"upgraded"`
		Extended bool   `json:"extended"`
	} `json:"potionData"`
	Entity    string   `json:"entity"`
	Fallbacks []string `json:"fallbacks"`
}

// ItemDB resolves the item names players type: Essentials' aliases from
// items.json and custom_items.yml, registry names, and /give's syntax.
// Essentials: FlatItemDb
type ItemDB struct {
	ess     *Essentials
	entries map[string]itemEntry
	aliases map[string]string // alias to entry key
	byItem  map[registry.Item][]string
	custom  map[string]string
}

func newItemDB(e *Essentials) (*ItemDB, error) {
	db := &ItemDB{ess: e}
	return db, db.reload()
}

func (db *ItemDB) reload() error {
	raw := string(Resource("items.json"))
	if i := strings.Index(raw, "\n{"); i >= 0 {
		raw = raw[i+1:]
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &all); err != nil {
		return fmt.Errorf("items.json: %w", err)
	}
	db.entries = map[string]itemEntry{}
	db.aliases = map[string]string{}
	db.byItem = map[registry.Item][]string{}
	for k, v := range all {
		var alias string
		if json.Unmarshal(v, &alias) == nil {
			db.aliases[k] = alias
			continue
		}
		var ent itemEntry
		if err := json.Unmarshal(v, &ent); err == nil {
			db.entries[k] = ent
		}
	}
	for alias, key := range db.aliases {
		if ent, ok := db.entries[key]; ok && ent.PotionData == nil && ent.Entity == "" {
			if it, ok := db.material(ent); ok {
				db.byItem[it] = append(db.byItem[it], alias)
			}
		}
	}
	for key, ent := range db.entries {
		if ent.PotionData == nil && ent.Entity == "" {
			if it, ok := db.material(ent); ok {
				db.byItem[it] = append(db.byItem[it], key)
			}
		}
	}
	for it, names := range db.byItem {
		slices.SortFunc(names, func(a, b string) int {
			if len(a) != len(b) {
				return len(a) - len(b)
			}
			return strings.Compare(a, b)
		})
		db.byItem[it] = slices.Compact(names)
	}
	writeDefault(db.ess.DataPath("custom_items.yml"), "custom_items.yml")
	c, err := LoadConfig(db.ess.DataPath("custom_items.yml"), "")
	if err != nil {
		return err
	}
	db.custom = map[string]string{}
	for k, v := range c.Map("aliases") {
		db.custom[strings.ToLower(k)] = strings.ToLower(fmt.Sprint(v))
	}
	return nil
}

// material answers the item of an entry, trying its fallbacks.
func (db *ItemDB) material(ent itemEntry) (registry.Item, bool) {
	for _, m := range append([]string{ent.Material}, ent.Fallbacks...) {
		if it, ok := registry.ItemByName("minecraft:" + strings.ToLower(m)); ok {
			return it, true
		}
	}
	return 0, false
}

// Get answers one of the item name names, with no count set: an
// Essentials alias, an item key (with or without minecraft:), or /give's
// syntax with components. Essentials: ItemDb.get
func (db *ItemDB) Get(name string) (plugin.ItemStack, error) {
	orig := name
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return plugin.ItemStack{}, Tlf("unknownItemName", orig)
	}
	if strings.ContainsAny(name, "[{") {
		s, err := db.ess.Server.Items().Parse(orig, 1)
		if err != nil {
			return plugin.ItemStack{}, Tlf("unknownItemName", orig)
		}
		return s, nil
	}
	name = strings.TrimPrefix(name, "minecraft:")
	name, _, _ = strings.Cut(name, ":") // legacy id:data
	if alias, ok := db.custom[name]; ok {
		name = strings.TrimPrefix(alias, "minecraft:")
	}
	key := name
	if a, ok := db.aliases[name]; ok {
		key = a
	}
	if ent, ok := db.entries[key]; ok {
		return db.stackOf(ent)
	}
	if it, ok := registry.ItemByName("minecraft:" + name); ok {
		return plugin.NewStack(it, 1), nil
	}
	if it, ok := registry.ItemByName(orig); ok {
		return plugin.NewStack(it, 1), nil
	}
	return plugin.ItemStack{}, Tlf("unknownItemName", orig)
}

// GetFor is Get with "hand" and "inhand" meaning the item the player holds.
func (db *ItemDB) GetFor(u *User, name string) (plugin.ItemStack, error) {
	if u != nil && (strings.EqualFold(name, "hand") || strings.EqualFold(name, "inhand")) {
		if p, ok := u.Player(); ok {
			s := p.Inventory().Get(plugin.SlotMainHand)
			if s.Empty() {
				return s, Tlf("itemSellAir")
			}
			return s, nil
		}
	}
	return db.Get(name)
}

func (db *ItemDB) stackOf(ent itemEntry) (plugin.ItemStack, error) {
	it, ok := db.material(ent)
	if !ok {
		return plugin.ItemStack{}, Tlf("unknownItemName", ent.Material)
	}
	s := plugin.NewStack(it, 1)
	if ent.PotionData != nil {
		if id := potionID(ent.PotionData.Type, ent.PotionData.Upgraded, ent.PotionData.Extended); id != "" {
			s.Patch.Other = map[plugin.Key]string{plugin.MinecraftKey("potion_contents"): `{potion:"minecraft:` + id + `"}`}
		}
	}
	if ent.Entity != "" {
		s.Patch.Other = map[plugin.Key]string{plugin.MinecraftKey("block_entity_data"): `{id:"minecraft:mob_spawner",SpawnData:{entity:{id:"minecraft:` + strings.ToLower(ent.Entity) + `"}}}`}
	}
	return s, nil
}

// bukkitPotions maps Bukkit's PotionType names to vanilla potion IDs.
var bukkitPotions = map[string]string{
	"SPEED": "swiftness", "JUMP": "leaping", "INSTANT_HEAL": "healing", "INSTANT_DAMAGE": "harming",
	"REGEN": "regeneration", "UNCRAFTABLE": "", "LUCK": "luck",
}

func potionID(t string, upgraded, extended bool) string {
	id, ok := bukkitPotions[t]
	if !ok {
		id = strings.ToLower(t)
	}
	if id == "" {
		return ""
	}
	switch {
	case upgraded:
		return "strong_" + id
	case extended:
		return "long_" + id
	}
	return id
}

// Names answers an item's aliases, shortest first. Essentials: ItemDb.names
func (db *ItemDB) Names(it registry.Item) []string { return db.byItem[it] }

// Name answers the item's primary name, its key's path.
func (db *ItemDB) Name(it registry.Item) string { return strings.TrimPrefix(it.Name(), "minecraft:") }

// Completions answers names for completion: every item key's path.
func (db *ItemDB) Completions() []string {
	out := make([]string, 0, registry.ItemCount)
	for i := range registry.Item(registry.ItemCount) {
		if i != registry.ItemAir {
			out = append(out, strings.TrimPrefix(i.Name(), "minecraft:"))
		}
	}
	return out
}

// ItemName answers a stack's name for messages: its custom name, else its
// item's key path with spaces. Essentials: ItemDb.name / getType().toString()
func (e *Essentials) ItemName(s plugin.ItemStack) string {
	if s.Patch.CustomName != nil {
		return s.Patch.CustomName.PlainString()
	}
	return strings.ReplaceAll(strings.TrimPrefix(s.Item.Name(), "minecraft:"), "_", " ")
}

// MaxStack answers the stack size /give and /item hand out: the item's
// own, or oversized-stacksize with essentials.oversizedstacks, or
// default-stack-size when set.
func (e *Essentials) MaxStack(p plugin.Player, s plugin.ItemStack) int {
	if p != nil && p.HasPermission("essentials.oversizedstacks") {
		return e.Settings.OversizedStackSize()
	}
	if d := e.Settings.DefaultStackSize(); d > 0 {
		return d
	}
	return e.Server.Items().MaxStackSize(s)
}

// GiveItems puts stacks in the player's inventory, splitting counts above
// the stack size; what does not fit is dropped at their feet when drop is
// set and answered otherwise. Essentials: InventoryWorkaround.addItems
func (e *Essentials) GiveItems(p plugin.Player, drop bool, stacks ...plugin.ItemStack) []plugin.ItemStack {
	oversized := 0
	if p.HasPermission("essentials.oversizedstacks") {
		// A slot holds at most 99 (the item codec's maximum); the server
		// clamps a larger count, so a bigger size would lose items.
		oversized = min(e.Settings.OversizedStackSize(), 99)
	}
	var left []plugin.ItemStack
	for _, s := range stacks {
		if itemMax := max(e.Server.Items().MaxStackSize(s), 1); oversized > itemMax {
			// Inventory.Add splits at the item's own maximum.
			left = append(left, e.addOversized(p.Inventory(), s, oversized)...)
		} else {
			left = append(left, p.Inventory().Add(s)...)
		}
	}
	if drop {
		for _, s := range left {
			p.World().DropItem(p.Pos(), s, false)
		}
		return nil
	}
	return left
}

// addOversized puts s into the 36 storage slots in stacks of up to size,
// filling similar stacks first, and answers what did not fit.
// Essentials: InventoryWorkaround.addOversizedItems
func (e *Essentials) addOversized(inv plugin.Inventory, s plugin.ItemStack, size int) []plugin.ItemStack {
	n := s.Count
	slots := min(inv.Size(), 36)
	for i := 0; i < slots && n > 0; i++ {
		c := inv.Item(i)
		if c.Empty() || c.Count >= size || !e.Server.Items().Similar(c, s) {
			continue
		}
		add := min(size-c.Count, n)
		c.Count += add
		inv.SetItem(i, c)
		n -= add
	}
	for i := 0; i < slots && n > 0; i++ {
		if !inv.Item(i).Empty() {
			continue
		}
		part := s
		part.Count = min(size, n)
		inv.SetItem(i, part)
		n -= part.Count
	}
	if n == 0 {
		return nil
	}
	s.Count = n
	return []plugin.ItemStack{s}
}

// ParseCount reads an amount argument: a positive whole number.
func ParseCount(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, Tlf("numberRequired")
	}
	return n, nil
}
