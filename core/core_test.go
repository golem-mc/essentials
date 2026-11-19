package core_test

import (
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
