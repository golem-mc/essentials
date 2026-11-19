// Package protect is EssentialsXProtect, ported to golem: the
// protect: section of config.yml turns off fire spread, liquid flow,
// explosions, mob griefing and transformations, kinds of damage, PvP and
// weather.
package protect

import (
	_ "embed"
	"errors"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

//go:embed plugin.yml
var pluginYML []byte

// Protect is the essentialsprotect plugin. Essentials: EssentialsProtect
type Protect struct {
	ess *core.Essentials
}

func init() { plugin.Register("essentialsprotect", &Protect{}, plugin.Depends("essentials")) }

// Enable registers the listeners. Essentials' emergency mode (Protect
// without Essentials) is not ported: the plugin depends on essentials.
// Essentials: EssentialsProtect.initialize
func (m *Protect) Enable(s plugin.Server) error {
	e, ok := core.Get()
	if !ok {
		return errors.New("essentials is not enabled")
	}
	m.ess = e
	if err := e.RegisterModulePermissions(s, pluginYML); err != nil {
		return err
	}
	o := plugin.ListenOptions{Priority: plugin.Highest, IgnoreCancelled: true}
	// EssentialsProtectBlockListener and _1_16_R1
	plugin.Listen(s, m.onBlockIgnite, o)
	plugin.Listen(s, m.onBlockFromTo, o)
	plugin.Listen(s, m.onBlockBurn, o)
	plugin.Listen(s, m.onPortalCreate, o)
	plugin.Listen(s, m.onInteract, o)
	// EssentialsProtectEntityListener, _1_13_2_R1, _1_14_R1, _1_21_3_R1
	plugin.Listen(s, m.onEntityDamage, o)
	plugin.Listen(s, m.onEntityExplode, o)
	plugin.Listen(s, m.onCreatureSpawn, o)
	plugin.Listen(s, m.onEntityTarget, o)
	plugin.Listen(s, m.onExplosionPrime, o)
	plugin.Listen(s, m.onEntityChangeBlock, o)
	plugin.Listen(s, m.onDoorBreak, o)
	plugin.Listen(s, m.onHangingBreak, o)
	plugin.Listen(s, m.onEntityTransform, o)
	plugin.Listen(s, m.onCreeperPower, o)
	// EssentialsProtectWeatherListener
	plugin.Listen(s, m.onWeatherChange, o)
	plugin.Listen(s, m.onThunderChange, o)
	plugin.Listen(s, m.onLightningStrike, o)
	return nil
}

// Disable does nothing.
func (m *Protect) Disable() {}

// on reads a protect setting from config.yml; it is read each time, so
// /essentials reload applies at once. Essentials: ProtectConfig,
// IProtect.getSettingBool
func (m *Protect) on(key string, def bool) bool { return m.ess.Config.Bool("protect."+key, def) }

// prevent reads protect.prevent.<key>, with ProtectConfig's default.
func (m *Protect) prevent(key string) bool {
	return m.on("prevent."+key, preventDefaults[key])
}

// disabled reads protect.disable.<key>.
func (m *Protect) disabled(key string) bool { return m.on("disable."+key, false) }

// preventDefaults are the protect.prevent settings ProtectConfig defaults
// to true.
var preventDefaults = map[string]bool{
	"fire-spread": true, "lava-fire-spread": true, "lightning-fire-spread": true,
	"creeper-explosion": true, "enderdragon-blockdamage": true,
}

// typeName answers an entity's type without its namespace, "" for none.
func typeName(ent plugin.Entity) string {
	if ent == nil {
		return ""
	}
	return entityTypeName(ent.Type())
}

func entityTypeName(t registry.EntityType) string { return strings.TrimPrefix(t.Name(), "minecraft:") }

func blockName(b registry.Block) string { return strings.TrimPrefix(b.Name(), "minecraft:") }

// Essentials: EssentialsProtectBlockListener.onBlockIgnite
func (m *Protect) onBlockIgnite(ev *plugin.BlockIgniteEvent) {
	b := ev.Block
	if b.State().Block() == registry.BlockObsidian || b.Relative(plugin.Down, 1).State().Block() == registry.BlockObsidian {
		ev.SetCancelled(m.prevent("portal-creation"))
		return
	}
	switch ev.Cause {
	case plugin.IgniteSpread:
		ev.SetCancelled(m.prevent("fire-spread"))
	case plugin.IgniteFlintAndSteel:
		ev.SetCancelled(m.prevent("flint-fire"))
	case plugin.IgniteLava:
		ev.SetCancelled(m.prevent("lava-fire-spread"))
	case plugin.IgniteLightning:
		ev.SetCancelled(m.prevent("lightning-fire-spread"))
	case plugin.IgniteFireball:
		ev.SetCancelled(m.prevent("fireball-fire"))
	}
}

// Essentials: EssentialsProtectBlockListener.onBlockFromTo
func (m *Protect) onBlockFromTo(ev *plugin.BlockFromToEvent) {
	switch ev.From.State().Block() {
	case registry.BlockWater:
		ev.SetCancelled(m.prevent("water-flow"))
	case registry.BlockLava:
		ev.SetCancelled(m.prevent("lava-flow"))
	case registry.BlockAir:
		ev.SetCancelled(m.prevent("water-bucket-flow"))
	}
}

// Essentials: EssentialsProtectBlockListener.onBlockBurn
func (m *Protect) onBlockBurn(ev *plugin.BlockBurnEvent) {
	if m.prevent("fire-spread") {
		ev.SetCancelled(true)
	}
}

// Essentials: EssentialsProtectBlockListener.onPortalLight
func (m *Protect) onPortalCreate(ev *plugin.PortalCreateEvent) {
	if ev.Reason == plugin.PortalFire {
		ev.SetCancelled(m.prevent("portal-creation"))
	}
}

// onInteract stops beds outside the overworld and respawn anchors outside
// the nether from exploding. Essentials:
// EssentialsProtectBlockListener.onPlayerInteract and _1_16_R1
func (m *Protect) onInteract(ev *plugin.PlayerInteractEvent) {
	if ev.Action != plugin.RightClickBlock || ev.Block == nil {
		return
	}
	dim := ev.Block.World.Dimension().Path
	state := ev.Block.State()
	name := blockName(state.Block())
	if strings.HasSuffix(name, "_bed") && dim != "overworld" {
		ev.SetCancelled(m.prevent("bed-explosion"))
		return
	}
	if state.Block() == registry.BlockRespawnAnchor && dim != "the_nether" {
		charges := registry.StateProps(uint16(state))["charges"]
		if (charges != "0" && (ev.Item.Empty() || ev.Item.Item != registry.ItemGlowstone)) || charges == "4" {
			ev.SetCancelled(m.prevent("respawn-anchor-explosion"))
		}
	}
}

// shouldBeDamaged reports whether a protected player still takes damage of
// a kind: essentials.protect.damage.<kind> without .disable.
// Essentials: EssentialsProtectEntityListener.shouldBeDamaged
func shouldBeDamaged(u *core.User, kind string) bool {
	return u.HasPermission("essentials.protect.damage."+kind) && !u.HasPermission("essentials.protect.damage.disable")
}

// damageCauses map Bukkit's damage causes to vanilla damage types.
var damageCauses = map[string]string{
	"cactus": "contact", "sweet_berry_bush": "contact", "stalagmite": "contact",
	"lava": "lava", "fall": "fall", "in_wall": "suffocation", "in_fire": "fire", "on_fire": "fire",
	"drown": "drowning", "lightning_bolt": "lightning", "wither": "wither", "explosion": "explosion",
	"player_explosion": "explosion",
}

// fireballs are what Bukkit's Fireball covers, wind charges aside.
var fireballs = map[string]bool{
	"fireball": true, "small_fireball": true, "dragon_fireball": true, "wither_skull": true, "breeze_wind_charge": true,
}

// onEntityDamage applies the damage settings. Essentials:
// EssentialsProtectEntityListener.onEntityDamage and
// EssentialsProtectBlockListener.onEntityDamageByEntity
func (m *Protect) onEntityDamage(ev *plugin.EntityDamageEvent) {
	target := ev.Entity
	targetType := typeName(target)
	cause := damageCauses[strings.TrimPrefix(ev.Source.Type.Name(), "minecraft:")]
	// The attacker is Bukkit's damager: the entity that hit. For a blast
	// that is the exploder (TNT, creeper, minecart, fireball), already
	// removed but still answering Type.
	attacker := ev.Source.Direct
	if attacker == nil {
		attacker = ev.Source.Causing
	}
	attackerType := typeName(attacker)
	byEntity := attackerType != ""

	// Essentials applies these to an item hurt by an entity only (its lava
	// rule never fires on Bukkit, where lava is a block's damage); here
	// they apply to every hurt of an item, as meant.
	if targetType == "item" {
		switch {
		case cause == "fire" || cause == "lava":
			ev.SetCancelled(m.prevent("lava-itemdamage"))
		case cause == "explosion" && attackerType == "tnt":
			ev.SetCancelled(m.prevent("tnt-itemdamage"))
		case cause == "explosion" && attackerType == "creeper":
			ev.SetCancelled(m.prevent("creeper-itemdamage"))
		case cause == "explosion" && attackerType == "tnt_minecart":
			ev.SetCancelled(m.prevent("tnt-minecart-itemdamage"))
		case cause == "explosion" && attackerType == "wither_skull":
			ev.SetCancelled(m.prevent("witherskull-itemdamage"))
		case cause == "explosion" && fireballs[attackerType]:
			ev.SetCancelled(m.prevent("fireball-itemdamage"))
		}
		if ev.Cancelled() {
			return
		}
	}

	if targetType == "villager" && m.prevent("villager-death") {
		ev.SetCancelled(true)
		return
	}
	var user *core.User
	if p, ok := target.(plugin.Player); ok {
		user = m.ess.Users.Of(p)
	}
	// exempt reports whether a player target takes this damage anyway.
	exempt := func(kind string) bool { return user != nil && shouldBeDamaged(user, kind) }

	if cause == "contact" && m.disabled("contactdmg") && !exempt("contact") {
		ev.SetCancelled(true)
		return
	}
	if cause == "lava" && m.disabled("lavadmg") && !exempt("lava") {
		ev.SetCancelled(true)
		return
	}

	if byEntity {
		switch {
		case attackerType == "creeper" && m.prevent("creeper-playerdamage") && !exempt("creeper"),
			// Essentials tests the target for a fireball or wither skull,
			// which can never be; the damager is what was meant.
			(attackerType == "fireball" || attackerType == "small_fireball") && m.prevent("fireball-playerdamage") && !exempt("fireball"),
			attackerType == "wither_skull" && m.prevent("witherskull-playerdamage") && !exempt("witherskull"),
			attackerType == "tnt" && m.prevent("tnt-playerdamage") && !exempt("tnt"),
			attackerType == "tnt_minecart" && m.prevent("tnt-minecart-playerdamage") && !exempt("tnt-minecart"):
			ev.SetCancelled(true)
			return
		}
		if ap, ok := attacker.(plugin.Player); ok && user != nil && m.disabled("pvp") && !strings.EqualFold(user.Name(), ap.Name()) &&
			(!user.HasPermission("essentials.protect.pvp") || !ap.HasPermission("essentials.protect.pvp")) {
			ev.SetCancelled(true)
			return
		}
		if proj, ok := attacker.(plugin.Projectile); ok && user != nil {
			shooter, _ := proj.Shooter()
			sp, byPlayer := shooter.(plugin.Player)
			if (m.disabled("projectiles") && !shouldBeDamaged(user, "projectiles")) ||
				(byPlayer && m.disabled("pvp") && (!user.HasPermission("essentials.protect.pvp") || !sp.HasPermission("essentials.protect.pvp"))) {
				ev.SetCancelled(true)
				return
			}
		}
	}

	if user == nil {
		return
	}
	for _, d := range []struct{ cause, setting, kind string }{
		{"fall", "fall", "fall"},
		{"suffocation", "suffocate", "suffocation"},
		{"fire", "firedmg", "fire"},
		{"drowning", "drown", "drowning"},
		{"lightning", "lightning", "lightning"},
		{"wither", "wither", "wither"},
	} {
		if cause == d.cause && m.disabled(d.setting) && !shouldBeDamaged(user, d.kind) {
			ev.SetCancelled(true)
			return
		}
	}
}

// onEntityExplode stops explosions by what exploded; a stopped creeper's
// blast is still shown, harmlessly. Essentials:
// EssentialsProtectEntityListener.onEntityExplode and _1_21_3_R1
func (m *Protect) onEntityExplode(ev *plugin.EntityExplodeEvent) {
	t := entityTypeName(ev.Type)
	if t == "" {
		return
	}
	loc := ev.Location
	fake := func() {
		if loc.World != nil {
			loc.World.Explode(loc.Pos, 0, false, plugin.ExplosionKeep, nil)
		}
	}
	maxHeight := m.ess.Config.Int("protect.creeper.max-height", -1)
	switch {
	case t == "ender_dragon" && m.prevent("enderdragon-blockdamage"):
		ev.SetCancelled(true)
		if m.on("enderdragon-fakeexplosions", false) {
			fake()
		}
	case t == "wither" && m.prevent("wither-spawnexplosion"):
		ev.SetCancelled(true)
	case t == "creeper" && (m.prevent("creeper-explosion") || m.prevent("creeper-blockdamage") ||
		(maxHeight >= 0 && loc.Pos.Block().Y > maxHeight)):
		ev.SetCancelled(true)
		fake()
	case t == "tnt" && m.prevent("tnt-explosion"):
		ev.SetCancelled(true)
	// Bukkit's Fireball covers the wither skull, so fireball-explosion
	// stops it too.
	case fireballs[t] && m.prevent("fireball-explosion"):
		ev.SetCancelled(true)
	case t == "wither_skull" && m.prevent("witherskull-explosion"):
		ev.SetCancelled(true)
	case t == "tnt_minecart" && m.prevent("tnt-minecart-explosion"):
		ev.SetCancelled(true)
	case t == "end_crystal" && m.prevent("ender-crystal-explosion"):
		ev.SetCancelled(true)
	case t == "wind_charge" && m.prevent("windcharge-explosion"):
		ev.SetCancelled(true)
	}
}

// onCreatureSpawn stops the creatures protect.prevent.spawn lists. It is
// Bukkit's CreatureSpawnEvent, which is for living entities only: the
// items, projectiles, boats and minecarts EntitySpawnEvent also reports
// are left alone. Essentials:
// EssentialsProtectEntityListener.onCreatureSpawn
func (m *Protect) onCreatureSpawn(ev *plugin.EntitySpawnEvent) {
	if _, ok := ev.Entity.(plugin.Player); ok {
		return
	}
	if _, ok := ev.Entity.(plugin.LivingEntity); !ok {
		return
	}
	if name := typeName(ev.Entity); name != "" && m.ess.Config.Bool("protect.prevent.spawn."+name, false) {
		ev.SetCancelled(true)
	}
}

// onEntityTarget stops mobs targeting players. Bukkit's reasons map to
// TargetClosestPlayer (CLOSEST_PLAYER, RANDOM_TARGET: any target change to
// a player read off the tick), TargetAttacked (TARGET_ATTACKED_ENTITY and
// its alert, TARGET_ATTACKED_NEARBY_ENTITY), TargetDefendOwner
// (DEFEND_VILLAGE, the fox defending whom it trusts), TargetOwnerAttacked
// (TARGET_ATTACKED_OWNER) and TargetOwnerAttackedTarget
// (OWNER_ATTACKED_TARGET). Essentials:
// EssentialsProtectEntityListener.onEntityTarget
func (m *Protect) onEntityTarget(ev *plugin.EntityTargetEvent) {
	p, ok := ev.Target.(plugin.Player)
	if !ok {
		return
	}
	switch ev.Reason {
	case plugin.TargetClosestPlayer, plugin.TargetAttacked, plugin.TargetDefendOwner,
		plugin.TargetOwnerAttacked, plugin.TargetOwnerAttackedTarget:
	default:
		return
	}
	if m.prevent("entitytarget") && !p.HasPermission("essentials.protect.entitytarget.bypass") {
		ev.SetCancelled(true)
	}
}

// Essentials: EssentialsProtectEntityListener.onExplosionPrime
func (m *Protect) onExplosionPrime(ev *plugin.ExplosionPrimeEvent) {
	if fireballs[entityTypeName(ev.Type)] && m.prevent("fireball-fire") {
		ev.Fire = false
	}
}

// Essentials: EssentialsProtectEntityListener.onDoorBreak
func (m *Protect) onDoorBreak(ev *plugin.EntityBreakDoorEvent) {
	if typeName(ev.Entity) == "zombie" && m.prevent("zombie-door-break") {
		ev.SetCancelled(true)
	}
}

// Essentials: EssentialsProtectEntityListener.onEntityChangeBlock and
// _1_14_R1
func (m *Protect) onEntityChangeBlock(ev *plugin.EntityChangeBlockEvent) {
	switch typeName(ev.Entity) {
	case "enderman":
		if m.prevent("enderman-pickup") {
			ev.SetCancelled(true)
		}
	case "wither":
		if m.prevent("wither-blockreplace") {
			ev.SetCancelled(true)
		}
	case "sheep":
		if m.prevent("sheep-eat-grass") {
			ev.SetCancelled(true)
		}
	case "ravager":
		if m.prevent("ravager-thief") {
			ev.SetCancelled(true)
		}
	}
}

// onHangingBreak keeps paintings and item frames from the blasts and
// fireballs the explosion rules stop. Essentials tests for cause ENTITY,
// which is what Bukkit reported for an entity's blast when it was written
// (the remover being the exploder); Bukkit has since moved blasts to
// EXPLOSION, so the rule matches both here.
// Essentials: EssentialsProtectEntityListener.onPaintingBreak
func (m *Protect) onHangingBreak(ev *plugin.HangingBreakEvent) {
	if ev.Cause != plugin.HangingBreakEntity && ev.Cause != plugin.HangingBreakExplosion {
		return
	}
	t := typeName(ev.Remover)
	if t == "creeper" && m.prevent("creeper-explosion") ||
		(t == "fireball" || t == "small_fireball") && m.prevent("fireball-explosion") ||
		t == "tnt" && m.prevent("tnt-explosion") ||
		t == "wither_skull" && m.prevent("witherskull-explosion") {
		ev.SetCancelled(true)
	}
}

// Essentials: EssentialsProtectEntityListener_1_13_2_R1.onEntityTransform
func (m *Protect) onEntityTransform(ev *plugin.EntityTransformEvent) {
	t := typeName(ev.Entity)
	switch ev.Reason {
	case plugin.TransformInfection:
		ev.SetCancelled(m.prevent("transformation.zombie-villager"))
	case plugin.TransformCured:
		ev.SetCancelled(m.prevent("transformation.villager"))
	case plugin.TransformLightning:
		switch t {
		case "villager":
			ev.SetCancelled(m.prevent("transformation.witch"))
		case "pig":
			ev.SetCancelled(m.prevent("transformation.zombie-pigman"))
		case "creeper":
			ev.SetCancelled(m.prevent("transformation.charged-creeper"))
		case "mooshroom":
			ev.SetCancelled(m.prevent("transformation.mooshroom"))
		}
	case plugin.TransformDrowned:
		ev.SetCancelled(m.prevent("transformation.drowned"))
	}
}

// onCreeperPower stops lightning charging a creeper, which Bukkit reports
// as CreeperPowerEvent rather than the transformation Essentials listens
// for. Essentials: prevent_creeper_charge
func (m *Protect) onCreeperPower(ev *plugin.CreeperPowerEvent) {
	if m.prevent("transformation.charged-creeper") {
		ev.SetCancelled(true)
	}
}

// Essentials: EssentialsProtectWeatherListener.onWeatherChange
func (m *Protect) onWeatherChange(ev *plugin.WeatherChangeEvent) {
	if m.disabled("weather.storm") && ev.Raining {
		ev.SetCancelled(true)
	}
}

// Essentials: EssentialsProtectWeatherListener.onThunderChange
func (m *Protect) onThunderChange(ev *plugin.ThunderChangeEvent) {
	if m.disabled("weather.thunder") && ev.Thundering {
		ev.SetCancelled(true)
	}
}

// Essentials: EssentialsProtectWeatherListener.onLightningStrike
func (m *Protect) onLightningStrike(ev *plugin.LightningStrikeEvent) {
	if m.disabled("weather.lightning") {
		ev.SetCancelled(true)
	}
}
