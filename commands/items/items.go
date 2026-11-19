// Package items holds EssentialsX's item, inventory and kit commands, with
// the listeners powertools, unlimited items, direct hats and the /invsee,
// /enderchest and /recipe windows need.
package items

import (
	"slices"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/shopspring/decimal"
)

// Register registers the group's commands and listeners.
func Register(e *core.Essentials) {
	registerGive(e)
	registerMeta(e)
	registerPowertools(e)
	registerInventory(e)
	registerKits(e)
	registerEditsign(e)
	listenSeeing(e)
	listenHat(e)
}

// typeName answers an item as Bukkit's Material.toString() lower-cased:
// diamond_sword.
func typeName(it registry.Item) string { return strings.TrimPrefix(it.Name(), "minecraft:") }

// spacedName answers diamond sword, as the messages show an item type.
func spacedName(it registry.Item) string { return strings.ReplaceAll(typeName(it), "_", " ") }

// flatName answers diamondsword, the form of item permissions and messages.
func flatName(it registry.Item) string { return strings.ReplaceAll(typeName(it), "_", "") }

// canSpawnItem reports whether the user may spawn the item: with
// permission-based-item-spawn the essentials.itemspawn.item-<name> nodes,
// otherwise anything off item-spawn-blacklist. Essentials: User.canSpawnItem
func canSpawnItem(e *core.Essentials, u *core.User, it registry.Item) bool {
	if u == nil {
		return true
	}
	if e.Settings.PermissionBasedItemSpawn() {
		return u.HasPermission("essentials.itemspawn.item-all") || u.HasPermission("essentials.itemspawn.item-"+flatName(it))
	}
	if u.HasPermission("essentials.itemspawn.exempt") {
		return true
	}
	for _, name := range e.Settings.ItemSpawnBlacklist() {
		if s, err := e.Items.Get(name); err == nil && s.Item == it {
			return false
		}
	}
	return true
}

// cost answers what the named charge (a command-costs key such as
// repair-all or kit-tools) costs the user, trying the fallbacks while it is
// zero; nil when free or the user has essentials.nocommandcost.<name>.
// Essentials: Trade(command, fallback).getCommandCost
func cost(e *core.Essentials, u *core.User, name string, fallbacks ...string) *core.Trade {
	v := decimal.Zero
	for _, n := range append([]string{name}, fallbacks...) {
		if v = e.Settings.CommandCost(n); v.Sign() != 0 {
			break
		}
	}
	if v.Sign() == 0 || u == nil ||
		u.HasPermission("essentials.nocommandcost.all") || u.HasPermission("essentials.nocommandcost."+name) {
		return nil
	}
	return core.MoneyTrade(v)
}

// heldSlot answers the hand Essentials means by the item in hand: the main
// hand, or the off hand when the main one is empty.
// Essentials: Inventories.getItemInHand
func heldSlot(p plugin.Player) (plugin.EquipmentSlot, plugin.ItemStack) {
	inv := p.Inventory()
	if s := inv.Get(plugin.SlotMainHand); !s.Empty() {
		return plugin.SlotMainHand, s
	}
	return plugin.SlotOffHand, inv.Get(plugin.SlotOffHand)
}

// The armour and off-hand slots of a PlayerInventory, in Bukkit's
// numbering after the 36 storage slots.
const (
	slotFeet    = 36
	slotHead    = 39
	slotOffhand = 40
)

func isArmorSlot(i int) bool { return i >= slotFeet && i <= slotHead }

// plain reports whether s is the bare item with no components, which is what
// Bukkit's isSimilar against a new ItemStack(material) matches.
func plain(e *core.Essentials, s plugin.ItemStack) bool {
	return e.Server.Items().Similar(s, plugin.NewStack(s.Item, 1))
}

// maxDurability answers the item's max damage, 0 for one that never wears.
func maxDurability(s plugin.ItemStack) int {
	if s.Patch.MaxDamage != nil {
		return *s.Patch.MaxDamage
	}
	return int(registry.MaxDamage(int32(s.Item)))
}

func damage(s plugin.ItemStack) int {
	if s.Patch.Damage != nil {
		return *s.Patch.Damage
	}
	return 0
}

func isBlock(it registry.Item) bool { _, ok := it.Block(); return ok }

