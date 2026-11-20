// Package esstest starts a golem test server (plugintest) with
// Essentials (and any module) enabled, joins fake players and reads what they
// are sent, for the command groups' black-box tests.
package esstest

import (
	"strings"
	"testing"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
)

// Env is a running test server with Essentials.
type Env struct {
	T   *testing.T
	Srv *plugintest.Server
	Ess *core.Essentials
}

// Client is a joined test player.
type Client struct {
	*plugintest.Player
	env *Env
}

// Start starts a flat test server in a temporary directory with the plugins
// registered by the imported packages' init functions (the essentials
// package and any module), and answers it once they are enabled. opts are
// plugintest's, such as WithUsercache.
func Start(t *testing.T, opts ...plugintest.Option) *Env {
	t.Helper()
	t.Chdir(t.TempDir())
	srv := plugintest.Start(t, opts...)
	e, ok := core.Get()
	if !ok {
		t.Fatal("essentials did not enable")
	}
	return &Env{T: t, Srv: srv, Ess: e}
}

// Join joins a player; op makes them an operator of level 4.
func (env *Env) Join(name string, op bool) *Client {
	env.T.Helper()
	c := &Client{Player: env.Srv.Join(name), env: env}
	if op {
		c.Op()
	}
	env.Drain(c)
	return c
}

// ChatBestEffort says message in chat.
func (c *Client) ChatBestEffort(message string) { c.Run(message) }

// Sync runs fn on the tick goroutine, as a plugin's own code runs, and
// returns once it has run. Use it for a state change a tick could race with:
// golem compares some state (the food bar, for one) before and after a tick,
// and takes a change made from another goroutine in between for the tick's.
func (env *Env) Sync(fn func()) {
	env.T.Helper()
	done := make(chan struct{})
	env.Ess.Server.Scheduler().RunTask(func() { fn(); close(done) })
	for range 100 {
		select {
		case <-done:
			return
		default:
			env.Srv.RunTicks(1)
		}
	}
	env.T.Fatal("Sync: the task did not run within 100 ticks")
}

// Player answers the plugin handle of the online player name.
func (env *Env) Player(name string) plugin.Player {
	env.T.Helper()
	p, ok := env.Ess.Server.PlayerNamed(name)
	if !ok {
		env.T.Fatalf("%s is not online", name)
	}
	return p
}

// User answers the Essentials user of the online player name.
func (env *Env) User(name string) *core.User { return env.Ess.Users.Of(env.Player(name)) }

// Messages answers the plain text of the system messages c receives within
// ticks ticks.
func (env *Env) Messages(c *Client, ticks int) []string {
	env.T.Helper()
	return c.Messages(ticks)
}

// Drain reads and drops what c has been sent over a few ticks.
func (env *Env) Drain(c *Client) { env.Messages(c, 3) }

// Run runs a command line (without its slash) as the player c, answering
// the system messages they receive within 5 ticks.
func (env *Env) Run(c *Client, line string) []string {
	env.T.Helper()
	c.Run("/" + line)
	return env.Messages(c, 5)
}

// Console runs a command line at the console and answers its output.
func (env *Env) Console(line string) []string {
	env.T.Helper()
	return env.Srv.Console(line)
}

// Has reports whether any message contains sub.
func Has(msgs []string, sub string) bool {
	for _, m := range msgs {
		if strings.Contains(m, sub) {
			return true
		}
	}
	return false
}

// Expect fails the test unless a message contains sub.
func (env *Env) Expect(msgs []string, sub string) {
	env.T.Helper()
	if !Has(msgs, sub) {
		env.T.Fatalf("no message contains %q; got %q", sub, msgs)
	}
}

// corePlugin is an "essentials" plugin that enables core and only the given
// command groups, so a group's tests do not depend on the other groups.
type corePlugin struct {
	e      *core.Essentials
	groups []func(*core.Essentials)
}

func (p *corePlugin) Enable(s plugin.Server) error {
	e, err := core.Enable(s)
	if err != nil {
		return err
	}
	p.e = e
	for _, g := range p.groups {
		g(e)
	}
	return nil
}

func (p *corePlugin) Disable() {
	if p.e != nil {
		p.e.Disable()
	}
}

// RegisterCore registers the "essentials" plugin with core and the given
// command groups; call it from the test package's init, in place of
// importing the essentials package:
//
//	func init() { esstest.RegisterCore(teleport.Register) }
func RegisterCore(groups ...func(*core.Essentials)) {
	plugin.Register("essentials", &corePlugin{groups: groups})
}

// Chat says message in chat.
func (c *Client) Chat(message string) { c.Run(message) }
