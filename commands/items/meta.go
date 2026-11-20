package items

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/google/uuid"
)

func registerMeta(e *core.Essentials) {
	e.RegisterCommand(
		core.Cmd{Name: "itemlore", PlayerOnly: true, Run: runItemlore, Complete: completeItemlore},
		core.Cmd{Name: "itemname", PlayerOnly: true, Run: runItemname, Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 && ctx.Player != nil {
				if _, s := heldSlot(ctx.Player); !s.Empty() && s.Patch.CustomName != nil {
					return []string{unformatText(ctx.Ess, ctx.User, "essentials.itemname", *s.Patch.CustomName)}
				}
			}
			return nil
		}},
		core.Cmd{Name: "enchant", PlayerOnly: true, Run: runEnchant, Complete: func(ctx *core.Ctx) []string {
			switch len(ctx.Args) {
			case 1:
				return ctx.Ess.EnchantmentNames()
			case 2:
				ench, _, ok := ctx.Ess.Enchantment(ctx.Args[0])
				if !ok {
					return nil
				}
				var out []string
				for i := 1; i <= core.EnchantmentMaxLevel(ench); i++ {
					out = append(out, strconv.Itoa(i))
				}
				return out
			}
			return nil
		}},
		core.Cmd{Name: "hat", PlayerOnly: true, Run: runHat, Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 {
				return []string{"remove", "wear"}
			}
			return nil
		}},
		core.Cmd{Name: "skull", PlayerOnly: true, Run: runSkull, Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) > 2 {
				return nil
			}
			if ctx.Has("essentials.skull.others") {
				return ctx.CompletePlayers()
			}
			return []string{ctx.User.Name()}
		}},
		core.Cmd{Name: "book", PlayerOnly: true, Run: runBook, Complete: func(ctx *core.Ctx) []string {
			switch {
			case len(ctx.Args) == 1:
				out := []string{"sign", "unsign"}
				if ctx.Has("essentials.book.author") {
					out = append(out, "author")
				}
				if ctx.Has("essentials.book.title") {
					out = append(out, "title")
				}
				return out
			case len(ctx.Args) == 2 && strings.EqualFold(ctx.Args[0], "author") && ctx.Has("essentials.book.author"):
				return append(ctx.CompletePlayers(), "Herobrine")
			}
			return nil
		}},
		core.Cmd{Name: "firework", PlayerOnly: true, Run: runFirework, Complete: completeFirework},
		core.Cmd{Name: "potion", PlayerOnly: true, Run: runPotion, Complete: completePotion},
		core.Cmd{Name: "repair", PlayerOnly: true, Run: runRepair, Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) != 1 {
				return nil
			}
			if ctx.Has("essentials.repair.all") {
				return []string{"hand", "all"}
			}
			return []string{"hand"}
		}},
	)
}

// Essentials: Commanditemlore
func runItemlore(ctx *core.Ctx) error {
	e := ctx.Ess
	slot, s := heldSlot(ctx.Player)
	if s.Empty() {
		return core.Tlf("itemloreInvalidItem")
	}
	if len(ctx.Args) == 0 {
		return core.ErrNotEnoughArgs
	}
	sub := strings.ToLower(ctx.Args[0])
	switch {
	case sub == "add" && len(ctx.Args) > 1:
		if len(s.Patch.Lore) >= e.Config.Int("max-itemlore-lines", 10) && !ctx.Has("essentials.itemlore.bypass") {
			return core.Tlf("itemloreMaxLore")
		}
		line := strings.TrimSpace(formatString(e, ctx.User, "essentials.itemlore", ctx.FinalArg(1)))
		s.Patch.Lore = append(slices.Clone(s.Patch.Lore), legacyText(e, line))
		ctx.Player.Inventory().Set(slot, s)
		ctx.Send("itemloreSuccess", line)
	case sub == "clear":
		s.Patch.Lore = nil
		ctx.Player.Inventory().Set(slot, s)
		ctx.Send("itemloreClear")
	case sub == "set" && len(ctx.Args) > 2 && core.IsInt(ctx.Args[1]):
		if len(s.Patch.Lore) == 0 {
			return core.Tlf("itemloreNoLore")
		}
		n, _ := strconv.Atoi(ctx.Args[1])
		line := strings.TrimSpace(formatString(e, ctx.User, "essentials.itemlore", ctx.FinalArg(2)))
		if n < 1 || n > len(s.Patch.Lore) {
			return core.Tlf("itemloreNoLine", n)
		}
		s.Patch.Lore = slices.Clone(s.Patch.Lore)
		s.Patch.Lore[n-1] = legacyText(e, line)
		ctx.Player.Inventory().Set(slot, s)
		ctx.Send("itemloreSuccessLore", n, line)
	default:
		return core.ErrNotEnoughArgs
	}
	return nil
}

