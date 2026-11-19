package spawn_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golem-mc/essentials/internal/esstest"
	_ "github.com/golem-mc/essentials/spawn"
	"github.com/golem-mc/golem/plugin"
)

func init() { esstest.RegisterCore() }

// TestSetspawnSpawn sets the default spawn, moves away and comes back with
// /spawn; spawn.yml keeps the point.
func TestSetspawnSpawn(t *testing.T) {
	env := esstest.Start(t)
	c := env.Join("Alice", true)
	home := env.Player("Alice").Location().Pos
	env.Expect(env.Run(c, "setspawn"), "Spawn location set for group default.")
	b, err := os.ReadFile(filepath.Join(env.Ess.Server.DataDir(), "spawn.yml"))
	if err != nil || !strings.Contains(string(b), "default:") {
		t.Fatalf("spawn.yml = %q, %v", b, err)
	}

	env.Console("tp Alice ~40 ~ ~40")
	env.Drain(c)
	if d := env.Player("Alice").Location().Pos.DistanceSq(home); d < 100 {
		t.Fatalf("tp did not move Alice: %v", d)
	}
	env.Expect(env.Run(c, "spawn"), "Teleporting...")
	if d := env.Player("Alice").Location().Pos.DistanceSq(home); d > 2 {
		t.Fatalf("Alice is %v blocks² from spawn after /spawn", d)
	}

	env.Console("tp Alice ~40 ~ ~40")
	env.Drain(c)
	env.Expect(env.Console("spawn Alice"), "Teleporting...")
	if d := env.Player("Alice").Location().Pos.DistanceSq(home); d > 2 {
		t.Fatalf("Alice is %v blocks² from spawn after console /spawn", d)
	}
}

// TestNewbieAnnounce checks that a first join is announced to the others.
func TestNewbieAnnounce(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", false)
	env.Join("Bob", false)
	env.Expect(env.Messages(a, 10), "Welcome Bob to the server!")
}

// noBypass is a permission provider taking the warmup bypass away.
type noBypass struct{}

func (noBypass) Has(_ plugin.CommandSender, node string) (has, decided bool) {
	return false, node == "essentials.teleport.timer.bypass"
}

// TestSpawnWarmup checks that with a warmup "Teleporting..." and
// teleportAtoB come once the teleport has happened, not when it starts.
func TestSpawnWarmup(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	b := env.Join("Bob", false)
	home := env.Player("Alice").Location().Pos
	env.Run(a, "setspawn")
	plugin.Provide[plugin.PermissionProvider](env.Ess.Server, noBypass{}, 0)
	env.Ess.Config.Set("teleport-delay", 1)
	env.Console("tp Bob ~40 ~ ~40")
	env.Drain(b)

	msgs := env.Run(a, "spawn Bob")
	env.Expect(msgs, "Teleportation will commence")
	if esstest.Has(msgs, "Teleporting...") {
		t.Fatalf("Teleporting... sent before the warmup: %q", msgs)
	}
	env.Expect(env.Messages(a, 40), "Teleporting...")
	env.Expect(env.Messages(b, 3), "Alice teleported you to spawn.")
	if d := env.Player("Bob").Location().Pos.DistanceSq(home); d > 2 {
		t.Fatalf("Bob is %v blocks² from spawn", d)
	}
}
