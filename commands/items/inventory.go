package items

import (
	"slices"
	"strconv"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

func registerInventory(e *core.Essentials) {
	cond := &condenser{e: e, cache: map[registry.Item]*condenseRecipe{}}
	none := func(*core.Ctx) []string { return nil }
	e.RegisterCommand(
		core.Cmd{Name: "clearinventory", Run: runClearinventory, Complete: completeClearinventory},
		core.Cmd{Name: "clearinventoryconfirmtoggle", PlayerOnly: true, Run: func(ctx *core.Ctx) error {
			u := ctx.User
			confirm := !promptingClearConfirm(ctx.Ess, u)
			label := strings.ToLower(ctx.Label)
			if strings.HasSuffix(label, "on") {
				confirm = true
			} else if strings.HasSuffix(label, "off") {
				confirm = false
			}
			u.Data.ConfirmClear = &confirm
			u.Save()
			if confirm {
				ctx.Send("clearInventoryConfirmToggleOn")
			} else {
				ctx.Send("clearInventoryConfirmToggleOff")
			}
			u.ConfirmingClear = ""
			return nil
		}, Complete: none},
		core.Cmd{Name: "condense", PlayerOnly: true, Run: cond.run, Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) != 1 {
				return nil
			}
			out := []string{"hand", "inventory", "blocks"}
			if ctx.Args[0] != "" {
				out = append(out, itemNames(ctx.Ess)...)
			}
			return out
		}},
		core.Cmd{Name: "invsee", PlayerOnly: true, Run: runInvsee, Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 {
				return slices.DeleteFunc(ctx.CompletePlayers(), func(n string) bool { return n == ctx.User.Name() })
			}
			return nil
		}},
		core.Cmd{Name: "enderchest", PlayerOnly: true, Run: runEnderchest, Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 && ctx.Has("essentials.enderchest.others") {
				return ctx.CompletePlayers()
			}
			return nil
		}},
		core.Cmd{Name: "disposal", PlayerOnly: true, Run: func(ctx *core.Ctx) error {
			ctx.Send("openingDisposal")
			title := ctx.Ess.TlFor(ctx.Sender, "disposal")
			ctx.Player.OpenInventory(ctx.Ess.Server.CreateInventory(plugin.InvChest, 4, title, disposal{}))
			return nil
		}, Complete: none},
		core.Cmd{Name: "recipe", Run: runRecipe, Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 {
				return itemNames(ctx.Ess)
			}
			return nil
		}},
	)
	for _, st := range stations {
		e.RegisterCommand(core.Cmd{Name: st.name, PlayerOnly: true, Run: func(ctx *core.Ctx) error {
			inv := ctx.Ess.Server.CreateInventory(st.kind, 0, plugin.Translate(st.title), nil)
			if inv == nil {
				ctx.Send("unsupportedBrand")
				return nil
			}
			ctx.Player.OpenInventory(inv)
			return nil
		}, Complete: none})
	}
}

// stations are the workstation commands, each opening its window with no
// block, as Bukkit's openWorkbench and Essentials' ContainerProvider do.
// Essentials: Commandworkbench, Commandanvil, Commandcartographytable,
// Commandgrindstone, Commandloom, Commandsmithingtable, Commandstonecutter
var stations = []struct {
	name  string
	kind  plugin.InventoryType
	title string
}{
	{"workbench", plugin.InvCrafting, "container.crafting"},
	{"anvil", plugin.InvAnvil, "container.repair"},
	{"cartographytable", plugin.InvCartography, "container.cartography_table"},
	{"grindstone", plugin.InvGrindstone, "container.grindstone_title"},
	{"loom", plugin.InvLoom, "container.loom"},
	{"smithingtable", plugin.InvSmithing, "container.upgrade"},
	{"stonecutter", plugin.InvStonecutter, "container.stonecutter"},
}

// disposal is the holder of a /disposal menu, whose contents are dropped
// with it.
type disposal struct{}

func promptingClearConfirm(e *core.Essentials, u *core.User) bool {
	if u.Data.ConfirmClear != nil {
		return *u.Data.ConfirmClear
	}
	return slices.ContainsFunc(e.Config.StringList("default-enabled-confirm-commands"),
		func(c string) bool { return strings.EqualFold(c, "clearinventory") })
}

