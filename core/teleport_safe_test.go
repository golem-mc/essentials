package core_test

import (
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"

	"github.com/golem-mc/essentials/internal/esstest"
)

// netherWorld is a nether-shaped column world: bedrock 0 and 123-127, a
// netherrack ceiling at 100-122 and floor up to 30, lava at y=31 under open
// air, bedrock roof above and air over it up to 255.
type netherWorld struct {
	plugin.World
}

func block(name string) plugin.BlockState {
	b, _ := registry.BlockByName("minecraft:" + name)
	return plugin.BlockState(b.DefaultState())
}

func (netherWorld) MinY() int          { return 0 }
func (netherWorld) Height() int        { return 256 }
func (netherWorld) LogicalHeight() int { return 128 }
func (netherWorld) Border() plugin.WorldBorder {
	return nil
}
func (netherWorld) BlockState(p plugin.BlockPos) plugin.BlockState {
	switch {
	case p.Y == 0 || (p.Y >= 123 && p.Y <= 127):
		return block("bedrock")
	case p.Y >= 100 && p.Y < 123:
		return block("netherrack")
	case p.Y == 31:
		if p.X >= -2 && p.X <= 2 && p.Z >= -2 && p.Z <= 2 {
			return block("lava")
		}
		return block("netherrack")
	case p.Y < 31:
		return block("netherrack")
	}
	return block("air")
}
func (netherWorld) HighestBlockY(x, z int, h plugin.Heightmap) int { return 127 }

func TestSafeDestinationNetherRoof(t *testing.T) {
	env := esstest.Start(t)
	w := netherWorld{}
	at := func(y float64) plugin.Location {
		return plugin.Location{World: w, Pos: plugin.Vec3{X: 0.5, Y: y, Z: 0.5}}
	}
	for _, y := range []float64{31, 32, 34} {
		got, err := env.Ess.SafeDestination(at(y))
		if err != nil || got.Pos.Y >= 128 {
			t.Errorf("y=%v: landed at %v, %v", y, got.Pos, err)
		}
	}
	got, err := env.Ess.SafeDestination(at(130))
	if err != nil || got.Pos.Y < 128 {
		t.Errorf("above the roof: landed at %v, %v", got.Pos, err)
	}
	env.Ess.Config.Set("consider-world-height-for-teleport-safety", false)
	got, err = env.Ess.SafeDestination(at(60))
	if err != nil || got.Pos.Y < 128 {
		t.Errorf("without the cap: landed at %v, %v", got.Pos, err)
	}
}
