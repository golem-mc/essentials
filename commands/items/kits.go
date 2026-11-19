package items

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

func registerKits(e *core.Essentials) {
	kitNames := func(ctx *core.Ctx) []string {
		if len(ctx.Args) == 1 {
			return ctx.Ess.Kits.Names()
		}
		return nil
	}
	e.RegisterCommand(
		core.Cmd{Name: "kit", Run: runKit, Complete: func(ctx *core.Ctx) []string {
			switch len(ctx.Args) {
			case 1:
				var out []string
				for _, k := range ctx.Ess.Kits.Names() {
					if ctx.Has("essentials.kits." + strings.ToLower(k)) {
						out = append(out, k)
					}
				}
				return out
			case 2:
				if ctx.User == nil || ctx.Has("essentials.kit.others") {
					return ctx.CompletePlayers()
				}
			}
			return nil
		}},
		core.Cmd{Name: "createkit", PlayerOnly: true, Run: runCreatekit, Complete: func(*core.Ctx) []string { return nil }},
		core.Cmd{Name: "delkit", Run: runDelkit, Complete: kitNames},
		core.Cmd{Name: "showkit", PlayerOnly: true, Run: runShowkit, Complete: kitNames},
		core.Cmd{Name: "kitreset", Run: runKitreset, Complete: func(ctx *core.Ctx) []string {
			switch len(ctx.Args) {
			case 1:
				return ctx.Ess.Kits.Names()
			case 2:
				if ctx.Has("essentials.kitreset.others") {
					return ctx.CompletePlayers()
				}
			}
			return nil
		}},
	)
}

// kitCharge is what taking a kit costs: kit-<name>, else kit-kit.
func kitCharge(e *core.Essentials, u *core.User, name string) *core.Trade {
	return cost(e, u, "kit-"+strings.ToLower(name), "kit-kit")
}

// listKits answers the kits the user may take as MiniMessage: capitalised,
// with their cost, struck through while on cooldown. Essentials:
// Kits.listKits
func listKits(ctx *core.Ctx, u *core.User) string {
	e := ctx.Ess
	var parts []string
	for _, name := range e.Kits.Names() {
		label := core.EscapeTags(capitalCase(name))
		if u == nil {
			parts = append(parts, label)
			continue
		}
		if !u.HasPermission("essentials.kits." + strings.ToLower(name)) {
			continue
		}
		price := ""
		if c := cost(e, u, "kit-"+strings.ToLower(name)); c != nil {
			price = tlRaw(e, ctx.Sender, "kitCost", e.Economy.Format(*c.Money))
		}
		kit, _ := e.Kits.Get(name)
		next, ok := kit.NextUse(u)
		if !ok && e.Config.Bool("skip-used-one-time-kits-from-kit-list", false) {
			continue
		}
		if !ok || !next.IsZero() {
			label = tlRaw(e, ctx.Sender, "kitDelay", capitalCase(name))
		}
		parts = append(parts, label+price)
	}
	return strings.Join(parts, " ")
}

func sendKitList(ctx *core.Ctx, u *core.User) {
	list := listKits(ctx, u)
	if list == "" {
		ctx.Send("noKits", core.Raw(list))
		return
	}
	ctx.Send("kits", core.Raw(list))
}

// sanitize drops control characters. Essentials: StringUtil.sanitizeString
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' || r >= 0x20 && r <= 0x7E || r == 0x85 || r >= 0xA0 && r <= 0xD7FF || r >= 0xE000 && r <= 0xFFFC {
			return r
		}
		return -1
	}, s)
}

// Essentials: Commandkit
func runKit(ctx *core.Ctx) error {
	e := ctx.Ess
	if ctx.User == nil {
		if len(ctx.Args) < 2 {
			sendKitList(ctx, nil)
			return core.ErrNoCharge
		}
		to, err := e.MatchUser(nil, ctx.Args[1], true, false)
		if err != nil {
			return err
		}
		for name := range strings.SplitSeq(strings.ToLower(ctx.Args[0]), ",") {
			kit, ok := e.Kits.Get(name)
			if !ok {
				return core.Tlf("kitNotFound")
			}
			if err := e.Kits.Expand(kit, to); err != nil {
				return err
			}
			ctx.Send("kitGiveTo", name, to.DisplayName())
			to.Send("kitReceive", name)
		}
		return nil
	}
	if len(ctx.Args) < 1 {
		sendKitList(ctx, ctx.User)
		return core.ErrNoCharge
	}
	to := ctx.User
	if len(ctx.Args) > 1 && ctx.Has("essentials.kit.others") {
		u, err := ctx.GetPlayer(1, false)
		if err != nil {
			return err
		}
		to = u
	}
	return giveKits(ctx, to, ctx.User, strings.TrimSpace(sanitize(strings.ToLower(ctx.Args[0]))))
}

