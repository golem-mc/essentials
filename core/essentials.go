// Package core is EssentialsX's foundation, ported to golem: the
// configuration, messages, users and their data, teleports, charges, the
// economy, the item database, warps, kits, jails and worth, and the command
// framework every command group is written against. Everything here runs on
// the tick goroutine unless a comment says otherwise.
package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/golem-mc/golem/plugin"
)

// Essentials is the plugin's state, shared by its command groups and by the
// modules (spawn, chat, protect, antibuild). Essentials: IEssentials
type Essentials struct {
	Server plugin.Server
	Log    *plugin.Logger

	Config   *Config
	Settings *Settings
	I18n     *I18n
	Users    *UserMap
	Items    *ItemDB
	Warps    *Warps
	Kits     *Kits
	Jails    *Jails
	Worth    *Worth
	Economy  *Economy

	theme Theme
	spawn SpawnProvider
	// invisibility is the effect a vanished player wears.
	invisibility plugin.Effect
	commands     map[string]*registered
	meta         *pluginYML
	reloads      []func()
	stops        []func()
}

var (
	instMu   sync.Mutex
	instance *Essentials
)

// Get answers the running Essentials, for the modules; false before it is
// enabled or after it is disabled.
func Get() (*Essentials, bool) {
	instMu.Lock()
	defer instMu.Unlock()
	return instance, instance != nil
}

// Enable loads everything from the data directory and registers the core
// listeners; the command groups register after it. Essentials: onEnable
func Enable(s plugin.Server) (*Essentials, error) {
	e := &Essentials{Server: s, Log: s.Logger(), commands: map[string]*registered{}}
	if err := os.MkdirAll(s.DataDir(), 0o755); err != nil {
		return nil, err
	}
	meta, err := parsePluginYML(Resource("plugin.yml"))
	if err != nil {
		return nil, fmt.Errorf("plugin.yml: %w", err)
	}
	e.meta = meta
	if err := e.loadConfig(); err != nil {
		return nil, err
	}
	e.I18n = newI18n(s.DataDir(), e.Settings.Locale())
	e.registerPermissions()

	e.Users = newUserMap(e)
	e.Economy = newEconomy(e)
	if e.Items, err = newItemDB(e); err != nil {
		return nil, err
	}
	if e.Warps, err = newWarps(e); err != nil {
		return nil, err
	}
	if e.Kits, err = newKits(e); err != nil {
		return nil, err
	}
	if e.Jails, err = newJails(e); err != nil {
		return nil, err
	}
	if e.Worth, err = newWorth(e); err != nil {
		return nil, err
	}
	for _, f := range []string{"motd.txt", "rules.txt", "info.txt", "book.txt", "custom.txt"} {
		writeDefault(filepath.Join(s.DataDir(), f), f)
	}

	e.registerCoreListeners()
	e.startTimer()

	instMu.Lock()
	instance = e
	instMu.Unlock()
	return e, nil
}

// Disable saves every user and data file. Essentials: onDisable
func (e *Essentials) Disable() {
	for _, f := range e.stops {
		f()
	}
	e.Users.SaveAll(true)
	instMu.Lock()
	if instance == e {
		instance = nil
	}
	instMu.Unlock()
}

// OnReload registers fn to run when /essentials reload rereads the files.
func (e *Essentials) OnReload(fn func()) { e.reloads = append(e.reloads, fn) }

// OnDisable registers fn to run as the plugin disables, before users save.
func (e *Essentials) OnDisable(fn func()) { e.stops = append(e.stops, fn) }

func (e *Essentials) loadConfig() error {
	c, err := LoadConfig(filepath.Join(e.Server.DataDir(), "config.yml"), "config.yml")
	if err != nil {
		return err
	}
	e.Config = c
	e.Settings = &Settings{c: c}
	e.theme = Theme{
		Primary:   themeColor(c.String("message-colors.primary", DefaultTheme.Primary)),
		Secondary: themeColor(c.String("message-colors.secondary", DefaultTheme.Secondary)),
	}
	return nil
}

func themeColor(s string) string {
	if c, ok := colorOf(s, DefaultTheme); ok {
		return c
	}
	return s
}

// Reload rereads config.yml, the messages and every data file, as
// /essentials reload does.
func (e *Essentials) Reload() error {
	if err := e.loadConfig(); err != nil {
		return err
	}
	e.I18n.SetLocale(e.Settings.Locale())
	for _, r := range []interface{ reload() error }{e.Items, e.Warps, e.Kits, e.Jails, e.Worth} {
		if err := r.reload(); err != nil {
			return err
		}
	}
	for _, f := range e.reloads {
		f()
	}
	return nil
}

// writeDefault copies the bundled resource name to path when path is
// missing.
func writeDefault(path, name string) {
	if _, err := os.Stat(path); err == nil {
		return
	}
	_ = os.WriteFile(path, Resource(name), 0o644)
}

// DataPath answers a path inside the plugin's data directory.
func (e *Essentials) DataPath(parts ...string) string {
	return filepath.Join(append([]string{e.Server.DataDir()}, parts...)...)
}

// Theme answers the message colours.
func (e *Essentials) Theme() Theme { return e.theme }

// Tl answers the message key in the server's locale. Essentials: tlLiteral
func (e *Essentials) Tl(key string, args ...any) plugin.Text {
	return ParseMini(e.I18n.Format("", key, args...), e.theme)
}

// TlString answers the message key as plain text, for logs and kick
// reasons that are not components.
func (e *Essentials) TlString(key string, args ...any) string {
	return e.Tl(key, args...).PlainString()
}

// TlFor answers the message key in the locale of the sender when
// per-player-locale is on.
func (e *Essentials) TlFor(sender plugin.CommandSender, key string, args ...any) plugin.Text {
	loc := ""
	if p, ok := sender.(plugin.Player); ok && e.Settings.PerPlayerLocale() {
		loc = p.Locale()
	}
	return ParseMini(e.I18n.Format(loc, key, args...), e.theme)
}

// Send sends sender the message key. Essentials: sendTl
func (e *Essentials) Send(sender plugin.CommandSender, key string, args ...any) {
	if sender == nil {
		return
	}
	// A message translated to nothing is not sent, so a server can turn one
	// off by blanking it.
	if msg := e.TlFor(sender, key, args...); strings.TrimSpace(msg.PlainString()) != "" {
		sender.SendMessage(msg)
	}
}

// Broadcast sends the message key to every online player and logs it,
// skipping players who ignore cause (nil for none) and, with perm set,
// players without it. A hidden cause broadcasts nothing.
// Essentials: broadcastTl
func (e *Essentials) Broadcast(cause *User, perm, key string, args ...any) {
	if cause != nil && cause.Hidden {
		return
	}
	for _, p := range e.Server.Players() {
		if perm != "" && !p.HasPermission(perm) {
			continue
		}
		if cause != nil && perm == "" && e.Users.Get(p.UUID()).IsIgnoring(cause) {
			continue
		}
		e.Send(p, key, args...)
	}
	if msg := e.TlString(key, args...); strings.TrimSpace(msg) != "" {
		e.Log.Info(msg)
	}
}

// OnlineUsers answers the users of the online players.
func (e *Essentials) OnlineUsers() []*User {
	ps := e.Server.Players()
	out := make([]*User, 0, len(ps))
	for _, p := range ps {
		out = append(out, e.Users.Get(p.UUID()))
	}
	return out
}