// sameStack reports whether two stacks are the same stack and count.
func sameStack(e *core.Essentials, a, b plugin.ItemStack) bool {
	if a.Empty() || b.Empty() {
		return a.Empty() && b.Empty()
	}
	return a.Count == b.Count && e.Server.Items().Similar(a, b)
}

// permExact answers a node's value and whether a permission provider set it
// explicitly (an operator's implicit nodes are not set).
// Essentials: User.isAuthorizedExact
func permExact(p plugin.Player, node string) (has, set bool) {
	if !p.PermissionSet(node) {
		return false, false
	}
	return p.HasPermission(node), true
}

// preventedType applies a prevent-type permission prefix (hat, itemname):
// the wildcard set true unless the item's node is set false, or the item's
// node set true.
func preventedType(p plugin.Player, prefix string, it registry.Item) bool {
	wild, wildSet := permExact(p, prefix+"*")
	mat, matSet := permExact(p, prefix+typeName(it))
	wildTrue := wildSet && wild
	return (wildTrue && !(matSet && !mat)) || (!wildTrue && matSet && mat)
}

// tlRaw answers a message as MiniMessage in the sender's locale, for
// building a message out of others. Essentials: playerTl / tl
func tlRaw(e *core.Essentials, sender plugin.CommandSender, key string, args ...any) string {
	loc := ""
	if p, ok := sender.(plugin.Player); ok && e.Settings.PerPlayerLocale() {
		loc = p.Locale()
	}
	return e.I18n.Format(loc, key, args...)
}

// usageWith shows the command's usage followed by a message, as Java's
// NotEnoughArgumentsException(message) does.
func usageWith(ctx *core.Ctx, key string, args ...any) error {
	ctx.Ess.ShowUsage(ctx)
	ctx.Send(key, args...)
	return core.ErrQuiet
}

// sendError shows an error's message to the user without errorWithMessage,
// as Java's user.sendMessage(ex.getMessage()) does.
func sendError(e *core.Essentials, u *core.User, err error) {
	if tl, ok := err.(*core.TlError); ok {
		u.Send(tl.Key, tl.Args...)
		return
	}
	u.SendText(plugin.Plain(err.Error()))
}

// joinList joins with ", ". Essentials: StringUtil.joinList
func joinList(items []string) string { return strings.Join(items, ", ") }

// onlineTargets answers the users a loop command acts on: everyone for * or
// ** (skipping players the sender may not see), else the one player named.
// Essentials: EssentialsLoopCommand.loopOnlinePlayersConsumer
func onlineTargets(ctx *core.Ctx, term string) ([]*core.User, error) {
	if term == "" {
		return nil, core.ErrPlayerNotFound
	}
	if ctx.User != nil && (term == "@s" || term == "@p") {
		return []*core.User{ctx.User}, nil
	}
	if term == "*" || term == "**" {
		var out []*core.User
		for _, u := range ctx.Ess.OnlineUsers() {
			if ctx.User == nil || ctx.User.CanSee(u) {
				out = append(out, u)
			}
		}
		return out, nil
	}
	u, err := ctx.Ess.MatchUser(ctx.User, term, false, false)
	if err != nil {
		return nil, err
	}
	return []*core.User{u}, nil
}

// loopPlayers completes a loop command's player argument.
func loopPlayers(ctx *core.Ctx) []string { return append(ctx.CompletePlayers(), "*", "**") }

// matchPlayers answers the online players whose name contains term, or the
// one named exactly. Bukkit: Server.matchPlayer
func matchPlayers(e *core.Essentials, term string) []plugin.Player {
	var out []plugin.Player
	low := strings.ToLower(term)
	for _, p := range e.Server.Players() {
		name := strings.ToLower(p.Name())
		if name == low {
			return []plugin.Player{p}
		}
		if strings.Contains(name, low) {
			out = append(out, p)
		}
	}
	return out
}

// commandNames answers the plugins' command names, for the completion of a
// command bound to a powertool. Essentials: EssentialsCommand.getCommands
func commandNames(e *core.Essentials) []string {
	var out []string
	for _, c := range e.Server.Commands().All() {
		if c.Plugin != "" {
			out = append(out, c.Name)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// itemNames completes item names. Essentials: getItems
func itemNames(e *core.Essentials) []string { return e.Items.Completions() }

// capitalCase upper-cases the first letter and lower-cases the rest.
// Essentials: I18n.capitalCase
func capitalCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}
