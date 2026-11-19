package economy

import (
	"errors"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/shopspring/decimal"
)

// ecoCmd gives, takes, sets or resets balances. Essentials: Commandeco
func ecoCmd() core.Cmd {
	return core.Cmd{
		Name: "eco",
		Run: func(ctx *core.Ctx) error {
			e := ctx.Ess
			if len(ctx.Args) < 2 {
				return core.ErrNotEnoughArgs
			}
			cmd := strings.ToLower(ctx.Args[0])
			var amount decimal.Decimal
			percent := false
			switch cmd {
			case "reset":
				amount = e.Settings.StartingBalance()
			case "give", "take", "set":
				if len(ctx.Args) < 3 {
					return core.ErrNotEnoughArgs
				}
				arg := ctx.Args[2]
				// Essentials' parser refuses the % it checks for, so
				// percentages never worked there; they do here.
				if percent = strings.HasSuffix(arg, "%"); percent {
					arg = strings.TrimSuffix(arg, "%")
				}
				var err error
				if amount, err = parseAmount(arg); err != nil {
					return core.ErrNotEnoughArgs
				}
			default:
				return core.ErrNotEnoughArgs
			}
			return loopOfflinePlayers(ctx, false, true, ctx.Args[1], func(u *core.User) error {
				amount := amount
				if percent {
					amount = u.Money().Mul(amount).Shift(-2)
				}
				switch cmd {
				case "give":
					return u.GiveMoney(amount, ctx.Sender)
				case "take":
					if minimum := e.Settings.MinMoney(); !u.Money().Sub(amount).LessThan(minimum) {
						u.TakeMoney(amount, ctx.Sender)
					} else {
						e.ShowError(ctx, core.Tlf("minimumBalanceError", core.Raw(format(e, minimum))))
					}
				default: // set, reset
					minimum, maximum := e.Settings.MinMoney(), e.Settings.MaxMoney()
					switch {
					case amount.LessThan(minimum):
						amount = minimum
					case amount.GreaterThan(maximum):
						amount = maximum
					}
					if err := u.SetMoney(amount); err != nil {
						return err
					}
					u.Send("setBal", core.Raw(format(e, u.Money())))
					ctx.Send("setBalOthers", u.DisplayName(), core.Raw(format(e, u.Money())))
				}
				return nil
			})
		},
		Complete: func(ctx *core.Ctx) []string {
			switch len(ctx.Args) {
			case 1:
				return []string{"give", "take", "set", "reset"}
			case 2:
				return loopPlayers(ctx)
			case 3:
				switch strings.ToLower(ctx.Args[0]) {
				case "reset":
					return nil
				case "set":
					return []string{"0", ctx.Ess.Settings.StartingBalance().String()}
				}
				return []string{"1", "10", "100", "1000"}
			}
			return nil
		},
	}
}

