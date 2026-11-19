package core

import (
	"slices"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/shopspring/decimal"
)

// User is a player who has played on the server, online or not: their saved
// Data and, while online, their session's state. Methods run on the tick
// goroutine. Essentials: User
type User struct {
	ess  *Essentials
	ID   plugin.UUID
	Data *UserData
	// Loaded reports whether userdata/<uuid>.yml existed.
	Loaded bool
	dirty  bool

	// Session state, reset when the player joins.
	LastActivity        time.Time
	LastMove            plugin.Location
	AfkSince            time.Time
	AfkMessage          string
	AfkPosition         plugin.Location
	Vanished            bool
	ReplyTo             plugin.UUID
	ReplyAt             time.Time
	Teleports           []*TpRequest
	TeleportRequestHere bool
	Hidden              bool
	RecipeSee           bool
	InvSee              bool
	EnderSee            bool
	ConfirmingPay       string
	ConfirmingClear     string
	pending             *pendingTeleport
	// Session is a map for command groups' own session state, cleared at
	// join.
	Session map[string]any
}

// Ess answers the Essentials the user belongs to.
func (u *User) Ess() *Essentials { return u.ess }

// Player answers the online player.
func (u *User) Player() (plugin.Player, bool) {
	p, ok := u.ess.Server.Player(u.ID)
	if !ok || !p.IsValid() {
		return nil, false
	}
	return p, true
}

// Online reports whether the user is in the game.
func (u *User) Online() bool { _, ok := u.Player(); return ok }

// Name answers the account name: the online player's, else the last seen.
func (u *User) Name() string {
	if p, ok := u.Player(); ok {
		return p.Name()
	}
	if u.Data.LastAccountName != "" {
		return u.Data.LastAccountName
	}
	if op, ok := u.ess.Server.OfflinePlayer(u.ID); ok {
		return op.Name()
	}
	return u.ID.String()
}

// Save marks the data to be written at the next save.
func (u *User) Save() { u.dirty = true }

// HasPermission decides node for the online player; an offline user has
// only what a permission provider would grant, which here is nothing.
// Essentials: isAuthorized
func (u *User) HasPermission(node string) bool {
	p, ok := u.Player()
	return ok && p.HasPermission(node)
}

// Send sends the message key to the online user. Essentials: sendTl
func (u *User) Send(key string, args ...any) {
	if p, ok := u.Player(); ok {
		u.ess.Send(p, key, args...)
	}
}

// SendText sends msg to the online user.
func (u *User) SendText(msg plugin.Text) {
	if p, ok := u.Player(); ok {
		p.SendMessage(msg)
	}
}

// Tl answers the message key in the user's locale.
func (u *User) Tl(key string, args ...any) plugin.Text {
	if p, ok := u.Player(); ok {
		return u.ess.TlFor(p, key, args...)
	}
	return u.ess.Tl(key, args...)
}

// Nick answers the display name as MiniMessage: the nickname with its
// prefix, else the name, with the operators' colour; plain when the
// nickname is off. Essentials: getNick
func (u *User) Nick() string {
	name := u.Name()
	if u.Data.Nickname != "" && u.Data.Nickname != name {
		name = u.ess.Settings.NicknamePrefix() + u.Data.Nickname
	}
	if p, ok := u.Player(); ok && p.OpLevel() > 0 {
		if c := u.ess.Settings.OpsNameColor(); c != "" && c != "none" && len(c) == 1 {
			name = "&" + c + name + "&r"
		}
	}
	return LegacyToMini(name)
}

// DisplayName answers the plain display name, for messages that format a
// name as an argument.
func (u *User) DisplayName() string { return ParseMini(u.Nick(), u.ess.theme).PlainString() }

