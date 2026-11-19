package economy

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/shopspring/decimal"
)

// sellCmd sells items to the server for their worth. Essentials: Commandsell
func sellCmd() core.Cmd {
	return core.Cmd{
		Name:       "sell",
		PlayerOnly: true,
		Run: func(ctx *core.Ctx) error {
			e, args := ctx.Ess, ctx.Args
			if len(args) < 1 {
				return core.ErrNotEnoughArgs
			}
			switch strings.ToLower(args[0]) {
			case "hand":
				if !ctx.Has("essentials.sell.hand") {
					return core.Tlf("sellHandPermission")
				}
			case "inventory", "invent", "all":
				if !ctx.Has("essentials.sell.bulk") {
					return core.Tlf("sellBulkPermission")
				}
			}
			stacks, err := matchingItems(ctx, args)
			if err != nil {
				return err
			}
			bulk := len(stacks) > 1
			allowNamed := e.Config.Bool("allow-selling-named-items", false)
			total, count := decimal.Zero, 0
			var notSold []string
			for i := range stacks {
				s := stacks[i]
				if !allowNamed && s.Patch.CustomName != nil {
					if bulk {
						notSold = append(notSold, s.Patch.CustomName.PlainString())
						continue
					}
					return core.Tlf("cannotSellNamedItem")
				}
				if s.Count <= 0 {
					continue
				}
				worth, err := sellItem(ctx, s, args, bulk)
				if err != nil {
					if !bulk {
						return err
					}
					continue
				}
				total = total.Add(worth)
				count++
				zeroSimilar(e, stacks, s)
			}
			if len(notSold) > 0 {
				e.ShowError(ctx, core.Tlf("cannotSellTheseNamedItems", strings.Join(notSold, ", ")))
			}
			if count != 1 {
				t := core.Raw(format(e, total))
				if strings.EqualFold(args[0], "blocks") {
					ctx.Send("totalWorthBlocks", t, t)
				} else {
					ctx.Send("totalWorthAll", t, t)
				}
			}
			return nil
		},
		Complete: completeItems,
	}
}

// sellItem sells the user's items like s for their worth times the sell
// multiplier. Essentials: Commandsell.sellItem
func sellItem(ctx *core.Ctx, s plugin.ItemStack, args []string, bulk bool) (decimal.Decimal, error) {
	e, u, p := ctx.Ess, ctx.User, ctx.Player
	amount, err := sellAmount(ctx, s, args, bulk)
	if err != nil {
		return decimal.Zero, err
	}
	price, ok := e.Worth.Price(s)
	if !ok {
		return decimal.Zero, core.Tlf("itemCannotBeSold")
	}
	worth := price.Mul(sellMultiplier(u))
	typeName := itemKey(s)
	if amount <= 0 {
		if !bulk {
			u.Send("itemSold", core.Raw(format(e, decimal.Zero)), 0, typeName, format(e, worth))
		}
		return decimal.Zero, nil
	}
	result := worth.Mul(decimal.NewFromInt(int64(amount)))
	sold := s
	sold.Count = amount
	if !removeItemAmount(e, p, sold, amount) {
		return decimal.Zero, errors.New("Trying to remove more items than are available.")
	}
	loc := p.Location()
	e.Economy.Log("Command", "Sell", "Item", u.Name(), result, u.Name(), &loc)
	if err := u.GiveMoney(result, nil); err != nil {
		return decimal.Zero, err
	}
	u.Send("itemSold", core.Raw(format(e, result)), amount, typeName, core.Raw(format(e, worth)))
	e.Log.Info(e.TlString("itemSoldConsole", u.Name(), typeName, format(e, result), amount, format(e, worth), u.DisplayName()))
	return result, nil
}

// sellMultiplier answers the highest sell-multipliers entry the user has
// essentials.sell.multiplier.<name> for, at least the default.
// Essentials: Settings.getMultiplier
func sellMultiplier(u *core.User) decimal.Decimal {
	c := u.Ess().Config
	mult := decimal.NewFromFloat(c.Float("sell-multipliers.default", 1))
	if _, ok := c.Section("sell-multipliers"); !ok {
		return mult
	}
	for _, name := range c.Keys("sell-multipliers") {
		if !u.HasPermission("essentials.sell.multiplier." + name) {
			continue
		}
		if v := decimal.NewFromFloat(c.Float("sell-multipliers."+name, 0)); v.GreaterThan(mult) {
			mult = v
		}
	}
	return mult
}

