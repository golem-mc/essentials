package messaging

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"gopkg.in/yaml.v3"
)

// seen shows when a player was last online and what is known of them;
// under the alts labels, the accounts that share their address.
func (g *group) seen(ctx *core.Ctx) error {
	if len(ctx.Args) < 1 {
		return core.ErrNotEnoughArgs
	}
	e := g.e
	term := ctx.Args[0]
	opts := seenOpts{
		ban:       ctx.Has("essentials.seen.banreason"),
		ip:        ctx.Has("essentials.seen.ip"),
		location:  ctx.Has("essentials.seen.location"),
		whitelist: ctx.Has("essentials.seen.whitelist"),
	}
	alts := strings.Contains(ctx.Label, "alts") && ctx.Has("essentials.seen.alts")
	var u *core.User
	if id, err := parseUUID(term); err == nil {
		if e.Users.Known(id) {
			u = e.Users.Get(id)
		}
	} else if found, ok := e.Users.ByName(term); ok {
		u = found
	}
	if u == nil {
		if !alts {
			_, ipBanned := e.Server.IPBans().Entry(term)
			if ctx.Has("essentials.seen.ipsearch") && validIP(term) {
				if ipBanned {
					ctx.Send("isIpBanned", term)
				}
				g.seenIP(ctx, term, term)
				return nil
			}
			if ipBanned {
				ctx.Send("isIpBanned", term)
				return nil
			}
			if ban, ok := g.banByName(term); ok {
				var reason any = g.trueFalse(ctx.Sender, true)
				if opts.ban {
					reason = ban.Reason
				}
				ctx.Send("whoisBanned", reason)
				return nil
			}
		}
		found, err := g.getPlayer(ctx, term, false)
		if err != nil {
			return core.Tlf("playerNeverOnServer", term)
		}
		u = found
	}
	switch {
	case alts:
		g.seenIP(ctx, u.Data.IPAddress, u.DisplayName())
	case u.Online() && g.canInteractWith(ctx, u):
		g.seenOnline(ctx, u, opts.ip)
	default:
		g.seenOffline(ctx, u, opts)
	}
	return nil
}

// banByName answers the ban of the player who last had name: by the UUID
// usercache.json gives the name, else by the name the ban list holds.
// Essentials: BanLookup.getBanEntry
func (g *group) banByName(name string) (plugin.Ban, bool) {
	bans := g.e.Server.Bans()
	if op, ok := g.e.Server.OfflinePlayerNamed(name); ok {
		if ban, ok := bans.Entry(op.UUID().String()); ok {
			return ban, true
		}
	}
	return bans.Entry(name)
}

type seenOpts struct{ ban, ip, location, whitelist bool }

// canInteractWith reports whether the sender may see u's online status.
// Essentials: EssentialsCommand.canInteractWith
func (g *group) canInteractWith(ctx *core.Ctx, u *core.User) bool {
	if ctx.User == nil {
		return true
	}
	if isHidden(u) || hiddenFrom(u, ctx.Player) {
		return canInteractVanished(ctx.User)
	}
	return true
}

// joinSkip joins list with ", ", leaving out skip. Essentials:
// StringUtil.joinListSkip
func joinSkip(skip string, list []string) string {
	var out []string
	for _, s := range list {
		if !strings.EqualFold(s, skip) {
			out = append(out, s)
		}
	}
	return strings.Join(out, ", ")
}

// formatDate writes a time in full. Essentials: DateUtil.formatDate
func formatDate(t time.Time) string { return t.Format("January 2, 2006 3:04:05 PM MST") }

func (g *group) sendMuted(ctx *core.Ctx, u *core.User) {
	if !u.IsMuted() {
		return
	}
	var state any = g.trueFalse(ctx.Sender, true)
	if u.Data.Timestamps.Mute > 0 {
		state = core.FormatDateDiff(time.UnixMilli(u.Data.Timestamps.Mute))
	}
	if u.Data.MuteReason == "" {
		ctx.Send("whoisMuted", state)
	} else {
		ctx.Send("whoisMutedReason", state, u.Data.MuteReason)
	}
}

func (g *group) sendGeo(ctx *core.Ctx, u *core.User) {
	if u.Data.Geolocation != "" && ctx.Has("essentials.geoip.show") {
		ctx.Send("whoisGeoLocation", u.Data.Geolocation)
	}
}

