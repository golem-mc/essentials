package messaging

import (
	"errors"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

func (g *group) registerPlayers() {
	e := g.e
	players := func(ctx *core.Ctx) []string {
		if len(ctx.Args) == 1 {
			return ctx.CompletePlayers()
		}
		return nil
	}
	e.RegisterCommand(core.Cmd{
		// Essentials: Commandrealname
		Name: "realname",
		Run:  g.realname,
	}, core.Cmd{
		// Essentials: Commandnick
		Name: "nick",
		Run:  g.nick,
		Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 && ctx.Has("essentials.nick.others") {
				return ctx.CompletePlayers()
			}
			return nil
		},
	}, core.Cmd{
		// Essentials: Commandlist
		Name: "list",
		Run:  g.list,
		Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) != 1 {
				return nil
			}
			var out []string
			for k := range g.playerLists(ctx.User, false) {
				out = append(out, k)
			}
			return out
		},
	}, core.Cmd{
		// Essentials: Commandnear
		Name: "near",
		Run:  g.near,
		Complete: func(ctx *core.Ctx) []string {
			radius := []string{strconv.Itoa(e.Settings.NearRadius())}
			if ctx.User == nil || ctx.Has("essentials.near.others") {
				switch len(ctx.Args) {
				case 1:
					return ctx.CompletePlayers()
				case 2:
					return radius
				}
				return nil
			}
			if len(ctx.Args) == 1 {
				return radius
			}
			return nil
		},
	}, core.Cmd{
		// Essentials: Commandseen
		Name:     "seen",
		Run:      g.seen,
		Complete: players,
	}, core.Cmd{
		// Essentials: Commandwhois
		Name:     "whois",
		Run:      g.whois,
		Complete: players,
	}, core.Cmd{
		// Essentials: Commandping
		Name: "ping",
		Run: func(ctx *core.Ctx) error {
			if len(ctx.Args) == 0 {
				ctx.Send("pong")
			} else {
				ctx.SendText(g.legacyText(replaceFormat(ctx.FinalArg(0))))
			}
			return nil
		},
	}, core.Cmd{
		// Essentials: Commandplaytime
		Name: "playtime",
		Run:  g.playtime,
		Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 && ctx.Has("essentials.playtime.others") {
				return ctx.CompletePlayers()
			}
			return nil
		},
	})
}

// realname finds the players whose display name contains the term.
func (g *group) realname(ctx *core.Ctx) error {
	if len(ctx.Args) == 0 {
		return core.ErrNotEnoughArgs
	}
	lookup := strings.ToLower(ctx.Args[0])
	found := false
	for _, u := range g.e.OnlineUsers() {
		if skipHidden(ctx, u) {
			continue
		}
		u.UpdateDisplayName()
		if strings.Contains(strings.ToLower(stripFormat(u.DisplayName())), lookup) {
			found = true
			ctx.Send("realName", u.DisplayName(), u.Name())
		}
	}
	if !found {
		return core.ErrPlayerNotFound
	}
	return nil
}

// errNickInUse is nickInUse shown after the usage, as Essentials' message
// carrying NotEnoughArgumentsException does.
var errNickInUse = errors.New("nickInUse")

// nick sets or clears a nickname, the sender's or, with
// essentials.nick.others, other players'.
func (g *group) nick(ctx *core.Ctx) error {
	var err error
	if ctx.User == nil || len(ctx.Args) > 1 && ctx.Has("essentials.nick.others") {
		if len(ctx.Args) < 2 {
			return core.ErrNotEnoughArgs
		}
		var nick string
		if nick, err = g.formatNickname(ctx.User, ctx.Args[1]); err != nil {
			return err
		}
		err = g.loopOffline(ctx, true, ctx.Args[0], func(u *core.User) error { return g.setNick(ctx, u, nick) })
		if err == nil {
			ctx.Send("nickChanged")
		}
	} else {
		if len(ctx.Args) < 1 {
			return core.ErrNotEnoughArgs
		}
		var nick string
		if nick, err = g.formatNickname(ctx.User, ctx.Args[0]); err != nil {
			return err
		}
		err = g.setNick(ctx, ctx.User, nick)
	}
	if errors.Is(err, errNickInUse) {
		g.e.ShowUsage(ctx)
		ctx.Send("nickInUse")
		return core.ErrQuiet
	}
	return err
}