func completeItemlore(ctx *core.Ctx) []string {
	switch len(ctx.Args) {
	case 1:
		return []string{"add", "set", "clear"}
	case 2, 3:
		if !strings.EqualFold(ctx.Args[0], "set") || ctx.Player == nil {
			return nil
		}
		_, s := heldSlot(ctx.Player)
		if len(ctx.Args) == 2 {
			var out []string
			for i := range s.Patch.Lore {
				out = append(out, strconv.Itoa(i+1))
			}
			return out
		}
		if n, err := strconv.Atoi(ctx.Args[1]); err == nil && n >= 1 && n <= len(s.Patch.Lore) {
			return []string{unformatText(ctx.Ess, ctx.User, "essentials.itemlore", s.Patch.Lore[n-1])}
		}
	}
	return nil
}

// Essentials: Commanditemname
func runItemname(ctx *core.Ctx) error {
	e := ctx.Ess
	slot, s := heldSlot(ctx.Player)
	if s.Empty() || preventedType(ctx.Player, "essentials.itemname.prevent-type.", s.Item) {
		ctx.Send("itemnameInvalidItem")
		return nil
	}
	name := strings.TrimSpace(formatString(e, ctx.User, "essentials.itemname", ctx.FinalArg(0)))
	if name == "" {
		s.Patch.CustomName = nil
	} else {
		t := legacyText(e, name)
		s.Patch.CustomName = &t
	}
	ctx.Player.Inventory().Set(slot, s)
	if name == "" {
		ctx.Send("itemnameClear")
	} else {
		ctx.Send("itemnameSuccess", name)
	}
	return nil
}

// Essentials: Commandenchant
func runEnchant(ctx *core.Ctx) error {
	e := ctx.Ess
	slot, s := heldSlot(ctx.Player)
	if s.Empty() {
		return core.Tlf("nothingInHand")
	}
	if len(ctx.Args) == 0 {
		var usable []string
		for _, name := range e.EnchantmentNames() {
			ench, key, ok := e.Enchantment(name)
			if ok && ctx.Has("essentials.enchantments."+key) && core.EnchantmentFits(ench, s.Item) {
				usable = append(usable, name)
			}
		}
		slices.Sort(usable)
		return usageWith(ctx, "enchantments", joinList(slices.Compact(usable)))
	}
	level := 1
	if len(ctx.Args) > 1 {
		n, err := strconv.Atoi(ctx.Args[1])
		if err != nil {
			return core.ErrNotEnoughArgs
		}
		level = n
	}
	ench, key, ok := e.Enchantment(ctx.Args[0])
	if !ok {
		return core.Tlf("enchantmentNotFound")
	}
	if !ctx.Has("essentials.enchantments." + key) {
		return core.Tlf("enchantmentPerm", key)
	}
	unsafe := allowUnsafe(ctx)
	if !unsafe && level != 0 {
		if level < 1 || level > core.EnchantmentMaxLevel(ench) {
			return fmt.Errorf("Enchantment %s: Enchantment level is either too low or too high (given %d, bounds are 1 to %d)",
				key, level, core.EnchantmentMaxLevel(ench))
		}
		if s.Item != registry.ItemEnchantedBook && !core.EnchantmentFits(ench, s.Item) {
			return fmt.Errorf("Enchantment %s: Specified enchantment cannot be applied to this itemstack", key)
		}
	}
	s.Patch.Enchantments = cloneEnch(s.Patch.Enchantments)
	s.Patch.StoredEnchantments = cloneEnch(s.Patch.StoredEnchantments)
	if err := e.AddEnchantment(&s, ench, level, true); err != nil {
		return err
	}
	ctx.Player.Inventory().Set(slot, s)
	name := strings.ReplaceAll(key, "_", " ")
	if level == 0 {
		ctx.Send("enchantmentRemoved", name)
	} else {
		ctx.Send("enchantmentApplied", name)
	}
	return nil
}