// Essentials: Commandclearinventory
func runClearinventory(ctx *core.Ctx) error {
	e := ctx.Ess
	allowOthers, allowAll := true, true
	var players []plugin.Player
	previous := ""
	if u := ctx.User; u != nil {
		allowOthers = ctx.Has("essentials.clearinventory.others")
		allowAll = ctx.Has("essentials.clearinventory.all") || ctx.Has("essentials.clearinventory.multiple")
		players = append(players, ctx.Player)
		previous = u.ConfirmingClear
		u.ConfirmingClear = ""
	}
	offset := 0
	args := ctx.Args
	if allowAll && len(args) > 0 && args[0] == "*" {
		ctx.Send("inventoryClearingFromAll")
		offset = 1
		players = e.Server.Players()
	} else if allowOthers && len(args) > 0 && len(strings.TrimSpace(args[0])) > 2 {
		offset = 1
		players = matchPlayers(e, strings.TrimSpace(args[0]))
	}
	if len(players) < 1 {
		return core.ErrPlayerNotFound
	}
	formatted := "/" + ctx.Label
	if len(args) > 0 {
		formatted += " " + strings.Join(args, " ")
	}
	if u := ctx.User; u != nil && promptingClearConfirm(e, u) && formatted != previous {
		u.ConfirmingClear = formatted
		u.Send("confirmClear", formatted)
		return nil
	}
	for _, p := range players {
		if err := clearHandler(ctx, p, args, offset, len(players) < 8); err != nil {
			return err
		}
	}
	return nil
}

func clearHandler(ctx *core.Ctx, p plugin.Player, args []string, offset int, extended bool) error {
	e := ctx.Ess
	inv := p.Inventory()
	name := e.Users.Of(p).DisplayName()
	amount := -1
	if len(args) > offset+1 && core.IsInt(args[offset+1]) {
		amount, _ = strconv.Atoi(args[offset+1])
	}
	if len(args) <= offset || args[offset] == "*" || args[offset] == "**" {
		armor := len(args) > offset && args[offset] == "**"
		if extended {
			key := "inventoryClearingAllItems"
			if armor {
				key = "inventoryClearingAllArmor"
			}
			ctx.Send(key, name)
		}
		for i := range inv.Size() {
			if armor || !isArmorSlot(i) {
				inv.SetItem(i, plugin.ItemStack{})
			}
		}
		return nil
	}
	var items []registry.Item
	for part := range strings.SplitSeq(args[offset], ",") {
		id, _, _ := strings.Cut(part, ":")
		if s, err := e.Items.Get(id); err == nil && !slices.Contains(items, s.Item) {
			items = append(items, s.Item)
		}
	}
	for _, it := range items {
		if amount < -1 {
			return core.Tlf("cannotRemoveNegativeItems")
		}
		itemName := typeName(it)
		if amount == -1 {
			removed := 0
			for i, s := range inv.Contents() {
				if !s.Empty() && s.Item == it && plain(e, s) {
					removed += s.Count
					inv.SetItem(i, plugin.ItemStack{})
				}
			}
			if removed > 0 || extended {
				ctx.Send("inventoryClearingStack", removed, itemName, name)
			}
		} else if removeAmount(e, p, it, amount) {
			ctx.Send("inventoryClearingStack", amount, itemName, name)
		} else if extended {
			ctx.Send("inventoryClearFail", name, amount, itemName)
		}
	}
	return nil
}

// removeAmount takes amount plain stacks of the item, or nothing when there
// are fewer. Essentials: Inventories.removeItemAmount
func removeAmount(e *core.Essentials, p plugin.Player, it registry.Item, amount int) bool {
	var clear []int
	inv := p.Inventory()
	for i, s := range inv.Contents() {
		if s.Empty() || s.Item != it || !plain(e, s) {
			continue
		}
		if s.Count >= amount {
			s.Count -= amount
			if s.Count == 0 {
				s = plugin.ItemStack{}
			}
			inv.SetItem(i, s)
			for _, c := range clear {
				inv.SetItem(c, plugin.ItemStack{})
			}
			return true
		}
		amount -= s.Count
		clear = append(clear, i)
	}
	return false
}

func completeClearinventory(ctx *core.Ctx) []string {
	items := func() []string { return append(itemNames(ctx.Ess), "*", "**") }
	if ctx.Has("essentials.clearinventory.others") {
		switch len(ctx.Args) {
		case 1:
			out := ctx.CompletePlayers()
			if ctx.Has("essentials.clearinventory.all") || ctx.Has("essentials.clearinventory.multiple") {
				out = append(out, "*")
			}
			return out
		case 2:
			return items()
		}
		return nil
	}
	if len(ctx.Args) == 1 {
		return items()
	}
	return nil
}

// condenseRecipe is a recipe that packs Input of an item into Result.
type condenseRecipe struct {
	input  int
	result plugin.ItemStack
}

// condenser runs /condense, remembering each item's best recipe.
// Essentials: Commandcondense
type condenser struct {
	e     *core.Essentials
	cache map[registry.Item]*condenseRecipe
}