// setNick applies a formatted nickname to target. Essentials:
// Commandnick.updatePlayer
func (g *group) setNick(ctx *core.Ctx, target *core.User, nick string) error {
	nick, _, _ = strings.Cut(nick, " ")
	shown := func() string {
		if g.e.Settings.ChangeDisplayName() {
			return target.DisplayName()
		}
		return nick
	}
	switch {
	case strings.EqualFold(nick, "off"):
		g.applyNick(target, "")
		target.Send("nickNoMore")
	case strings.EqualFold(target.Name(), nick):
		g.applyNick(target, nick)
		if !strings.EqualFold(stripFormat(target.DisplayName()), target.Name()) {
			target.Send("nickNoMore")
		}
		target.Send("nickSet", shown())
	case g.nickInUse(target, nick):
		return errNickInUse
	default:
		g.applyNick(target, nick)
		target.Send("nickSet", shown())
	}
	return nil
}

func (g *group) applyNick(u *core.User, nick string) {
	u.Data.Nickname = nick
	u.Save()
	u.UpdateDisplayName()
}

// formatNickname applies the codes u may use to nick and checks it against
// the nickname rules; u is nil for the console. Essentials:
// Commandnick.formatNickname
func (g *group) formatNickname(u *core.User, nick string) (string, error) {
	var n string
	if u == nil {
		n = replaceFormat(nick)
	} else {
		n = g.formatString(u, "essentials.nick", nick)
	}
	re, err := regexp.Compile("^(?:" + g.e.Config.String("allowed-nicks-regex", "^[a-zA-Z_0-9§]+$") + ")$")
	switch {
	case err == nil && !re.MatchString(n) && u != nil && !u.HasPermission("essentials.nick.allowunsafe"):
		return "", core.Tlf("nickNamesAlpha")
	case g.nickLength(n) > g.e.Settings.MaxNickLength():
		return "", core.Tlf("nickTooLong")
	case len([]rune(stripFormat(n))) < 1:
		return "", core.Tlf("nickNamesAlpha")
	case u != nil && u.HasPermission("essentials.nick.changecolors") && !u.HasPermission("essentials.nick.changecolors.bypass") &&
		stripFormat(n) != u.Name() && !strings.EqualFold(nick, "off"):
		return "", core.Tlf("nickNamesOnlyColorChanges")
	case u != nil && !u.HasPermission("essentials.nick.blacklist.bypass") && g.nickBanned(n):
		return "", core.Tlf("nickNameBlacklist", nick)
	}
	return n, nil
}

// nickBanned reports whether a nick-blacklist pattern is found in nick.
func (g *group) nickBanned(nick string) bool {
	for _, p := range g.e.Config.StringList("nick-blacklist") {
		re, err := regexp.Compile(p)
		if err != nil {
			g.e.Log.Warn("Invalid nickname blacklist regex: " + p)
			continue
		}
		if re.MatchString(nick) {
			return true
		}
	}
	return false
}

func (g *group) nickLength(nick string) int {
	if g.e.Config.Bool("ignore-colors-in-max-nick-length", false) {
		return len([]rune(stripFormat(nick)))
	}
	return len([]rune(unformatString(nick)))
}

