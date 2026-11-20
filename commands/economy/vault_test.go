package economy_test

import (
	"crypto/md5"
	"fmt"
	"testing"

	"github.com/golem-mc/essentials/internal/esstest"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/vault"
)

// shop charges through Vault only, knowing nothing of Essentials.
type shop struct{}

func init() { plugin.Register("shop", shop{}) }

func (shop) Enable(s plugin.Server) error {
	buy := func(ctx *plugin.CommandContext) (int, error) {
		p := ctx.Sender.(plugin.Player)
		eco, ok := vault.LookupEconomy(s)
		if !ok {
			p.SendMessage(plugin.Plain("no economy"))
			return 0, nil
		}
		price := float64(plugin.Arg[int32](ctx, "price"))
		if r := eco.Withdraw(p.UUID(), price); !r.Success() {
			p.SendMessage(plugin.Plain(eco.Name() + " refused: " + r.Err.Error()))
			return 0, nil
		}
		p.SendMessage(plugin.Plain(fmt.Sprintf("Paid %s, %s left", eco.Format(price), eco.Format(eco.Balance(p.UUID())))))
		return 1, nil
	}
	return s.Commands().Register(plugin.Command{Name: "buy",
		Children: []plugin.Node{{Arg: plugin.IntArg("price", 0, 1_000_000), Run: buy}}})
}

func (shop) Disable() {}

// TestVaultEconomy checks that a plugin charging through vault.Economy reaches
// Essentials' balances, and the hook's account creation.
func TestVaultEconomy(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", false)
	env.Console("eco set Alice 100")
	env.Drain(a)

	env.Expect(env.Run(a, "buy 30"), "Paid $30, $70 left")
	expectMoney(t, env, "Alice", 70)
	// Without essentials.eco.loan the balance stays at zero or more.
	env.Expect(env.Run(a, "buy 71"), "EssentialsX Economy refused: Loan was not permitted!")
	expectMoney(t, env, "Alice", 70)

	// An NPC account, named by an offline-mode style (version 3) UUID.
	sum := md5.Sum([]byte("NPC:Town"))
	sum[6] = sum[6]&0x0f | 0x30
	sum[8] = sum[8]&0x3f | 0x80
	town := plugin.UUID(sum)
	env.Sync(func() {
		eco, _ := vault.LookupEconomy(env.Ess.Server)
		if eco.HasAccount(town) {
			t.Error("Town has an account before it was created")
		}
		if !eco.CreateAccount(town, "Town") || eco.CreateAccount(town, "Town") {
			t.Error("CreateAccount should make the account once")
		}
		if r := eco.Deposit(town, 12.5); !r.Success() || r.Balance != 12.5 {
			t.Errorf("deposit: %+v", r)
		}
		if r := eco.Withdraw(town, -1); r.Success() {
			t.Error("a negative withdrawal succeeded")
		}
		if u, ok := env.Ess.Users.ByName("Town"); !ok || !u.Data.NPC {
			t.Error("Town is not an NPC user found by name")
		}
		if got := eco.Format(1234.5); got != "$1,234.50" {
			t.Errorf("Format(1234.5) = %q", got)
		}
	})
}