// setworthCmd sets an item's worth, the held item's when only a price is
// given. Essentials: Commandsetworth
func setworthCmd() core.Cmd {
	return core.Cmd{
		Name: "setworth",
		Run: func(ctx *core.Ctx) error {
			e, args := ctx.Ess, ctx.Args
			var stack plugin.ItemStack
			var price string
			switch {
			case ctx.User == nil && len(args) < 2, len(args) < 1:
				return core.ErrNotEnoughArgs
			case len(args) == 1:
				stack, price = itemInHand(ctx.Player), args[0]
			default:
				s, err := e.Items.Get(args[0])
				if err != nil {
					return err
				}
				stack, price = s, args[1]
			}
			f, err := parseDouble(price)
			if err != nil {
				return err
			}
			if err := e.Worth.SetPrice(stack, decimal.NewFromFloat(f)); err != nil {
				return err
			}
			ctx.Send("worthSet")
			return nil
		},
	}
}

// parseDouble reads a number, refusing NaN and infinities.
// Essentials: FloatUtil.parseDouble
func parseDouble(s string) (float64, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	switch {
	case err != nil:
		return 0, fmt.Errorf("For input string: %q", s)
	case math.IsNaN(f):
		return 0, errors.New("NaN is not valid")
	case math.IsInf(f, 0):
		return 0, errors.New("Infinity is not valid")
	}
	return f, nil
}

// worthCmd shows what items sell for. Essentials: Commandworth
func worthCmd() core.Cmd {
	return core.Cmd{
		Name: "worth",
		Run: func(ctx *core.Ctx) error {
			e, args := ctx.Ess, ctx.Args
			if ctx.User == nil {
				if len(args) == 0 {
					return core.ErrNotEnoughArgs
				}
				s, err := e.Items.Get(args[0])
				if err != nil {
					return err
				}
				_, err = itemWorth(ctx, false, s, args)
				return err
			}
			stacks, err := matchingItems(ctx, args)
			if err != nil {
				return err
			}
			bulk := len(stacks) > 1
			total, count := decimal.Zero, 0
			for i := range stacks {
				s := stacks[i]
				if s.Count <= 0 {
					continue
				}
				w, err := itemWorth(ctx, true, s, args)
				if err != nil {
					if !bulk {
						return err
					}
					continue
				}
				total = total.Add(w)
				count++
				zeroSimilar(e, stacks, s)
			}
			if count > 1 {
				t := core.Raw(format(e, total))
				if len(args) > 0 && strings.EqualFold(args[0], "blocks") {
					ctx.Send("totalSellableBlocks", t, t)
				} else {
					ctx.Send("totalSellableAll", t, t)
				}
			}
			return nil
		},
		Complete: func(ctx *core.Ctx) []string {
			switch len(ctx.Args) {
			case 1:
				if ctx.User == nil {
					return ctx.Ess.Items.Completions()
				}
				return completeItems(ctx)
			case 2:
				return []string{"1", "64"}
			}
			return nil
		},
	}
}

// itemWorth shows what s is worth: for a player as many as /sell would
// sell, for the console args[1] of them. Essentials: Commandworth.itemWorth
func itemWorth(ctx *core.Ctx, player bool, s plugin.ItemStack, args []string) (decimal.Decimal, error) {
	e := ctx.Ess
	amount := 1
	if !player {
		if len(args) > 1 {
			n, err := strconv.Atoi(digitsOf(args[1]))
			if err != nil {
				return decimal.Zero, core.ErrNotEnoughArgs
			}
			amount = n
		}
	} else {
		n, err := sellAmount(ctx, s, args, true)
		if err != nil {
			return decimal.Zero, err
		}
		amount = n
	}
	worth, ok := e.Worth.Price(s)
	if !ok {
		return decimal.Zero, core.Tlf("itemCannotBeSold")
	}
	amount = max(amount, 0)
	result := worth.Mul(decimal.NewFromInt(int64(amount)))
	typeName := strings.ReplaceAll(itemKey(s), "_", "")
	res, each := core.Raw(format(e, result)), core.Raw(format(e, worth))
	if s.Patch.Damage != nil && *s.Patch.Damage != 0 {
		ctx.Send("worthMeta", typeName, *s.Patch.Damage, res, amount, each)
	} else {
		ctx.Send("worth", typeName, res, amount, each)
	}
	return result, nil
}

// completeItems completes an item argument of /sell and /worth: hand,
// inventory and blocks first, every item once a letter is typed, then an
// amount. Essentials: getMatchingItems
func completeItems(ctx *core.Ctx) []string {
	switch len(ctx.Args) {
	case 1:
		items := []string{"hand", "inventory", "blocks"}
		if ctx.Args[0] != "" {
			items = append(items, ctx.Ess.Items.Completions()...)
		}
		return items
	case 2:
		return []string{"1", "64"}
	}
	return nil
}

// itemKey is the item's name as Bukkit's Material, lower case.
func itemKey(s plugin.ItemStack) string { return strings.TrimPrefix(s.Item.Name(), "minecraft:") }

func digitsOf(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}

// The armour slots of a PlayerInventory (Bukkit's numbering): boots to
// helmet after the 36 storage slots; 40 is the off hand.
const (
	armorFrom = 36
	armorTo   = 39
)