// UpdateDisplayName applies the nickname to the player's chat and list
// names. Essentials: setDisplayNick
func (u *User) UpdateDisplayName() {
	p, ok := u.Player()
	if !ok || !u.ess.Settings.ChangeDisplayName() {
		return
	}
	p.SetDisplayName(u.ess.Mini(u.Nick()))
	if !u.ess.Settings.RealNamesOnList() {
		n := u.ess.Mini(u.Nick())
		p.SetListName(&n)
	}
}

// Money answers the balance, starting-balance for a user with none.
func (u *User) Money() decimal.Decimal {
	if u.Data.Money == nil {
		return u.ess.Settings.StartingBalance()
	}
	return *u.Data.Money
}

// SetMoney sets the balance: maxMoney above max-money, raised to min-money
// below it, and nothing at all with disable-eco. Essentials: User.setMoney
func (u *User) SetMoney(v decimal.Decimal) error {
	if u.ess.Config.Bool("disable-eco", false) {
		return nil
	}
	if v.GreaterThan(u.ess.Settings.MaxMoney()) {
		return Tlf("maxMoney")
	}
	if minimum := u.ess.Settings.MinMoney(); v.LessThan(minimum) {
		v = minimum
	}
	u.Data.Money = &v
	u.Save()
	if u.ess.Config.Bool("economy-log-update-enabled", false) {
		u.ess.Economy.Log("Update", "Set", "API", u.Name(), v, "", nil)
	}
	return nil
}

// GiveMoney adds amount, telling the user and from (nil for nobody).
// Essentials: giveMoney
func (u *User) GiveMoney(amount decimal.Decimal, from plugin.CommandSender) error {
	if amount.Sign() == 0 {
		return nil
	}
	if err := u.SetMoney(u.Money().Add(amount)); err != nil {
		return err
	}
	u.Send("addedToAccount", u.ess.Economy.Format(amount))
	if from != nil {
		u.ess.Send(from, "addedToOthersAccount", u.ess.Economy.Format(amount), u.DisplayName(), u.ess.Economy.Format(u.Money()))
	}
	return nil
}

// TakeMoney subtracts amount, telling the user. Essentials: takeMoney
func (u *User) TakeMoney(amount decimal.Decimal, from plugin.CommandSender) {
	if amount.Sign() == 0 {
		return
	}
	// A balance still above max-money after the take is refused, but the
	// messages go out anyway, as in Essentials.
	if err := u.SetMoney(u.Money().Sub(amount)); err != nil {
		u.ess.Log.Warn("Invalid call to takeMoney, total balance can't be more than the max-money limit.")
	}
	u.Send("takenFromAccount", u.ess.Economy.Format(amount))
	if from != nil {
		u.ess.Send(from, "takenFromOthersAccount", u.ess.Economy.Format(amount), u.DisplayName(), u.ess.Economy.Format(u.Money()))
	}
}

// CanAfford reports whether the balance covers amount, allowing for
// min-money; essentials.eco.loan lets it go below zero.
func (u *User) CanAfford(amount decimal.Decimal) bool {
	if amount.Sign() <= 0 {
		return true
	}
	left := u.Money().Sub(amount)
	if u.HasPermission("essentials.eco.loan") {
		return !left.LessThan(u.ess.Settings.MinMoney())
	}
	return left.Sign() >= 0
}