func cloneEnch(m map[plugin.Enchantment]int) map[plugin.Enchantment]int {
	if m == nil {
		return nil
	}
	out := make(map[plugin.Enchantment]int, len(m))
	maps.Copy(out, m)
	return out
}

// hasBinding reports whether a worn item has the curse of binding and the
// user may not take it off anyway.
func hasBinding(ctx *core.Ctx, s plugin.ItemStack) bool {
	ench, _, ok := ctx.Ess.Enchantment("binding_curse")
	if !ok || s.Empty() {
		return false
	}
	_, cursed := s.Patch.Enchantments[ench]
	return cursed && !ctx.Has("essentials.hat.ignore-binding")
}

// Essentials: Commandhat
func runHat(ctx *core.Ctx) error {
	e := ctx.Ess
	inv := ctx.Player.Inventory()
	if len(ctx.Args) == 0 || (!strings.Contains(ctx.Args[0], "rem") && !strings.Contains(ctx.Args[0], "off") && ctx.Args[0] != "0") {
		hand := inv.Get(plugin.SlotMainHand)
		if hand.Empty() || preventedType(ctx.Player, "essentials.hat.prevent-type.", hand.Item) {
			ctx.Send("hatFail")
			return nil
		}
		if maxDurability(hand) != 0 {
			ctx.Send("hatArmor")
			return nil
		}
		head := inv.Get(plugin.SlotHead)
		if hasBinding(ctx, head) {
			ctx.Send("hatCurse")
			return nil
		}
		inv.Set(plugin.SlotHead, hand)
		inv.Set(plugin.SlotMainHand, head)
		ctx.Send("hatPlaced")
		return nil
	}
	head := inv.Get(plugin.SlotHead)
	switch {
	case head.Empty():
		ctx.Send("hatEmpty")
	case hasBinding(ctx, head):
		ctx.Send("hatCurse")
	default:
		inv.Set(plugin.SlotHead, plugin.ItemStack{})
		e.GiveItems(ctx.Player, false, head)
		ctx.Send("hatRemoved")
	}
	return nil
}

