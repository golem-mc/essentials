package antibuild

import (
	"strconv"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

func itoa(n int) string { return strconv.Itoa(n) }

// registerListeners registers the build checks. Essentials:
// EssentialsAntiBuildListener
func (m *AntiBuild) registerListeners(s plugin.Server) {
	highest := plugin.ListenOptions{Priority: plugin.Highest, IgnoreCancelled: true}
	high := plugin.ListenOptions{Priority: plugin.High, IgnoreCancelled: true}
	low := plugin.ListenOptions{Priority: plugin.Low, IgnoreCancelled: true}
	plugin.Listen(s, m.onBlockPlace, highest)
	plugin.Listen(s, m.onBlockBreak, highest)
	plugin.Listen(s, m.onHangingBreak, highest)
	plugin.Listen(s, m.onInteractEntity, high)
	plugin.Listen(s, m.onEntityDamage, highest)
	plugin.Listen(s, m.onPiston, highest)
	// The interact event has two cancelled states, so it runs even when
	// cancelled.
	plugin.Listen(s, m.onInteract, plugin.ListenOptions{Priority: plugin.Low})
	plugin.Listen(s, m.onCraft, low)
	plugin.Listen(s, m.onDrop, low)
	plugin.Listen(s, m.onPickup, low)
	plugin.Listen(s, m.onDispense, highest)
}

// Essentials: EssentialsAntiBuildListener.onBlockPlace
func (m *AntiBuild) onBlockPlace(ev *plugin.BlockPlaceEvent) {
	u := m.ess.Users.Of(ev.Player)
	b := ev.Placed.Block()
	// A state change (an eye of ender in a frame) is no placement.
	if b == ev.Replaced.Block() {
		return
	}
	mat := blockMaterial(b)
	if m.disableBuild() && !canBuild(u) && !permitted(u, "place", mat) ||
		m.listed(blacklistPlacement, mat) && !u.HasPermission("essentials.protect.exemptplacement") {
		m.deny(u, "antiBuildPlace", nameOf(mat))
		ev.SetCancelled(true)
		return
	}
	if m.listed(alertOnPlacement, mat) && !u.HasPermission("essentials.protect.alerts.notrigger") {
		m.alert(u, nameOf(mat), "alertPlaced")
	}
}

// Essentials: EssentialsAntiBuildListener.onBlockBreak
func (m *AntiBuild) onBlockBreak(ev *plugin.BlockBreakEvent) {
	u := m.ess.Users.Of(ev.Player)
	mat := blockMaterial(ev.State.Block())
	if m.wouldBreakSigns(ev.Block, func(sign plugin.Block) bool { return m.signProtected(sign, u) }) ||
		m.disableBuild() && !canBuild(u) && !permitted(u, "break", mat) ||
		m.listed(blacklistBreak, mat) && !u.HasPermission("essentials.protect.exemptbreak") {
		m.deny(u, "antiBuildBreak", nameOf(mat))
		ev.SetCancelled(true)
		return
	}
	if m.listed(alertOnBreak, mat) && !u.HasPermission("essentials.protect.alerts.notrigger") {
		m.alert(u, nameOf(mat), "alertBroke")
	}
}

// signProtected reports whether the user may not break a sign.
// Essentials: EssentialsAntiBuildListener.isSignProtected
func (m *AntiBuild) signProtected(sign plugin.Block, u *core.User) bool {
	mat := blockMaterial(sign.State().Block())
	if m.disableBuild() && !canBuild(u) && !permitted(u, "break", mat) {
		return true
	}
	return m.listed(blacklistBreak, mat) && !u.HasPermission("essentials.protect.exemptbreak")
}

// sign kinds by registry name.
func isSignPost(name string) bool {
	return strings.HasSuffix(name, "_sign") && !strings.Contains(name, "wall_") && !strings.Contains(name, "hanging")
}
func isHangingSign(name string) bool {
	return strings.HasSuffix(name, "_hanging_sign") && !strings.Contains(name, "wall_")
}
func isWallSign(name string) bool { return strings.HasSuffix(name, "_wall_sign") }
func isWallHangingSign(name string) bool {
	return strings.HasSuffix(name, "_wall_hanging_sign")
}

// wouldBreakSigns reports whether breaking or moving b drops a sign that
// check refuses: a post above, a hanging sign below, a wall sign on a side.
// Essentials: EssentialsAntiBuildListener.wouldBreakAttachedSigns
func (m *AntiBuild) wouldBreakSigns(b plugin.Block, check func(plugin.Block) bool) bool {
	name := func(b plugin.Block) string { return b.State().Block().Name() }
	if up := b.Relative(plugin.Up, 1); isSignPost(name(up)) && check(up) {
		return true
	}
	if down := b.Relative(plugin.Down, 1); isHangingSign(name(down)) && check(down) {
		return true
	}
	for _, f := range []struct {
		face plugin.BlockFace
		name string
	}{{plugin.North, "north"}, {plugin.East, "east"}, {plugin.South, "south"}, {plugin.West, "west"}} {
		side := b.Relative(f.face, 1)
		n := name(side)
		if !isWallSign(n) && !isWallHangingSign(n) {
			continue
		}
		if registry.StateProps(uint16(side.State()))["facing"] == f.name && check(side) {
			return true
		}
	}
	return false
}

// Essentials: EssentialsAntiBuildListener.onHangingBreak
func (m *AntiBuild) onHangingBreak(ev *plugin.HangingBreakEvent) {
	p, ok := ev.Remover.(plugin.Player)
	if !ok {
		return
	}
	u := m.ess.Users.Of(p)
	if !m.disableBuild() || canBuild(u) {
		return
	}
	var mat string
	switch ev.Entity.Type().Name() {
	case "minecraft:painting":
		mat = "PAINTING"
	case "minecraft:item_frame":
		mat = "ITEM_FRAME"
	default:
		return
	}
	if !permitted(u, "break", mat) {
		m.deny(u, "antiBuildBreak", mat)
		ev.SetCancelled(true)
	}
}

// heldItem answers what the player holds in hand.
func heldItem(p plugin.Player, h plugin.Hand) plugin.ItemStack {
	if h == plugin.OffHand {
		return p.Inventory().Get(plugin.SlotOffHand)
	}
	return p.Inventory().Get(plugin.SlotMainHand)
}

// onInteractEntity guards shearing, putting items into item frames and
// onto armour stands. The one event is Bukkit's PlayerInteractEntityEvent
// and, for armour stands, PlayerInteractAtEntityEvent. Essentials:
// EssentialsAntiBuildListener.onPlayerInteractEntity and
// onPlayerInteractAtEntity
func (m *AntiBuild) onInteractEntity(ev *plugin.PlayerInteractEntityEvent) {
	u := m.ess.Users.Of(ev.Player)
	hand := heldItem(ev.Player, ev.Hand)
	if !hand.Empty() && hand.Item == registry.ItemShears {
		if m.disableUse() && !canBuild(u) && !permitted(u, "interact", itemMaterial(hand.Item)) {
			m.deny(u, "antiBuildUse", itemMaterial(hand.Item))
			ev.SetCancelled(true)
			return
		}
	}
	var mat string
	switch ev.Target.Type().Name() {
	case "minecraft:item_frame", "minecraft:glow_item_frame":
		mat = "ITEM_FRAME"
	case "minecraft:armor_stand":
		mat = "ARMOR_STAND"
	default:
		return
	}
	if m.placeDenied(u, mat) {
		ev.SetCancelled(true)
	}
}

// placeDenied applies the place checks to using an item frame or armour
// stand, alerting when it is allowed and listed.
func (m *AntiBuild) placeDenied(u *core.User, mat string) bool {
	if m.disableBuild() && !canBuild(u) && !u.HasPermission("essentials.build") && !permitted(u, "place", mat) ||
		m.listed(blacklistPlacement, mat) && !u.HasPermission("essentials.protect.exemptplacement") {
		m.deny(u, "antiBuildPlace", mat)
		return true
	}
	if m.listed(alertOnPlacement, mat) && !u.HasPermission("essentials.protect.alerts.notrigger") {
		m.alert(u, mat, "alertPlaced")
	}
	return false
}

// breakableEntity answers the material of an item frame, armour stand or
// end crystal.
func breakableEntity(e plugin.Entity) (string, bool) {
	switch e.Type().Name() {
	case "minecraft:item_frame", "minecraft:glow_item_frame":
		return "ITEM_FRAME", true
	case "minecraft:armor_stand":
		return "ARMOR_STAND", true
	case "minecraft:end_crystal":
		return "END_CRYSTAL", true
	}
	return "", false
}

// onEntityDamage guards item frames, armour stands and end crystals from
// a player's blows and projectiles. Essentials:
// EssentialsAntiBuildListener.onBlockEntityDamage
func (m *AntiBuild) onEntityDamage(ev *plugin.EntityDamageEvent) {
	var p plugin.Player
	switch d := ev.Source.Direct.(type) {
	case plugin.Player:
		p = d
	case plugin.Projectile:
		shooter, _ := d.Shooter()
		sp, ok := shooter.(plugin.Player)
		if !ok {
			return
		}
		p = sp
	default:
		return
	}
	if m.entityBreakDenied(p, ev.Entity) {
		ev.SetCancelled(true)
	}
}

// entityBreakDenied applies the break checks to damaging a breakable
// entity, alerting when it is allowed and listed.
func (m *AntiBuild) entityBreakDenied(p plugin.Player, target plugin.Entity) bool {
	mat, ok := breakableEntity(target)
	if !ok {
		return false
	}
	u := m.ess.Users.Of(p)
	if m.disableBuild() && !canBuild(u) && !u.HasPermission("essentials.build") && !permitted(u, "break", mat) ||
		m.listed(blacklistBreak, mat) && !u.HasPermission("essentials.protect.exemptbreak") {
		m.deny(u, "antiBuildBreak", mat)
		return true
	}
	if m.listed(alertOnBreak, mat) && !u.HasPermission("essentials.protect.alerts.notrigger") {
		m.alert(u, mat, "alertBroke")
	}
	return false
}

// onPiston stops pistons moving listed blocks or breaking signs; a plain
// piston's retraction moves nothing. Essentials:
// EssentialsAntiBuildListener.onBlockPistonExtend and onBlockPistonRetract
func (m *AntiBuild) onPiston(ev *plugin.BlockPistonEvent) {
	if !ev.Extending && ev.Piston.State().Block() != registry.BlockStickyPiston {
		return
	}
	for _, b := range ev.Blocks {
		if m.listed(blacklistPiston, blockMaterial(b.State().Block())) ||
			m.wouldBreakSigns(b, func(plugin.Block) bool { return true }) {
			ev.SetCancelled(true)
			return
		}
	}
}

// Essentials: EssentialsAntiBuildListener.onPlayerInteract
func (m *AntiBuild) onInteract(ev *plugin.PlayerInteractEvent) {
	u := m.ess.Users.Of(ev.Player)
	hasItem := !ev.Item.Empty()
	item := ""
	if hasItem {
		item = itemMaterial(ev.Item.Item)
	}
	if hasItem && m.listed(blacklistUsage, item) && !u.HasPermission("essentials.protect.exemptusage") {
		m.deny(u, "antiBuildUse", item)
		ev.SetCancelled(true)
		return
	}
	if hasItem && m.listed(alertOnUse, item) && !u.HasPermission("essentials.protect.alerts.notrigger") {
		m.alert(u, item, "alertUsed")
	}
	if !m.disableUse() || canBuild(u) {
		return
	}
	if hasItem && !permitted(u, "interact", item) {
		ev.SetCancelled(true)
		m.deny(u, "antiBuildUse", item)
		return
	}
	if ev.Block != nil {
		if mat := blockMaterial(ev.Block.State().Block()); !permitted(u, "interact", mat) {
			ev.SetCancelled(true)
			m.deny(u, "antiBuildInteract", mat)
		}
	}
}

// Essentials: EssentialsAntiBuildListener.onCraftItemEvent
func (m *AntiBuild) onCraft(ev *plugin.CraftItemEvent) {
	u := m.ess.Users.Of(ev.Player)
	if ev.Result.Empty() || !m.disableUse() || canBuild(u) {
		return
	}
	if mat := itemMaterial(ev.Result.Item); !permitted(u, "craft", mat) {
		ev.SetCancelled(true)
		m.deny(u, "antiBuildCraft", mat)
	}
}

// Essentials: EssentialsAntiBuildListener.onPlayerDropItem
func (m *AntiBuild) onDrop(ev *plugin.PlayerDropItemEvent) {
	u := m.ess.Users.Of(ev.Player)
	if !m.disableUse() || canBuild(u) || ev.Item == nil {
		return
	}
	if mat := itemMaterial(ev.Item.Stack().Item); !permitted(u, "drop", mat) {
		ev.SetCancelled(true)
		m.deny(u, "antiBuildDrop", mat)
	}
}

// Essentials: EssentialsAntiBuildListener.EntityPickupItemListener
func (m *AntiBuild) onPickup(ev *plugin.EntityPickupItemEvent) {
	p, ok := ev.Entity.(plugin.Player)
	if !ok || ev.Item == nil {
		return
	}
	u := m.ess.Users.Of(p)
	if m.disableUse() && !canBuild(u) && !permitted(u, "pickup", itemMaterial(ev.Item.Stack().Item)) {
		ev.SetCancelled(true)
	}
}

// Essentials: EssentialsAntiBuildListener.onBlockDispense
func (m *AntiBuild) onDispense(ev *plugin.BlockDispenseEvent) {
	if !ev.Item.Empty() && m.listed(blacklistDispenser, itemMaterial(ev.Item.Item)) {
		ev.SetCancelled(true)
	}
}