// payCmd pays other players, asking to repeat the command first when the
// sender has payment confirmation on. Essentials: Commandpay
func payCmd() core.Cmd {
	return core.Cmd{
		Name:       "pay",
		PlayerOnly: true,
		Run: func(ctx *core.Ctx) error {
			e, user := ctx.Ess, ctx.User
			if len(ctx.Args) < 2 {
				return core.ErrNotEnoughArgs
			}
			og := ctx.Args[1]
			if strings.Contains(og, "-") {
				return core.Tlf("payMustBePositive")
			}
			if strings.IndexAny(og, "0123456789.") < 0 {
				return core.ErrNotEnoughArgs
			}
			amount, err := parseAmount(og)
			if err != nil {
				return err
			}
			minPay := minimumPayAmount(e)
			if amount.LessThan(minPay) {
				return core.Tlf("minimumPayAmount", core.Raw(formatExact(e, minPay)))
			}
			canPayOffline := ctx.Has("essentials.pay.offline")
			if !canPayOffline && ctx.Args[0] == "**" {
				ctx.Send("payOffline")
				return nil
			}
			confirming := confirmingPayments(user)
			informToConfirm := false
			err = loopOfflinePlayers(ctx, false, ctx.Has("essentials.pay.multiple"), ctx.Args[0], func(target *core.User) error {
				if (!target.Online() || target.Vanished || !user.CanSee(target)) && !canPayOffline {
					ctx.Send("payOffline")
					return nil
				}
				if !target.Data.AcceptingPay || (e.Config.Bool("pay-excludes-ignore-list", false) && target.IsIgnoring(user)) {
					ctx.Send("notAcceptingPay", target.DisplayName())
					return nil
				}
				if prev, ok := confirming[target.ID]; promptingPayConfirm(user) && (!ok || !prev.Equal(amount)) {
					if !informToConfirm {
						clear(confirming)
						informToConfirm = true
					}
					confirming[target.ID] = amount
					return nil
				}
				if err := payUser(user, target, amount); err != nil {
					return err
				}
				delete(confirming, target.ID)
				loc := ctx.Player.Location()
				e.Economy.Log("Command", "Pay", "Player", user.Name(), amount, target.Name(), &loc)
				return nil
			})
			if err != nil {
				return err
			}
			if informToConfirm {
				cmd := "/" + ctx.Label + " " + strings.Join(ctx.Args, " ")
				ctx.Send("confirmPayment", core.Raw(format(e, amount)), cmd)
			}
			return nil
		},
		Complete: func(ctx *core.Ctx) []string {
			switch len(ctx.Args) {
			case 1:
				return loopPlayers(ctx)
			case 2:
				return []string{minimumPayAmount(ctx.Ess).String()}
			}
			return nil
		},
	}
}

// minimumPayAmount is minimum-pay-amount, read exactly as written.
func minimumPayAmount(e *core.Essentials) decimal.Decimal {
	if d, err := decimal.NewFromString(e.Config.String("minimum-pay-amount", "0.001")); err == nil {
		return d
	}
	return decimal.RequireFromString("0.001")
}

// payUser moves amount from user to receiver; when the receiver would pass
// max-money the user is told and refunded. Essentials: User.payUser and
// Commandpay's MaxMoneyException handling
func payUser(user, receiver *core.User, amount decimal.Decimal) error {
	e := user.Ess()
	if amount.Sign() <= 0 {
		return errors.New(e.TlString("payMustBePositive"))
	}
	if !user.CanAfford(amount) {
		return core.Tlf("notEnoughMoney", core.Raw(format(e, amount)))
	}
	if err := user.SetMoney(user.Money().Sub(amount)); err != nil {
		return err
	}
	if err := receiver.SetMoney(receiver.Money().Add(amount)); err != nil {
		user.Send("maxMoney")
		_ = user.SetMoney(user.Money().Add(amount))
		return nil
	}
	user.Send("moneySentTo", core.Raw(format(e, amount)), receiver.DisplayName())
	receiver.Send("moneyRecievedFrom", core.Raw(format(e, amount)), user.DisplayName())
	return nil
}

// paytoggleCmd turns accepting payments on or off; /payon and /payoff set
// it. Essentials: Commandpaytoggle
func paytoggleCmd() core.Cmd {
	t := toggle{others: "essentials.paytoggle.others", set: func(ctx *core.Ctx, u *core.User, on *bool) {
		enabled := !u.Data.AcceptingPay
		if on != nil {
			enabled = *on
		}
		u.Data.AcceptingPay = enabled
		u.Save()
		if enabled {
			u.Send("payToggleOn")
		} else {
			u.Send("payToggleOff")
		}
		if ctx.User == nil || ctx.User.ID != u.ID {
			key := "payDisabledFor"
			if enabled {
				key = "payEnabledFor"
			}
			ctx.Send(key, u.DisplayName())
		}
	}}
	return core.Cmd{
		Name: "paytoggle",
		Run: func(ctx *core.Ctx) error {
			if ctx.User == nil {
				return t.otherPlayers(ctx)
			}
			on, off := true, false
			switch {
			case strings.Contains(ctx.Label, "payon"):
				t.set(ctx, ctx.User, &on)
			case strings.Contains(ctx.Label, "payoff"):
				t.set(ctx, ctx.User, &off)
			default:
				return t.withArgs(ctx)
			}
			return nil
		},
		Complete: t.complete,
	}
}

