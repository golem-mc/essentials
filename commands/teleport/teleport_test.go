package teleport_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/golem-mc/essentials/commands/teleport"
	"github.com/golem-mc/essentials/internal/esstest"
	"github.com/golem-mc/golem/plugin"
)

func init() { esstest.RegisterCore(teleport.Register) }

// moveTo puts the player at x, z at the height they are at, with vanilla's
// /tp (Essentials owns /tp), and lets c's messages pass.
func moveTo(env *esstest.Env, c *esstest.Client, name string, x, z float64) {
	env.T.Helper()
	y := env.Player(name).Pos().Y
	env.Console(fmt.Sprintf("minecraft:tp %s %v %v %v", name, x, y, z))
	env.Drain(c)
}

// expectNear fails unless the player is within a block of x, z.
func expectNear(env *esstest.Env, name string, x, z float64) {
	env.T.Helper()
	pos := env.Player(name).Pos()
	if math.Abs(pos.X-x) > 1 || math.Abs(pos.Z-z) > 1 {
		env.T.Fatalf("%s is at %v, want near %v, %v", name, pos, x, z)
	}
}

// TestHomesAndBack sets a home, goes home, returns with /back and deletes
// the home.
func TestHomesAndBack(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	c := a
	moveTo(env, a, "Alice", 10.5, 10.5)
	env.Expect(env.Run(c, "sethome"), "Home set to current location.")
	env.Expect(env.Run(c, "sethome 5"), "Invalid home name!")
	moveTo(env, a, "Alice", 0.5, 0.5)
	env.Expect(env.Run(c, "home"), "Teleporting to home.")
	expectNear(env, "Alice", 10.5, 10.5)
	env.Expect(env.Run(c, "back"), "Returning to previous location.")
	expectNear(env, "Alice", 0.5, 0.5)
	env.Expect(env.Run(c, "renamehome home house"), "Home home has been renamed to house.")
	env.Expect(env.Run(c, "delhome house"), "Home house has been removed.")
	env.Expect(env.Run(c, "delhome house"), "Home house doesn't exist!")
}

// TestWarps sets, lists, uses, describes and deletes a warp.
func TestWarps(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	c := a
	moveTo(env, a, "Alice", -7.5, 3.5)
	env.Expect(env.Run(c, "setwarp market"), "Warp market set.")
	env.Expect(env.Run(c, "warp"), "Warps: market")
	moveTo(env, a, "Alice", 0.5, 0.5)
	env.Expect(env.Run(c, "warp market"), "Warping to market.")
	expectNear(env, "Alice", -7.5, 3.5)
	env.Expect(env.Run(c, "warpinfo market"), "Information for warp market:")
	env.Console("warp market Alice")
	env.Expect(env.Run(c, "delwarp market"), "Warp market has been removed.")
	env.Expect(env.Run(c, "warp market"), "That warp does not exist.")
}

// TestTpaAccept sends a /tpa and a /tpahere and accepts them.
func TestTpaAccept(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	b := env.Join("Bob", true)
	moveTo(env, b, "Bob", 20.5, 0.5)
	env.Drain(a)
	env.Drain(b)

	env.Expect(env.Run(a, "tpa Bob"), "Request sent to Bob.")
	env.Expect(env.Messages(b, 2), "Alice has requested to teleport to you.")
	env.Expect(env.Run(a, "tpa Bob"), "You have already sent Bob a teleport request.")
	env.Expect(env.Run(b, "tpaccept"), "Teleport request accepted.")
	env.Expect(env.Messages(a, 2), "Bob accepted your teleport request.")
	expectNear(env, "Alice", 20.5, 0.5)

	moveTo(env, a, "Alice", 0.5, -15.5)
	env.Drain(b)
	env.Expect(env.Run(a, "tpahere Bob"), "Request sent to Bob.")
	env.Expect(env.Messages(b, 2), "Alice has requested that you teleport to them.")
	env.Expect(env.Run(b, "tpdeny"), "Teleport request denied.")
	env.Expect(env.Messages(a, 2), "Bob denied your teleport request.")
	env.Expect(env.Run(b, "tpaccept"), "You do not have a pending request.")

	env.Expect(env.Run(b, "tptoggle"), "Teleportation disabled.")
	env.Expect(env.Run(a, "tpa Bob"), "Bob has teleportation disabled.")
}

