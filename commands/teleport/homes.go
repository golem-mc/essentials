package teleport

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

func (g *group) registerHomes() {
	g.e.RegisterCommand(
		core.Cmd{Name: "home", PlayerOnly: true, Run: g.runHome, Complete: g.completeHome},
		core.Cmd{Name: "sethome", PlayerOnly: true, Run: g.runSetHome, Complete: func(*core.Ctx) []string { return nil }},
		core.Cmd{Name: "delhome", Run: g.runDelHome, Complete: g.completeOthersHomes("essentials.delhome.others", "*")},
		core.Cmd{Name: "renamehome", PlayerOnly: true, Run: g.runRenameHome, Complete: g.completeOthersHomes("essentials.renamehome.others", "")},
	)
}

// Session keys of the home overwrite confirmation.
const (
	sessHomeConfirm   = "teleport.homeConfirm"
	sessHomeConfirmAt = "teleport.homeConfirmAt"
)

// homeKey answers the saved key of the user's home name: a number picks the
// nth home. Essentials: UserData.getHomeName
func homeKey(u *core.User, name string) string {
	if n, err := strconv.Atoi(name); err == nil {
		if homes := u.HomeNames(); n >= 1 && n <= len(homes) {
			return homes[n-1]
		}
	}
	return name
}

// homeLocation answers the home's location; false when there is none or its
// world is not loaded. Essentials: UserData.getHome
func (g *group) homeLocation(u *core.User, name string) (plugin.Location, bool) {
	l, ok := u.Data.Homes[homeKey(u, name)]
	if !ok {
		return plugin.Location{}, false
	}
	return l.Resolve(g.e.Server)
}

// hasValidHomes reports whether a home is in a loaded world.
func (g *group) hasValidHomes(u *core.User) bool {
	for _, l := range u.Data.Homes {
		if _, ok := l.Resolve(g.e.Server); ok {
			return true
		}
	}
	return false
}

// homeLimit answers how many homes u may set: 1, sethome-multiple.default
// with essentials.sethome.multiple, or the largest group's limit u has
// essentials.sethome.multiple.<group> for. Essentials: Settings.getHomeLimit
func (g *group) homeLimit(u *core.User) int {
	limit := 1
	if u.HasPermission("essentials.sethome.multiple") {
		limit = g.setLimit("default")
	}
	for _, set := range g.e.Config.Keys("sethome-multiple") {
		if n := g.setLimit(set); u.HasPermission("essentials.sethome.multiple."+set) && limit < n {
			limit = n
		}
	}
	return limit
}

func (g *group) setLimit(set string) int {
	return g.e.Config.Int("sethome-multiple."+set, g.e.Config.Int("sethome-multiple.default", 3))
}

// homeLimitText answers the limit for /home's list: ? offline, * unlimited.
func (g *group) homeLimitText(u *core.User) string {
	switch {
	case !u.Online():
		return "?"
	case u.HasPermission("essentials.sethome.multiple.unlimited"):
		return "*"
	}
	return strconv.Itoa(g.homeLimit(u))
}

// bedLocation answers where the online user would respawn: their bed or
// anchor.
func bedLocation(u *core.User) (plugin.Location, bool) {
	p, ok := u.Player()
	if !ok {
		return plugin.Location{}, false
	}
	return p.RespawnPoint()
}

// errNoHome is goHome's NotEnoughArgumentsException: no such home, so /home
// lists them instead.
var errNoHome = core.ErrNotEnoughArgs

// Essentials: Commandhome
func (g *group) runHome(ctx *core.Ctx) error {
	user := ctx.User
	charge := ctx.Charge
	player := user
	homeName := ""
	if len(ctx.Args) > 0 {
		parts := javaSplit(ctx.Args[0], ":")
		if len(parts) == 0 {
			parts = []string{""}
		}
		if len(parts[0]) == len(ctx.Args[0]) || !user.HasPermission("essentials.home.others") {
			homeName = parts[0]
		} else {
			if parts[0] == "" {
				return core.ErrPlayerNotFound
			}
			p, err := g.e.MatchUser(nil, parts[0], true, true)
			if err != nil {
				return err
			}
			player = p
			if len(parts) > 1 {
				homeName = parts[1]
			}
		}
	}
	if strings.EqualFold(homeName, "bed") {
		if !user.HasPermission("essentials.home.bed") {
			return core.Tlf("noAccessCommand")
		}
		if !player.Online() {
			return core.Tlf("bedOffline")
		}
		bed, ok := bedLocation(player)
		if !ok {
			return core.Tlf("bedMissing")
		}
		if err := g.teleport(user, user, core.LocationTarget(bed), charge, func() {
			user.Send("teleportHome", "bed")
		}); err != nil {
			return err
		}
		return core.ErrNoCharge
	}
	err := g.goHome(user, player, strings.ToLower(homeName), charge)
	if err != errNoHome {
		if err != nil {
			return err
		}
		return core.ErrNoCharge
	}
	bed, hasBed := bedLocation(player)
	homes := player.HomeNames()
	switch {
	case len(homes) == 0 && player.ID == user.ID:
		if !g.e.Settings.SpawnIfNoHome() {
			return core.Tlf("noHomeSetPlayer")
		}
		dest := location(player).World.SpawnPoint()
		if hasBed && user.HasPermission("essentials.home.bed") {
			dest = bed
		}
		if err := g.teleport(user, user, core.LocationTarget(dest), charge, nil); err != nil {
			return err
		}
	case len(homes) == 0 || !g.hasValidHomes(player):
		return core.Tlf("noHomeSetPlayer")
	case len(homes) == 1 && player.ID == user.ID:
		if err := g.goHome(user, player, homes[0], charge); err != nil {
			return err
		}
	default:
		list := make([]string, 0, len(homes)+1)
		for _, h := range homes {
			list = append(list, core.EscapeTags(h))
		}
		if user.HasPermission("essentials.home.bed") {
			if hasBed {
				list = append(list, g.rawTl(user, "bed"))
			} else {
				list = append(list, g.rawTl(user, "bedNull"))
			}
		}
		user.Send("homes", core.Raw(joinList(list)), len(homes), g.homeLimitText(player))
	}
	return core.ErrNoCharge
}