// listenHat lets a player put any item on their head by clicking it into
// the helmet slot, with allow-direct-hat. Essentials:
// EssentialsPlayerListener.onInventoryClickEvent
func listenHat(e *core.Essentials) {
	plugin.Listen(e.Server, func(ev *plugin.InventoryClickEvent) {
		// Slot 39 of the player's own inventory is the helmet. Essentials
		// tries it only when the top is none of the windows its other
		// branches handle; so a click on slot 39 of an /invsee window (an
		// InvPlayer too) is not taken for the viewer's own helmet.
		if ev.View != nil && javaHandledTop(ev.View.Top()) {
			return
		}
		if ev.Inventory == nil || ev.Inventory.Type() != plugin.InvPlayer || ev.Slot != slotHead || ev.Click != plugin.ClickLeft {
			return
		}
		cursor := ev.Cursor
		if !e.Config.Bool("allow-direct-hat", true) || cursor.Empty() || maxDurability(cursor) != 0 || isSkull(cursor.Item) {
			return
		}
		p := ev.Player
		if !p.HasPermission("essentials.hat") || p.HasPermission("essentials.hat.prevent-type."+typeName(cursor.Item)) {
			return
		}
		inv := p.Inventory()
		head := inv.Get(plugin.SlotHead)
		if ench, _, ok := e.Enchantment("binding_curse"); ok && !head.Empty() {
			if _, cursed := head.Patch.Enchantments[ench]; cursed && !p.HasPermission("essentials.hat.ignore-binding") {
				return
			}
		}
		ev.SetCancelled(true)
		inv.Set(plugin.SlotHead, cursor)
		p.SetCursor(head)
	}, plugin.ListenOptions{Priority: plugin.Lowest, IgnoreCancelled: true})
}

func isSkull(it registry.Item) bool {
	n := it.Name()
	return strings.HasSuffix(n, "_skull") || strings.HasSuffix(n, "_head")
}

var (
	skullName   = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	skullHash   = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
	skullBase64 = regexp.MustCompile(`^[A-Za-z0-9+/=]{180}$`)
)

// Essentials: Commandskull
func runSkull(ctx *core.Ctx) error {
	e := ctx.Ess
	user := ctx.User
	receiver := user
	if len(ctx.Args) == 2 {
		u, err := e.MatchUser(user, ctx.Args[1], false, false)
		if err != nil {
			return err
		}
		receiver = u
	}
	owner := user.Name()
	if len(ctx.Args) > 0 && receiver.HasPermission("essentials.skull.others") {
		switch arg := ctx.Args[0]; {
		case skullBase64.MatchString(arg):
			var tex struct {
				Textures struct {
					Skin struct {
						URL string `json:"url"`
					} `json:"SKIN"`
				} `json:"textures"`
			}
			raw, err := base64.StdEncoding.DecodeString(arg)
			if err != nil || json.Unmarshal(raw, &tex) != nil || tex.Textures.Skin.URL == "" {
				return core.Tlf("skullInvalidBase64")
			}
			url := tex.Textures.Skin.URL
			owner = url[strings.LastIndex(url, "/")+1:]
			if !skullHash.MatchString(owner) {
				return core.Tlf("skullInvalidBase64")
			}
		case !skullName.MatchString(arg):
			return core.Tlf("alphaNames")
		default:
			owner = arg
		}
	}
	slot, s := heldSlot(ctx.Player)
	spawn := false
	switch {
	case s.Item == registry.ItemPlayerHead && receiver == user:
	case receiver == user && user.HasPermission("essentials.skull.spawn"),
		receiver != user && user.HasPermission("essentials.skull.spawn.others"):
		s = plugin.NewStack(registry.ItemPlayerHead, 1)
		spawn = true
	default:
		return core.Tlf("invalidSkull")
	}
	if s.Patch.Profile != nil && !user.HasPermission("essentials.skull.modify") {
		return core.Tlf("noPermissionSkull")
	}
	short := owner
	if skullHash.MatchString(owner) {
		tex := fmt.Sprintf(`{"textures":{"SKIN":{"url":"https://textures.minecraft.net/texture/%s"}}}`, owner)
		s.Patch.Profile = &plugin.Profile{ID: uuid.New(),
			Properties: []plugin.ProfileProperty{{Name: "textures", Value: base64.StdEncoding.EncodeToString([]byte(tex))}}}
		short = owner[:7]
	} else if prof, ok := e.Server.ProfileByName(owner); ok {
		s.Patch.Profile = &prof
	} else {
		// Not online or cached: the name alone, as Bukkit's setOwner,
		// left for the client to resolve.
		prof := &plugin.Profile{Name: owner}
		if op, ok := e.Server.OfflinePlayerNamed(owner); ok {
			prof.ID = op.UUID()
		}
		s.Patch.Profile = prof
	}
	name := legacyText(e, "§fSkull of "+short)
	s.Patch.CustomName = &name
	if spawn {
		if p, ok := receiver.Player(); ok {
			e.GiveItems(p, false, s)
		}
		receiver.Send("givenSkull", short)
		if receiver != user {
			user.Send("givenSkullOther", receiver.DisplayName(), short)
		}
		return nil
	}
	ctx.Player.Inventory().Set(slot, s)
	user.Send("skullChanged", short)
	return nil
}

