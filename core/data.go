package core

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/shopspring/decimal"
)

// Warps are the warps, one file each in warps/<name>.yml.
// Essentials: Warps
type Warps struct {
	ess   *Essentials
	dir   string
	warps map[string]warpFile // lower-case name
}

type warpFile struct {
	LazyLocation `yaml:",inline"`
	Name         string `yaml:"name"`
	LastOwner    string `yaml:"lastowner,omitempty"`
}

func newWarps(e *Essentials) (*Warps, error) {
	w := &Warps{ess: e, dir: e.DataPath("warps")}
	return w, w.reload()
}

func (w *Warps) reload() error {
	w.warps = map[string]warpFile{}
	_ = os.MkdirAll(w.dir, 0o755)
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return err
	}
	for _, ent := range entries {
		if !strings.HasSuffix(ent.Name(), ".yml") {
			continue
		}
		var f warpFile
		if err := ReadYAML(filepath.Join(w.dir, ent.Name()), &f); err != nil {
			w.ess.Log.Warn("cannot read warp", "file", ent.Name(), "err", err)
			continue
		}
		if f.Name == "" {
			f.Name = strings.TrimSuffix(ent.Name(), ".yml")
		}
		w.warps[strings.ToLower(f.Name)] = f
	}
	return nil
}

