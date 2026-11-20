package admin_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/golem-mc/essentials/commands/admin"
	"github.com/golem-mc/essentials/internal/esstest"
	"github.com/golem-mc/golem/plugin"
)

func init() {
	esstest.RegisterCore(admin.Register)
	plugin.Register("otherplugin", &otherPlugin{})
}

// otherPlugin holds /warp, which Essentials' plugin.yml also lists but the
// admin group does not register, so Essentials relays it.
type otherPlugin struct{}

func (*otherPlugin) Enable(s plugin.Server) error {
	return s.Commands().Register(plugin.Command{Name: "warp",
		Run: func(*plugin.CommandContext) (int, error) { return 1, nil }})
}

func (*otherPlugin) Disable() {}

// playTime answers the play_time statistic.
func playTime(env *esstest.Env) plugin.Statistic {
	st, ok := env.Ess.Server.Registries().Statistic(plugin.MinecraftKey("play_time"))
	if !ok {
		env.T.Fatal("no play_time statistic")
	}
	return st
}

// gone waits until name has left the server.
func gone(env *esstest.Env, name string) bool {
	for range 40 {
		if _, ok := env.Ess.Server.PlayerNamed(name); !ok {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// TestBanUnban bans a player, who is kicked and listed, then unbans them
// while offline.
func TestBanUnban(t *testing.T) {
	env := esstest.Start(t)
	alice := env.Join("Alice", true)
	env.Join("Bob", false)
	id := env.Player("Bob").UUID().String()

	env.Expect(env.Run(alice, "eban Bob griefing &cbad"), "banned Bob for: griefing bad")
	b, ok := env.Ess.Server.Bans().Entry(id)
	if !ok || !strings.HasPrefix(b.Reason, "griefing") || !b.Expires.IsZero() {
		t.Fatalf("ban entry = %+v, %v", b, ok)
	}
	if !gone(env, "Bob") {
		t.Fatal("Bob was not kicked")
	}

	env.Expect(env.Run(alice, "eunban Bob"), "unbanned Bob")
	if _, ok := env.Ess.Server.Bans().Entry(id); ok {
		t.Fatal("Bob is still banned")
	}
	env.Expect(env.Run(alice, "eunban Nobody"), "was never on this server")
}

// TestTempban bans for a time, with the rest of the line as the reason.
func TestTempban(t *testing.T) {
	env := esstest.Start(t)
	alice := env.Join("Alice", true)
	env.Join("Bob", false)
	id := env.Player("Bob").UUID().String()

	env.Expect(env.Run(alice, "tempban Bob 1h being rude"), "temporarily banned Bob")
	b, ok := env.Ess.Server.Bans().Entry(id)
	if !ok || b.Reason != "being rude" {
		t.Fatalf("ban entry = %+v, %v", b, ok)
	}
	if d := time.Until(b.Expires); math.Abs(d.Hours()-1) > 0.01 {
		t.Fatalf("expires in %v, want 1h", d)
	}
	if !gone(env, "Bob") {
		t.Fatal("Bob was not kicked")
	}
	env.Expect(env.Run(alice, "tempban Bob nonsense"), "Illegal date format")
}

// TestKick kicks a player and reports it to those with kick.notify.
func TestKick(t *testing.T) {
	env := esstest.Start(t)
	alice := env.Join("Alice", true)
	env.Join("Bob", false)
	env.Expect(env.Run(alice, "ekick Bob go away"), "kicked Bob for go away")
	if !gone(env, "Bob") {
		t.Fatal("Bob was not kicked")
	}
	env.Expect(env.Run(alice, "ekick Nobody"), "Player not found")
	// Every name of an overriding command runs Essentials' with arguments.
	for _, line := range []string{"kick Nobody x", "essentials:kick Nobody x", "essentials:ekick Nobody x"} {
		env.Expect(env.Run(alice, line), "Player not found")
	}
	env.Expect(env.Run(alice, "minecraft:kick Nobody x"), "No player was found")
}

// TestJail sets a jail, jails a player there, keeps them there and frees
// them.
func TestJail(t *testing.T) {
	env := esstest.Start(t)
	alice := env.Join("Alice", true)
	bob := env.Join("Bob", false)

	env.Console("tp Alice 20 -60 20")
	env.Drain(alice)
	env.Expect(env.Run(alice, "setjail cell"), "Jail cell has been set")
	env.Expect(env.Run(alice, "jails"), "cell")

	env.Console("tp Bob 0 -60 0")
	env.Drain(bob)
	env.Expect(env.Run(alice, "togglejail Bob cell 1h"), "Bob jailed for")
	env.Expect(env.Messages(bob, 3), "You have been jailed!")
	u := env.User("Bob")
	if !u.Data.Jailed || u.Data.Jail != "cell" || u.Data.Timestamps.Jail == 0 {
		t.Fatalf("jailed=%v jail=%q until=%d", u.Data.Jailed, u.Data.Jail, u.Data.Timestamps.Jail)
	}
	near := func() bool {
		pos := env.Player("Bob").Location().Pos
		return math.Abs(pos.X-20.5) < 1.5 && math.Abs(pos.Z-20.5) < 1.5
	}
	if !near() {
		t.Fatalf("Bob is at %v, not in the jail", env.Player("Bob").Location().Pos)
	}
	env.Console("tp Bob 50 -60 50")
	env.Drain(bob)
	if !near() {
		t.Fatalf("Bob left the jail for %v", env.Player("Bob").Location().Pos)
	}
	env.Expect(env.Run(alice, "jailedplayers"), "Bob")
	env.Expect(env.Run(alice, "togglejail Bob other"), "already in jail")

	env.Expect(env.Run(alice, "togglejail Bob"), "Player Bob unjailed")
	if u.Data.Jailed {
		t.Fatal("Bob is still jailed")
	}
	env.Expect(env.Run(alice, "deljail cell"), "has been removed")
	env.Expect(env.Run(alice, "jails"), "No jails defined")
}

// TestEssentialsReload reloads the configuration.
func TestEssentialsReload(t *testing.T) {
	env := esstest.Start(t)
	env.Expect(env.Console("essentials reload"), "Essentials reloaded")
	env.Expect(env.Console("essentials debug on"), "debug mode enabled")
	env.Expect(env.Console("essentials"), "/essentials")
}

// TestMisc runs /gc, /backup and the /essentials easter egg.
func TestMisc(t *testing.T) {
	env := esstest.Start(t)
	alice := env.Join("Alice", true)
	msgs := env.Run(alice, "gc")
	env.Expect(msgs, "Current TPS")
	env.Expect(msgs, `World "world"`)
	env.Expect(env.Run(alice, "essentials moo"), "Have you mooed today?")
	env.Expect(env.Run(alice, "backup"), "An external backup script has not been configured.")
}

// TestExt extinguishes the sender and another player.
func TestExt(t *testing.T) {
	env := esstest.Start(t)
	alice := env.Join("Alice", true)
	env.Expect(env.Run(alice, "ext"), "You extinguished yourself.")
	env.Expect(env.Run(alice, "ext Alice"), "You extinguished Alice")
}

// TestBanCached bans and unbans by name a player Essentials has no name
// for but the server's usercache.json has, without the userUnknown warning.
func TestBanCached(t *testing.T) {
	env := esstest.Start(t)
	bob := env.Join("Bob", false)
	id := env.Player("Bob").UUID()
	bob.Disconnect()
	env.Ess.Users.Rename(id, "Robert") // Essentials no longer knows "Bob"

	if msgs := env.Console("ban Bob cheating"); esstest.Has(msgs, "never joined") {
		t.Fatalf("userUnknown for a cached player: %q", msgs)
	}
	if _, ok := env.Ess.Server.Bans().Entry(id.String()); !ok {
		t.Fatal("Bob's UUID is not banned")
	}
	env.Console("unban Bob")
	if _, ok := env.Ess.Server.Bans().Entry(id.String()); ok {
		t.Fatal("Bob is still banned")
	}
	env.Console("tempban Bob 1h rude")
	if _, ok := env.Ess.Server.Bans().Entry(id.String()); !ok {
		t.Fatal("Bob is not tempbanned")
	}
	// A name nobody has used is banned by name, as Bukkit's NAME list.
	env.Expect(env.Console("ban Nobody"), "never joined")
	if n := len(env.Ess.Server.Bans().Entries()); n != 2 {
		t.Fatalf("%d bans, want Bob's and Nobody's", n)
	}
	if ban, ok := env.Ess.Server.Bans().Entry("nobody"); !ok || ban.Target != "" || ban.Name != "Nobody" {
		t.Fatalf("Nobody's ban = %+v, %v; want a ban by name", ban, ok)
	}
	env.Console("unban nobody")
	if _, ok := env.Ess.Server.Bans().Entry("Nobody"); ok {
		t.Fatal("Nobody is still banned")
	}
}

// TestBanIPKicksAddress kicks every player connected from the banned
// address, with Essentials' message.
func TestBanIPKicksAddress(t *testing.T) {
	env := esstest.Start(t)
	clients := []*esstest.Client{env.Join("Alice", false), env.Join("Bob", false)}
	env.Console("banip Bob spam")
	for _, c := range clients {
		c.Messages(5) // reads up to the disconnect
		reason, ok := c.Kicked()
		if !ok || !strings.Contains(reason, "You have been banned") || !strings.Contains(reason, "spam") {
			t.Fatalf("%s kicked with %q (%v), want Essentials' banFormat", c.Name(), reason, ok)
		}
	}
	if !gone(env, "Alice") || !gone(env, "Bob") {
		t.Fatal("the players from the address were not all kicked")
	}
	if len(env.Ess.Server.IPBans().Entries()) != 1 {
		t.Fatalf("ip bans = %+v", env.Ess.Server.IPBans().Entries())
	}
}

// TestCommandMap lists the commands Essentials leaves to another plugin.
func TestCommandMap(t *testing.T) {
	env := esstest.Start(t)
	msgs := env.Console("essentials commands")
	env.Expect(msgs, "relaying the following commands")
	env.Expect(msgs, "warp => otherplugin:warp")
	if esstest.Has(msgs, "ban =>") {
		t.Fatalf("Essentials' own command listed: %q", msgs)
	}
}

// TestJailOnlineTime jails with jail-online-time as Essentials stores it:
// timestamps.jail the sentence and onlinejail the play time at which it
// is served, online and offline; core releases on the play time.
func TestJailOnlineTime(t *testing.T) {
	env := esstest.Start(t)
	env.Ess.Config.Set("jail-online-time", true)
	alice := env.Join("Alice", true)
	bob := env.Join("Bob", false)
	st := playTime(env)
	env.Expect(env.Run(alice, "setjail cell"), "Jail cell has been set")

	env.Expect(env.Run(alice, "togglejail Bob cell 1h"), "Bob jailed for")
	u := env.User("Bob")
	played := int64(env.Player("Bob").Statistic(st))
	if u.Data.Timestamps.Jail != 3_600_000 {
		t.Fatalf("timestamps.jail = %d, want the sentence 3600000", u.Data.Timestamps.Jail)
	}
	if d := u.Data.Timestamps.OnlineJail - played - 72_000; d < -40 || d > 0 {
		t.Fatalf("onlinejail = %d, play time %d", u.Data.Timestamps.OnlineJail, played)
	}
	// Served: core's timer releases.
	env.Player("Bob").SetStatistic(st, int(u.Data.Timestamps.OnlineJail))
	for range 60 {
		if !u.Data.Jailed {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if u.Data.Jailed {
		t.Fatal("Bob was not released when his online time was served")
	}

	// Offline: the play time comes from the stats file.
	env.Player("Bob").SetStatistic(st, 1000)
	bob.Disconnect()
	env.Expect(env.Run(alice, "togglejail Bob cell 10s"), "Bob jailed for")
	if u.Data.Timestamps.Jail != 10_000 {
		t.Fatalf("timestamps.jail = %d, want 10000", u.Data.Timestamps.Jail)
	}
	if got := u.Data.Timestamps.OnlineJail; got < 1200 || got > 1300 {
		t.Fatalf("onlinejail = %d, want the stats file's play time + 200", got)
	}
}

// TestJailOnlineTimeLegacy converts the sentences earlier versions of the
// port saved when the player joins.
func TestJailOnlineTimeLegacy(t *testing.T) {
	env := esstest.Start(t)
	env.Ess.Config.Set("jail-online-time", true)
	alice := env.Join("Alice", true)
	env.Expect(env.Run(alice, "setjail cell"), "Jail cell has been set")
	bob := env.Join("Bob", false)
	u := env.User("Bob")
	bob.Disconnect()

	u.Data.Jailed, u.Data.Jail = true, "cell"
	u.Data.Timestamps.Jail = time.Now().Add(time.Hour).UnixMilli()
	u.Data.Extra = map[string]any{"jail-online-pending-ticks": 400}
	bob = env.Join("Bob", false)
	played := int64(env.Player("Bob").Statistic(playTime(env)))
	if _, ok := u.Data.Extra["jail-online-pending-ticks"]; ok || u.Data.Timestamps.Jail != 20_000 {
		t.Fatalf("pending sentence not converted: jail=%d extra=%v", u.Data.Timestamps.Jail, u.Data.Extra)
	}
	if d := u.Data.Timestamps.OnlineJail - played - 400; d < -40 || d > 0 {
		t.Fatalf("onlinejail = %d, play time %d", u.Data.Timestamps.OnlineJail, played)
	}
	bob.Disconnect()

	// A wall-clock timestamps.jail becomes the time left.
	u.Data.Timestamps.Jail = time.Now().Add(time.Hour).UnixMilli()
	env.Join("Bob", false)
	if j := u.Data.Timestamps.Jail; j <= 0 || j > 20_000 || !u.Data.Jailed {
		t.Fatalf("timestamps.jail = %d (jailed %v), want the time left", j, u.Data.Jailed)
	}
}
