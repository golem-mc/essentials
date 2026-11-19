package messaging

import (
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

// mailDateFormat is how a mail's date is shown. Essentials: MailServiceImpl.df
const mailDateFormat = "2006/01/02 15:04"

// mailLine answers the message key and arguments that show m. Essentials:
// MailServiceImpl.getMailTlKey, getMailTlArgs
func mailLine(m core.MailMessage) (string, []any) {
	if m.Legacy {
		return "mailMessage", []any{m.Message}
	}
	key := "mailFormatNew"
	if m.Read {
		key = "mailFormatNewRead"
	}
	if m.Expire != 0 {
		key += "Timed"
	}
	return key, []any{time.UnixMilli(m.Timestamp).Format(mailDateFormat), m.SenderName, m.Message}
}

func expired(m core.MailMessage) bool { return m.Expire != 0 && m.Expire <= time.Now().UnixMilli() }

// countMail counts a mail against mails-per-minute, server wide.
func (g *group) countMail() error {
	if time.Since(g.mailSince).Abs() > time.Minute {
		g.mailSince = time.Now()
		g.mailCount = 0
	}
	g.mailCount++
	if limit := g.e.Config.Int("mails-per-minute", 1000); g.mailCount > limit {
		return core.Tlf("mailDelay", limit)
	}
	return nil
}

// mailTarget finds the user a mail goes to, online or not.
func (g *group) mailTarget(name string) (*core.User, error) {
	u, err := g.e.MatchUser(nil, name, true, true)
	if err != nil {
		return nil, core.Tlf("playerNeverOnServer", name)
	}
	return u, nil
}

// sendAll mails every known user. Essentials: Commandmail.SendAll
func (g *group) sendAll(from *core.User, msg string, expire time.Duration) {
	for _, id := range g.e.Users.All() {
		g.e.SendMail(g.e.Users.Get(id), from, msg, expire)
	}
}

// clearAll empties every known user's mail. Essentials: Commandmail.ClearAll
func (g *group) clearAll() {
	for _, id := range g.e.Users.All() {
		u := g.e.Users.Get(id)
		if len(u.Data.Mail) > 0 {
			u.Data.Mail = nil
			u.Save()
		}
	}
}

// clearMail removes mail number n (1-based) of u, or all of it for n <= 0;
// to is told how it went.
func (g *group) clearMail(to plugin.CommandSender, u *core.User, self bool, n int) error {
	if len(u.Data.Mail) == 0 {
		if self {
			g.e.Send(to, "noMail")
		} else {
			g.e.Send(to, "noMailOther", u.DisplayName())
		}
		return core.ErrNoCharge
	}
	if n > 0 {
		if n > len(u.Data.Mail) {
			g.e.Send(to, "mailClearIndex", len(u.Data.Mail))
			return core.ErrNoCharge
		}
		u.Data.Mail = append(u.Data.Mail[:n-1:n-1], u.Data.Mail[n:]...)
	} else {
		u.Data.Mail = nil
	}
	u.Save()
	g.e.Send(to, "mailCleared")
	return nil
}

// readMail shows the user's mail a page at a time and marks it read,
// dropping expired mail. Essentials: Commandmail "read" with SimpleTextPager
func (g *group) readMail(ctx *core.Ctx) error {
	u := ctx.User
	if len(u.Data.Mail) == 0 {
		ctx.Send("noMail")
		return core.ErrNoCharge
	}
	type line struct {
		key  string
		args []any
	}
	var lines []line
	kept := u.Data.Mail[:0:0]
	for _, m := range u.Data.Mail {
		if expired(m) {
			continue
		}
		key, args := mailLine(m)
		lines = append(lines, line{key, args})
		m.Read = true
		kept = append(kept, m)
	}
	if len(lines) == 0 {
		ctx.Send("noMail")
		return core.ErrNoCharge
	}
	page := 1
	if len(ctx.Args) > 1 {
		if n, err := strconv.Atoi(ctx.Args[1]); err == nil {
			page = max(n, 1)
		}
	}
	pages := (len(lines) + 8) / 9
	if page > pages {
		ctx.Send("infoUnknownChapter")
	} else {
		name := ctx.Label + " " + ctx.Args[0]
		ctx.Send("infoPages", page, pages, capitalTitle(name))
		for i := (page - 1) * 9; i < len(lines) && i < page*9; i++ {
			ctx.Send(lines[i].key, lines[i].args...)
		}
		if page < pages {
			ctx.Send("readNextPage", name, page+1)
		}
	}
	ctx.Send("mailClear")
	u.Data.Mail = kept
	u.Save()
	return nil
}

// capitalTitle capitalises a command name for a page header: "mail read"
// is "Mail: read". Essentials: I18n.capitalCase
func capitalTitle(name string) string {
	first, rest, ok := strings.Cut(name, " ")
	if first == "" {
		return name
	}
	t := strings.ToUpper(first[:1]) + first[1:]
	if ok {
		return t + ": " + rest
	}
	return t
}

func (g *group) registerMail() {
	g.e.RegisterCommand(core.Cmd{
		// Essentials: Commandmail
		Name: "mail",
		Run: func(ctx *core.Ctx) error {
			if ctx.User == nil {
				return g.mailConsole(ctx)
			}
			return g.mailUser(ctx)
		},
		Complete: g.completeMail,
	})
}

func (g *group) mailUser(ctx *core.Ctx) error {
	u := ctx.User
	args := ctx.Args
	sub := strings.ToLower(ctx.Arg(0))
	needs := func(node string) error {
		if !ctx.Has(node) {
			return core.Tlf("noPerm", node)
		}
		return nil
	}
	switch {
	case len(args) >= 1 && sub == "read":
		return g.readMail(ctx)
	case len(args) >= 3 && sub == "send", len(args) >= 4 && sub == "sendtemp":
		if err := needs("essentials.mail." + sub); err != nil {
			return err
		}
		if u.IsMuted() {
			return muteError(u)
		}
		to, err := g.mailTarget(args[1])
		if err != nil {
			return err
		}
		var until time.Time
		msgAt := 2
		if sub == "sendtemp" {
			if until, err = core.ParseDateDiff(args[2], true); err != nil {
				return err
			}
			msgAt = 3
		}
		msg := g.formatMessage(u, "essentials.mail", sanitize(stripFormat(ctx.FinalArg(msgAt))))
		if len([]rune(msg)) > 1000 {
			return core.Tlf("mailTooLong")
		}
		if !to.IsIgnoring(u) {
			if err := g.countMail(); err != nil {
				return err
			}
			var expire time.Duration
			if !until.IsZero() {
				expire = time.Until(until)
			}
			g.e.SendMail(to, u, msg, expire)
		}
		if until.IsZero() {
			ctx.Send("mailSentTo", to.DisplayName(), to.Name())
		} else {
			ctx.Send("mailSentToExpire", to.DisplayName(), core.FormatDateDiff(until), to.Name())
		}
		ctx.SendText(g.legacyText(msg))
		return nil
	case len(args) > 1 && sub == "sendall":
		if err := needs("essentials.mail.sendall"); err != nil {
			return err
		}
		g.sendAll(u, g.formatMessage(u, "essentials.mail", sanitize(stripFormat(ctx.FinalArg(1)))), 0)
		ctx.Send("mailSent")
		return nil
	case len(args) >= 3 && sub == "sendtempall":
		if err := needs("essentials.mail.sendtempall"); err != nil {
			return err
		}
		until, err := core.ParseDateDiff(args[1], true)
		if err != nil {
			return err
		}
		g.sendAll(u, g.formatMessage(u, "essentials.mail", sanitize(stripFormat(ctx.FinalArg(2)))), time.Until(until))
		ctx.Send("mailSent")
		return nil
	case len(args) >= 1 && sub == "clear":
		target, n := u, -1
		if len(args) > 1 {
			if core.IsPositiveInt(args[1]) {
				n, _ = strconv.Atoi(args[1])
			} else if err := needs("essentials.mail.clear.others"); err != nil {
				return err
			} else {
				t, err := g.getPlayer(ctx, args[1], true)
				if err != nil {
					return err
				}
				target = t
				if len(args) > 2 && core.IsPositiveInt(args[2]) {
					n, _ = strconv.Atoi(args[2])
				}
			}
		}
		return g.clearMail(ctx.Sender, target, target == u, n)
	case len(args) >= 1 && sub == "clearall":
		if err := needs("essentials.mail.clearall"); err != nil {
			return err
		}
		g.clearAll()
		ctx.Send("mailClearedAll")
		return nil
	}
	return core.ErrNotEnoughArgs
}

func (g *group) mailConsole(ctx *core.Ctx) error {
	args := ctx.Args
	sub := strings.ToLower(ctx.Arg(0))
	switch {
	case len(args) >= 1 && sub == "read":
		return core.TlPlain("onlyPlayers", ctx.Label+" read")
	case len(args) > 1 && sub == "clear":
		u, err := g.e.MatchUser(nil, args[1], true, true)
		if err != nil {
			return err
		}
		n := -1
		if len(args) > 2 && core.IsPositiveInt(args[2]) {
			n, _ = strconv.Atoi(args[2])
		}
		return g.clearMail(ctx.Sender, u, false, n)
	case len(args) >= 1 && sub == "clearall":
		g.clearAll()
		ctx.Send("mailClearedAll")
		return nil
	case len(args) >= 3 && sub == "send":
		u, err := g.mailTarget(args[1])
		if err != nil {
			return err
		}
		g.e.SendMail(u, nil, replaceFormat(ctx.FinalArg(2)), 0)
	case len(args) >= 4 && sub == "sendtemp":
		u, err := g.mailTarget(args[1])
		if err != nil {
			return err
		}
		until, err := core.ParseDateDiff(args[2], true)
		if err != nil {
			return err
		}
		g.e.SendMail(u, nil, replaceFormat(ctx.FinalArg(3)), time.Until(until))
	case len(args) >= 2 && sub == "sendall":
		g.sendAll(nil, replaceFormat(ctx.FinalArg(1)), 0)
	case len(args) >= 3 && sub == "sendtempall":
		until, err := core.ParseDateDiff(args[1], true)
		if err != nil {
			return err
		}
		g.sendAll(nil, replaceFormat(ctx.FinalArg(2)), time.Until(until))
	case len(args) >= 2:
		// The console may leave out "send", the only thing it can do.
		u, err := g.mailTarget(args[0])
		if err != nil {
			return err
		}
		g.e.SendMail(u, nil, replaceFormat(ctx.FinalArg(1)), 0)
	default:
		return core.ErrNotEnoughArgs
	}
	ctx.Send("mailSent")
	return nil
}

func (g *group) completeMail(ctx *core.Ctx) []string {
	args := ctx.Args
	sub := strings.ToLower(ctx.Arg(0))
	if ctx.User == nil {
		switch {
		case len(args) == 1:
			return []string{"send", "sendall", "sendtemp", "sendtempall", "clearall", "clear"}
		case len(args) == 2 && (sub == "send" || sub == "sendtemp" || sub == "clear"):
			return ctx.CompletePlayers()
		case len(args) == 2 && sub == "sendtempall", len(args) == 3 && sub == "sendtemp":
			return core.Durations
		}
		return nil
	}
	switch len(args) {
	case 1:
		opts := []string{"read", "clear"}
		for _, s := range []string{"send", "sendtemp", "sendall", "sendtempall", "clearall"} {
			if ctx.Has("essentials.mail." + s) {
				opts = append(opts, s)
			}
		}
		return opts
	case 2:
		switch {
		case (sub == "send" || sub == "sendtemp") && ctx.Has("essentials.mail."+sub), sub == "clear" && ctx.Has("essentials.mail.clear.others"):
			return ctx.CompletePlayers()
		case sub == "sendtempall" && ctx.Has("essentials.mail.sendtempall"):
			return core.Durations
		case sub == "read":
			pages := (len(ctx.User.Data.Mail) + 8) / 9
			if pages == 0 {
				return []string{"0"}
			}
			return numbers(pages)
		case sub == "clear":
			return numbers(min(len(ctx.User.Data.Mail), 9))
		}
	case 3:
		if sub == "sendtemp" && ctx.Has("essentials.mail.sendtemp") {
			return core.Durations
		}
	}
	return nil
}

// numbers answers "1" to n.
func numbers(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = strconv.Itoa(i + 1)
	}
	return out
}