// HomeNames answers the user's home names, sorted.
func (u *User) HomeNames() []string {
	names := make([]string, 0, len(u.Data.Homes))
	for n := range u.Data.Homes {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// Home answers the home named name, ignoring case.
func (u *User) Home(name string) (LazyLocation, bool) {
	name = strings.ToLower(name)
	for n, l := range u.Data.Homes {
		if strings.ToLower(n) == name {
			return l, true
		}
	}
	return LazyLocation{}, false
}

// SetHome saves a home.
func (u *User) SetHome(name string, loc plugin.Location) {
	if u.Data.Homes == nil {
		u.Data.Homes = map[string]LazyLocation{}
	}
	u.Data.Homes[strings.ToLower(name)] = LazyOf(loc)
	u.Save()
}

// DelHome removes a home; false when there is none.
func (u *User) DelHome(name string) bool {
	name = strings.ToLower(name)
	for n := range u.Data.Homes {
		if strings.ToLower(n) == name {
			delete(u.Data.Homes, n)
			u.Save()
			return true
		}
	}
	return false
}

// LastLocation answers where /back returns to.
func (u *User) LastLocation() (plugin.Location, bool) {
	if u.Data.LastLocation == nil {
		return plugin.Location{}, false
	}
	return u.Data.LastLocation.Resolve(u.ess.Server)
}

// SetLastLocation saves where /back returns to.
func (u *User) SetLastLocation(loc plugin.Location) {
	if loc.World == nil {
		return
	}
	l := LazyOf(loc)
	u.Data.LastLocation = &l
	u.Save()
}

// IsAfk reports whether the user is away.
func (u *User) IsAfk() bool { return u.Data.Afk }

// SetAfk changes the away status, broadcasting it when broadcast-afk-message
// is on, and the player list name when afk-list-name is set.
// Essentials: setAfk + updateActivity
func (u *User) SetAfk(afk bool, message string) {
	if u.Data.Afk == afk {
		return
	}
	u.Data.Afk = afk
	u.Save()
	p, online := u.Player()
	if afk {
		u.AfkSince = time.Now()
		u.AfkMessage = message
		if online {
			u.AfkPosition = p.Location()
		}
	} else {
		u.AfkSince = time.Time{}
		u.AfkMessage = ""
	}
	if online {
		if ln := u.ess.Settings.AfkListName(); ln != "" && ln != "none" {
			if afk {
				n := u.ess.Mini(strings.ReplaceAll(strings.ReplaceAll(ln, "{USERNAME}", u.Name()), "{PLAYER}", u.Nick()))
				p.SetListName(&n)
			} else {
				u.UpdateDisplayName()
				if !u.ess.Settings.ChangeDisplayName() || u.ess.Settings.RealNamesOnList() {
					p.SetListName(nil)
				}
			}
		}
	}
	if u.Vanished {
		return
	}
	key := "userIsNotAway"
	selfKey := "userIsNotAwaySelf"
	if afk {
		key, selfKey = "userIsAway", "userIsAwaySelf"
		if message != "" {
			key, selfKey = "userIsAwayWithMessage", "userIsAwaySelfWithMessage"
		}
	}
	if u.ess.Settings.BroadcastAfkMessage() {
		for _, o := range u.ess.Server.Players() {
			if o.UUID() != u.ID && !u.ess.Users.Of(o).IsIgnoring(u) {
				u.ess.Send(o, key, u.DisplayName(), message)
			}
		}
	}
	u.Send(selfKey, message)
}

// UpdateActivity notes that the user did something, ending AFK unless the
// cause is off in the config. Essentials: updateActivity
func (u *User) UpdateActivity(broadcast bool) {
	u.LastActivity = time.Now()
	if u.Data.Afk && broadcast {
		u.SetAfk(false, "")
	}
}

// IsMuted reports whether the user is muted, ending an expired mute.
func (u *User) IsMuted() bool {
	if u.Data.Muted && u.Data.Timestamps.Mute > 0 && time.Now().UnixMilli() >= u.Data.Timestamps.Mute {
		u.Data.Muted = false
		u.Data.Timestamps.Mute = 0
		u.Data.MuteReason = ""
		u.Save()
		u.Send("canTalkAgain")
	}
	return u.Data.Muted
}

// SetMuted mutes or unmutes until (zero for ever) with reason.
func (u *User) SetMuted(muted bool, until time.Time, reason string) {
	u.Data.Muted = muted
	u.Data.MuteReason = reason
	u.Data.Timestamps.Mute = 0
	if muted && !until.IsZero() {
		u.Data.Timestamps.Mute = until.UnixMilli()
	}
	if !muted {
		u.Data.MuteReason = ""
	}
	u.Save()
}

// IsIgnoring reports whether the user ignores other, unless other may not
// be ignored (essentials.chat.ignoreexempt).
func (u *User) IsIgnoring(other *User) bool {
	if other == nil || other.ID == u.ID {
		return false
	}
	if other.HasPermission("essentials.chat.ignoreexempt") {
		return false
	}
	return slices.Contains(u.Data.Ignore, other.ID.String())
}

// SetIgnoring adds other to the ignore list or takes them off it.
func (u *User) SetIgnoring(other *User, ignore bool) {
	id := other.ID.String()
	u.Data.Ignore = slices.DeleteFunc(u.Data.Ignore, func(s string) bool { return s == id })
	if ignore {
		u.Data.Ignore = append(u.Data.Ignore, id)
	}
	u.Save()
}

// IsGodMode reports whether the user takes no damage; never in a world
// listed in no-god-in-worlds.
func (u *User) IsGodMode() bool {
	if !u.Data.GodMode {
		return false
	}
	if p, ok := u.Player(); ok {
		for _, w := range u.ess.Settings.NoGodWorlds() {
			if WorldMatches(p.World(), w) {
				return false
			}
		}
	}
	return true
}

// SetVanished hides the user from every player without
// essentials.vanish.see, or shows them again.
func (u *User) SetVanished(v bool) {
	u.Vanished = v
	p, ok := u.Player()
	if !ok {
		return
	}
	for _, o := range u.ess.Server.Players() {
		if o.UUID() == u.ID {
			continue
		}
		if v && !o.HasPermission("essentials.vanish.see") {
			o.Hide(p)
		} else {
			o.Show(p)
		}
	}
	if v {
		p.AddEffect(plugin.EffectInstance{Effect: u.ess.invisibility, Duration: -1, Particles: false})
	} else {
		p.RemoveEffect(u.ess.invisibility)
	}
}

// CanSee reports whether the user may see other: other is not vanished, or
// the user has essentials.vanish.see.
func (u *User) CanSee(other *User) bool {
	if other == nil || !other.Vanished || other.ID == u.ID {
		return true
	}
	return u.HasPermission("essentials.vanish.see")
}

// UnreadMail answers the unread, unexpired mails.
func (u *User) UnreadMail() []MailMessage {
	now := time.Now().UnixMilli()
	var out []MailMessage
	for _, m := range u.Data.Mail {
		if !m.Read && (m.Expire == 0 || m.Expire > now) {
			out = append(out, m)
		}
	}
	return out
}

// KitTimestamp answers when the user last took kit, Unix milliseconds.
func (u *User) KitTimestamp(kit string) int64 {
	return u.Data.Timestamps.Kits[strings.ToLower(kit)]
}

// SetKitTimestamp records when the user took kit.
func (u *User) SetKitTimestamp(kit string, t int64) {
	if u.Data.Timestamps.Kits == nil {
		u.Data.Timestamps.Kits = map[string]int64{}
	}
	u.Data.Timestamps.Kits[strings.ToLower(kit)] = t
	u.Save()
}

// SessionValue answers a command group's session state.
func SessionValue[T any](u *User, key string) (T, bool) {
	v, ok := u.Session[key].(T)
	return v, ok
}

// PlayTime answers the user's play_time statistic in ticks, online or off;
// 0 when it cannot be read. Essentials: getStatistic(PLAY_ONE_TICK)
func (u *User) PlayTime() int {
	st, ok := u.ess.Server.Registries().Statistic(plugin.MinecraftKey("play_time"))
	if !ok {
		return 0
	}
	if p, ok := u.Player(); ok {
		return p.Statistic(st)
	}
	if op, ok := u.ess.Server.OfflinePlayer(u.ID); ok {
		return op.Statistic(st)
	}
	return 0
}
