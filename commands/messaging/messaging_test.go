package messaging_test

import (
	"testing"
	"time"

	"github.com/golem-mc/essentials/commands/messaging"
	"github.com/golem-mc/essentials/internal/esstest"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
)

func init() { esstest.RegisterCore(messaging.Register) }

// playerNodes are what the tests' permission provider decides for players
// who are not operators: Essentials leaves its nodes to operators by
// default, and operators hold the exempt nodes too. A node set to false is
// denied explicitly.
var playerNodes = map[string]bool{
	"essentials.msg": true, "essentials.r": true, "essentials.msgtoggle": true, "essentials.afk": true,
	"essentials.ignore": true, "essentials.mail": true, "essentials.mail.send": true, "essentials.me": true,
	"essentials.list": true, "essentials.help": true, "essentials.help.otherplug.freebie": true,
	"essentials.nick": true, "essentials.nick.color": true, "essentials.nick.red": false, "essentials.nick.italic": true,
}

// perms decides playerNodes for players who are not operators.
type perms struct{}

func (perms) Has(s plugin.CommandSender, node string) (bool, bool) {
	has, ok := playerNodes[node]
	if s.OpLevel() > 0 || !ok {
		return false, false
	}
	return has, true
}

// otherPlugin is a plugin besides Essentials, whose commands /help lists.
type otherPlugin struct{}

func (otherPlugin) Enable(s plugin.Server) error {
	run := func(*plugin.CommandContext) (int, error) { return 1, nil }
	if err := s.Commands().Register(plugin.Command{Name: "zap", Aliases: []string{"zapper"}, Description: "Zaps things.", Usage: "/<command> <target>",
		Permission: "otherplug.zap", Run: run}); err != nil {
		return err
	}
	if err := s.Commands().Register(plugin.Command{Name: "freebie", Description: "Free stuff.", Run: run}); err != nil {
		return err
	}
	return s.Commands().Register(plugin.Command{Name: "secret", Description: "Nothing to see.", Run: run})
}

func (otherPlugin) Disable() {}

func init() { plugin.Register("otherplug", otherPlugin{}) }

// start starts a test server with the permission provider.
func start(t *testing.T, opts ...plugintest.Option) *esstest.Env {
	env := esstest.Start(t, opts...)
	plugin.Provide[plugin.PermissionProvider](env.Ess.Server, perms{}, 0)
	return env
}

// TestMsgReply checks /msg and /r between two players, social spy, and
// /msgtoggle.
func TestMsgReply(t *testing.T) {
	env := start(t)
	// Operators are exempt from social spy, so the two talking are not.
	alice := env.Join("Alice", false)
	bob := env.Join("Bob", false)
	spy := env.Join("Spy", true)
	env.Run(spy, "socialspy on")

	env.Expect(env.Run(alice, "msg Bob hello there"), "[me -> Bob] hello there")
	env.Expect(env.Messages(bob, 3), "[Alice -> me] hello there")
	env.Expect(env.Messages(spy, 3), "[Alice -> Bob] hello there")

	env.Expect(env.Run(bob, "r hi back"), "[me -> Alice] hi back")
	env.Expect(env.Messages(alice, 3), "[Bob -> me] hi back")

	env.Expect(env.Run(spy, "r anyone"), "You have nobody to whom you can reply.")

	env.Expect(env.Run(bob, "msgtoggle"), "Receiving messages disabled")
	env.Expect(env.Run(alice, "msg Bob still there?"), "Bob has messages disabled.")
	// Operators hold essentials.msgtoggle.bypass.
	env.Expect(env.Run(spy, "msg Bob still there?"), "[me -> Bob] still there?")

	env.Expect(env.Console("msg Alice from console"), "[me -> Alice] from console")
	env.Expect(env.Messages(alice, 3), "-> me] from console")
}