// nickInUse reports whether another player has nick as name or nickname.
func (g *group) nickInUse(target *core.User, nick string) bool {
	low := stripFormat(strings.ToLower(nick))
	for _, o := range g.e.OnlineUsers() {
		if o.ID == target.ID {
			continue
		}
		if n := stripFormat(o.Data.Nickname); n != "" && strings.EqualFold(n, low) || strings.EqualFold(o.Name(), low) {
			return true
		}
	}
	u, ok := g.e.Users.ByName(low)
	return ok && u.ID != target.ID
}

// defaultGroup is the group every player is in: golem has no
// permission groups.
const defaultGroup = "default"

// playerLists answers the online players by group. Essentials:
// PlayerList.getPlayerLists
func (g *group) playerLists(viewer *core.User, showHidden bool) map[string][]*core.User {
	out := map[string][]*core.User{}
	var vp plugin.Player
	if viewer != nil {
		vp, _ = viewer.Player()
	}
	for _, u := range g.e.OnlineUsers() {
		if viewer == nil && !showHidden && isHidden(u) || viewer != nil && !showHidden && hiddenFrom(u, vp) {
			continue
		}
		out[defaultGroup] = append(out[defaultGroup], u)
	}
	return out
}

// listGroups answers config.yml's list section in file order.
func (g *group) listGroups() []yamlEntry {
	if entries, ok := orderedSection(g.e.Config.Path, "list"); ok {
		return entries
	}
	entries, _ := orderedBytes(core.Resource("config.yml"), "list")
	return entries
}

// listUsers writes users for /list. Essentials: PlayerList.listUsers
func (g *group) listUsers(loc string, users []*core.User) string {
	users = append([]*core.User(nil), users...)
	sort.SliceStable(users, func(i, j int) bool {
		return strings.ToLower(stripFormat(users[i].DisplayName())) < strings.ToLower(stripFormat(users[j].DisplayName()))
	})
	var b strings.Builder
	for i, u := range users {
		if i > 0 {
			b.WriteString(", ")
		}
		if u.IsAfk() {
			b.WriteString(g.e.I18n.Format(loc, "listAfkTag"))
		}
		if isHidden(u) {
			b.WriteString(g.e.I18n.Format(loc, "listHiddenTag"))
		}
		u.UpdateDisplayName()
		b.WriteString(u.Nick())
		if n := stripFormat(u.Data.Nickname); g.e.Settings.RealNamesOnList() && n != "" && n != u.Name() {
			b.WriteString(" " + g.e.I18n.Format(loc, "listRealName", u.Name()))
		}
		b.WriteString("<white>")
	}
	return b.String()
}

func (g *group) outputFormat(loc, group, msg string) string {
	return g.outputTag(loc, replaceFormat(group), msg)
}

// outputTag is outputFormat for a group name that is already a message,
// such as connectedPlayers: Essentials renders it before the tag wraps it,
// so its own tags colour it rather than show as text.
func (g *group) outputTag(loc string, group any, msg string) string {
	return g.e.I18n.Format(loc, "listGroupTag", group) + msg
}

// mergedList takes the users of the groups a list entry merges out of
// lists. Essentials: PlayerList.getMergedList
func (g *group) mergedList(lists map[string][]*core.User, name string) []*core.User {
	var users []*core.User
	for _, ent := range g.listGroups() {
		if !strings.EqualFold(ent.key, name) {
			continue
		}
		for _, v := range strings.Split(strings.TrimSpace(ent.value), " ") {
			v = strings.ToLower(strings.TrimSpace(v))
			if v == "" || len(lists[v]) == 0 {
				continue
			}
			users = append(users, lists[v]...)
			delete(lists, v)
		}
	}
	return users
}