// seenOnline is /seen for an online player. Essentials: Commandseen.seenOnline
func (g *group) seenOnline(ctx *core.Ctx, u *core.User, showIP bool) {
	u.UpdateDisplayName()
	p, _ := u.Player()
	ctx.Send("seenOnline", u.DisplayName(), core.FormatDateDiff(time.UnixMilli(u.Data.Timestamps.Login)))
	if len(u.Data.PastUsernames) > 0 {
		ctx.Send("seenAccounts", joinSkip(u.Name(), u.Data.PastUsernames))
	}
	if ctx.Has("essentials.seen.uuid") {
		ctx.Send("whoisUuid", u.ID.String())
	}
	if ctx.Has("essentials.seen.firstlogin") {
		ctx.Send("whoisFirstLogin", formatDate(p.FirstPlayed()))
	}
	if u.IsAfk() {
		ctx.Send("whoisAFK", g.trueFalse(ctx.Sender, true))
	}
	if u.Data.Jailed {
		var state any = g.trueFalse(ctx.Sender, true)
		if u.Data.Timestamps.Jail > 0 {
			state = core.FormatDateDiff(time.UnixMilli(u.Data.Timestamps.Jail))
		}
		ctx.Send("whoisJail", state)
	}
	g.sendMuted(ctx, u)
	g.sendGeo(ctx, u)
	if showIP {
		ctx.Send("whoisIPAddress", "/"+core.HostOf(p))
	}
}

// seenOffline is /seen for an offline (or hidden) player. Essentials:
// Commandseen.seenOffline
func (g *group) seenOffline(ctx *core.Ctx, u *core.User, opts seenOpts) {
	e := g.e
	u.UpdateDisplayName()
	if u.Data.Timestamps.Logout > 0 {
		ctx.Send("seenOffline", u.Name(), core.FormatDateDiff(time.UnixMilli(u.Data.Timestamps.Logout)))
		if len(u.Data.PastUsernames) > 1 {
			ctx.Send("seenAccounts", joinSkip(u.Name(), u.Data.PastUsernames))
		}
		if ctx.Has("essentials.seen.uuid") {
			ctx.Send("whoisUuid", u.ID.String())
		}
	} else {
		ctx.Send("userUnknown", u.Name())
	}
	if opts.whitelist {
		ctx.Send("whoisWhitelist", g.trueFalse(ctx.Sender, e.Server.Whitelist().Contains(u.ID)))
	}
	if ban, ok := e.Server.Bans().Entry(u.ID.String()); ok {
		var reason any = g.trueFalse(ctx.Sender, true)
		if opts.ban {
			reason = ban.Reason
		}
		ctx.Send("whoisBanned", reason)
		if !ban.Expires.IsZero() {
			var expires any = g.tl(ctx.Sender, "now")
			if ban.Expires.After(time.Now()) {
				expires = core.FormatDateDiff(ban.Expires)
			}
			ctx.Send("whoisTempBanned", expires)
		}
	}
	g.sendMuted(ctx, u)
	g.sendGeo(ctx, u)
	if opts.ip && u.Data.IPAddress != "" {
		ctx.Send("whoisIPAddress", u.Data.IPAddress)
	}
	if opts.location && u.Data.LogoutLocation != nil {
		l := *u.Data.LogoutLocation
		world := l.WorldName
		if loc, ok := l.Resolve(e.Server); ok {
			world = core.WorldName(loc.World)
		}
		b := plugin.Vec3{X: l.X, Y: l.Y, Z: l.Z}.Block()
		ctx.Send("whoisLocation", world, b.X, b.Y, b.Z)
	}
}

// seenIP lists the users who last logged in from ip. Essentials:
// Commandseen.seenIP
func (g *group) seenIP(ctx *core.Ctx, ip, display string) {
	ctx.Send("runningPlayerMatch", core.Raw(sectionMini(display)))
	var matches []string
	for _, id := range g.e.Users.All() {
		u := g.e.Users.Get(id)
		if u.Data.IPAddress != "" && strings.EqualFold(u.Data.IPAddress, ip) {
			matches = append(matches, u.Name())
		}
	}
	if len(matches) == 0 {
		ctx.Send("noMatchingPlayers")
		return
	}
	ctx.Send("matchingIPAddress")
	ctx.Send("matchingAccounts", strings.Join(matches, ", "))
}

