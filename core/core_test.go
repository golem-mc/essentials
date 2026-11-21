package core_test

import (
	"slices"

	"github.com/golem-mc/golem/plugin"
	"testing"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/essentials/internal/esstest"
)

func init() { esstest.RegisterCore() }

// TestCoreFramework checks the shared pieces the command groups stand on: a
// command runs with its messages, usage shows for missing arguments, and the
// user's data is recorded at join.
func TestCoreFramework(t *testing.T) {
	env := esstest.Start(t)
	env.Ess.RegisterCommand(core.Cmd{Name: "ping", Run: func(ctx *core.Ctx) error {
		if len(ctx.Args) > 0 && ctx.Args[0] == "usage" {
			return core.ErrNotEnoughArgs
		}
		ctx.Send("pong", "x")
		return nil
	}})
	c := env.Join("Alice", true)
	if u := env.User("Alice"); u.Data.LastAccountName != "Alice" {
		t.Fatalf("last account name = %q", u.Data.LastAccountName)
	}
	env.Expect(env.Run(c, "ping"), "Pong!")
	// ping has no numbered usages, so the description and the plain usage show.
	env.Expect(env.Run(c, "essentials:ping usage"), "/ping")
	env.Expect(env.Console("ping"), "Pong!")
}

// TestRegisterKeepsFreeNamesWhenOneIsTaken checks that a command whose alias
// another command holds still registers its other names.
func TestRegisterKeepsFreeNamesWhenOneIsTaken(t *testing.T) {
	env := esstest.Start(t)
	srv := env.Ess.Server
	err := srv.Commands().Register(plugin.Command{Name: "tps", Override: true,
		Run: func(*plugin.CommandContext) (int, error) { return 1, nil }})
	if err != nil {
		t.Fatal(err)
	}
	yml := []byte("commands:\n  gc:\n    aliases: [lag, mem, tps]\n")
	if err := env.Ess.RegisterModuleCommands(srv, yml, core.Cmd{Name: "gc", Run: func(ctx *core.Ctx) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	got, ok := env.Ess.CommandNames("gc")
	if !ok || !slices.Equal(got, []string{"gc", "lag", "mem"}) {
		t.Fatalf("names = %v, %v", got, ok)
	}
	for _, n := range []string{"gc", "lag", "mem"} {
		if !slices.ContainsFunc(srv.Commands().All(), func(c plugin.CommandInfo) bool { return c.Name == n || slices.Contains(c.Aliases, n) }) {
			t.Errorf("%s not registered", n)
		}
	}
}