// TestMailSendRead checks /mail send, read and clear.
func TestMailSendRead(t *testing.T) {
	env := start(t)
	alice := env.Join("Alice", true)
	bob := env.Join("Bob", true)

	env.Expect(env.Run(alice, "mail send Bob see you at spawn"), "Bob has been sent the following mail:")
	if n := len(env.User("Bob").UnreadMail()); n != 1 {
		t.Fatalf("Bob has %d unread mails, want 1", n)
	}
	read := env.Run(bob, "mail read")
	env.Expect(read, "see you at spawn")
	env.Expect(read, "Mail: read")
	if n := len(env.User("Bob").UnreadMail()); n != 0 {
		t.Fatalf("Bob has %d unread mails after reading, want 0", n)
	}
	env.Expect(env.Run(bob, "mail clear"), "Mail cleared!")
	env.Expect(env.Run(bob, "mail read"), "You do not have any mail.")
	env.Expect(env.Console("mail Bob from the console"), "Mail sent!")
	env.Expect(env.Run(bob, "mail read"), "from the console")
}

// TestIgnore checks that an ignored player's messages and mail are dropped.
func TestIgnore(t *testing.T) {
	env := start(t)
	alice := env.Join("Alice", false)
	bob := env.Join("Bob", false)

	env.Expect(env.Run(bob, "ignore Alice"), "You ignore player Alice from now on.")
	env.Expect(env.Run(bob, "ignore"), "Ignored: Alice")
	env.Run(alice, "msg Bob psst")
	if esstest.Has(env.Messages(bob, 3), "psst") {
		t.Fatal("Bob got a message from Alice, whom he ignores")
	}
	env.Expect(env.Run(bob, "ignore Alice"), "You are not ignoring player Alice anymore.")
	env.Run(alice, "msg Bob psst again")
	env.Expect(env.Messages(bob, 3), "psst again")
	env.Expect(env.Run(bob, "ignore Bob"), "Ignoring yourself won't solve your problems.")
}

// TestMute checks that a muted player cannot message, act or chat, and
// that /unmute lifts it.
func TestMute(t *testing.T) {
	env := start(t)
	admin := env.Join("Admin", true)
	bob := env.Join("Bob", false)

	env.Expect(env.Run(admin, "mute Bob 10m spamming"), "Player Bob muted for")
	env.Expect(env.Messages(bob, 3), "You have been muted for")
	if !env.User("Bob").IsMuted() {
		t.Fatal("Bob is not muted")
	}
	env.Expect(env.Run(bob, "msg Admin hi"), "Your voice has been silenced")
	if esstest.Has(env.Messages(admin, 3), "hi") {
		t.Fatal("a muted player's message was delivered")
	}
	env.Expect(env.Run(bob, "me waves"), "Your voice has been silenced")
	bob.ChatBestEffort("hello everyone")
	env.Expect(env.Messages(bob, 5), "Your voice has been silenced")

	env.Expect(env.Run(admin, "unmute Bob"), "Player Bob unmuted.")
	env.Expect(env.Run(admin, "unmute Bob"), "Bob is not muted.")
	env.Expect(env.Console("mute Nobody"), "has never joined this server")
}

// TestNick checks nicknames: setting, the rules, uniqueness and removal.
func TestNick(t *testing.T) {
	env := start(t)
	alice := env.Join("Alice", true)
	env.Join("Bob", false)

	env.Expect(env.Run(alice, "nick Wonder"), "Your nickname is now ~Wonder")
	if n := env.User("Alice").Data.Nickname; n != "Wonder" {
		t.Fatalf("nickname = %q", n)
	}
	env.Expect(env.Run(alice, "realname Wonder"), "is Alice")
	env.Expect(env.Run(alice, "nick bad-nick!"), "Nicknames must be alphanumeric.")
	env.Expect(env.Run(alice, "nick ThisNicknameIsFarTooLong"), "That nickname is too long.")
	env.Expect(env.Run(alice, "nick Alice Bob"), "That name is already in use.")
	env.Expect(env.Run(alice, "nick &cRed"), "Your nickname is now ~Red")
	env.Expect(env.Run(alice, "nick off"), "You no longer have a nickname.")
	if n := env.User("Alice").Data.Nickname; n != "" {
		t.Fatalf("nickname after off = %q", n)
	}
	env.Expect(env.Console("nick Bob Builder"), "Nickname changed.")
	if n := env.User("Bob").Data.Nickname; n != "Builder" {
		t.Fatalf("Bob's nickname = %q", n)
	}
}

