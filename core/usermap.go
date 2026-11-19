package core

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// UserMap loads users from userdata/<uuid>.yml on first use and keeps them;
// it knows every user's last name, so offline players can be found by name.
// Essentials: UserMap / ModernUserMap
type UserMap struct {
	ess    *Essentials
	dir    string
	users  map[plugin.UUID]*User
	byName map[string]plugin.UUID // lower-case last account name
	// writeMu orders the asynchronous file writes.
	writeMu sync.Mutex
}

func newUserMap(e *Essentials) *UserMap {
	m := &UserMap{ess: e, dir: e.DataPath("userdata"), users: map[plugin.UUID]*User{}, byName: map[string]plugin.UUID{}}
	_ = os.MkdirAll(m.dir, 0o755)
	m.indexNames()
	return m
}

// indexNames reads every userdata file's last-account-name. A server with
// tens of thousands of users pays this once at startup.
func (m *UserMap) indexNames() {
	entries, _ := os.ReadDir(m.dir)
	for _, ent := range entries {
		id, err := uuid.Parse(strings.TrimSuffix(ent.Name(), ".yml"))
		if err != nil || !strings.HasSuffix(ent.Name(), ".yml") {
			continue
		}
		var head struct {
			Name string `yaml:"last-account-name"`
		}
		b, err := os.ReadFile(filepath.Join(m.dir, ent.Name()))
		if err != nil || yaml.Unmarshal(b, &head) != nil || head.Name == "" {
			continue
		}
		m.byName[strings.ToLower(head.Name)] = id
	}
}

// Get answers the user with this UUID, loading or creating them.
func (m *UserMap) Get(id plugin.UUID) *User {
	if u, ok := m.users[id]; ok {
		return u
	}
	u := &User{ess: m.ess, ID: id, Data: newUserData(), Session: map[string]any{}}
	path := filepath.Join(m.dir, id.String()+".yml")
	if _, err := os.Stat(path); err == nil {
		if err := ReadYAML(path, u.Data); err != nil {
			m.ess.Log.Error("cannot read user data", "file", path, "err", err)
		} else {
			u.Loaded = true
		}
	}
	m.users[id] = u
	if u.Data.LastAccountName != "" {
		m.byName[strings.ToLower(u.Data.LastAccountName)] = id
	}
	return u
}

// Of answers the user of an online player.
func (m *UserMap) Of(p plugin.Player) *User { return m.Get(p.UUID()) }

// Known reports whether the UUID has played here: a data file, or the
// server's own player data.
func (m *UserMap) Known(id plugin.UUID) bool {
	if u, ok := m.users[id]; ok && (u.Loaded || u.Online()) {
		return true
	}
	if _, err := os.Stat(filepath.Join(m.dir, id.String()+".yml")); err == nil {
		return true
	}
	_, ok := m.ess.Server.OfflinePlayer(id)
	return ok
}

// ByName answers the user whose current or last name is name, ignoring
// case: online players first, then the index of saved users.
func (m *UserMap) ByName(name string) (*User, bool) {
	if p, ok := m.ess.Server.PlayerNamed(name); ok {
		return m.Get(p.UUID()), true
	}
	if id, ok := m.byName[strings.ToLower(name)]; ok {
		return m.Get(id), true
	}
	if id, err := uuid.Parse(name); err == nil && m.Known(id) {
		return m.Get(id), true
	}
	return nil, false
}

// Rename records that id now goes by name, for ByName.
func (m *UserMap) Rename(id plugin.UUID, name string) {
	for n, other := range m.byName {
		if other == id && n != strings.ToLower(name) {
			delete(m.byName, n)
		}
	}
	m.byName[strings.ToLower(name)] = id
}

// Names answers every known user's name.
func (m *UserMap) Names() []string {
	out := make([]string, 0, len(m.byName))
	for n := range m.byName {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}

// All answers every saved user's UUID.
func (m *UserMap) All() []plugin.UUID {
	seen := map[plugin.UUID]bool{}
	var out []plugin.UUID
	for _, id := range m.byName {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for id := range m.users {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// SaveAll writes every changed user. The data is encoded on the tick, where
// it may be read safely, and written on another goroutine unless sync.
// Users who are offline and saved are dropped from memory.
func (m *UserMap) SaveAll(sync bool) {
	type file struct {
		path string
		b    []byte
	}
	var files []file
	for id, u := range m.users {
		if u.dirty {
			b, err := yaml.Marshal(u.Data)
			if err != nil {
				m.ess.Log.Error("cannot encode user data", "user", id, "err", err)
				continue
			}
			files = append(files, file{filepath.Join(m.dir, id.String()+".yml"), b})
			u.dirty = false
			u.Loaded = true
		}
		if !u.Online() && u.pending == nil {
			delete(m.users, id)
		}
	}
	if len(files) == 0 {
		return
	}
	write := func() {
		m.writeMu.Lock()
		defer m.writeMu.Unlock()
		for _, f := range files {
			if err := WriteFileAtomic(f.path, f.b); err != nil {
				m.ess.Log.Error("cannot save user data", "file", f.path, "err", err)
			}
		}
	}
	if sync {
		write()
		return
	}
	m.ess.Server.Scheduler().RunAsync(write)
}

// Cleanup deletes users not seen for daysOld days with no more than
// moneyLeft and no homes, as /essentials cleanup does; it answers the count.
func (m *UserMap) Cleanup(daysOld int, moneyLeft float64, homesLeft int) int {
	n := 0
	cutoff := time.Now().AddDate(0, 0, -daysOld).UnixMilli()
	for _, id := range m.All() {
		u := m.Get(id)
		if u.Online() || u.Data.Timestamps.Logout == 0 || u.Data.Timestamps.Logout > cutoff {
			continue
		}
		if f, _ := u.Money().Float64(); f > moneyLeft || len(u.Data.Homes) > homesLeft {
			continue
		}
		_ = os.Remove(filepath.Join(m.dir, id.String()+".yml"))
		delete(m.users, id)
		delete(m.byName, strings.ToLower(u.Data.LastAccountName))
		n++
	}
	return n
}
