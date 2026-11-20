package core

import (
	"errors"
	"math"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/vault"
	"github.com/shopspring/decimal"
)

// vaultEconomy is the economy as Vault's Economy service, for plugins that
// charge through Vault. Like the rest of the economy it is called on the
// tick goroutine. Essentials: VaultEconomyProvider
type vaultEconomy struct{ eco *Economy }

var _ vault.Economy = vaultEconomy{}

func (vaultEconomy) Name() string          { return "EssentialsX Economy" }
func (vaultEconomy) FractionalDigits() int { return -1 }

func (v vaultEconomy) Format(amount float64) string {
	return v.eco.Format(decimal.NewFromFloat(amount))
}

// CurrencyNameSingular and CurrencyNamePlural are both the currency symbol.
func (v vaultEconomy) CurrencyNameSingular() string { return v.eco.ess.Settings.CurrencySymbol() }
func (v vaultEconomy) CurrencyNamePlural() string   { return v.CurrencyNameSingular() }

func (v vaultEconomy) HasAccount(id plugin.UUID) bool { return v.eco.ess.Users.Known(id) }

// CreateAccount makes a user with the starting balance. A version 2 or 3
// UUID (an offline-mode name's, or Citizens') is an NPC account, kept out
// of /balancetop unless npcs-in-balance-ranking is on.
func (v vaultEconomy) CreateAccount(id plugin.UUID, name string) bool {
	if v.HasAccount(id) {
		return false
	}
	users := v.eco.ess.Users
	u := users.Get(id)
	if ver := id.Version(); ver == 2 || ver == 3 {
		u.Data.NPC = true
	}
	if name != "" {
		u.Data.LastAccountName = name
		users.Rename(id, name)
	}
	start := v.eco.ess.Settings.StartingBalance()
	u.Data.Money = &start
	// Known answers true from here, before SaveAll writes the file.
	u.Loaded = true
	u.Save()
	return true
}

// Balance answers the balance rounded down to a float64, so that it never
// claims more than the user has; an unknown user gets an account.
func (v vaultEconomy) Balance(id plugin.UUID) float64 {
	if !v.HasAccount(id) {
		v.CreateAccount(id, "")
		return floorFloat(v.eco.ess.Settings.StartingBalance())
	}
	return floorFloat(v.eco.ess.Users.Get(id).Money())
}

// floorFloat answers the largest float64 not above d.
// Essentials: VaultEconomyProvider.getDoubleValue
func floorFloat(d decimal.Decimal) float64 {
	f := d.InexactFloat64()
	if decimal.NewFromFloat(f).GreaterThan(d) {
		f = math.Nextafter(f, math.Inf(-1))
	}
	return f
}

func (v vaultEconomy) Has(id plugin.UUID, amount float64) bool {
	return v.HasAccount(id) && v.eco.ess.Users.Get(id).CanAfford(decimal.NewFromFloat(amount))
}

func (v vaultEconomy) Withdraw(id plugin.UUID, amount float64) vault.Response {
	if amount < 0 {
		return vault.Response{Err: errors.New("Cannot withdraw negative funds!")}
	}
	if !v.HasAccount(id) {
		return vault.Response{Err: errors.New("User does not exist!")}
	}
	u := v.eco.ess.Users.Get(id)
	d := decimal.NewFromFloat(amount)
	if !u.CanAfford(d) {
		return vault.Response{Balance: v.Balance(id), Err: errors.New("Loan was not permitted!")}
	}
	return v.set(id, amount, u.Money().Sub(d))
}

func (v vaultEconomy) Deposit(id plugin.UUID, amount float64) vault.Response {
	if amount < 0 {
		return vault.Response{Err: errors.New("Cannot deposit negative funds")}
	}
	if !v.HasAccount(id) {
		return vault.Response{Err: errors.New("User does not exist!")}
	}
	u := v.eco.ess.Users.Get(id)
	return v.set(id, amount, u.Money().Add(decimal.NewFromFloat(amount)))
}

func (v vaultEconomy) set(id plugin.UUID, amount float64, balance decimal.Decimal) vault.Response {
	if err := v.eco.ess.Users.Get(id).SetMoney(balance); err != nil {
		return vault.Response{Balance: v.Balance(id), Err: errors.New("User goes over maximum money limit!")}
	}
	return vault.Response{Amount: amount, Balance: v.Balance(id)}
}
