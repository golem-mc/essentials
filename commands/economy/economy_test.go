package economy_test

import (
	"testing"

	"github.com/golem-mc/essentials/commands/economy"
	"github.com/golem-mc/essentials/internal/esstest"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/shopspring/decimal"
)

func init() { esstest.RegisterCore(economy.Register) }

func expectMoney(t *testing.T, env *esstest.Env, name string, want int64) {
	t.Helper()
	if got := env.User(name).Money(); !got.Equal(decimal.NewFromInt(want)) {
		t.Fatalf("%s's balance = %s, want %d", name, got, want)
	}
}

// TestBalanceAndEco checks /eco's subcommands and /balance.
func TestBalanceAndEco(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Expect(env.Console("eco give Alice 100"), "$100 added to Alice account")
	env.Expect(env.Run(a, "balance"), "Balance: $100")
	env.Expect(env.Console("balance Alice"), "Balance of Alice: $100")
	env.Expect(env.Console("eco take Alice 30"), "$30 taken from Alice account")
	expectMoney(t, env, "Alice", 70)
	env.Expect(env.Console("eco take Alice 20000"), "The minimum balance a user can have is -$10,000")
	env.Expect(env.Console("eco set Alice 1k"), "You set Alice's balance to $1,000")
	env.Expect(env.Console("eco give Alice 10%"), "$100 added to Alice account")
	expectMoney(t, env, "Alice", 1100)
	env.Expect(env.Console("eco reset Alice"), "You set Alice's balance to $0")
	env.Expect(env.Console("eco foo Alice 1"), "/eco")
	env.Expect(env.Console("eco give Nobody 1"), "Player not found")
}

// TestPay checks payments, /paytoggle and payment confirmation.
func TestPay(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	b := env.Join("Bob", true)
	env.Console("eco set Alice 100")
	env.Drain(a)
	env.Drain(b)

	env.Expect(env.Run(a, "pay Bob 25"), "$25 has been sent to Bob")
	env.Expect(env.Messages(b, 3), "$25 has been received from Alice")
	expectMoney(t, env, "Alice", 75)
	expectMoney(t, env, "Bob", 25)
	env.Expect(env.Run(a, "pay Bob -5"), "Amount to pay must be positive")
	env.Expect(env.Run(a, "pay Bob 0.0001"), "The minimum amount you can pay is $0.001")
	// An operator has essentials.eco.loan: down to min-money (-10,000).
	env.Expect(env.Run(a, "pay Bob 20000"), "You do not have sufficient funds")

	env.Expect(env.Console("paytoggle Bob off"), "Disabled accepting payments for Bob")
	env.Expect(env.Run(a, "pay Bob 5"), "Bob is not accepting payment")
	env.Expect(env.Run(b, "payon"), "You are now accepting payments")

	env.Expect(env.Run(a, "payconfirmon"), "You will now be prompted to confirm payments")
	env.Expect(env.Run(a, "pay Bob 5"), "To CONFIRM payment of $5, please repeat command: /pay Bob 5")
	expectMoney(t, env, "Bob", 25)
	env.Expect(env.Run(a, "pay Bob 5"), "$5 has been sent to Bob")
	expectMoney(t, env, "Bob", 30)

	msgs := env.Run(a, "balancetop")
	env.Expect(msgs, "Top balances")
	env.Expect(msgs, "1. Alice, $70")
	env.Expect(msgs, "2. Bob, $30")
}

// TestWorthAndSell checks /setworth, /worth and /sell.
func TestWorthAndSell(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Expect(env.Console("setworth diamond 10"), "Worth value set")
	env.Expect(env.Console("worth diamond 3"), "Stack of diamond worth $30 (3 item(s) at $10 each)")
	env.Console("give Alice minecraft:diamond 5")
	env.Drain(a)

	env.Expect(env.Run(a, "worth diamond"), "Stack of diamond worth $50 (5 item(s) at $10 each)")
	// An operator has every essentials.sell.multiplier.*: triple is the highest.
	env.Expect(env.Run(a, "sell diamond 2"), "Sold for $60 (2 diamond at $30 each)")
	expectMoney(t, env, "Alice", 60)
	env.Expect(env.Run(a, "sell diamond 9"), "You do not have enough of that item to sell")
	env.Expect(env.Run(a, "sell diamond"), "Sold for $90 (3 diamond at $30 each)")
	expectMoney(t, env, "Alice", 150)
	diamond, _ := registry.ItemByName("minecraft:diamond")
	if n := env.Player("Alice").Inventory().Count(diamond); n != 0 {
		t.Fatalf("Alice still has %d diamonds", n)
	}
	env.Expect(env.Console("worth barrier"), "That item cannot be sold to the server")
}

// TestSellOffHandAndArmor counts and takes the off hand (slot 40 of the
// player's inventory) and leaves the worn armour out, as Bukkit's
// PlayerInventory numbering does for Essentials' Worth.getAmount.
func TestSellOffHandAndArmor(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Console("setworth diamond 10")
	env.Console("setworth iron_helmet 5")
	inv := env.Player("Alice").Inventory()
	inv.SetItem(0, plugin.NewStack(registry.ItemDiamond, 1))
	inv.Set(plugin.SlotOffHand, plugin.NewStack(registry.ItemDiamond, 2))
	inv.Set(plugin.SlotHead, plugin.NewStack(registry.ItemIronHelmet, 1))
	if s := inv.Item(40); s.Item != registry.ItemDiamond || s.Count != 2 {
		t.Fatalf("slot 40 = %+v, want the off hand's diamonds", s)
	}

	env.Expect(env.Run(a, "sell iron_helmet"), "You do not have enough of that item to sell")
	env.Expect(env.Run(a, "sell diamond"), "Sold for $90 (3 diamond at $30 each)")
	if !inv.Item(0).Empty() || !inv.Get(plugin.SlotOffHand).Empty() {
		t.Fatalf("left after the sale: slot 0 %+v, off hand %+v", inv.Item(0), inv.Get(plugin.SlotOffHand))
	}
	if inv.Get(plugin.SlotHead).Item != registry.ItemIronHelmet {
		t.Fatal("the worn helmet was taken")
	}
}

// TestBaltopPlaytime leaves out of /balancetop the users, online or
// offline, who played less than baltop-requirements.minimum-playtime.
func TestBaltopPlaytime(t *testing.T) {
	env := esstest.Start(t)
	env.Ess.Config.Set("baltop-requirements.minimum-playtime", 100) // seconds
	st, ok := env.Ess.Server.Registries().Statistic(plugin.MinecraftKey("play_time"))
	if !ok {
		t.Fatal("no play_time statistic")
	}
	a := env.Join("Alice", true)
	env.Player("Alice").SetStatistic(st, 4000) // 200s
	for name, ticks := range map[string]int{"Bob": 1000, "Carol": 4000} {
		c := env.Join(name, false)
		env.Player(name).SetStatistic(st, ticks)
		c.Disconnect()
	}
	env.Console("eco give Bob 50")
	env.Console("eco give Carol 60")
	msgs := env.Run(a, "balancetop force")
	env.Expect(msgs, "Carol, $60")
	env.Expect(msgs, "Alice")
	if esstest.Has(msgs, "Bob, ") {
		t.Fatalf("Bob (50s played, offline) is listed: %q", msgs)
	}
}
