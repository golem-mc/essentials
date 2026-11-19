package teleport

import (
	"math"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// The block checks of Essentials' LocationUtil that commands use directly;
// core keeps its own copies private.
var (
	damagingBlocks = registry.BlocksOf(func(b registry.Block) bool {
		switch strings.TrimPrefix(b.Name(), "minecraft:") {
		case "cactus", "campfire", "soul_campfire", "fire", "soul_fire", "magma_block", "sweet_berry_bush", "wither_rose":
			return true
		}
		return strings.HasSuffix(b.Name(), "_bed")
	})
	lavaBlock, _    = registry.BlockByName("minecraft:lava")
	waterBlock, _   = registry.BlockByName("minecraft:water")
	portalBlock, _  = registry.BlockByName("minecraft:nether_portal")
	bedrockBlock, _ = registry.BlockByName("minecraft:bedrock")
)

// hollow reports whether a player can stand inside s. Essentials:
// LocationUtil.HOLLOW_MATERIALS
func (g *group) hollow(s plugin.BlockState) bool {
	switch s.Block() {
	case lavaBlock:
		return false
	case waterBlock:
		return g.e.Settings.WaterSafe()
	}
	return registry.CollisionHeight(uint16(s)) == 0
}

// transparent reports whether a line of sight passes s: hollow blocks and
// water. Essentials: LocationUtil.TRANSPARENT_MATERIALS
func transparent(s plugin.BlockState) bool {
	b := s.Block()
	return b == waterBlock || b != lavaBlock && registry.CollisionHeight(uint16(s)) == 0
}

func state(w plugin.World, x, y, z int) plugin.BlockState {
	return w.BlockState(plugin.BlockPos{X: x, Y: y, Z: z})
}

// maxHeight is the world's build limit, exclusive. Bukkit: World.getMaxHeight
func maxHeight(w plugin.World) int { return w.MinY() + w.Height() }

// damaging reports whether standing at x, y, z hurts. Essentials:
// LocationUtil.isBlockDamaging
func (g *group) damaging(w plugin.World, x, y, z int) bool {
	below := state(w, x, y-1, z).Block()
	if damagingBlocks[below] || below == lavaBlock {
		return true
	}
	at := state(w, x, y, z)
	if at.Block() == portalBlock {
		return true
	}
	return !g.hollow(at) || !g.hollow(state(w, x, y+1, z))
}

// aboveAir reports whether nothing holds a player up at x, y, z.
// Essentials: LocationUtil.isBlockAboveAir
func (g *group) aboveAir(w plugin.World, x, y, z int) bool {
	return y > maxHeight(w) || g.hollow(state(w, x, y-1, z))
}

// outsideBorder reports whether x, z is past the world border.
// Essentials: LocationUtil.isBlockOutsideWorldBorder
func outsideBorder(w plugin.World, x, z int) bool {
	b := w.Border()
	if b == nil {
		return false
	}
	cx, cz := b.Center()
	r := int(b.Size()) / 2
	bx, bz := int(math.Floor(cx)), int(math.Floor(cz))
	return x < bx-r || x > bx+r || z < bz-r || z > bz+r
}

// unsafe reports whether standing at x, y, z hurts or falls.
// Essentials: LocationUtil.isBlockUnsafe
func (g *group) unsafe(w plugin.World, x, y, z int) bool {
	return g.damaging(w, x, y, z) || g.aboveAir(w, x, y, z)
}

// unsafeFor is unsafe, with the border, for u; never for a flying player in
// creative, spectator or god mode in that world.
// Essentials: LocationUtil.isBlockUnsafeForUser
func (g *group) unsafeFor(u *core.User, w plugin.World, x, y, z int) bool {
	if p, ok := u.Player(); ok && sameWorld(w, p.World()) && p.AllowFlight() {
		if gm := p.GameMode(); gm == plugin.Creative || gm == plugin.Spectator || u.IsGodMode() {
			return false
		}
	}
	return g.unsafe(w, x, y, z) || outsideBorder(w, x, z)
}

// targetDistance is how far Essentials looks for /jump's target; Bukkit
// caps the line of sight at 120.
const targetDistance = 300

// targetBlock answers the first block the player looks at that a line of
// sight does not pass (lava, solid blocks), or the last block within
// reach; ok is false for a player who is gone.
// Essentials: LocationUtil.getTarget (Bukkit: getTargetBlock)
func targetBlock(p plugin.Player) (plugin.BlockPos, bool) {
	b := p.TargetBlockPassing(targetDistance, transparent)
	return b.Pos, b.World != nil
}