// payconfirmtoggleCmd turns payment confirmation on or off; /payconfirmon
// and /payconfirmoff set it. Essentials: Commandpayconfirmtoggle
func payconfirmtoggleCmd() core.Cmd {
	return core.Cmd{
		Name:       "payconfirmtoggle",
		PlayerOnly: true,
		Run: func(ctx *core.Ctx) error {
			u := ctx.User
			confirm := !promptingPayConfirm(u)
			switch {
			case strings.Contains(ctx.Label, "payconfirmon"):
				confirm = true
			case strings.Contains(ctx.Label, "payconfirmoff"):
				confirm = false
			}
			u.Data.ConfirmPay = &confirm
			u.Save()
			if confirm {
				ctx.Send("payConfirmToggleOn")
			} else {
				ctx.Send("payConfirmToggleOff")
			}
			clear(confirmingPayments(u))
			return nil
		},
	}
}

// toggle is the argument handling of a command that toggles a setting for
// the sender or, with the others permission, for other players.
// Essentials: EssentialsToggleCommand
type toggle struct {
	others string
	// set changes the setting; on nil toggles it.
	set func(ctx *core.Ctx, u *core.User, on *bool)
}

// matchToggle reads on/enable/1 and off/disable/0. Essentials: matchToggleArgument
func matchToggle(arg string) *bool {
	on, off := true, false
	switch {
	case strings.EqualFold(arg, "on") || strings.HasPrefix(arg, "ena") || arg == "1":
		return &on
	case strings.EqualFold(arg, "off") || strings.HasPrefix(arg, "dis") || arg == "0":
		return &off
	}
	return nil
}

// withArgs toggles the sender, or the players named.
// Essentials: handleToggleWithArgs
func (t toggle) withArgs(ctx *core.Ctx) error {
	switch {
	case len(ctx.Args) == 1:
		on := matchToggle(ctx.Args[0])
		if on == nil && ctx.Has(t.others) {
			return t.otherPlayers(ctx)
		}
		t.set(ctx, ctx.User, on)
	case len(ctx.Args) == 2 && ctx.Has(t.others):
		return t.otherPlayers(ctx)
	default:
		t.set(ctx, ctx.User, nil)
	}
	return nil
}

// otherPlayers toggles every online player whose name contains args[0], to
// args[1] when given. Essentials: toggleOtherPlayers
func (t toggle) otherPlayers(ctx *core.Ctx) error {
	if len(ctx.Args) < 1 || len(strings.TrimSpace(ctx.Args[0])) < 2 {
		return core.ErrPlayerNotFound
	}
	skipHidden := !canInteractVanished(ctx)
	found := false
	for _, u := range matchPlayer(ctx.Ess, ctx.Args[0]) {
		if skipHidden && !ctx.User.CanSee(u) {
			continue
		}
		found = true
		var on *bool
		if len(ctx.Args) > 1 {
			on = matchToggle(ctx.Args[1])
		}
		t.set(ctx, u, on)
	}
	if !found {
		return core.ErrPlayerNotFound
	}
	return nil
}

func (t toggle) complete(ctx *core.Ctx) []string {
	others := ctx.User == nil || ctx.Has(t.others)
	switch {
	case len(ctx.Args) == 1 && others:
		return ctx.CompletePlayers()
	case len(ctx.Args) == 1:
		return []string{"enable", "disable"}
	case len(ctx.Args) == 2 && others:
		return []string{"enable", "disable"}
	}
	return nil
}