// number writes a number as Java's MessageFormat does: at most three
// decimals, without trailing zeros.
func number(f float64) string {
	s := strconv.FormatFloat(f, 'f', 3, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "-0" {
		return "0"
	}
	return s
}

// whois shows what is known of an online player.
func (g *group) whois(ctx *core.Ctx) error {
	if len(ctx.Args) < 1 {
		return core.ErrNotEnoughArgs
	}
	u, err := g.getPlayer(ctx, ctx.Args[0], false)
	if err != nil {
		return err
	}
	p, ok := u.Player()
	if !ok {
		return core.ErrPlayerNotFound
	}
	e := g.e
	s := ctx.Sender
	ctx.Send("whoisTop", u.Name())
	u.UpdateDisplayName()
	ctx.Send("whoisNick", u.DisplayName())
	ctx.Send("whoisUuid", u.ID.String())
	ctx.Send("whoisHealth", number(float64(p.Health())))
	ctx.Send("whoisHunger", p.Food(), number(float64(p.Saturation())))
	ctx.Send("whoisExp", core.TotalExp(p), p.Level())
	b := p.Pos().Block()
	ctx.Send("whoisLocation", core.WorldName(p.World()), b.X, b.Y, b.Z)
	played := time.Duration(u.PlayTime()) * 50 * time.Millisecond
	ctx.Send("whoisPlaytime", core.FormatDateDiff(time.Now().Add(-played)))
	if !e.Config.Bool("disable-eco", false) {
		ctx.Send("whoisMoney", e.Economy.Format(u.Money()))
	}
	if ctx.Has("essentials.whois.ip") {
		ctx.Send("whoisIPAddress", "/"+core.HostOf(p))
	}
	g.sendGeo(ctx, u)
	ctx.Send("whoisGamemode", g.tl(s, gameModeKey(p.GameMode())))
	ctx.Send("whoisGod", g.trueFalse(s, u.IsGodMode()))
	ctx.Send("whoisOp", g.trueFalse(s, p.OpLevel() > 0))
	flying := g.tl(s, "notFlying")
	if p.Flying() {
		flying = g.tl(s, "flying")
	}
	ctx.Send("whoisFly", g.trueFalse(s, p.AllowFlight()), flying)
	speed := p.WalkSpeed()
	if p.Flying() {
		speed = p.FlySpeed()
	}
	ctx.Send("whoisSpeed", number(float64(speed)))
	ctx.Send("whoisWhitelist", g.trueFalse(s, e.Server.Whitelist().Contains(u.ID)))
	if u.IsAfk() {
		ctx.Send("whoisAFKSince", g.trueFalse(s, true), core.FormatDateDiff(u.AfkSince))
	} else {
		ctx.Send("whoisAFK", g.trueFalse(s, false))
	}
	jail := g.tl(s, "false")
	if u.Data.Jailed {
		jail = g.tl(s, "true")
		if u.Data.Timestamps.Jail > 0 {
			jail = core.Raw(core.EscapeTags(core.FormatDateDiff(time.UnixMilli(u.Data.Timestamps.Jail))))
		}
	}
	ctx.Send("whoisJail", jail)
	muted := g.tl(s, "false")
	if u.IsMuted() {
		muted = g.tl(s, "true")
		if u.Data.Timestamps.Mute > 0 {
			muted = core.Raw(core.EscapeTags(core.FormatDateDiff(time.UnixMilli(u.Data.Timestamps.Mute))))
		}
	}
	if u.Data.MuteReason == "" {
		ctx.Send("whoisMuted", muted)
	} else {
		ctx.Send("whoisMutedReason", muted, u.Data.MuteReason)
	}
	return nil
}

// gameModeKey answers the message key naming a game mode.
func gameModeKey(m plugin.GameMode) string {
	switch m {
	case plugin.Creative:
		return "creative"
	case plugin.Adventure:
		return "adventure"
	case plugin.Spectator:
		return "spectator"
	}
	return "survival"
}

// yamlEntry is a key and scalar value of a YAML mapping.
type yamlEntry struct{ key, value string }

// orderedSection answers the entries of the mapping at key in the YAML
// file path, in file order, which Config's maps lose.
func orderedSection(path, key string) ([]yamlEntry, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return orderedBytes(b, key)
}

func orderedBytes(b []byte, key string) ([]yamlEntry, bool) {
	var doc yaml.Node
	if yaml.Unmarshal(b, &doc) != nil || len(doc.Content) == 0 {
		return nil, false
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != key {
			continue
		}
		m := root.Content[i+1]
		if m.Kind != yaml.MappingNode {
			return nil, true
		}
		var out []yamlEntry
		for j := 0; j+1 < len(m.Content); j += 2 {
			out = append(out, yamlEntry{m.Content[j].Value, m.Content[j+1].Value})
		}
		return out, true
	}
	return nil, false
}
