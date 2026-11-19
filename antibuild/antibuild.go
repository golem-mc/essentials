// Package antibuild is EssentialsXAntiBuild, ported to golem: the
// essentials.build permissions and the placement, break, usage, piston and
// dispenser blacklists of config.yml's protect: section, with alerts to
// essentials.protect.alerts.
package antibuild

import (
	_ "embed"
	"errors"
	"strings"
	"sync"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

//go:embed plugin.yml
var pluginYML []byte

// AntiBuild is the essentialsantibuild plugin. Essentials: EssentialsAntiBuild
type AntiBuild struct {
	ess   *core.Essentials
	mu    sync.Mutex
	lists map[string]map[string]bool
}

func init() { plugin.Register("essentialsantibuild", &AntiBuild{}, plugin.Depends("essentials")) }

// Enable reads the lists and registers the listeners.
func (m *AntiBuild) Enable(s plugin.Server) error {
	e, ok := core.Get()
	if !ok {
		return errors.New("essentials is not enabled")
	}
	m.ess = e
	if err := e.RegisterModulePermissions(s, pluginYML); err != nil {
		return err
	}
	m.reload()
	e.OnReload(m.reload)
	m.registerListeners(s)
	return nil
}

// Disable does nothing.
func (m *AntiBuild) Disable() {}

// The lists of config.yml. Essentials: AntiBuildConfig
const (
	alertOnPlacement   = "protect.alert.on-placement"
	alertOnUse         = "protect.alert.on-use"
	alertOnBreak       = "protect.alert.on-break"
	blacklistPlacement = "protect.blacklist.placement"
	blacklistUsage     = "protect.blacklist.usage"
	blacklistBreak     = "protect.blacklist.break"
	blacklistPiston    = "protect.blacklist.piston"
	blacklistDispenser = "protect.blacklist.dispenser"
)

// reload reads the lists. Essentials: EssentialsConnect.AntiBuildReloader
func (m *AntiBuild) reload() {
	lists := map[string]map[string]bool{}
	for _, key := range []string{alertOnPlacement, alertOnUse, alertOnBreak, blacklistPlacement, blacklistUsage,
		blacklistBreak, blacklistPiston, blacklistDispenser} {
		lists[key] = m.materialList(key)
	}
	m.mu.Lock()
	m.lists = lists
	m.mu.Unlock()
}

// materialList reads a comma-separated list of materials: Bukkit material
// names (block or item registry names in capitals), else item names the
// item database knows. Essentials: Settings.getProtectList
func (m *AntiBuild) materialList(key string) map[string]bool {
	out := map[string]bool{}
	for name := range strings.SplitSeq(m.ess.Config.String(key, ""), ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		reg := "minecraft:" + strings.ToLower(name)
		_, isBlock := registry.BlockByName(reg)
		_, isItem := registry.ItemByName(reg)
		if isBlock || isItem {
			out[strings.ToUpper(name)] = true
			continue
		}
		if s, err := m.ess.Items.Get(name); err == nil {
			out[itemMaterial(s.Item)] = true
			continue
		}
		m.ess.Log.Warn(m.ess.TlString("unknownItemInList", name, key))
	}
	return out
}

// listed reports whether a list holds material. Essentials:
// EssentialsAntiBuild.checkProtectionItems
func (m *AntiBuild) listed(list, material string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lists[list][material]
}

// blockMaterial and itemMaterial answer Bukkit's Material name.
func blockMaterial(b registry.Block) string {
	return strings.ToUpper(strings.TrimPrefix(b.Name(), "minecraft:"))
}

func itemMaterial(i registry.Item) string {
	return strings.ToUpper(strings.TrimPrefix(i.Name(), "minecraft:"))
}

// nameOf answers a material as messages show it: "lava bucket".
// Essentials: EssentialsAntiBuild.getNameForType
func nameOf(material string) string {
	return strings.ReplaceAll(strings.ToLower(material), "_", " ")
}

// disableBuild and disableUse read protect.disable.build and .use.
func (m *AntiBuild) disableBuild() bool { return m.ess.Config.Bool("protect.disable.build", true) }
func (m *AntiBuild) disableUse() bool   { return m.ess.Config.Bool("protect.disable.use", true) }

// warn reports whether to tell a player they may not build.
// Essentials: Settings.warnOnBuildDisallow
func (m *AntiBuild) warn() bool {
	return m.ess.Config.Bool("protect.disable.warn-on-build-disallow", false)
}

// deny tells the user why, when warn-on-build-disallow is on.
func (m *AntiBuild) deny(u *core.User, key, what string) {
	if m.warn() {
		u.Send(key, what)
	}
}

// canBuild reports whether the user builds freely: an operator, or
// essentials.build. Essentials: User.canBuild, SuperpermsHandler.canBuild
func canBuild(u *core.User) bool {
	p, ok := u.Player()
	return ok && (p.OpLevel() > 0 || p.HasPermission("essentials.build"))
}

// permitted reports essentials.build.<action>.<material>.
// Essentials: EssentialsAntiBuildListener.metaPermCheck
func permitted(u *core.User, action, material string) bool {
	return u.HasPermission("essentials.build." + action + "." + material)
}

// alert logs what the user did and tells everyone with
// essentials.protect.alerts. Essentials: EssentialsConnect.alert
func (m *AntiBuild) alert(u *core.User, item, key string) {
	e := m.ess
	p, ok := u.Player()
	if !ok {
		return
	}
	loc := p.Location()
	b := loc.Pos.Block()
	where := core.WorldName(loc.World) + "," + itoa(b.X) + "," + itoa(b.Y) + "," + itoa(b.Z)
	e.Log.Warn(e.TlString("alertFormat", u.Name(), core.Raw(e.I18n.Format("", key)), item, where))
	for _, o := range e.OnlineUsers() {
		if o.HasPermission("essentials.protect.alerts") {
			o.Send("alertFormat", u.Name(), o.Tl(key).PlainString(), item, where)
		}
	}
}