// list shows who is online, by group.
func (g *group) list(ctx *core.Ctx) error {
	showHidden := true
	if ctx.User != nil {
		showHidden = ctx.Has("essentials.list.hidden") || canInteractVanished(ctx.User)
	}
	loc := g.locale(ctx.Sender)
	ctx.SendText(core.ParseMini(g.listSummary(ctx, showHidden), g.e.Theme()))
	lists := g.playerLists(ctx.User, showHidden)
	if len(ctx.Args) > 0 {
		name := strings.ToLower(ctx.Args[0])
		users := append(g.mergedList(lists, name), lists[name]...)
		if len(users) == 0 {
			return core.Tlf("groupDoesNotExist")
		}
		display := strings.ToUpper(name[:1]) + name[1:]
		ctx.SendText(core.ParseMini(g.outputFormat(loc, display, g.listUsers(loc, users)), g.e.Theme()))
		return nil
	}
	for _, line := range g.groupedList(ctx, loc, lists) {
		ctx.SendText(core.ParseMini(line, g.e.Theme()))
	}
	return nil
}

// listSummary answers how many players are online. Essentials:
// PlayerList.listSummary
func (g *group) listSummary(ctx *core.Ctx, showHidden bool) string {
	hidden, shown := 0, 0
	online := g.e.OnlineUsers()
	for _, o := range online {
		if isHidden(o) || ctx.User != nil && hiddenFrom(o, ctx.Player) {
			hidden++
			if showHidden || ctx.User != nil && !hiddenFrom(o, ctx.Player) {
				shown++
			}
		}
	}
	loc := g.locale(ctx.Sender)
	if shown > 0 {
		return g.e.I18n.Format(loc, "listAmountHidden", len(online)-hidden, shown, g.e.Server.MaxPlayers())
	}
	return g.e.I18n.Format(loc, "listAmount", len(online)-hidden, g.e.Server.MaxPlayers())
}

// groupedList answers /list's lines: the list section's groups, merged,
// truncated or hidden, then the rest. Essentials: PlayerList.prepareGroupedList
func (g *group) groupedList(ctx *core.Ctx, loc string, lists map[string][]*core.User) []string {
	var out, asterisk []string
	for _, ent := range g.listGroups() {
		value := strings.TrimSpace(ent.value)
		key := strings.ToLower(ent.key)
		if value == "*" {
			asterisk = append(asterisk, ent.key)
			continue
		}
		if strings.EqualFold(value, "hidden") {
			delete(lists, key)
			continue
		}
		if limit, err := strconv.Atoi(value); err == nil {
			if matched := lists[key]; len(matched) > 0 {
				delete(lists, key)
				if len(matched) > limit {
					out = append(out, g.outputFormat(loc, ent.key, g.e.I18n.Format(loc, "groupNumber", len(matched), ctx.Label, stripFormat(key))))
				} else {
					out = append(out, g.outputFormat(loc, ent.key, g.listUsers(loc, matched)))
				}
				continue
			}
		}
		if users := g.mergedList(lists, key); len(users) > 0 {
			out = append(out, g.outputFormat(loc, ent.key, g.listUsers(loc, users)))
		}
	}
	groups := make([]string, 0, len(lists))
	for k := range lists {
		groups = append(groups, k)
	}
	sort.Slice(groups, func(i, j int) bool { return strings.ToLower(groups[i]) < strings.ToLower(groups[j]) })
	if len(asterisk) > 0 {
		var all []*core.User
		for _, k := range groups {
			all = append(all, lists[k]...)
		}
		for _, k := range asterisk {
			lists[k] = all
		}
		groups = asterisk
	}
	for _, k := range groups {
		users := lists[k]
		if len(users) == 0 {
			continue
		}
		if len(asterisk) == 0 {
			// Every player is in the default group: show it as Essentials
			// does without a permissions plugin.
			name := core.Raw(g.e.I18n.Format(loc, "connectedPlayers"))
			out = append(out, g.outputTag(loc, name, g.listUsers(loc, users)))
			continue
		}
		out = append(out, g.outputFormat(loc, k, g.listUsers(loc, users)))
	}
	return out
}

