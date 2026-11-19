// Package chat is EssentialsXChat, ported to golem: chat formats per
// group and chat type, local chat with a radius, shouts and questions, spies
// on local chat, colour permissions and /toggleshout.
package chat

import (
	_ "embed"
	"errors"
	"strings"
	"sync"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

//go:embed plugin.yml
var pluginYML []byte

// Chat is the essentialschat plugin. Essentials: EssentialsChat
type Chat struct {
	ess *core.Essentials
	mu  sync.Mutex
	// chats are the messages being processed, by sender: the format stage
	// sets them up, the submit stage clears them. Essentials:
	// ChatProcessingCache
	chats map[plugin.UUID]*processedChat
}

func init() { plugin.Register("essentialschat", &Chat{}, plugin.Depends("essentials")) }

// Enable registers /toggleshout and the chat listeners.
func (m *Chat) Enable(s plugin.Server) error {
	e, ok := core.Get()
	if !ok {
		return errors.New("essentials is not enabled")
	}
	m.ess = e
	m.chats = map[plugin.UUID]*processedChat{}
	if err := e.RegisterModulePermissions(s, pluginYML); err != nil {
		return err
	}
	if err := e.RegisterModuleCommands(s, pluginYML, m.toggleshoutCmd()); err != nil {
		return err
	}
	// Essentials: ChatHandler.registerListeners
	plugin.Listen(s, m.handleFormat, plugin.ListenOptions{Priority: plugin.Lowest})
	plugin.Listen(s, m.handleRecipients, plugin.ListenOptions{Priority: plugin.Normal})
	plugin.Listen(s, func(ev *plugin.PlayerChatEvent) {
		m.handlePostFormat(ev)
		m.handleSubmit(ev)
	}, plugin.ListenOptions{Priority: plugin.Highest})
	return nil
}

// Disable does nothing.
func (m *Chat) Disable() {}

// chatType is a kind of chat message. Essentials: ChatType
type chatType int

const (
	typeShout chatType = iota
	typeQuestion
	typeLocal
	typeSpy
	typeUnknown
)

// key answers the type's config and message key.
func (t chatType) key() string {
	return [...]string{"shout", "question", "local", "spy", "normal"}[t]
}

// processedChat is one message between the format and submit stages.
// Essentials: ChatProcessingCache.ProcessedChat
type processedChat struct {
	user   *core.User
	typ    chatType
	radius int
	charge *core.Trade
	// format is the line as MiniMessage, with %1$s for the display name and
	// %2$s for the message.
	format string
}

// longType names the message's command cost: chat, chat-shout or
// chat-question. Essentials: Chat.getLongType
func (c *processedChat) longType() string {
	if c.typ == typeUnknown {
		return "chat"
	}
	return "chat-" + c.typ.key()
}

func (m *Chat) cached(id plugin.UUID) *processedChat {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.chats[id]
}

func (m *Chat) setCached(id plugin.UUID, c *processedChat) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c == nil {
		delete(m.chats, id)
		return
	}
	m.chats[id] = c
}

// send sends the message key unless it is blank, as Essentials' sendTl
// skips empty translations (localNoOne is empty by default).
func send(e *core.Essentials, u *core.User, key string, args ...any) {
	if strings.TrimSpace(e.I18n.Format("", key, args...)) == "" {
		return
	}
	u.Send(key, args...)
}

// handleFormat colours the message as the sender may, picks the chat type
// and builds the format the recipients see. Essentials:
// AbstractChatHandler.handleChatFormat
func (m *Chat) handleFormat(ev *plugin.PlayerChatEvent) {
	if ev.Cancelled() {
		return
	}
	e := m.ess
	p := ev.Player
	u := e.Users.Of(p)
	u.UpdateDisplayName()

	c := m.cached(p.UUID())
	if c == nil {
		c = &processedChat{user: u, typ: m.chatTypeOf(u, ev.Message)}
		c.charge = m.chargeFor(u, c.longType())
		m.setCached(p.UUID(), c)
	}
	c.radius = max(chatRadius(e.Config), 0)

	ev.Message = formatMessage(u, "essentials.chat", ev.Message)
	if stripCodes(ev.Message) == "" {
		ev.SetCancelled(true)
		return
	}

	group := groupOf(u)
	world := core.WorldName(p.World())
	username := u.Name()
	nickname := username
	if n := u.Data.Nickname; n != "" {
		nickname = e.Settings.NicknamePrefix() + n
	}
	var teamPrefix, teamSuffix, teamName string
	if team, ok := p.Scoreboard().TeamOf(p.Name()); ok {
		teamPrefix, teamSuffix, teamName = team.Prefix().PlainString(), team.Suffix().PlainString(), team.Display().PlainString()
	}
	short := ""
	if world != "" {
		short = strings.ToUpper(world[:1])
	}

	typ := c.typ
	if c.radius > 0 && typ == typeUnknown {
		typ = typeLocal
	}
	format := strings.NewReplacer(
		"{0}", core.EscapeTags(group),
		"{1}", core.EscapeTags(worldAlias(e.Config, world)),
		"{2}", core.EscapeTags(short),
		"{3}", core.EscapeTags(teamPrefix),
		"{4}", core.EscapeTags(teamSuffix),
		"{5}", core.EscapeTags(teamName),
		"{6}", "", // prefix and suffix come from a permissions plugin
		"{7}", "",
		"{8}", core.EscapeTags(username),
		"{9}", core.EscapeTags(nickname),
	).Replace(chatFormat(e.Config, group, typ))
	format = core.LegacyToMini(format)

	if c.radius > 0 && ev.Message != "" {
		if len(ev.Message) > 1 && ((c.typ == typeShout && ev.Message[0] == shoutChar(e.Config)) ||
			(c.typ == typeQuestion && ev.Message[0] == questionChar(e.Config))) {
			ev.Message = strings.TrimSpace(ev.Message[1:])
		}
		if stripCodes(ev.Message) == "" {
			ev.SetCancelled(true)
			return
		}
		if c.typ == typeUnknown {
			format = e.I18n.Format("", "chatTypeLocal") + format
		} else {
			format = e.I18n.Format("", c.typ.key()+"Format", core.Raw(format))
		}
	}
	c.format = format
	theme := e.Theme()
	ev.Format = func(sender plugin.Player, message plugin.Text, _ plugin.Player) plugin.Text {
		return core.ParseMini(fill(format, u.Nick(), messageMini(message.PlainString())), theme)
	}
}

