package messaging

import (
	"strings"
	"time"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

// Session keys of a user's private message state.
const (
	sessionIgnoreMsg    = "messaging.ignoreMsg"
	sessionReplyConsole = "messaging.replyConsole"
)

// recipient is a party to a private message: a user, or the console when u
// is nil. Essentials: IMessageRecipient, SimpleMessageRecipient
type recipient struct {
	g *group
	u *core.User
	// cs is the console's sender for this command, which may capture its
	// output; nil for the server console.
	cs plugin.CommandSender
}

// replyState is the console's reply recipient and when it last received a
// message; a user's live in User.ReplyTo and User.ReplyAt.
type replyState struct {
	to   *recipient
	last time.Time
}

// response is how a private message went. Essentials: MessageResponse
type response int

const (
	sent response = iota
	sentButAfk
	messagesIgnored
	senderIgnored
	unreachable
	senderMuted
)

func (r response) success() bool { return r == sent || r == sentButAfk }

func (g *group) consoleRecipient() recipient { return recipient{g: g} }

func (g *group) userRecipient(u *core.User) recipient { return recipient{g: g, u: u} }

func (r recipient) commandSender() plugin.CommandSender {
	if r.u == nil {
		if r.cs != nil {
			return r.cs
		}
		return r.g.e.Server.Console()
	}
	return sender(r.u)
}

func (r recipient) name() string {
	if r.u == nil {
		return "Console"
	}
	return r.u.Name()
}

func (r recipient) displayName() string {
	if r.u == nil {
		return r.g.e.TlString("consoleName")
	}
	return r.u.DisplayName()
}

func (r recipient) reachable() bool { return r.u == nil || r.u.Online() }

func (r recipient) send(key string, args ...any) {
	if s := r.commandSender(); s != nil {
		r.g.e.Send(s, key, args...)
	}
}

// tl answers the message key in the recipient's language, as MiniMessage.
func (r recipient) tl(key string) core.Raw {
	if s := r.commandSender(); s != nil {
		return r.g.tl(s, key)
	}
	return core.Raw(r.g.e.I18n.Format("", key))
}

// same reports whether r and o are the same party.
func (r recipient) same(o recipient) bool {
	if r.u == nil || o.u == nil {
		return r.u == nil && o.u == nil
	}
	return r.u.ID == o.u.ID
}

// replyTo answers whom r replies to.
func (r recipient) replyTo() (recipient, bool) {
	if r.u == nil {
		if r.g.console.to == nil {
			return recipient{}, false
		}
		return *r.g.console.to, true
	}
	if c, _ := core.SessionValue[bool](r.u, sessionReplyConsole); c {
		return r.g.consoleRecipient(), true
	}
	if r.u.ReplyTo == (plugin.UUID{}) {
		return recipient{}, false
	}
	return r.g.userRecipient(r.g.e.Users.Get(r.u.ReplyTo)), true
}

// setReplyTo sets whom r replies to; ok false clears it.
func (r recipient) setReplyTo(to recipient, ok bool) {
	if r.u == nil {
		if ok {
			to.cs = nil
			r.g.console.to = &to
		} else {
			r.g.console.to = nil
		}
		return
	}
	delete(r.u.Session, sessionReplyConsole)
	r.u.ReplyTo = plugin.UUID{}
	switch {
	case !ok:
	case to.u == nil:
		r.u.Session[sessionReplyConsole] = true
	default:
		r.u.ReplyTo = to.u.ID
	}
}

func (r recipient) lastMessage() time.Time {
	if r.u == nil {
		return r.g.console.last
	}
	return r.u.ReplyAt
}

func (r recipient) setLastMessage(t time.Time) {
	if r.u == nil {
		r.g.console.last = t
	} else {
		r.u.ReplyAt = t
	}
}

// hiddenFrom reports whether p cannot see r.
func (r recipient) hiddenFrom(p plugin.Player) bool { return r.u != nil && hiddenFrom(r.u, p) }

// ignoresMessages reports whether u turned private messages off with
// /msgtoggle.
func ignoresMessages(u *core.User) bool {
	v, _ := core.SessionValue[bool](u, sessionIgnoreMsg)
	return v
}

// lastMessageReplyRecipient reports whether u's /r goes to whom they last
// wrote rather than who last wrote to them.
func (g *group) lastMessageReplyRecipient(u *core.User) bool {
	if u.Data.LastMessageReplyRecipient != nil {
		return *u.Data.LastMessageReplyRecipient
	}
	return g.e.Settings.LastMessageReplyRecipient()
}

// sendPrivate sends msg (§ formatted) from r to to, telling r how it went
// and showing it to social spies. Essentials:
// SimpleMessageRecipient.sendMessage
func (r recipient) sendPrivate(to recipient, msg string) response {
	if r.u != nil && r.u.IsMuted() {
		r.spy(to, msg, true)
		r.g.e.NotifyMuted(r.u)
		return senderMuted
	}
	resp := to.receive(r, msg)
	switch resp {
	case unreachable:
		r.send("recentlyForeverAlone", to.displayName())
	case messagesIgnored:
		r.send("msgIgnore", to.displayName())
	case senderIgnored:
	default:
		if resp == sentButAfk {
			if to.u.AfkMessage != "" {
				r.send("userAFKWithMessage", to.displayName(), to.u.AfkMessage)
			} else {
				r.send("userAFK", to.displayName())
			}
		}
		r.send("msgFormat", r.tl("meSender"), to.displayName(), msg)
		r.spy(to, msg, false)
	}
	if resp.success() {
		r.setReplyTo(to, true)
	}
	return resp
}

// spy shows a private message to the online social spies, unless either
// party is exempt. Essentials: SimpleMessageRecipient.sendSocialSpy
func (r recipient) spy(to recipient, msg string, muted bool) {
	e := r.g.e
	if !e.Config.Bool("socialspy-messages", true) {
		return
	}
	if muted && !e.Config.Bool("socialspy-listen-muted-players", true) {
		return
	}
	if r.u == nil || r.u.HasPermission("essentials.chat.spy.exempt") || to.u == nil || to.u.HasPermission("essentials.chat.spy.exempt") {
		return
	}
	senderName, toName := r.name(), to.name()
	if e.Config.Bool("socialspy-uses-displaynames", true) {
		senderName, toName = r.displayName(), to.displayName()
	}
	var prefix plugin.Text
	if muted {
		prefix = e.TlFor(r.commandSender(), "socialSpyMutedPrefix")
	} else {
		prefix = e.Tl("socialSpyPrefix")
	}
	line := e.Tl("socialSpyMsgFormat", senderName, toName, msg)
	for _, spy := range e.OnlineUsers() {
		if spy.Data.SocialSpy && spy.ID != r.u.ID && spy.ID != to.u.ID {
			spy.SendText(prefix.Append(line))
		}
	}
}

// receive shows msg from from to r and updates r's reply recipient.
// Essentials: SimpleMessageRecipient.onReceiveMessage
func (r recipient) receive(from recipient, msg string) response {
	if !r.reachable() {
		return unreachable
	}
	afk := false
	lastReply := r.g.e.Settings.LastMessageReplyRecipient()
	if r.u != nil {
		if ignoresMessages(r.u) && from.u != nil && !from.u.HasPermission("essentials.msgtoggle.bypass") {
			return messagesIgnored
		}
		afk = r.u.IsAfk()
		lastReply = r.g.lastMessageReplyRecipient(r.u)
		if from.u != nil && r.u.IsIgnoring(from.u) {
			return senderIgnored
		}
	}
	r.send("msgFormat", from.displayName(), r.tl("meRecipient"), msg)
	if lastReply {
		timeout := time.Duration(r.g.e.Config.Int("last-message-reply-recipient-timeout", 180)) * time.Second
		cur, ok := r.replyTo()
		if !ok || !cur.reachable() || time.Since(r.lastMessage()) > timeout {
			r.setReplyTo(from, true)
		}
	} else {
		r.setReplyTo(from, true)
	}
	r.setLastMessage(time.Now())
	if afk {
		return sentButAfk
	}
	return sent
}

// from answers the sender of ctx as a message party.
func (g *group) from(ctx *core.Ctx) recipient {
	if ctx.User != nil {
		return g.userRecipient(ctx.User)
	}
	return recipient{g: g, cs: ctx.Sender}
}

// formatPrivate formats a private message as its sender may.
func (g *group) formatPrivate(ctx *core.Ctx, msg string) string {
	if ctx.User != nil {
		// A muted sender is refused in sendPrivate, after social spies see
		// the attempt.
		return g.formatMessage(ctx.User, "essentials.msg", msg)
	}
	return replaceFormat(msg)
}

func (g *group) registerMsg() {
	e := g.e
	e.RegisterCommand(core.Cmd{
		// Essentials: Commandmsg
		Name: "msg",
		Run: func(ctx *core.Ctx) error {
			if len(ctx.Args) < 2 {
				return core.ErrNotEnoughArgs
			}
			msg := g.formatPrivate(ctx, ctx.FinalArg(1))
			from := g.from(ctx)
			if strings.EqualFold(ctx.Args[0], "Console") || strings.EqualFold(ctx.Args[0], e.TlString("consoleName")) {
				from.sendPrivate(g.consoleRecipient(), msg)
				return nil
			}
			return g.loopOnline(ctx, false, ctx.Has("essentials.msg.multiple"), ctx.Args[0], func(u *core.User) error {
				from.sendPrivate(g.userRecipient(u), msg)
				return nil
			})
		},
		Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 {
				return append(ctx.CompletePlayers(), loopCompletions...)
			}
			return nil
		},
	}, core.Cmd{
		// Essentials: Commandr
		Name: "r",
		Run: func(ctx *core.Ctx) error {
			if len(ctx.Args) < 1 {
				return core.ErrNotEnoughArgs
			}
			msg := g.formatPrivate(ctx, ctx.FinalArg(0))
			from := g.from(ctx)
			to, ok := from.replyTo()
			if !ok || (!e.Config.Bool("last-message-reply-vanished", true) && ctx.Player != nil && to.hiddenFrom(ctx.Player)) {
				from.setReplyTo(recipient{}, false)
				return core.Tlf("foreverAlone")
			}
			from.sendPrivate(to, msg)
			return nil
		},
		Complete: func(*core.Ctx) []string { return nil },
	})

	msgToggle := toggle{g: g, others: "essentials.msgtoggle.others", set: func(ctx *core.Ctx, u *core.User, on *bool) {
		ignore := !ignoresMessages(u)
		if on != nil {
			ignore = *on
		}
		u.Session[sessionIgnoreMsg] = ignore
		key := "msgDisabled"
		if !ignore {
			key = "msgEnabled"
		}
		u.Send(key)
		if !isSelf(ctx, u) {
			ctx.Send(key+"For", u.DisplayName())
		}
	}}
	rToggle := toggle{g: g, others: "essentials.rtoggle.others", set: func(ctx *core.Ctx, u *core.User, on *bool) {
		enabled := !g.lastMessageReplyRecipient(u)
		if on != nil {
			enabled = *on
		}
		u.Data.LastMessageReplyRecipient = &enabled
		u.Save()
		key := "replyLastRecipientDisabled"
		if enabled {
			key = "replyLastRecipientEnabled"
		}
		u.Send(key)
		if !isSelf(ctx, u) {
			ctx.Send(key+"For", u.DisplayName())
		}
	}}
	socialSpy := toggle{g: g, others: "essentials.socialspy.others", set: func(ctx *core.Ctx, u *core.User, on *bool) {
		enabled := !u.Data.SocialSpy
		if on != nil {
			enabled = *on
		}
		u.Data.SocialSpy = enabled
		u.Save()
		u.Send("socialSpy", u.DisplayName(), g.enableDisable(sender(u), enabled))
		if !isSelf(ctx, u) {
			ctx.Send("socialSpy", u.DisplayName(), g.enableDisable(ctx.Sender, enabled))
		}
	}}
	e.RegisterCommand(
		// Essentials: Commandmsgtoggle
		core.Cmd{Name: "msgtoggle", Run: msgToggle.run, Complete: msgToggle.complete},
		// Essentials: Commandrtoggle
		core.Cmd{Name: "rtoggle", Run: rToggle.run, Complete: rToggle.complete},
		// Essentials: Commandsocialspy
		core.Cmd{Name: "socialspy", Run: socialSpy.run, Complete: socialSpy.complete},
	)
}