// near lists the players within a radius of the sender or another player.
func (g *group) near(ctx *core.Ctx) error {
	if ctx.User == nil {
		if len(ctx.Args) == 0 {
			return core.ErrNotEnoughArgs
		}
		other, err := g.e.MatchUser(nil, ctx.Args[0], true, false)
		if err != nil {
			return err
		}
		radius := int64(200)
		if len(ctx.Args) > 1 {
			if n, err := strconv.ParseInt(ctx.Args[1], 10, 64); err == nil {
				radius = n
			}
		}
		ctx.Send("nearbyPlayers", g.nearby(ctx.Sender, other, radius))
		return nil
	}
	maxRadius := int64(g.e.Settings.NearRadius())
	if maxRadius == 0 {
		maxRadius = 200
	}
	radius := maxRadius
	var other *core.User
	if len(ctx.Args) > 0 {
		if n, err := strconv.ParseInt(ctx.Args[0], 10, 64); err == nil {
			radius = n
		} else if u, err := g.getPlayer(ctx, ctx.Args[0], false); err == nil {
			other = u
		}
		if len(ctx.Args) > 1 && other != nil {
			if n, err := strconv.ParseInt(ctx.Args[1], 10, 64); err == nil {
				radius = n
			}
		}
	}
	radius = max(radius, -radius)
	if radius > maxRadius && !ctx.Has("essentials.near.maxexempt") {
		ctx.Send("radiusTooBig", maxRadius)
		radius = maxRadius
	}
	if other == nil || !ctx.Has("essentials.near.others") {
		other = ctx.User
	}
	ctx.Send("nearbyPlayers", g.nearby(ctx.Sender, other, radius))
	return nil
}

// nearby lists the players near u, nearest first. Essentials:
// Commandnear.getLocal
func (g *group) nearby(src plugin.CommandSender, u *core.User, radius int64) core.Raw {
	p, ok := u.Player()
	if !ok {
		return g.tl(src, "none")
	}
	loc := p.Location()
	showHidden := canInteractVanished(u)
	type near struct {
		u    *core.User
		dist float64
	}
	var found []near
	for _, o := range g.e.OnlineUsers() {
		if o.ID == u.ID || o.HasPermission("essentials.near.exclude") || !showHidden && hiddenFrom(o, p) {
			continue
		}
		op, ok := o.Player()
		if !ok || op.World() != loc.World {
			continue
		}
		if d := op.Pos().DistanceSq(loc.Pos); int64(d) < radius*radius {
			found = append(found, near{o, d})
		}
	}
	if len(found) == 0 {
		return g.tl(src, "none")
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].dist < found[j].dist })
	parts := make([]string, len(found))
	for i, n := range found {
		parts[i] = g.e.I18n.Format(g.locale(p), "nearbyPlayersList", n.u.DisplayName(), int64(math.Sqrt(n.dist)))
	}
	return core.Raw(strings.Join(parts, ", "))
}

// playtime shows the time a player has played, online or not.
func (g *group) playtime(ctx *core.Ctx) error {
	var name string
	var ticks int
	key := "playtime"
	switch {
	case len(ctx.Args) > 0 && ctx.Has("essentials.playtime.others"):
		key = "playtimeOther"
		u, err := g.getPlayer(ctx, ctx.Args[0], false)
		if err != nil {
			if !errors.Is(err, core.ErrPlayerNotFound) {
				return err
			}
			// An offline player's statistic comes from their stats file; a
			// vanished player's name is shown rather than their nickname.
			if u, err = g.e.MatchUser(nil, ctx.Args[0], true, true); err != nil {
				return err
			}
			name = core.EscapeTags(u.Name())
		} else {
			name = u.Nick()
		}
		ticks = u.PlayTime()
	case ctx.User != nil:
		name, ticks = ctx.User.Nick(), ctx.User.PlayTime()
	default:
		return core.ErrNotEnoughArgs
	}
	ctx.Send(key, core.FormatDateDiff(time.Now().Add(-time.Duration(ticks)*50*time.Millisecond)), core.Raw(name))
	return nil
}
