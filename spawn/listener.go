package spawn

import (
	"fmt"
	"strings"
	"sync"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

// priority reads a listener priority setting: none (no listener), lowest,
// low, normal, high or highest. Essentials: Settings.getPriority
func priority(s string) (plugin.Priority, bool) {
	switch strings.ToLower(s) {
	case "none":
		return 0, false
	case "lowest":
		return plugin.Lowest, true
	case "low":
		return plugin.Low, true
	case "high":
		return plugin.High, true
	case "highest":
		return plugin.Highest, true
	}
	return plugin.Normal, true
}

// registerListeners registers the respawn and join listeners at the
// priorities config.yml asks for. Essentials: EssentialsSpawn.onEnable
func (m *Spawn) registerListeners(s plugin.Server) {
	e := m.ess
	if pr, ok := priority(e.Config.String("respawn-listener-priority", "normal")); ok {
		plugin.Listen(s, m.onRespawn, plugin.ListenOptions{Priority: pr})
	}
	pr, ok := priority(e.Config.String("spawn-join-listener-priority", "normal"))
	if !ok {
		return
	}
	// Essentials listens to AsyncUserDataLoadEvent, which knows whether the
	// user's file is new; here a user who never logged in is a first join,
	// noted before the core's join listener (Highest) stamps the login.
	var mu sync.Mutex
	first := map[plugin.UUID]bool{}
	plugin.Listen(s, func(ev *plugin.PlayerJoinEvent) {
		u := e.Users.Of(ev.Player)
		mu.Lock()
		first[u.ID] = u.Data.Timestamps.Login == 0
		mu.Unlock()
	}, plugin.ListenOptions{Priority: plugin.Lowest})
	plugin.Listen(s, func(ev *plugin.PlayerJoinEvent) {
		id := ev.Player.UUID()
		mu.Lock()
		f := first[id]
		delete(first, id)
		mu.Unlock()
		m.delayedJoin(ev.Player, f)
	}, plugin.ListenOptions{Priority: pr})
}

// onRespawn sends the player to their bed or home with respawn-at-home, a
// random location with random-respawn-location, else their group's spawn;
// a jailed player is left to the jail. Essentials:
// EssentialsSpawnPlayerListener.onPlayerRespawn
func (m *Spawn) onRespawn(ev *plugin.PlayerRespawnEvent) {
	e := m.ess
	u := e.Users.Of(ev.Player)
	if u.Data.Jailed && u.Data.Jail != "" {
		return
	}
	if e.Config.Bool("respawn-at-home", false) {
		// Bukkit's getRespawnLocation is the bed or anchor the event says
		// was used; the event's location is then that point.
		if e.Config.Bool("respawn-at-home-bed", true) && (!ev.Anchor || e.Config.Bool("respawn-at-anchor", false)) &&
			(ev.Bed || ev.Anchor) {
			return
		}
		if home, ok := homeNear(u, ev.Player.World()); ok {
			ev.Location = home
			return
		}
	}
	if m.tryRandomTeleport(u, e.Config.String("random-respawn-location", "none")) {
		return
	}
	if spawn, ok := m.spawns.Spawn(Group(u)); ok {
		ev.Location = spawn
	}
}

// homeNear answers the user's first home in world, else their first home.
// Essentials: UserData.getHome(Location)
func homeNear(u *core.User, world plugin.World) (plugin.Location, bool) {
	names := u.HomeNames()
	if len(names) == 0 {
		return plugin.Location{}, false
	}
	s := u.Ess().Server
	for _, n := range names {
		if l, ok := u.Data.Homes[n].Resolve(s); ok && l.World == world {
			return l, true
		}
	}
	return u.Data.Homes[names[0]].Resolve(s)
}

// delayedJoin sends a returning player in a spawn-on-join group to spawn,
// and welcomes a new one: a random or newbies spawn point, the
// announcement and the newbies kit. Essentials:
// EssentialsSpawnPlayerListener.delayedJoin
func (m *Spawn) delayedJoin(p plugin.Player, firstJoin bool) {
	e := m.ess
	u := e.Users.Of(p)
	sched := e.Server.Scheduler()
	if !firstJoin {
		groups := spawnOnJoinGroups(e.Config)
		if len(groups) == 0 || !inSpawnOnJoinGroup(u, groups) || u.HasPermission("essentials.spawn-on-join.exempt") {
			return
		}
		sched.RunTask(func() {
			spawn, ok := m.spawns.Spawn(Group(u))
			if !ok {
				return
			}
			if err := teleportUnsafe(u, spawn); err != nil {
				e.ShowError(&core.Ctx{Ess: e, Sender: p}, err)
			}
		})
		return
	}

	random := m.tryRandomTeleport(u, e.Config.String("random-spawn-location", "none"))
	if newbies := e.Config.String("newbies.spawnpoint", "default"); !random && !strings.EqualFold(newbies, "none") {
		sched.RunLater(1, func() {
			if !u.Online() {
				return
			}
			spawn, ok := m.spawns.Spawn(newbies)
			if !ok {
				return
			}
			if err := u.TeleportNow(core.LocationTarget(spawn), plugin.TeleportPlugin); err != nil {
				e.Log.Warn(e.TlString("teleportNewPlayerError"), "err", err)
			}
		})
	}
	sched.RunLater(2, func() {
		if !u.Online() {
			return
		}
		if format := e.Config.String("newbies.announce-format", "-"); format != "" {
			text := e.ReplaceKeywords(e.Config.String("newbies.announce-format", "&dWelcome {DISPLAYNAME} to the server!"), u)
			for _, line := range strings.Split(text, "\n") {
				broadcast(e, u, line)
			}
		}
		if name := e.Config.String("newbies.kit", ""); name != "" {
			kit, ok := e.Kits.Get(strings.ToLower(name))
			if !ok {
				e.Log.Warn(e.TlString("kitNotFound"))
			} else if err := e.Kits.Expand(kit, u); err != nil {
				e.Log.Warn(fmt.Sprint(err))
			}
		}
	})
}

// broadcast sends line to every player who does not ignore u, and the
// console. Essentials: Essentials.broadcastMessage(IUser, String)
func broadcast(e *core.Essentials, u *core.User, line string) {
	msg := e.Mini(line)
	for _, o := range e.OnlineUsers() {
		if o.IsIgnoring(u) {
			continue
		}
		o.SendText(msg)
	}
	e.Server.Console().SendMessage(msg)
}

// teleportUnsafe teleports u to loc at once, without looking for a safe
// spot, setting /back. Essentials: AsyncTeleport.nowUnsafe
func teleportUnsafe(u *core.User, loc plugin.Location) error {
	p, ok := u.Player()
	if !ok {
		return core.ErrPlayerNotFound
	}
	if !u.Ess().Settings.RegisterBackInListener() {
		u.SetLastLocation(p.Location())
	}
	if !p.Teleport(loc, plugin.TeleportPlugin) {
		return core.Tlf("teleportNewPlayerError")
	}
	return nil
}

// spawnOnJoinGroups reads spawn-on-join: a list of groups, a single group,
// or true for every group ("*"). Essentials: Settings._getSpawnOnJoinGroups
func spawnOnJoinGroups(c *core.Config) []string {
	v, ok := c.Get("spawn-on-join")
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case []any:
		return c.StringList("spawn-on-join")
	case bool:
		if t {
			return []string{"*"}
		}
		return nil
	}
	if s := fmt.Sprint(v); s != "" {
		return []string{s}
	}
	return nil
}

// inSpawnOnJoinGroup reports whether u is in one of groups.
// Essentials: Settings.isUserInSpawnOnJoinGroup
func inSpawnOnJoinGroup(u *core.User, groups []string) bool {
	for _, g := range groups {
		if g == "*" || inGroup(u, g) {
			return true
		}
	}
	return false
}