// inventory answers the player's 41 slots, the armour slots empty unless
// includeArmor. Essentials: Inventories.getInventory
func inventory(p plugin.Player, includeArmor bool) []plugin.ItemStack {
	out := p.Inventory().Contents()
	if !includeArmor {
		for i := armorFrom; i <= armorTo && i < len(out); i++ {
			out[i] = plugin.ItemStack{}
		}
	}
	return out
}

// itemInHand answers the main hand's item, else the off hand's.
// Essentials: Inventories.getItemInHand
func itemInHand(p plugin.Player) plugin.ItemStack {
	if s := p.Inventory().Get(plugin.SlotMainHand); !s.Empty() {
		return s
	}
	return p.Inventory().Get(plugin.SlotOffHand)
}

// similar reports whether a and b stack, whatever their counts.
func similar(e *core.Essentials, a, b plugin.ItemStack) bool {
	a.Count, b.Count = 1, 1
	return e.Server.Items().Similar(a, b)
}

// zeroSimilar empties the stacks like s, so a bulk sale handles each kind
// of item once.
func zeroSimilar(e *core.Essentials, stacks []plugin.ItemStack, s plugin.ItemStack) {
	for j := range stacks {
		if similar(e, stacks[j], s) {
			stacks[j].Count = 0
		}
	}
}

// matchingItems answers the stacks args name: the held item (no argument
// or hand), every stack (inventory, invent, all), every block stack
// (blocks), or the item named. Essentials: ItemDb.getMatching
func matchingItems(ctx *core.Ctx, args []string) ([]plugin.ItemStack, error) {
	p := ctx.Player
	var out []plugin.ItemStack
	first := strings.ToLower(ctx.Arg(0))
	switch {
	case len(args) < 1, first == "hand":
		out = append(out, itemInHand(p))
	case first == "inventory", first == "invent", first == "all", first == "blocks":
		for _, s := range inventory(p, true) {
			if s.Empty() {
				continue
			}
			if _, block := s.Item.Block(); first == "blocks" && !block {
				continue
			}
			out = append(out, s)
		}
	default:
		s, err := ctx.Ess.Items.Get(args[0])
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if len(out) == 0 || out[0].Item == registry.ItemAir {
		return nil, core.Tlf("itemSellAir")
	}
	return out, nil
}

// sellAmount answers how many of s to sell for args[1]: a number, a
// negative to keep that many, "Ns" for stacks, none for all; items listed
// in trade-in-stacks sell only in whole stacks. Essentials: Worth.getAmount
func sellAmount(ctx *core.Ctx, s plugin.ItemStack, args []string, bulk bool) (int, error) {
	e, u, p := ctx.Ess, ctx.User, ctx.Player
	if s.Item == registry.ItemAir {
		return 0, core.Tlf("itemSellAir")
	}
	amount := 0
	if len(args) > 1 {
		n, err := strconv.Atoi(digitsOf(args[1]))
		if err != nil {
			return 0, core.ErrNotEnoughArgs
		}
		amount = n
		if strings.HasPrefix(args[1], "-") {
			amount = -amount
		}
	}
	stack := len(args) > 1 && strings.HasSuffix(args[1], "s")
	requireStack := e.Config.Bool("trade-in-stacks."+strings.ReplaceAll(itemKey(s), "_", ""), false)
	if requireStack && !stack {
		return 0, core.Tlf("itemMustBeStacked")
	}
	have := 0
	for _, c := range inventory(p, false) {
		if !c.Empty() && similar(e, c, s) {
			have += c.Count
		}
	}
	maxStack := e.Server.Items().MaxStackSize(s)
	if stack {
		amount *= maxStack
	}
	if amount < 1 {
		amount += have
	}
	if requireStack && maxStack > 0 {
		amount -= amount % maxStack
	}
	if (amount > have || amount < 1) && !bulk {
		u.Send("itemNotEnough2")
		u.Send("itemNotEnough3")
		return 0, core.Tlf("itemNotEnough1")
	}
	return amount, nil
}

// removeItemAmount takes amount of the stacks like s out of the slots in
// order, nothing when there are not that many. Essentials:
// Inventories.containsAtLeast and removeItemAmount
func removeItemAmount(e *core.Essentials, p plugin.Player, s plugin.ItemStack, amount int) bool {
	var clearSlots []int
	inv := p.Inventory()
	for i, item := range inv.Contents() {
		if item.Empty() || !similar(e, item, s) {
			continue
		}
		if item.Count >= amount {
			item.Count -= amount
			inv.SetItem(i, item)
			for _, c := range clearSlots {
				inv.SetItem(c, plugin.ItemStack{})
			}
			return true
		}
		amount -= item.Count
		clearSlots = append(clearSlots, i)
	}
	return false
}
