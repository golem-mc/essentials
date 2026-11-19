package economy

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/shopspring/decimal"
)

// balanceCmd shows a balance. Essentials: Commandbalance
func balanceCmd() core.Cmd {
	return core.Cmd{
		Name: "balance",
		Run: func(ctx *core.Ctx) error {
			e := ctx.Ess
			if ctx.User == nil {
				if len(ctx.Args) == 0 {
					return core.ErrNotEnoughArgs
				}
				target, err := e.MatchUser(nil, ctx.Args[0], true, true)
				if err != nil {
					return err
				}
				ctx.Send("balanceOther", displayName(target), core.Raw(format(e, target.Money())))
				return nil
			}
			switch {
			case len(ctx.Args) == 1 && ctx.Has("essentials.balance.others"):
				target, err := e.MatchUser(nil, ctx.Args[0], false, true)
				if err != nil {
					return err
				}
				ctx.Send("balanceOther", displayName(target), core.Raw(format(e, target.Money())))
			case len(ctx.Args) < 2:
				ctx.Send("balance", core.Raw(format(e, ctx.User.Money())))
			default:
				return core.ErrNotEnoughArgs
			}
			return nil
		},
		Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 && ctx.Has("essentials.balance.others") {
				return ctx.CompletePlayers()
			}
			return nil
		},
	}
}

// topEntry is a line of the balance ranking. Essentials: BalanceTop.Entry
type topEntry struct {
	ID      plugin.UUID
	Name    string
	Balance decimal.Decimal
}

// balanceTop is the ranking /balancetop shows, cached for two minutes,
// and the pages built from it. Essentials: BalanceTopImpl and
// Commandbalancetop's cache
type balanceTop struct {
	entries []topEntry
	total   decimal.Decimal
	age     time.Time
	lines   []string
}

const (
	// baltopCacheTime is how long a ranking is shown before it is
	// calculated again. Essentials: Commandbalancetop.CACHETIME
	baltopCacheTime = 2 * time.Minute
	// baltopMinUsers is the user count above which the sender is told to
	// wait. Essentials: Commandbalancetop.MINUSERS
	baltopMinUsers = 50
)

func (b *balanceTop) cmd() core.Cmd {
	return core.Cmd{
		Name: "balancetop",
		Run:  b.run,
		Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) != 1 {
				return nil
			}
			opts := []string{"1"}
			if ctx.Has("essentials.balancetop.force") {
				opts = append(opts, "force")
			}
			return opts
		},
	}
}

func (b *balanceTop) run(ctx *core.Ctx) error {
	e := ctx.Ess
	page, force := 0, false
	if len(ctx.Args) > 0 {
		if n, err := strconv.Atoi(ctx.Args[0]); err == nil {
			page = n
		} else if strings.EqualFold(ctx.Args[0], "force") && ctx.Has("essentials.balancetop.force") {
			force = true
		}
	}
	if !force && time.Since(b.age) < baltopCacheTime {
		b.output(ctx, page)
		return nil
	}
	if n := len(e.Users.All()); n > baltopMinUsers {
		ctx.Send("orderBalances", n)
	}
	if e.Config.Bool("disable-eco", false) {
		if e.Settings.Debug() {
			e.Log.Info("Internal economy functions disabled, aborting baltop.")
		}
		return nil
	}
	b.calculate(e)
	b.build(e)
	b.output(ctx, page)
	return nil
}

// calculate ranks every saved user, leaving out NPCs (unless
// npcs-in-balance-ranking) and users exempt from the ranking, and keeps
// baltop-entry-limit of them. Essentials: BalanceTopImpl.calculateBalanceTopMap
func (b *balanceTop) calculate(e *core.Essentials) {
	var entries []topEntry
	total := decimal.Zero
	npcs := e.Config.Bool("npcs-in-balance-ranking", false)
	for _, id := range e.Users.All() {
		u := e.Users.Get(id)
		if !npcs && u.Data.NPC {
			continue
		}
		if baltopExempt(u) {
			continue
		}
		money := u.Money()
		total = total.Add(money)
		name := u.DisplayName()
		switch {
		case !u.Online():
			name = u.Name()
		case u.Vanished:
			name = u.Name()
		}
		entries = append(entries, topEntry{ID: id, Name: name, Balance: money})
	}
	slices.SortStableFunc(entries, func(a, c topEntry) int { return c.Balance.Cmp(a.Balance) })
	if limit := e.Config.Int("baltop-entry-limit", -1); limit != -1 && limit < len(entries) {
		entries = entries[:max(limit, 0)]
	}
	b.entries, b.total, b.age = entries, total, time.Now()
}

// baltopExempt reports whether essentials.balancetop.exclude keeps the user
// out of the ranking, remembered for when they are offline.
// Essentials: User.isBaltopExempt
func baltopExempt(u *core.User) bool {
	if !u.Online() {
		return u.Data.BaltopExempt
	}
	exempt := u.HasPermission("essentials.balancetop.exclude")
	if exempt != u.Data.BaltopExempt {
		u.Data.BaltopExempt = exempt
		u.Save()
	}
	return exempt
}

// build writes the ranking's lines: the server total, then the users who
// meet show-zero-baltop and baltop-requirements. Essentials:
// Commandbalancetop.Viewer
func (b *balanceTop) build(e *core.Essentials) {
	lines := []string{e.I18n.Format("", "serverTotal", core.Raw(format(e, b.total)))}
	showZero := e.Config.Bool("show-zero-baltop", true)
	minBalance := decimal.NewFromFloat(e.Config.Float("baltop-requirements.minimum-balance", 0))
	minPlaytime := int64(e.Config.Int("baltop-requirements.minimum-playtime", 0))
	for i, ent := range b.entries {
		// The play time of online and offline players, in seconds; read
		// only when there is a minimum, since an offline player's is read
		// from their stats file.
		secs := int64(0)
		if minPlaytime > 0 {
			secs = max(int64(e.Users.Get(ent.ID).PlayTime())/20, 0)
		}
		if (showZero || ent.Balance.Sign() > 0) && !ent.Balance.LessThan(minBalance) && secs >= minPlaytime {
			lines = append(lines, e.I18n.Format("", "balanceTopLine", i+1, ent.Name, core.Raw(format(e, ent.Balance))))
		}
	}
	b.lines = lines
}

// output shows a page of the cached ranking under its date.
// Essentials: Commandbalancetop.outputCache
func (b *balanceTop) output(ctx *core.Ctx, page int) {
	ctx.Send("balanceTop", b.age.Format("1/2/06, 3:04 PM"))
	ctx.Ess.ShowText(ctx.Sender, b.lines, strconv.Itoa(page), "", "balancetop", false)
}