func giveKits(ctx *core.Ctx, to, from *core.User, names string) error {
	e := ctx.Ess
	if names == "" {
		return core.Tlf("kitNotFound")
	}
	var kits []*core.Kit
	for name := range strings.SplitSeq(names, ",") {
		if name == "" {
			return core.Tlf("kitNotFound")
		}
		kit, ok := e.Kits.Get(name)
		if !ok {
			return core.Tlf("kitNotFound")
		}
		if err := kit.CheckPerms(from); err != nil {
			return err
		}
		if err := kit.CheckDelay(from); err != nil {
			return err
		}
		if err := kitCharge(e, from, kit.Name).CheckAffordable(from); err != nil {
			return err
		}
		kits = append(kits, kit)
	}
	for _, kit := range kits {
		if err := kit.CheckDelay(from); err != nil {
			e.ShowError(ctx, err)
			continue
		}
		charge := kitCharge(e, to, kit.Name)
		err := charge.CheckAffordable(from)
		if err == nil {
			err = e.Kits.Expand(kit, to)
		}
		if err == nil {
			from.SetKitTimestamp(kit.Name, nowMillis())
			err = charge.Charge(to)
		}
		if err != nil {
			if !errors.Is(err, core.ErrNoCharge) {
				e.ShowError(ctx, err)
			}
			continue
		}
		if from != to {
			from.Send("kitGiveTo", kit.Name, to.DisplayName())
		}
		to.Send("kitReceive", kit.Name)
	}
	return nil
}

// formatKitItem writes a stack as a kit line's item and amount, in /give's
// syntax with the whitespace in its strings escaped, since kit lines are
// split on spaces.
func formatKitItem(e *core.Essentials, s plugin.ItemStack) string {
	one := s
	one.Count = 1
	return escapeSpaces(e.Server.Items().Format(one)) + " " + strconv.Itoa(s.Count)
}

// escapeSpaces replaces whitespace inside SNBT quoted strings with \u
// escapes and drops it outside them.
func escapeSpaces(s string) string {
	var b strings.Builder
	var quote rune
	escaped := false
	for _, r := range s {
		switch {
		case quote != 0 && escaped:
			escaped = false
		case quote != 0 && r == '\\':
			escaped = true
		case quote != 0 && r == quote:
			quote = 0
		case quote == 0 && (r == '"' || r == '\''):
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			if quote != 0 {
				fmt.Fprintf(&b, "\\u%04x", r)
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Essentials: Commandcreatekit
func runCreatekit(ctx *core.Ctx) error {
	e := ctx.Ess
	if len(ctx.Args) != 2 {
		return core.ErrNotEnoughArgs
	}
	delay, err := strconv.ParseInt(ctx.Args[1], 10, 64)
	if err != nil {
		return fmt.Errorf("For input string: \"%s\"", ctx.Args[1])
	}
	name := ctx.Args[0]
	var lines []string
	for i, s := range ctx.Player.Inventory().Contents() {
		if !s.Empty() {
			lines = append(lines, "slot:"+strconv.Itoa(i)+" "+formatKitItem(e, s))
		}
	}
	key := strings.ToLower(strings.NewReplacer(".", "_", "/", "_").Replace(name))
	if err := e.Kits.Add(key, float64(delay), lines); err != nil {
		return err
	}
	ctx.Send("createdKit", name, len(lines), delay)
	return nil
}

// Essentials: Commanddelkit
func runDelkit(ctx *core.Ctx) error {
	e := ctx.Ess
	if len(ctx.Args) == 0 {
		sendKitList(ctx, nil)
		return core.ErrNoCharge
	}
	kit, ok := e.Kits.Get(ctx.Args[0])
	if !ok {
		return core.Tlf("kitNotFound")
	}
	if ctx.User != nil {
		if err := kit.CheckPerms(ctx.User); err != nil {
			return err
		}
	}
	if err := e.Kits.Remove(kit.Name); err != nil {
		return err
	}
	ctx.Send("deleteKit", kit.Name)
	return nil
}

// Essentials: Commandshowkit
func runShowkit(ctx *core.Ctx) error {
	if len(ctx.Args) != 1 {
		return core.ErrNotEnoughArgs
	}
	for name := range strings.SplitSeq(strings.ToLower(ctx.Args[0]), ",") {
		ctx.Send("kitContains", name)
		kit, ok := ctx.Ess.Kits.Get(name)
		if !ok {
			return core.Tlf("kitNotFound")
		}
		for _, line := range kit.Items {
			ctx.Send("kitItem", line)
		}
	}
	return nil
}

// Essentials: Commandkitreset
func runKitreset(ctx *core.Ctx) error {
	e := ctx.Ess
	min := 1
	if ctx.User == nil {
		min = 2
	}
	if len(ctx.Args) < min {
		return core.ErrNotEnoughArgs
	}
	name := ctx.Args[0]
	if _, ok := e.Kits.Get(name); !ok {
		return core.Tlf("kitNotFound")
	}
	target := ctx.User
	if ctx.User == nil || len(ctx.Args) > 1 && ctx.Has("essentials.kitreset.others") {
		u, err := ctx.GetPlayer(1, false)
		if err != nil {
			return err
		}
		target = u
	}
	target.SetKitTimestamp(name, 0)
	if target == ctx.User {
		ctx.Send("kitReset", name)
	} else {
		ctx.Send("kitResetOther", name, target.DisplayName())
	}
	return nil
}

func nowMillis() int64 { return time.Now().UnixMilli() }
