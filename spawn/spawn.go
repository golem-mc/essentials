// Package spawn is EssentialsXSpawn, ported to golem: /setspawn and
// /spawn, per-group spawn points in spawn.yml, and where players go when
// they respawn and when they join for the first time.
package spawn

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

// Spawn is the essentialsspawn plugin. Essentials: EssentialsSpawn
type Spawn struct {
	ess    *core.Essentials
	spawns *Storage
}

func init() { plugin.Register("essentialsspawn", &Spawn{}, plugin.Depends("essentials")) }

// Enable loads spawn.yml and registers the commands and listeners.
func (m *Spawn) Enable(s plugin.Server) error {
	e, ok := core.Get()
	if !ok {
		return errors.New("essentials is not enabled")
	}
	m.ess = e
	st, err := newStorage(e)
	if err != nil {
		return err
	}
	m.spawns = st
	e.OnReload(func() {
		if err := st.reload(); err != nil {
			e.Log.Error("cannot reload spawn.yml", "err", err)
		}
	})
	e.SetSpawnProvider(func(u *core.User) (plugin.Location, bool) { return st.Spawn(Group(u)) })
	if err := e.RegisterModulePermissions(s, pluginYML); err != nil {
		return err
	}
	if err := e.RegisterModuleCommands(s, pluginYML, m.setspawnCmd(), m.spawnCmd()); err != nil {
		return err
	}
	m.registerListeners(s)
	return nil
}

// Disable does nothing; spawn.yml is saved as it changes.
func (m *Spawn) Disable() {}

// Group answers the user's permission group. Without a permissions plugin
// (none exists for golem yet) every user is in "default", as
// Essentials' SuperpermsHandler answers. Essentials: User.getGroup
func Group(*core.User) string { return "default" }

// inGroup reports whether the user is in group: the group.<name>
// permission. Essentials: SuperpermsHandler.inGroup
func inGroup(u *core.User, group string) bool { return u.HasPermission("group." + group) }

// Storage is spawn.yml: a spawn point per group. Essentials: SpawnStorage
type Storage struct {
	e      *core.Essentials
	mu     sync.Mutex
	path   string
	spawns map[string]core.LazyLocation
}

// spawnFile is spawn.yml's layout.
type spawnFile struct {
	Spawns map[string]core.LazyLocation `yaml:"spawns"`
}

func newStorage(e *core.Essentials) (*Storage, error) {
	st := &Storage{e: e, path: e.DataPath("spawn.yml")}
	return st, st.reload()
}

func (st *Storage) reload() error {
	var f spawnFile
	if err := core.ReadYAML(st.path, &f); err != nil {
		return err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.spawns = map[string]core.LazyLocation{}
	for k, v := range f.Spawns {
		st.spawns[strings.ToLower(k)] = v
	}
	return nil
}

// SetSpawn saves loc as group's spawn.
func (st *Storage) SetSpawn(loc plugin.Location, group string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.spawns[strings.ToLower(group)] = core.LazyOf(loc)
	return core.WriteYAML(st.path, spawnFile{Spawns: st.spawns})
}

// Spawn answers group's spawn, else the default group's, else the
// overworld's spawn point; false when the spawn's world is not loaded.
func (st *Storage) Spawn(group string) (plugin.Location, bool) {
	st.mu.Lock()
	l, ok := st.spawns[strings.ToLower(group)]
	if !ok {
		l, ok = st.spawns["default"]
	}
	st.mu.Unlock()
	if !ok {
		return st.e.Server.Overworld().SpawnPoint(), true
	}
	return l.Resolve(st.e.Server)
}

// Essentials: Commandsetspawn
func (m *Spawn) setspawnCmd() core.Cmd {
	return core.Cmd{
		Name:       "setspawn",
		PlayerOnly: true,
		Run: func(ctx *core.Ctx) error {
			group := "default"
			if len(ctx.Args) > 0 {
				group = ctx.FinalArg(0)
			}
			if err := m.spawns.SetSpawn(ctx.Player.Location(), group); err != nil {
				return err
			}
			ctx.Send("spawnSet", group)
			return nil
		},
		Complete: func(*core.Ctx) []string { return []string{} },
	}
}

// Essentials: Commandspawn
func (m *Spawn) spawnCmd() core.Cmd {
	return core.Cmd{
		Name: "spawn",
		Run: func(ctx *core.Ctx) error {
			if ctx.User == nil {
				if len(ctx.Args) == 0 {
					return core.ErrNotEnoughArgs
				}
				target, err := ctx.Ess.MatchUser(nil, ctx.Args[0], true, false)
				if err != nil {
					return err
				}
				m.respawn(ctx, nil, target, func() {
					target.Send("teleportAtoB", ctx.Ess.TlString("consoleName"), "spawn")
				})
				return nil
			}
			if err := ctx.Charge.CheckAffordable(ctx.User); err != nil {
				return err
			}
			if len(ctx.Args) > 0 && ctx.Has("essentials.spawn.others") {
				other, err := ctx.GetPlayer(0, false)
				if err != nil {
					return err
				}
				m.respawn(ctx, ctx.User, other, func() {
					if other.ID != ctx.User.ID {
						other.Send("teleportAtoB", ctx.User.DisplayName(), "spawn")
					}
				})
			} else {
				m.respawn(ctx, ctx.User, ctx.User, nil)
			}
			return core.ErrNoCharge
		},
		Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 && ctx.Has("essentials.spawn.others") {
				return ctx.CompletePlayers()
			}
			return []string{}
		},
	}
}

// respawn teleports teleportee to their group's spawn, with owner's warmup
// and charge (the console's teleport is immediate); once they are there it
// tells the sender and runs then. Essentials: Commandspawn.respawn
func (m *Spawn) respawn(ctx *core.Ctx, owner, teleportee *core.User, then func()) {
	spawn, ok := m.spawns.Spawn(Group(teleportee))
	if !ok {
		return
	}
	done := func() {
		b := spawn.Pos.Block()
		ctx.Send("teleporting", core.WorldName(spawn.World), b.X, b.Y, b.Z)
		if then != nil {
			then()
		}
	}
	var err error
	if owner == nil {
		if err = teleportee.TeleportNow(core.LocationTarget(spawn), plugin.TeleportCommand); err == nil {
			done()
		}
	} else {
		err = owner.TeleportOther(teleportee, core.LocationTarget(spawn), ctx.Charge, plugin.TeleportCommand, done)
	}
	if err != nil {
		ctx.Ess.ShowError(ctx, err)
	}
}