// TestTpAndPosition covers /tp between players, /tppos, /top and /getpos.
func TestTpAndPosition(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	b := env.Join("Bob", true)
	moveTo(env, b, "Bob", 30.5, 30.5)
	env.Expect(env.Run(a, "tp Bob"), "Teleporting to Bob.")
	expectNear(env, "Alice", 30.5, 30.5)
	env.Expect(env.Run(a, "tppos 5 ~ 5"), "Teleporting...")
	expectNear(env, "Alice", 5.5, 5.5)
	env.Expect(env.Run(a, "tppos 40000000 0 0"), "Value of coordinates cannot be over 30000000")
	env.Expect(env.Run(a, "top"), "Teleporting to top.")
	env.Expect(env.Run(a, "getpos"), "X: 5 (+East <-> -West)")
	env.Console("tp Alice Bob")
	expectNear(env, "Alice", 30.5, 30.5)

	// Looking south over the flat world, /jump goes to the end of the line
	// of sight, 120 blocks on, and lands on the ground.
	env.Console("minecraft:tp Alice 5.5 64 5.5 0 0")
	env.Drain(a)
	env.Run(a, "jump")
	if pos := env.Player("Alice").Pos(); pos.Z < 100 || pos.Y != 64 {
		t.Fatalf("after /jump Alice is at %v", pos)
	}
	env.Expect(env.Run(a, "compass"), "Bearing: S (180 degrees).")
	// A flat world's sea level is -63, as on Bukkit.
	env.Expect(env.Run(a, "depth"), "You are 127 block(s) above sea level.")
}

// noTimerBypass takes the warmup bypass from operators, so that
// teleport-delay applies to them.
type noTimerBypass struct{}

func (noTimerBypass) Has(_ plugin.CommandSender, node string) (bool, bool) {
	return false, node == "essentials.teleport.timer.bypass"
}

// TestSuccessMessages checks that a teleport's success message comes once
// the teleport happened: not for one a listener cancelled, and after the
// warmup.
func TestSuccessMessages(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	moveTo(env, a, "Alice", 10.5, 10.5)
	env.Expect(env.Run(a, "sethome"), "Home set")
	env.Expect(env.Run(a, "setwarp market"), "Warp market set.")
	moveTo(env, a, "Alice", 0.5, 0.5)

	cancel, cause := true, plugin.TeleportPlugin
	plugin.Listen(env.Ess.Server, func(ev *plugin.EntityTeleportEvent) {
		cause = ev.Cause
		if cancel {
			ev.SetCancelled(true)
		}
	})
	if msgs := env.Run(a, "warp market"); esstest.Has(msgs, "Warping to") {
		t.Fatalf("a cancelled warp says %q", msgs)
	}
	if cause != plugin.TeleportCommand {
		t.Fatalf("/warp teleports with cause %v, want TeleportCommand", cause)
	}
	expectNear(env, "Alice", 0.5, 0.5)
	cancel = false

	plugin.Provide[plugin.PermissionProvider](env.Ess.Server, noTimerBypass{}, 0)
	env.Ess.Config.Set("teleport-delay", 1)
	msgs := env.Run(a, "home")
	env.Expect(msgs, "Teleportation will commence in")
	if esstest.Has(msgs, "Teleporting to home") {
		t.Fatalf("/home says it teleported before the warmup: %q", msgs)
	}
	expectNear(env, "Alice", 0.5, 0.5)
	msgs = env.Messages(a, 40)
	env.Expect(msgs, "Teleporting to home.")
	expectNear(env, "Alice", 10.5, 10.5)
}

// TestJumpLineOfSight checks that /jump's line of sight passes short grass
// and torches and stops at lava (Bukkit's getTargetBlock with Essentials'
// transparent set), and that /jump lock's flying left click jumps there too.
func TestJumpLineOfSight(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	// Looking south at eye height (y 65): grass and a torch on the way,
	// then lava on a stone at z 20.
	env.Console("setblock 5 65 8 short_grass")
	env.Console("setblock 5 65 12 torch")
	env.Console("setblock 5 64 20 stone")
	env.Console("setblock 5 65 20 lava")
	env.Console("minecraft:tp Alice 5.5 64 5.5 0 0")
	env.Drain(a)
	env.Run(a, "jump")
	if pos := env.Player("Alice").Pos(); pos.Z < 15 || pos.Z > 25 {
		t.Fatalf("after /jump Alice is at %v, want near the lava at z 20", pos)
	}

	// /jump lock: a flying left click into the air jumps to the target,
	// keeping the height.
	env.Expect(env.Run(a, "jump lock"), "Flying wizard mode enabled.")
	alice := env.Player("Alice")
	env.Console("minecraft:tp Alice 5.5 70 5.5 0 0")
	env.Drain(a)
	alice.SetAllowFlight(true)
	alice.SetFlying(true)
	env.Ess.Server.Events().Fire(&plugin.PlayerInteractEvent{Player: alice, Action: plugin.LeftClickAir, Hand: plugin.MainHand})
	env.Drain(a)
	if pos := alice.Pos(); pos.Z < 100 {
		t.Fatalf("after the fly click Alice is at %v, want 120 blocks on", pos)
	}
}