// TestAfk checks /afk and the AFK notice a private message gets.
func TestAfk(t *testing.T) {
	env := start(t)
	alice := env.Join("Alice", true)
	bob := env.Join("Bob", false)

	env.Expect(env.Run(bob, "afk"), "You are now AFK.")
	env.Expect(env.Messages(alice, 3), "Bob is now AFK.")
	env.Expect(env.Run(alice, "msg Bob are you there"), "Bob is currently AFK and may not respond.")
	env.Expect(env.Run(alice, "list"), "[AFK]")
	env.Expect(env.Run(bob, "afk"), "You are no longer AFK.")
	if env.User("Bob").IsAfk() {
		t.Fatal("Bob is still AFK")
	}
}

// TestNoAccess checks that a player without a command's permission sees the
// command and is told they may not use it, as on Bukkit, rather than finding
// it unknown, and that golem's own help (/?) still answers them.
func TestNoAccess(t *testing.T) {
	env := esstest.Start(t) // no permission provider: plugin.yml's defaults
	bob := env.Join("Bob", false)
	env.Expect(env.Run(bob, "help"), "You do not have access to that command.")
	env.Expect(env.Run(bob, "essentials:help"), "You do not have access to that command.")
	if msgs := env.Run(bob, "help"); esstest.Has(msgs, "Unknown or incomplete command") {
		t.Fatalf("/help is unknown to a player without essentials.help: %q", msgs)
	}
	env.Expect(env.Run(bob, "?"), "Help: Index")
}

// TestInfoCommands checks /list, /whois, /seen, /ping, /help and the text
// files.
func TestInfoCommands(t *testing.T) {
	env := start(t)
	alice := env.Join("Alice", true)
	env.Join("Bob", false)

	list := env.Run(alice, "list")
	env.Expect(list, "There are 2 out of maximum")
	env.Expect(list, "Bob")
	env.Expect(list, "Connected players: ")
	if esstest.Has(list, "<primary>") {
		t.Errorf("/list shows raw tags: %q", list)
	}
	whois := env.Run(alice, "whois Bob")
	env.Expect(whois, "WhoIs: Bob")
	env.Expect(whois, "Health: 20/20")
	env.Expect(env.Run(alice, "seen Bob"), "has been online since")
	env.Expect(env.Run(alice, "ping"), "Pong!")
	env.Expect(env.Run(alice, "ping echo me"), "echo me")
	env.Expect(env.Run(alice, "rules"), "Be respectful")
	env.Expect(env.Run(alice, "customtext vote"), "This text here will be shown if you type /vote")
	help := env.Run(alice, "help")
	env.Expect(help, "Essentials: Plugin Help: /help essentials")
	env.Expect(env.Run(alice, "help /seen"), "Command Help: /seen")
	env.Expect(env.Console("help"), "To view help from the console")
}