func (c *condenser) run(ctx *core.Ctx) error {
	var stacks []plugin.ItemStack
	validateReverse := false
	if len(ctx.Args) > 0 {
		var err error
		if stacks, err = matching(ctx, ctx.Args[0]); err != nil {
			return err
		}
	} else {
		for i, s := range ctx.Player.Inventory().Contents() {
			if !s.Empty() && !isArmorSlot(i) {
				stacks = append(stacks, s)
			}
		}
		validateReverse = true
	}
	converted := false
	for _, s := range stacks {
		if c.condense(ctx.Player, s, validateReverse) {
			converted = true
		}
	}
	if !converted {
		ctx.Send("itemsNotConverted")
		return core.ErrNoCharge
	}
	ctx.Send("itemsConverted")
	return nil
}

// matching answers the stacks an item argument names: hand, inventory,
// blocks, or an item. Essentials: ItemDb.getMatching
func matching(ctx *core.Ctx, arg string) ([]plugin.ItemStack, error) {
	var out []plugin.ItemStack
	switch strings.ToLower(arg) {
	case "hand":
		_, s := heldSlot(ctx.Player)
		out = append(out, s)
	case "inventory", "invent", "all", "blocks":
		for _, s := range ctx.Player.Inventory().Contents() {
			if !s.Empty() && (strings.ToLower(arg) != "blocks" || isBlock(s.Item)) {
				out = append(out, s)
			}
		}
	default:
		s, err := ctx.Ess.Items.Get(arg)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if len(out) == 0 || out[0].Empty() {
		return nil, core.Tlf("itemSellAir")
	}
	return out, nil
}

func (c *condenser) condense(p plugin.Player, s plugin.ItemStack, validateReverse bool) bool {
	e := c.e
	if !plain(e, s) {
		return false
	}
	r := c.recipeFor(s.Item)
	if r == nil {
		return false
	}
	if validateReverse {
		pass := false
		for _, rev := range e.Server.Recipes().All() {
			if res := recipeResult(rev); res.Item == s.Item && recipeInputs(rev, r.result.Item) > 0 {
				pass = true
				break
			}
		}
		if !pass {
			return false
		}
	}
	amount := 0
	for i, c := range p.Inventory().Contents() {
		if !isArmorSlot(i) && !c.Empty() && c.Item == s.Item && plain(e, c) {
			amount += c.Count
		}
	}
	output := (amount / r.input) * r.result.Count
	amount -= amount % r.input
	if amount <= 0 {
		return false
	}
	removeAmount(e, p, s.Item, amount)
	out := r.result
	out.Count = output
	e.GiveItems(p, true, out)
	return true
}

// recipeFor finds the square recipe that packs 4 or 9 of the item into
// fewer of another, preferring 9. Essentials: Commandcondense.getCondenseType
func (c *condenser) recipeFor(it registry.Item) *condenseRecipe {
	if r, ok := c.cache[it]; ok {
		return r
	}
	var best *condenseRecipe
	for _, rec := range c.e.Server.Recipes().All() {
		n := recipeInputs(rec, it)
		res := recipeResult(rec)
		if (n == 4 || n == 9) && n > res.Count && (best == nil || n > best.input) {
			best = &condenseRecipe{input: n, result: res}
		}
	}
	c.cache[it] = best
	return best
}

func recipeResult(r plugin.Recipe) plugin.ItemStack {
	switch r := r.(type) {
	case plugin.ShapedRecipe:
		return r.Result
	case plugin.ShapelessRecipe:
		return r.Result
	case plugin.CookingRecipe:
		return r.Result
	case plugin.StonecuttingRecipe:
		return r.Result
	case plugin.SmithingRecipe:
		return r.Result
	}
	return plugin.ItemStack{}
}

// recipeInputs answers how many ingredients a square shaped or shapeless
// recipe takes when every one of them is just the item; 0 otherwise.
// Essentials: Commandcondense.getStackOnRecipeMatch
func recipeInputs(r plugin.Recipe, it registry.Item) int {
	var ings []plugin.Ingredient
	switch r := r.(type) {
	case plugin.ShapedRecipe:
		if len(r.Pattern) == 0 || len(r.Pattern) != len(r.Pattern[0]) {
			return 0
		}
		for _, row := range r.Pattern {
			for _, ch := range row {
				if ch != ' ' {
					ings = append(ings, r.Keys[ch])
				}
			}
		}
	case plugin.ShapelessRecipe:
		ings = r.Ingredients
	default:
		return 0
	}
	for _, ing := range ings {
		if ing.Tag != (plugin.Key{}) || len(ing.Items) != 1 || ing.Items[0] != it {
			return 0
		}
	}
	return len(ings)
}