// Essentials: Commandrepair
func runRepair(ctx *core.Ctx) error {
	if len(ctx.Args) == 0 || strings.EqualFold(ctx.Args[0], "hand") || !ctx.Has("essentials.repair.all") {
		return repairHand(ctx)
	}
	if !strings.EqualFold(ctx.Args[0], "all") {
		return core.ErrNotEnoughArgs
	}
	charge := cost(ctx.Ess, ctx.User, "repair-all")
	if err := charge.CheckAffordable(ctx.User); err != nil {
		return err
	}
	if err := repairAll(ctx); err != nil {
		return err
	}
	return charge.Charge(ctx.User)
}

// repairCharge is the cost of repairing an item: repair-<item-name>, else
// repair-item.
func repairCharge(ctx *core.Ctx, it registry.Item) *core.Trade {
	return cost(ctx.Ess, ctx.User, "repair-"+strings.ReplaceAll(typeName(it), "_", "-"), "repair-item")
}

func mayRepairEnchanted(ctx *core.Ctx, s plugin.ItemStack) bool {
	return len(s.Patch.Enchantments) == 0 || ctx.Ess.Config.Bool("repair-enchanted", true) || ctx.Has("essentials.repair.enchanted")
}

func repairHand(ctx *core.Ctx) error {
	slot, s := heldSlot(ctx.Player)
	if s.Empty() || isBlock(s.Item) || damage(s) == 0 {
		return core.Tlf("repairInvalidType")
	}
	if !mayRepairEnchanted(ctx, s) {
		return core.Tlf("repairEnchanted")
	}
	charge := repairCharge(ctx, s.Item)
	if err := charge.CheckAffordable(ctx.User); err != nil {
		return err
	}
	if err := repairItem(&s); err != nil {
		return err
	}
	if err := charge.Charge(ctx.User); err != nil {
		return err
	}
	ctx.Player.Inventory().Set(slot, s)
	ctx.Send("repair", spacedName(s.Item))
	return nil
}

func repairItem(s *plugin.ItemStack) error {
	if isBlock(s.Item) || maxDurability(*s) < 1 {
		return core.Tlf("repairInvalidType")
	}
	if damage(*s) == 0 {
		return core.Tlf("repairAlreadyFixed")
	}
	s.Patch.Damage = nil
	return nil
}

func repairAll(ctx *core.Ctx) error {
	var repaired []string
	armor := ctx.Has("essentials.repair.armor")
	inv := ctx.Player.Inventory()
	for i, s := range inv.Contents() {
		if isArmorSlot(i) && !armor {
			continue
		}
		if s.Empty() || isBlock(s.Item) || damage(s) == 0 {
			continue
		}
		charge := repairCharge(ctx, s.Item)
		if err := charge.CheckAffordable(ctx.User); err != nil {
			sendError(ctx.Ess, ctx.User, err)
			continue
		}
		if !mayRepairEnchanted(ctx, s) || repairItem(&s) != nil {
			continue
		}
		if err := charge.Charge(ctx.User); err != nil {
			sendError(ctx.Ess, ctx.User, err)
		}
		inv.SetItem(i, s)
		repaired = append(repaired, spacedName(s.Item))
	}
	if len(repaired) == 0 {
		return core.Tlf("repairNone")
	}
	ctx.Send("repair", joinList(repaired))
	return nil
}