// goHome teleports user to player's home; errNoHome when there is none.
// Essentials: Commandhome.goHome
func (g *group) goHome(user, player *core.User, home string, charge *core.Trade) error {
	if home == "" {
		return errNoHome
	}
	loc, ok := g.homeLocation(player, home)
	if !ok {
		return errNoHome
	}
	if !sameWorld(world(user), loc.World) && g.e.Settings.WorldHomePermissions() {
		if node := "essentials.worlds." + core.WorldName(loc.World); !user.HasPermission(node) {
			return core.Tlf("noPerm", node)
		}
	}
	return g.teleport(user, user, core.LocationTarget(loc), charge, func() {
		user.Send("teleportHome", home)
	})
}

func (g *group) completeHome(ctx *core.Ctx) []string {
	u := ctx.User
	if u == nil {
		return ctx.CompletePlayers()
	}
	if len(ctx.Args) != 1 {
		return nil
	}
	bed := u.HasPermission("essentials.home.bed")
	homes := u.HomeNames()
	if bed {
		homes = append(homes, "bed")
	}
	if !u.HasPermission("essentials.home.others") {
		return homes
	}
	name, _, found := strings.Cut(ctx.Args[0], ":")
	if !found {
		for _, p := range ctx.CompletePlayers() {
			homes = append(homes, p+":")
		}
		return homes
	}
	other, err := g.e.MatchUser(nil, name, true, true)
	if err != nil || name == "" {
		return homes
	}
	for _, h := range other.HomeNames() {
		homes = append(homes, name+":"+h)
	}
	if bed {
		homes = append(homes, name+":bed")
	}
	return homes
}

// completeOthersHomes completes the sender's homes, and with others
// player: and player:home, plus player:extra when extra is set.
// Essentials: Commanddelhome, Commandrenamehome getTabCompleteOptions
func (g *group) completeOthersHomes(others, extra string) func(ctx *core.Ctx) []string {
	return func(ctx *core.Ctx) []string {
		if len(ctx.Args) != 1 {
			return nil
		}
		var homes []string
		if ctx.User != nil {
			homes = ctx.User.HomeNames()
		}
		if !ctx.Has(others) {
			return homes
		}
		name, _, found := strings.Cut(ctx.Args[0], ":")
		if !found {
			for _, p := range ctx.CompletePlayers() {
				homes = append(homes, p+":")
			}
			return homes
		}
		other, err := g.e.MatchUser(nil, name, true, true)
		if err != nil || name == "" {
			return homes
		}
		for _, h := range other.HomeNames() {
			homes = append(homes, name+":"+h)
		}
		if extra != "" {
			homes = append(homes, name+":"+extra)
		}
		return homes
	}
}

// Essentials: Commandsethome
func (g *group) runSetHome(ctx *core.Ctx) error {
	user := ctx.User
	usersHome := user
	name := "home"
	args := ctx.Args
	if len(args) > 0 {
		// Both /sethome khobbits house and /sethome khobbits:house.
		if parts := javaSplit(args[0], ":"); len(parts) == 0 || len(parts[0]) != len(args[0]) {
			args = parts
		}
		switch {
		case len(args) == 0:
			return core.ErrNotEnoughArgs
		case len(args) < 2:
			name = strings.ToLower(args[0])
		default:
			name = strings.ToLower(args[1])
			if user.HasPermission("essentials.sethome.others") {
				u, err := g.e.MatchUser(nil, args[0], true, true)
				if err != nil {
					return err
				}
				usersHome = u
			}
		}
	}
	if !user.HasPermission("essentials.sethome.multiple.unlimited") {
		limit := g.homeLimit(user)
		if len(usersHome.Data.Homes) >= limit {
			if _, ok := usersHome.Data.Homes[name]; !ok {
				return core.Tlf("maxHomes", limit)
			}
		} else if limit == 1 {
			name = "home"
		}
	}
	if name == "bed" || core.IsInt(name) {
		return core.Tlf("invalidHomeName")
	}
	loc := ctx.Player.Location()
	b := loc.Pos.Block()
	s := g.e.Settings
	if (!s.TeleportSafety() || !s.ForceDisableTeleportSafety()) && g.unsafeFor(usersHome, loc.World, b.X, b.Y, b.Z) {
		return core.Tlf("unsafeTeleportDestination", locArgs(loc)...)
	}
	if usersHome.Session == nil {
		usersHome.Session = map[string]any{}
	}
	if _, exists := usersHome.Data.Homes[name]; s.ConfirmHomeOverwrite() && exists {
		last, _ := core.SessionValue[string](usersHome, sessHomeConfirm)
		at, _ := core.SessionValue[time.Time](usersHome, sessHomeConfirmAt)
		if name != last || time.Since(at) > 2*time.Minute {
			usersHome.Session[sessHomeConfirm] = name
			usersHome.Session[sessHomeConfirmAt] = time.Now()
			user.Send("homeConfirmation", name)
			return nil
		}
	}
	setHome(usersHome, name, loc)
	user.Send("homeSet", append(locArgs(loc), name)...)
	delete(usersHome.Session, sessHomeConfirm)
	return nil
}

