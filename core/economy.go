package core

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/shopspring/decimal"
)

// EconomyService is the economy other plugins find with plugin.Service, the
// role Vault's Economy plays on Bukkit. Amounts are decimals.
type EconomyService interface {
	Balance(id plugin.UUID) (decimal.Decimal, bool)
	Has(id plugin.UUID, amount decimal.Decimal) bool
	Deposit(id plugin.UUID, amount decimal.Decimal) error
	Withdraw(id plugin.UUID, amount decimal.Decimal) error
	Format(amount decimal.Decimal) string
}

// Economy keeps balances in the users' data. Essentials: Economy API
type Economy struct{ ess *Essentials }

func newEconomy(e *Essentials) *Economy {
	eco := &Economy{ess: e}
	plugin.Provide[EconomyService](e.Server, eco, 0)
	return eco
}

// Format writes amount with the currency symbol, two decimals when it has a
// fraction, and thousands separators. Essentials: NumberUtil.displayCurrency
func (eco *Economy) Format(amount decimal.Decimal) string {
	s := FormatAmount(amount)
	sym := eco.ess.Settings.CurrencySymbol()
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	if eco.ess.Settings.CurrencySymbolSuffix() {
		s += sym
	} else {
		s = sym + s
	}
	if neg {
		s = "-" + s
	}
	return s
}

// FormatAmount writes amount with grouping and up to two decimals, dropping
// a zero fraction. Essentials: NumberUtil.formatAsCurrency
func FormatAmount(amount decimal.Decimal) string {
	s := amount.Round(2).StringFixed(2)
	s = strings.TrimSuffix(s, ".00")
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	for i, c := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	out := b.String()
	if frac != "" {
		out += "." + frac
	}
	if neg {
		out = "-" + out
	}
	return out
}

// ParseMoney reads an amount a player typed: digits with an optional
// fraction, the currency symbol and separators stripped, and k/m/b
// suffixes. Essentials: NumberUtil / Commandpay's parsing
func (eco *Economy) ParseMoney(s string) (decimal.Decimal, error) {
	s = strings.ReplaceAll(s, eco.ess.Settings.CurrencySymbol(), "")
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	mult := decimal.NewFromInt(1)
	if s != "" {
		switch strings.ToLower(s[len(s)-1:]) {
		case "k":
			mult, s = decimal.NewFromInt(1_000), s[:len(s)-1]
		case "m":
			mult, s = decimal.NewFromInt(1_000_000), s[:len(s)-1]
		case "b":
			mult, s = decimal.NewFromInt(1_000_000_000), s[:len(s)-1]
		}
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, Tlf("invalidAmount")
	}
	return d.Mul(mult), nil
}

func (eco *Economy) Balance(id plugin.UUID) (decimal.Decimal, bool) {
	if !eco.ess.Users.Known(id) {
		return decimal.Zero, false
	}
	return eco.ess.Users.Get(id).Money(), true
}

func (eco *Economy) Has(id plugin.UUID, amount decimal.Decimal) bool {
	return eco.ess.Users.Get(id).CanAfford(amount)
}

func (eco *Economy) Deposit(id plugin.UUID, amount decimal.Decimal) error {
	u := eco.ess.Users.Get(id)
	return u.SetMoney(u.Money().Add(amount))
}

func (eco *Economy) Withdraw(id plugin.UUID, amount decimal.Decimal) error {
	u := eco.ess.Users.Get(id)
	if !u.CanAfford(amount) {
		return Tlf("notEnoughMoney")
	}
	return u.SetMoney(u.Money().Sub(amount))
}

// BalanceEntry is a line of /balancetop.
type BalanceEntry struct {
	ID      plugin.UUID
	Name    string
	Balance decimal.Decimal
}

// Top answers every user's balance, highest first, leaving out users
// exempt from the ranking. Essentials: BalanceTopImpl
func (eco *Economy) Top() []BalanceEntry {
	var out []BalanceEntry
	showZero := eco.ess.Config.Bool("show-zero-baltop", true)
	for _, id := range eco.ess.Users.All() {
		u := eco.ess.Users.Get(id)
		if u.Data.BaltopExempt || (u.Data.NPC && !eco.ess.Config.Bool("npcs-in-balance-ranking", false)) {
			continue
		}
		// An offline user's exclusion is the saved baltop-exempt above.
		if u.Online() && u.HasPermission("essentials.balancetop.exclude") {
			continue
		}
		m := u.Money()
		if !showZero && m.Sign() == 0 {
			continue
		}
		out = append(out, BalanceEntry{ID: id, Name: u.Name(), Balance: m})
	}
	slices.SortFunc(out, func(a, b BalanceEntry) int { return b.Balance.Cmp(a.Balance) })
	return out
}

// Log appends a transaction to trade.csv when economy-log-enabled is on.
// Essentials: Trade.log
func (eco *Economy) Log(kind, subtype, event string, sender string, amount decimal.Decimal, receiver string, loc *plugin.Location) {
	if !eco.ess.Settings.EcoLogEnabled() {
		return
	}
	pos := ",,,"
	if loc != nil && loc.World != nil {
		pos = fmt.Sprintf("%s,%d,%d,%d", loc.World.Key(), int(loc.Pos.X), int(loc.Pos.Y), int(loc.Pos.Z))
	}
	line := fmt.Sprintf("%q,%q,%q,%q,%s,%q,%s,%q\n", kind, subtype, event, sender, amount.String(), receiver, pos, time.Now().Format(time.RFC3339))
	path := eco.ess.DataPath("trade.csv")
	eco.ess.Server.Scheduler().RunAsync(func() {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		defer f.Close()
		_, _ = f.WriteString(line)
	})
}
