package items

import (
	"strconv"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// Essentials: Commandrecipe
func runRecipe(ctx *core.Ctx) error {
	e := ctx.Ess
	if len(ctx.Args) < 1 {
		return core.ErrNotEnoughArgs
	}
	var item plugin.ItemStack
	if strings.EqualFold(ctx.Args[0], "hand") {
		if ctx.Player == nil {
			return core.Tlf("consoleCannotUseCommand")
		}
		_, item = heldSlot(ctx.Player)
	} else {
		var err error
		if item, err = e.Items.Get(ctx.Args[0]); err != nil {
			return err
		}
	}
	n := 0
	if len(ctx.Args) > 1 {
		v, err := strconv.Atoi(ctx.Args[1])
		if err != nil {
			return core.Tlf("invalidNumber")
		}
		n = v - 1
	}
	var recipes []plugin.Recipe
	for _, r := range e.Server.Recipes().All() {
		if res := recipeResult(r); !res.Empty() && res.Item == item.Item {
			recipes = append(recipes, r)
		}
	}
	name := materialName(ctx, item.Item)
	if len(recipes) == 0 {
		return core.Tlf("recipeNone", name)
	}
	if n < 0 || n >= len(recipes) {
		return core.Tlf("recipeBadIndex")
	}
	ctx.Send("recipe", name, n+1, len(recipes))
	switch r := recipes[n].(type) {
	case plugin.CookingRecipe:
		if r.Kind == plugin.MinecraftKey("smelting") {
			ctx.Send("recipeFurnace", materialName(ctx, ingredientItem(r.Input)))
		}
	case plugin.ShapedRecipe:
		shapedRecipe(ctx, r)
	case plugin.ShapelessRecipe:
		if len(recipes) == 1 && item.Item == registry.ItemFireworkRocket {
			r.Ingredients = []plugin.Ingredient{{Items: []registry.Item{registry.ItemGunpowder}},
				{Items: []registry.Item{registry.ItemPaper}}, {Items: []registry.Item{registry.ItemFireworkStar}}}
		}
		shapelessRecipe(ctx, r)
	}
	if len(recipes) > 1 && len(ctx.Args) == 1 {
		ctx.Send("recipeMore", ctx.Label, ctx.Args[0], name)
	}
	return nil
}

// ingredientItem answers the item Bukkit shows for an ingredient: its first
// choice; air for an empty slot.
func ingredientItem(ing plugin.Ingredient) registry.Item {
	if len(ing.Items) > 0 {
		return ing.Items[0]
	}
	if ing.Tag != (plugin.Key{}) {
		if names, ok := registry.ItemTagMembers(ing.Tag.String()); ok && len(names) > 0 {
			if it, ok := registry.ItemByName(names[0]); ok {
				return it
			}
		}
	}
	return registry.ItemAir
}

func materialName(ctx *core.Ctx, it registry.Item) string {
	if it == registry.ItemAir {
		return ctx.Ess.TlFor(ctx.Sender, "recipeNothing").PlainString()
	}
	return spacedName(it)
}

// gridOf answers a shaped recipe's cells as Bukkit numbers them, a to i: the
// pattern's cells row by row, then empty ones.
func gridOf(r plugin.ShapedRecipe) (cells [9]registry.Item, at [3][3]registry.Item) {
	i := 0
	for j, row := range r.Pattern {
		for k, ch := range row {
			it := registry.ItemAir
			if ch != ' ' {
				it = ingredientItem(r.Keys[ch])
			}
			if i < 9 {
				cells[i] = it
			}
			if j < 3 && k < 3 {
				at[j][k] = it
			}
			i++
		}
	}
	return cells, at
}

func shapedRecipe(ctx *core.Ctx, r plugin.ShapedRecipe) {
	cells, at := gridOf(r)
	if ctx.Player != nil {
		recipeWindow(ctx, at)
		return
	}
	numbers := map[registry.Item]byte{}
	var order []registry.Item
	for _, it := range cells {
		if _, ok := numbers[it]; !ok {
			numbers[it] = byte('1' + len(numbers))
			order = append(order, it)
		}
	}
	tag := func(it registry.Item) core.Raw {
		c := numbers[it]
		if c == 0 {
			c = '1'
		}
		return core.Raw("<" + miniTags[c] + ">" + string(c))
	}
	for j := range 3 {
		ctx.Send("recipeGrid", tag(at[j][0]), tag(at[j][1]), tag(at[j][2]))
	}
	var b strings.Builder
	for _, it := range order {
		b.WriteString(tlRaw(ctx.Ess, ctx.Sender, "recipeGridItem", string(numbers[it]), materialName(ctx, it)) + " ")
	}
	ctx.Send("recipeWhere", core.Raw(b.String()))
}

func shapelessRecipe(ctx *core.Ctx, r plugin.ShapelessRecipe) {
	if ctx.Player != nil {
		var grid [3][3]registry.Item
		for i, ing := range r.Ingredients {
			if i < 9 {
				grid[i/3][i%3] = ingredientItem(ing)
			}
		}
		recipeWindow(ctx, grid)
		return
	}
	var parts []string
	for _, ing := range r.Ingredients {
		parts = append(parts, materialName(ctx, ingredientItem(ing)))
	}
	ctx.Send("recipeShapeless", strings.Join(parts, ", ")+" ")
}
