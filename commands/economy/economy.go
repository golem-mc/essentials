// Package economy holds EssentialsX's economy commands: balances, payments
// and selling items for their worth.
package economy

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Register registers the group's commands.
func Register(e *core.Essentials) {
	top := &balanceTop{}
	e.RegisterCommand(
		balanceCmd(),
		top.cmd(),
		ecoCmd(),
		payCmd(),
		paytoggleCmd(),
		payconfirmtoggleCmd(),
		sellCmd(),
		setworthCmd(),
		worthCmd(),
	)
}

// confirmingKey is the session key of the payments a user still has to
// confirm, by receiver. Essentials: User.getConfirmingPayments
const confirmingKey = "economy.confirmingPayments"

func confirmingPayments(u *core.User) map[plugin.UUID]decimal.Decimal {
	m, ok := core.SessionValue[map[plugin.UUID]decimal.Decimal](u, confirmingKey)
	if !ok {
		m = map[plugin.UUID]decimal.Decimal{}
		if u.Session == nil {
			u.Session = map[string]any{}
		}
		u.Session[confirmingKey] = m
	}
	return m
}

// promptingPayConfirm reports whether /pay asks the user to repeat it, by
// default when default-enabled-confirm-commands lists pay.
// Essentials: User.isPromptingPayConfirm
func promptingPayConfirm(u *core.User) bool {
	if u.Data.ConfirmPay != nil {
		return *u.Data.ConfirmPay
	}
	return slices.ContainsFunc(u.Ess().Config.StringList("default-enabled-confirm-commands"),
		func(s string) bool { return strings.EqualFold(s, "pay") })
}

// format writes an amount with the currency. Essentials: NumberUtil.displayCurrency
func format(e *core.Essentials, amount decimal.Decimal) string { return e.Economy.Format(amount) }

// formatExact writes an amount as it is, with the currency.
// Essentials: NumberUtil.displayCurrencyExactly
func formatExact(e *core.Essentials, amount decimal.Decimal) string {
	s, sign := amount.String(), ""
	if amount.Sign() < 0 {
		s, sign = s[1:], "-"
	}
	sym := e.Settings.CurrencySymbol()
	if e.Settings.CurrencySymbolSuffix() {
		return sign + e.I18n.Format("", "currency", s, sym)
	}
	return sign + e.I18n.Format("", "currency", sym, s)
}

// displayName is the name a balance message shows: the account name of a
// vanished user, else the display name.
func displayName(u *core.User) string {
	if u.Vanished {
		return u.Name()
	}
	return u.DisplayName()
}

// parseAmount reads an amount as Essentials does: everything but digits,
// dots and commas must be one of the suffixes K, M, B and T; commas group.
// Essentials: NumberUtil.parseStringToBDecimal
func parseAmount(s string) (decimal.Decimal, error) {
	if s == "" {
		return decimal.Zero, errors.New("empty amount")
	}
	sanitized := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == '.' || r == ',' {
			return r
		}
		return -1
	}, s)
	mult := decimal.NewFromInt(1)
	mod := s
	if sanitized != "" {
		mod = strings.ReplaceAll(s, sanitized, "")
	}
	switch strings.ToUpper(mod) {
	case "":
	case "K":
		mult = decimal.NewFromInt(1_000)
	case "M":
		mult = decimal.NewFromInt(1_000_000)
	case "B":
		mult = decimal.NewFromInt(1_000_000_000)
	case "T":
		mult = decimal.NewFromInt(1_000_000_000_000)
	default:
		return decimal.Zero, core.Tlf("invalidModifier")
	}
	// NumberFormat.parse: grouping commas before the point, and the number
	// ends at the first character that cannot continue it.
	var num strings.Builder
	point := false
	for _, r := range sanitized {
		if r == ',' && !point {
			continue
		}
		if r == '.' && !point {
			point = true
			num.WriteRune(r)
			continue
		}
		if r < '0' || r > '9' {
			break
		}
		num.WriteRune(r)
	}
	str := strings.TrimSuffix(num.String(), ".")
	if str == "" || str == "." {
		return decimal.Zero, fmt.Errorf("Unparseable number: %q", sanitized)
	}
	if strings.HasPrefix(str, ".") {
		str = "0" + str
	}
	d, err := decimal.NewFromString(str)
	if err != nil {
		return decimal.Zero, fmt.Errorf("Unparseable number: %q", sanitized)
	}
	return d.Mul(mult), nil
}

// matchPlayer answers the online players named term, or whose names
// contain it, ignoring case. Bukkit: Server.matchPlayer
func matchPlayer(e *core.Essentials, term string) []*core.User {
	low := strings.ToLower(term)
	var out []*core.User
	for _, u := range e.OnlineUsers() {
		name := u.Name()
		if strings.EqualFold(name, term) {
			return []*core.User{u}
		}
		if strings.Contains(strings.ToLower(name), low) {
			out = append(out, u)
		}
	}
	return out
}

// canInteractVanished reports whether the sender sees vanished players for
// commands. Essentials: User.canInteractVanished
func canInteractVanished(ctx *core.Ctx) bool {
	return ctx.User == nil || ctx.User.HasPermission("essentials.vanish.interact")
}

// loopOfflinePlayers calls fn for each user term names: @s, a UUID, every
// saved user (**), every online player (*), with multiple every online name
// containing it, else one online or offline user. An error from fn ends
// the loop. Essentials: EssentialsLoopCommand.loopOfflinePlayersConsumer
func loopOfflinePlayers(ctx *core.Ctx, multiple, wildcards bool, term string, fn func(*core.User) error) error {
	e := ctx.Ess
	if term == "" {
		return core.ErrPlayerNotFound
	}
	if ctx.User != nil && (term == "@s" || term == "@p") {
		return fn(ctx.User)
	}
	if id, err := uuid.Parse(term); err == nil {
		if !e.Users.Known(id) {
			return core.ErrPlayerNotFound
		}
		return fn(e.Users.Get(id))
	}
	switch {
	case wildcards && term == "**":
		for _, id := range e.Users.All() {
			if err := fn(e.Users.Get(id)); err != nil {
				return err
			}
		}
		return nil
	case wildcards && term == "*":
		skipHidden := !canInteractVanished(ctx)
		for _, u := range e.OnlineUsers() {
			if skipHidden && !ctx.User.CanSee(u) {
				continue
			}
			if err := fn(u); err != nil {
				return err
			}
		}
		return nil
	case multiple:
		if len(strings.TrimSpace(term)) < 3 {
			return core.ErrPlayerNotFound
		}
		matched := matchPlayer(e, term)
		if len(matched) == 0 {
			u, err := e.MatchUser(nil, term, true, true)
			if err != nil {
				return err
			}
			return fn(u)
		}
		for _, u := range matched {
			if err := fn(u); err != nil {
				return err
			}
		}
		return nil
	}
	u, err := e.MatchUser(nil, term, true, true)
	if err != nil {
		return err
	}
	return fn(u)
}

// loopPlayers is the completion of a loop command's player argument: the
// visible players, ** and *. Essentials: EssentialsLoopCommand.getPlayers
func loopPlayers(ctx *core.Ctx) []string {
	return append(ctx.CompletePlayers(), "**", "*")
}