// fill puts the display name and the message into a format.
func fill(format, name, message string) string {
	return strings.NewReplacer("%1$s", name, "%2$s", message).Replace(format)
}

// handleRecipients keeps local chat within the radius, checks the
// permissions to speak and hear each chat type, and shows local chat to
// spies. Essentials: AbstractChatHandler.handleChatRecipients
func (m *Chat) handleRecipients(ev *plugin.PlayerChatEvent) {
	if ev.Cancelled() {
		return
	}
	e := m.ess
	c := m.cached(ev.Player.UUID())
	if c == nil || c.radius < 1 {
		return
	}
	u := c.user
	keep := func(perm string) {
		out := ev.Recipients[:0]
		for _, r := range ev.Recipients {
			if r.HasPermission(perm) {
				out = append(out, r)
			}
		}
		ev.Recipients = out
	}
	if ev.Message != "" {
		if c.typ == typeUnknown {
			if !u.HasPermission("essentials.chat.local") {
				u.Send("notAllowedToLocal")
				ev.SetCancelled(true)
				return
			}
			keep("essentials.chat.receive.local")
		} else {
			key := c.typ.key()
			if u.HasPermission("essentials.chat." + key) {
				keep("essentials.chat.receive." + key)
			} else {
				u.Send("notAllowedTo" + strings.ToUpper(key[:1]) + key[1:])
				ev.SetCancelled(true)
			}
			return
		}
	}

	loc := ev.Player.Location()
	radiusSq := float64(c.radius) * float64(c.radius)
	var spies []plugin.Player
	out := ev.Recipients[:0]
	for _, r := range ev.Recipients {
		if r.UUID() != ev.Player.UUID() {
			rl := r.Location()
			if rl.World != loc.World || rl.Pos.DistanceSq(loc.Pos) > radiusSq {
				if r.HasPermission("essentials.chat.spy") {
					spies = append(spies, r)
				}
				continue
			}
		}
		out = append(out, r)
	}
	ev.Recipients = out

	if len(ev.Recipients) < 2 {
		send(e, u, "localNoOne")
	}

	// Essentials: LocalChatSpyEvent, whose format leads with chatTypeLocal
	// and chatTypeSpy before the format without its local prefix.
	base := strings.TrimPrefix(c.format, e.I18n.Format("", "chatTypeLocal"))
	spyFormat := e.I18n.Format("", "chatTypeLocal") + e.I18n.Format("", "chatTypeSpy") + base
	line := core.ParseMini(fill(spyFormat, u.Nick(), messageMini(ev.Message)), e.Theme())
	for _, s := range spies {
		s.SendMessage(line)
	}
}

// handlePostFormat forgets an aborted message. Essentials:
// AbstractChatHandler.handleChatPostFormat
func (m *Chat) handlePostFormat(ev *plugin.PlayerChatEvent) {
	if ev.Cancelled() {
		m.setCached(ev.Player.UUID(), nil)
	}
}

// handleSubmit charges the message's cost and forgets it. Essentials:
// AbstractChatHandler.handleChatSubmit
func (m *Chat) handleSubmit(ev *plugin.PlayerChatEvent) {
	if ev.Cancelled() {
		return
	}
	c := m.cached(ev.Player.UUID())
	if c == nil {
		return
	}
	if err := c.charge.Charge(c.user); err != nil {
		m.ess.ShowError(&core.Ctx{Ess: m.ess, Sender: ev.Player}, err)
		ev.SetCancelled(true)
	}
	m.setCached(ev.Player.UUID(), nil)
}

// chargeFor answers the cost of a chat type from command-costs (chat-shout,
// chat-question), nil when it is free or the user is exempt.
// Essentials: Trade(String command, IEssentials)
func (m *Chat) chargeFor(u *core.User, name string) *core.Trade {
	cost := m.ess.Settings.CommandCost(name)
	if cost.Sign() == 0 || u.HasPermission("essentials.nocommandcost.all") || u.HasPermission("essentials.nocommandcost."+name) {
		return nil
	}
	return core.MoneyTrade(cost)
}

// chatTypeOf answers a message's type from its first character and the
// user's shout mode. Essentials: AbstractChatHandler.getChatType
func (m *Chat) chatTypeOf(u *core.User, msg string) chatType {
	if msg == "" {
		return typeUnknown
	}
	c := m.ess.Config
	if len(msg) == 1 {
		if isToggleShout(u) {
			return typeShout
		}
		return typeUnknown
	}
	switch {
	case msg[0] == questionChar(c) && c.Bool("chat.question-enabled", true):
		return typeQuestion
	case msg[0] == shoutChar(c) || isToggleShout(u):
		return typeShout
	}
	return typeUnknown
}

// groupOf answers the user's permission group: "default" without a
// permissions plugin. Essentials: User.getGroup
func groupOf(*core.User) string { return "default" }