// TestHelp checks that /help lists Essentials' commands under their own
// names and other plugins' commands by their permissions.
func TestHelp(t *testing.T) {
	env := start(t)
	alice := env.Join("Alice", true)
	bob := env.Join("Bob", false)

	// Essentials owns /msg now, so it is listed as /msg, not /emsg.
	msg := env.Run(alice, "help msg")
	env.Expect(msg, "/msg: ")
	if esstest.Has(msg, "/emsg") {
		t.Fatalf("/help lists /emsg: %q", msg)
	}

	// Alice is an operator, so she holds essentials.help.otherplug, which
	// lists every command of the plugin, as on Bukkit.
	help := env.Run(alice, "help")
	env.Expect(help, "otherplug: Plugin Help: /help otherplug")
	other := env.Run(alice, "help otherplug")
	env.Expect(other, "Commands from otherplug:")
	env.Expect(other, "/zap: Zaps things.")
	env.Expect(other, "/secret: Nothing to see.")
	// Bob has neither otherplug.zap nor Essentials' operator nodes;
	// essentials.help.otherplug.freebie lists /freebie, and
	// hide-permissionless-help hides /secret.
	other = env.Run(bob, "help otherplug")
	env.Expect(other, "/freebie: Free stuff.")
	if esstest.Has(other, "/zap") || esstest.Has(other, "/secret") {
		t.Fatalf("Bob sees /zap or /secret: %q", other)
	}
	if esstest.Has(env.Run(bob, "help seen"), "/seen") {
		t.Fatal("Bob sees /seen without essentials.seen")
	}

	zap := env.Run(alice, "help /zapper")
	env.Expect(zap, "Command Help: /zapper")
	env.Expect(zap, "Description: Zaps things.")
	env.Expect(zap, "Aliases(s): [zapper]")
	env.Expect(zap, "/zapper <target>")
	env.Expect(env.Run(alice, "help /seen"), "/seen <playername>")
	env.Expect(env.Run(alice, "help /minecraft:msg"), "A Mojang provided command.")
}

// TestPlaytimeOffline checks /playtime for a player who has left.
func TestPlaytimeOffline(t *testing.T) {
	env := start(t)
	alice := env.Join("Alice", true)
	bob := env.Join("Bob", false)
	env.Messages(bob, 40)
	bob.Disconnect()
	env.Drain(alice)

	u, ok := env.Ess.Users.ByName("Bob")
	if !ok {
		t.Fatal("Bob has no user")
	}
	if u.Online() {
		t.Fatal("Bob is still online")
	}
	if n := u.PlayTime(); n < 40 {
		t.Fatalf("Bob's offline play time = %d ticks, want at least 40", n)
	}
	pt := env.Run(alice, "playtime Bob")
	env.Expect(pt, "Playtime of Bob: ")
	env.Expect(pt, " seconds")
}

// TestUsercacheNames checks /seen and /mute for a name the server knows
// from usercache.json but Essentials has never seen.
func TestUsercacheNames(t *testing.T) {
	ghost := plugin.UUID{0x6a, 0x05, 0x7e, 0x11}
	env := start(t, plugintest.WithUsercache(plugin.Profile{ID: ghost, Name: "Ghost"}))
	alice := env.Join("Alice", true)
	env.Expect(env.Run(alice, "seen Ghost"), "Player Ghost was never on this server.")
	env.Ess.Server.Bans().Ban(ghost.String(), "haunting", time.Time{}, "test")
	env.Expect(env.Run(alice, "seen ghost"), "Banned: haunting")

	env.Expect(env.Run(alice, "mute ghost 5m"), "Player Ghost muted for")
	if !env.Ess.Users.Get(ghost).IsMuted() {
		t.Fatal("the usercache UUID of Ghost is not muted")
	}
}

// TestColorPermissions checks the per-colour nodes: an explicit grant adds
// a code, an explicit denial removes one.
func TestColorPermissions(t *testing.T) {
	env := start(t)
	bob := env.Join("Bob", false)

	env.Expect(env.Run(bob, "nick &aGreen"), "Your nickname is now ~Green")
	if n := env.User("Bob").Data.Nickname; n != "§aGreen" {
		t.Fatalf("nickname = %q, want §aGreen", n)
	}
	// essentials.nick.red is denied, so &c stays text, which nicknames
	// may not hold.
	env.Expect(env.Run(bob, "nick &cRed"), "Nicknames must be alphanumeric.")
	// essentials.nick.italic is granted though .format is not.
	env.Run(bob, "nick &oSlanted")
	if n := env.User("Bob").Data.Nickname; n != "§oSlanted" {
		t.Fatalf("nickname = %q, want §oSlanted", n)
	}
}

// TestSeenNameBan checks /seen for a name banned before it ever joined,
// found by the name the ban list holds.
func TestSeenNameBan(t *testing.T) {
	env := start(t)
	alice := env.Join("Alice", true)
	env.Console("ban Stranger haunting")
	env.Expect(env.Run(alice, "seen stranger"), "Banned: haunting")
}