// Names answers the warp names, sorted.
func (w *Warps) Names() []string {
	out := make([]string, 0, len(w.warps))
	for _, f := range w.warps {
		out = append(out, f.Name)
	}
	slices.SortFunc(out, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	return out
}

// Get answers a warp's location; warpNotExist when there is none, and
// warpNotExist too when its world is not loaded.
func (w *Warps) Get(name string) (plugin.Location, error) {
	f, ok := w.warps[strings.ToLower(name)]
	if !ok {
		return plugin.Location{}, Tlf("warpNotExist")
	}
	loc, ok := f.Resolve(w.ess.Server)
	if !ok {
		return plugin.Location{}, Tlf("warpNotExist")
	}
	return loc, nil
}

// Owner answers who last set the warp.
func (w *Warps) Owner(name string) (string, bool) {
	f, ok := w.warps[strings.ToLower(name)]
	return f.LastOwner, ok && f.LastOwner != ""
}

// Exists reports whether there is a warp of that name.
func (w *Warps) Exists(name string) bool { _, ok := w.warps[strings.ToLower(name)]; return ok }

// Set saves a warp, by owner (nil for the console).
func (w *Warps) Set(name string, loc plugin.Location, owner *User) error {
	if name == "" || strings.ContainsAny(name, `/\.:`) {
		return Tlf("invalidWarpName")
	}
	f := warpFile{LazyLocation: LazyOf(loc), Name: name}
	if owner != nil {
		f.LastOwner = owner.ID.String()
	}
	if err := WriteYAML(filepath.Join(w.dir, w.fileName(name)), f); err != nil {
		return err
	}
	w.warps[strings.ToLower(name)] = f
	return nil
}

// Delete removes a warp.
func (w *Warps) Delete(name string) error {
	f, ok := w.warps[strings.ToLower(name)]
	if !ok {
		return Tlf("warpNotExist")
	}
	if err := os.Remove(filepath.Join(w.dir, w.fileName(f.Name))); err != nil && !os.IsNotExist(err) {
		return Tlf("warpDeleteError")
	}
	delete(w.warps, strings.ToLower(name))
	return nil
}

func (w *Warps) fileName(name string) string { return strings.ToLower(name) + ".yml" }

// Kit is a kit from kits.yml. Essentials: Kit
type Kit struct {
	Name string
	// Delay is the seconds between uses; negative is once only.
	Delay float64
	Items []string
}

// Kits are kits.yml. Essentials: Kits
type Kits struct {
	ess  *Essentials
	conf *Config
}

func newKits(e *Essentials) (*Kits, error) {
	k := &Kits{ess: e}
	return k, k.reload()
}

func (k *Kits) reload() error {
	c, err := LoadConfig(k.ess.DataPath("kits.yml"), "kits.yml")
	if err != nil {
		return err
	}
	// The bundled kits are an example; a server's file decides which exist.
	c.defaults = map[string]any{}
	k.conf = c
	return nil
}

// Names answers the kit names, sorted.
func (k *Kits) Names() []string {
	names := k.conf.Keys("kits")
	slices.Sort(names)
	return names
}

// Get answers the kit named name, ignoring case.
func (k *Kits) Get(name string) (*Kit, bool) {
	for _, n := range k.conf.Keys("kits") {
		if strings.EqualFold(n, name) {
			s, _ := k.conf.Section("kits." + n)
			return &Kit{Name: n, Delay: s.Float("delay", 0), Items: s.StringList("items")}, true
		}
	}
	return nil, false
}

// Add saves a kit, as /createkit does.
func (k *Kits) Add(name string, delay float64, items []string) error {
	k.conf.Set("kits."+strings.ToLower(name), map[string]any{"delay": delay, "items": items})
	return k.conf.Save()
}

// Remove deletes a kit.
func (k *Kits) Remove(name string) error {
	kit, ok := k.Get(name)
	if !ok {
		return Tlf("kitNotFound")
	}
	k.conf.Set("kits."+kit.Name, nil)
	return k.conf.Save()
}

// NextUse answers when the user may take kit again: zero for now, a time,
// or ok false for never (a one-time kit already taken).
// Essentials: Kit.getNextUse
func (k *Kit) NextUse(u *User) (time.Time, bool) {
	if u.HasPermission("essentials.kit.exemptdelay") {
		return time.Time{}, true
	}
	last := u.KitTimestamp(k.Name)
	now := time.Now()
	if last == 0 || last > now.UnixMilli() {
		return time.Time{}, true
	}
	if k.Delay < 0 {
		return time.Time{}, false
	}
	next := time.UnixMilli(last).Add(time.Duration(k.Delay * float64(time.Second)))
	if next.Before(now) {
		return time.Time{}, true
	}
	return next, true
}

// CheckDelay answers kitOnce or kitTimed while the user must wait.
// Essentials: Kit.checkDelay
func (k *Kit) CheckDelay(u *User) error {
	next, ok := k.NextUse(u)
	if !ok {
		return TlPlain("kitOnce")
	}
	if !next.IsZero() {
		return TlPlain("kitTimed", FormatDateDiff(next))
	}
	return nil
}

// CheckPerms answers noKitPermission unless the user has
// essentials.kits.<name>.
func (k *Kit) CheckPerms(u *User) error {
	if !u.HasPermission("essentials.kits." + strings.ToLower(k.Name)) {
		return Tlf("noKitPermission", "essentials.kits."+strings.ToLower(k.Name))
	}
	return nil
}

// Expand gives the user the kit's contents: items (with meta), money lines
// starting with $ and command lines starting with /. With
// drop-items-if-full off, a kit that does not fit is not given and answers
// kitInvFullNoDrop. Essentials: Kit.expandItems
func (k *Kits) Expand(kit *Kit, u *User) error {
	p, ok := u.Player()
	if !ok {
		return ErrPlayerNotFound
	}
	e := k.ess
	sym := e.Settings.CurrencySymbol()
	if sym == "" {
		sym = "$"
	}
	var stacks []plugin.ItemStack
	var cmds []string
	var money []decimal.Decimal
	for _, line := range kit.Items {
		line = e.ReplaceKeywords(line, u)
		switch {
		case strings.HasPrefix(line, "$") || strings.HasPrefix(line, sym):
			if d, err := e.Economy.ParseMoney(strings.TrimPrefix(strings.TrimPrefix(line, "$"), sym)); err == nil {
				money = append(money, d)
			}
			continue
		case strings.HasPrefix(line, "/"):
			cmds = append(cmds, strings.ReplaceAll(line[1:], "{player}", u.Name()))
			continue
		case strings.HasPrefix(line, "@"):
			e.Log.Warn(e.TlString("kitError3", kit.Name, u.Name()))
			continue
		}
		if strings.HasPrefix(line, "slot:") {
			if _, rest, ok := strings.Cut(line, " "); ok {
				line = rest
			}
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		s, err := e.Items.Get(parts[0])
		if err != nil {
			return err
		}
		s.Count = 1
		if len(parts) > 1 {
			if n, err := strconv.Atoi(parts[1]); err == nil {
				s.Count = n
			}
		}
		if len(parts) > 2 {
			m := e.NewMetaItem(s, nil)
			if err := m.Parse(parts, 2, e.Settings.UnsafeEnchantments()); err != nil {
				return err
			}
			s = m.Stack
		}
		stacks = append(stacks, s)
	}
	drop := e.Settings.DropItemsIfFull()
	if !drop && !e.HasSpace(p, stacks) {
		return TlPlain("kitInvFullNoDrop")
	}
	if e.Config.Bool("kit-auto-equip", false) {
		stacks = autoEquip(p, stacks)
	}
	if left := e.GiveItems(p, false, stacks...); len(left) > 0 {
		for _, l := range left {
			p.World().DropItem(p.Pos(), l, true)
		}
		u.Send("kitInvFull")
	}
	for _, m := range money {
		_ = (&Trade{Money: &m}).Pay(u)
	}
	for _, c := range cmds {
		_, _ = e.Server.Dispatch(e.Server.Console(), c)
	}
	return nil
}

// HasSpace reports whether every stack fits in the player's inventory, by
// trying it on a copy of the contents.
func (e *Essentials) HasSpace(p plugin.Player, stacks []plugin.ItemStack) bool {
	inv := p.Inventory()
	contents := inv.Contents()
	free := 0
	for i, s := range contents {
		if i >= 36 {
			break
		}
		if s.Empty() {
			free++
		}
	}
	need := 0
	for _, s := range stacks {
		left := s.Count
		maxSize := max(e.Server.Items().MaxStackSize(s), 1)
		for i, c := range contents {
			if i >= 36 || left <= 0 {
				break
			}
			if !c.Empty() && e.Server.Items().Similar(c, s) && c.Count < maxSize {
				left -= maxSize - c.Count
			}
		}
		if left > 0 {
			need += (left + maxSize - 1) / maxSize
		}
	}
	return need <= free
}

// autoEquip puts armour into empty armour slots and answers the rest.
func autoEquip(p plugin.Player, stacks []plugin.ItemStack) []plugin.ItemStack {
	var rest []plugin.ItemStack
	inv := p.Inventory()
	for _, s := range stacks {
		if slot, ok := armorSlot(s); ok && inv.Get(slot).Empty() {
			inv.Set(slot, s)
			continue
		}
		rest = append(rest, s)
	}
	return rest
}

// armorSlot answers the slot an armour piece goes in, from its item name.
func armorSlot(s plugin.ItemStack) (plugin.EquipmentSlot, bool) {
	n := s.Item.Name()
	switch {
	case strings.HasSuffix(n, "_helmet") || strings.HasSuffix(n, "_head") || strings.HasSuffix(n, "_skull") || n == "minecraft:carved_pumpkin":
		return plugin.SlotHead, true
	case strings.HasSuffix(n, "_chestplate") || n == "minecraft:elytra":
		return plugin.SlotChest, true
	case strings.HasSuffix(n, "_leggings"):
		return plugin.SlotLegs, true
	case strings.HasSuffix(n, "_boots"):
		return plugin.SlotFeet, true
	}
	return 0, false
}

// Jails are jail.yml. Essentials: Jails
type Jails struct {
	ess  *Essentials
	conf *Config
}

func newJails(e *Essentials) (*Jails, error) {
	j := &Jails{ess: e}
	if err := j.reload(); err != nil {
		return nil, err
	}
	j.listen()
	return j, nil
}

func (j *Jails) reload() error {
	c, err := LoadConfig(j.ess.DataPath("jail.yml"), "")
	if err != nil {
		return err
	}
	j.conf = c
	return nil
}

// Names answers the jail names, sorted.
func (j *Jails) Names() []string {
	n := j.conf.Keys("jails")
	slices.Sort(n)
	return n
}

// Get answers a jail's location.
func (j *Jails) Get(name string) (plugin.Location, error) {
	s, ok := j.conf.Section("jails." + strings.ToLower(name))
	if !ok {
		return plugin.Location{}, Tlf("jailNotExist")
	}
	l := LazyLocation{World: s.String("world", ""), WorldName: s.String("world-name", ""), X: s.Float("x", 0), Y: s.Float("y", 0),
		Z: s.Float("z", 0), Yaw: float32(s.Float("yaw", 0)), Pitch: float32(s.Float("pitch", 0))}
	loc, ok := l.Resolve(j.ess.Server)
	if !ok {
		return plugin.Location{}, Tlf("jailWorldNotExist")
	}
	return loc, nil
}

// Set saves a jail.
func (j *Jails) Set(name string, loc plugin.Location) error {
	l := LazyOf(loc)
	j.conf.Set("jails."+strings.ToLower(name), map[string]any{"world": l.World, "world-name": l.WorldName,
		"x": l.X, "y": l.Y, "z": l.Z, "yaw": float64(l.Yaw), "pitch": float64(l.Pitch)})
	return j.conf.Save()
}

// Remove deletes a jail.
func (j *Jails) Remove(name string) error {
	if _, ok := j.conf.Section("jails." + strings.ToLower(name)); !ok {
		return Tlf("jailNotExist")
	}
	j.conf.Set("jails."+strings.ToLower(name), nil)
	return j.conf.Save()
}

// Count answers how many jails there are. Essentials: Jails.getCount
func (j *Jails) Count() int { return len(j.conf.Keys("jails")) }

// SendToJail teleports the user to their jail, if online.
func (j *Jails) SendToJail(u *User, jail string) error {
	loc, err := j.Get(jail)
	if err != nil {
		return err
	}
	if p, ok := u.Player(); ok {
		if !j.ess.Settings.RegisterBackInListener() {
			u.SetLastLocation(p.Location())
		}
		p.Teleport(loc, plugin.TeleportCommand)
	}
	return nil
}

// Release frees the user: teleport-when-freed decides where they go (back,
// spawn or off). Essentials: User.checkJailTimeout / Commandtogglejail
func (j *Jails) Release(u *User) {
	u.Data.Jailed = false
	u.Data.Jail = ""
	u.Data.Timestamps.Jail = 0
	u.Data.Timestamps.OnlineJail = 0
	u.Save()
	p, ok := u.Player()
	if !ok {
		return
	}
	u.Send("haveBeenReleased")
	switch strings.ToLower(j.ess.Config.String("teleport-when-freed", "back")) {
	case "back":
		if loc, ok := u.LastLocation(); ok {
			_ = u.TeleportNow(LocationTarget(loc), plugin.TeleportPlugin)
		}
	case "spawn":
		if loc, ok := j.ess.SpawnFor(u); ok {
			_ = u.TeleportNow(LocationTarget(loc), plugin.TeleportPlugin)
		} else {
			p.Teleport(p.World().SpawnPoint(), plugin.TeleportPlugin)
		}
	}
}

// listen keeps jailed players from building, fighting, changing game mode
// and leaving. Essentials: Jails.JailListener
func (j *Jails) listen() {
	s := j.ess.Server
	low := plugin.ListenOptions{Priority: plugin.Low, IgnoreCancelled: true}
	jailed := func(p plugin.Player, allow string) bool {
		u := j.ess.Users.Of(p)
		return u.Data.Jailed && (allow == "" || !u.HasPermission(allow))
	}
	plugin.Listen(s, func(e *plugin.BlockBreakEvent) {
		if jailed(e.Player, "essentials.jail.allow-break") {
			e.SetCancelled(true)
		}
	}, low)
	plugin.Listen(s, func(e *plugin.BlockPlaceEvent) {
		if jailed(e.Player, "essentials.jail.allow-place") {
			e.SetCancelled(true)
		}
	}, low)
	plugin.Listen(s, func(e *plugin.BlockDamageEvent) {
		if jailed(e.Player, "essentials.jail.allow-block-damage") {
			e.SetCancelled(true)
		}
	}, low)
	plugin.Listen(s, func(e *plugin.PlayerInteractEvent) {
		if jailed(e.Player, "essentials.jail.allow-interact") {
			e.SetCancelled(true)
		}
	}, low)
	plugin.Listen(s, func(e *plugin.PlayerAttackEvent) {
		if _, ok := e.Target.(plugin.Player); ok && jailed(e.Player, "") {
			e.SetCancelled(true)
		}
	}, low)
	plugin.Listen(s, func(e *plugin.PlayerGameModeChangeEvent) {
		if jailed(e.Player, "") {
			e.SetCancelled(true)
		}
	}, low)
	plugin.Listen(s, func(e *plugin.PlayerRespawnEvent) {
		u := j.ess.Users.Of(e.Player)
		if u.Data.Jailed && u.Data.Jail != "" {
			if loc, err := j.Get(u.Data.Jail); err == nil {
				e.Location = loc
			}
		}
	}, plugin.ListenOptions{Priority: plugin.Highest})
	plugin.Listen(s, func(e *plugin.EntityTeleportEvent) {
		p, ok := e.Entity.(plugin.Player)
		if !ok {
			return
		}
		u := j.ess.Users.Of(p)
		if !u.Data.Jailed || u.Data.Jail == "" {
			return
		}
		if loc, err := j.Get(u.Data.Jail); err == nil {
			e.To = loc
		}
		u.Send("jailMessage")
	}, plugin.ListenOptions{Priority: plugin.High, IgnoreCancelled: true})
	plugin.Listen(s, func(e *plugin.PlayerJoinEvent) {
		u := j.ess.Users.Of(e.Player)
		if !u.Data.Jailed || u.Data.Jail == "" {
			return
		}
		if loc, err := j.Get(u.Data.Jail); err == nil {
			e.Player.Teleport(loc, plugin.TeleportPlugin)
		}
		u.Send("jailMessage")
	}, plugin.ListenOptions{Priority: plugin.Highest})
}

// checkTimeout releases a user whose jail time is up; with
// jail-online-time it counts only the time they are online.
func (j *Jails) checkTimeout(u *User) {
	if !u.Data.Jailed || u.Data.Timestamps.Jail == 0 {
		return
	}
	// With jail-online-time, onlinejail is the play_time at which the
	// sentence is served, and timestamps.jail only a duration.
	if u.Data.Timestamps.OnlineJail > 0 && int64(u.PlayTime()) < u.Data.Timestamps.OnlineJail {
		return
	}
	if time.Now().UnixMilli() >= u.Data.Timestamps.Jail {
		j.Release(u)
	}
}

// Worth is worth.yml: what /sell pays per item. Essentials: Worth
type Worth struct {
	ess  *Essentials
	conf *Config
}

func newWorth(e *Essentials) (*Worth, error) {
	w := &Worth{ess: e}
	return w, w.reload()
}

func (w *Worth) reload() error {
	c, err := LoadConfig(w.ess.DataPath("worth.yml"), "worth.yml")
	if err != nil {
		return err
	}
	c.defaults = map[string]any{}
	w.conf = c
	return nil
}

// worthKey is the item's worth.yml key: its name without underscores.
func worthKey(s plugin.ItemStack) string {
	return strings.ReplaceAll(strings.TrimPrefix(s.Item.Name(), "minecraft:"), "_", "")
}

// Price answers one item's worth; false when it has none.
// Essentials: Worth.getPrice
func (w *Worth) Price(s plugin.ItemStack) (decimal.Decimal, bool) {
	key := "worth." + worthKey(s)
	for _, k := range []string{key + ".0", key + ".*", key} {
		if k == key+".0" && len(w.conf.Keys(key)) != 1 {
			continue
		}
		v, ok := w.conf.Get(k)
		if !ok {
			continue
		}
		if d, err := decimal.NewFromString(fmt.Sprint(v)); err == nil && d.Sign() >= 0 {
			return d, true
		}
	}
	return decimal.Zero, false
}

// SetPrice sets an item's worth.
func (w *Worth) SetPrice(s plugin.ItemStack, price decimal.Decimal) error {
	f, _ := price.Float64()
	w.conf.Set("worth."+worthKey(s), f)
	return w.conf.Save()
}