// setHome saves a home under its safe name. Essentials: UserData.setHome
func setHome(u *core.User, name string, loc plugin.Location) {
	if u.Data.Homes == nil {
		u.Data.Homes = map[string]core.LazyLocation{}
	}
	u.Data.Homes[safeString(name)] = core.LazyOf(loc)
	u.Save()
}

// delHome removes a home by name, number or safe name; invalidHome when
// there is none. Essentials: UserData.delHome
func delHome(u *core.User, name string) error {
	key := homeKey(u, name)
	if _, ok := u.Data.Homes[key]; !ok {
		key = safeString(key)
	}
	if _, ok := u.Data.Homes[key]; !ok {
		return core.Tlf("invalidHome", key)
	}
	delete(u.Data.Homes, key)
	u.Save()
	return nil
}

// Essentials: Commanddelhome
func (g *group) runDelHome(ctx *core.Ctx) error {
	if len(ctx.Args) < 1 {
		return core.ErrNotEnoughArgs
	}
	user := ctx.User
	args := ctx.Args
	// Both /delhome khobbits house and /delhome khobbits:house.
	if parts := javaSplit(args[0], ":"); len(parts) == 0 || len(parts[0]) != len(args[0]) {
		args = parts
	}
	var name string
	switch {
	case len(args) > 1 && (user == nil || user.HasPermission("essentials.delhome.others")):
		if args[0] == "" {
			return core.ErrPlayerNotFound
		}
		u, err := g.e.MatchUser(nil, args[0], true, true)
		if err != nil {
			return err
		}
		user = u
		name = strings.ToLower(args[1])
	case user == nil || len(args) == 0:
		return core.ErrNotEnoughArgs
	default:
		name = strings.ToLower(args[0])
	}
	del := func(home string) {
		if err := delHome(user, home); err != nil {
			ctx.Send("invalidHome", home)
			return
		}
		ctx.Send("deleteHome", home)
	}
	switch name {
	case "bed":
		return core.Tlf("invalidHomeName")
	case "*":
		for _, h := range user.HomeNames() {
			del(h)
		}
	default:
		del(name)
	}
	return nil
}

// Essentials: Commandrenamehome
func (g *group) runRenameHome(ctx *core.Ctx) error {
	user := ctx.User
	usersHome := user
	args := ctx.Args
	var oldName, newName string
	switch len(args) {
	case 2:
		// Both /renamehome jroy home1 home and /renamehome jroy:home1 home.
		parts := strings.SplitN(args[0], ":", 2)
		newName = strings.ToLower(args[1])
		if len(parts) == 2 {
			oldName = strings.ToLower(parts[1])
			if user.HasPermission("essentials.renamehome.others") {
				u, err := g.e.MatchUser(nil, parts[0], true, true)
				if err != nil {
					return err
				}
				usersHome = u
			}
		} else {
			oldName = strings.ToLower(args[0])
		}
	case 3:
		if !user.HasPermission("essentials.renamehome.others") {
			return core.ErrNotEnoughArgs
		}
		u, err := g.e.MatchUser(nil, args[0], true, true)
		if err != nil {
			return err
		}
		usersHome = u
		oldName = strings.ToLower(args[1])
		newName = strings.ToLower(args[2])
	default:
		return core.ErrNotEnoughArgs
	}
	if slices.Contains([]string{newName, oldName}, "bed") || core.IsInt(newName) || core.IsInt(oldName) {
		return core.Tlf("invalidHomeName")
	}
	loc, ok := usersHome.Data.Homes[oldName]
	if !ok {
		return core.Tlf("invalidHome", oldName)
	}
	delete(usersHome.Data.Homes, oldName)
	usersHome.Data.Homes[safeString(newName)] = loc
	usersHome.Save()
	user.Send("homeRenamed", oldName, newName)
	delete(usersHome.Session, sessHomeConfirm)
	return nil
}
