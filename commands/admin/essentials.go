package admin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// node is a sub-command of /essentials. Essentials: EssentialsTreeNode
type node struct {
	names  []string
	hidden bool
	// run gets the arguments after the node's name.
	run      func(ctx *core.Ctx, args []string) error
	complete func(ctx *core.Ctx, args []string) []string
}

// essentialsNodes are /essentials' sub-commands. dump (which uploads to
// Essentials' paste service) and itemtest (a developer's tool) are not
// ported. Essentials: Commandessentials
var essentialsNodes = []*node{
	{names: []string{"version", "ver"}, run: runVersion},
	{names: []string{"debug", "verbose"}, run: runDebug, complete: fixed("on", "off")},
	{names: []string{"cmd", "commands"}, run: runCommandMap},
	{names: []string{"reload"}, run: runReload},
	{names: []string{"cleanup"}, run: runCleanup, complete: func(_ *core.Ctx, args []string) []string {
		switch len(args) {
		case 1:
			return commonDurations
		case 2, 3:
			return []string{"-1", "0"}
		}
		return nil
	}},
	{names: []string{"homes"}, run: runHomes, complete: func(ctx *core.Ctx, args []string) []string {
		if len(args) == 1 {
			return []string{"fix", "delete"}
		}
		if len(args) == 2 && strings.EqualFold(args[0], "delete") {
			return core.WorldNames(ctx.Ess.Server)
		}
		return nil
	}},
	{names: []string{"usermap"}, run: runUsermap, complete: fixed("full", "purge", "cache")},
	{names: []string{"nyan", "nya"}, hidden: true, run: runNyan},
	{names: []string{"moo"}, hidden: true, run: runMoo, complete: fixed("moo")},
}

// commonDurations are Essentials' COMMON_DURATIONS.
var commonDurations = []string{"1", "60", "600", "3600", "86400"}

func fixed(words ...string) func(*core.Ctx, []string) []string {
	return func(_ *core.Ctx, args []string) []string {
		if len(args) == 1 {
			return words
		}
		return nil
	}
}

func findNode(name string) *node {
	for _, n := range essentialsNodes {
		if slices.Contains(n.names, name) {
			return n
		}
	}
	return nil
}

func registerEssentials(e *core.Essentials) {
	e.RegisterCommand(core.Cmd{
		Name: "essentials",
		Run: func(ctx *core.Ctx) error {
			if len(ctx.Args) == 0 {
				return core.ErrNotEnoughArgs
			}
			n := findNode(ctx.Args[0])
			if n == nil {
				return core.ErrNotEnoughArgs
			}
			return n.run(ctx, ctx.Args[1:])
		},
		Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 {
				var out []string
				for _, n := range essentialsNodes {
					if !n.hidden {
						out = append(out, n.names...)
					}
				}
				return out
			}
			n := findNode(ctx.Args[0])
			if n == nil {
				return nil
			}
			if n.complete == nil {
				return ctx.CompletePlayers()
			}
			return n.complete(ctx, ctx.Args[1:])
		},
	})
}

// version answers the essentials plugin's version.
func version(e *core.Essentials) string {
	for _, p := range e.Server.Plugins() {
		if p.Name == e.Server.Name() && p.Version != "" {
			return p.Version
		}
	}
	return "unknown"
}

// Essentials: essentials.ReloadCommand
func runReload(ctx *core.Ctx, _ []string) error {
	if err := ctx.Ess.Reload(); err != nil {
		return err
	}
	ctx.Send("essentialsReload", version(ctx.Ess))
	return nil
}

// Essentials: essentials.VersionCommand
func runVersion(ctx *core.Ctx, _ []string) error {
	if isPlayerWithout(ctx, "essentials.version") {
		return nil
	}
	e := ctx.Ess
	name, protocol := e.Server.Version()
	ctx.Send("versionOutputFine", "Server", fmt.Sprintf("%s (protocol %d)", name, protocol))
	ctx.Send("versionOutputFine", "Brand", "golem")
	ver := version(e)
	ctx.Send("versionOutputFine", "EssentialsX", ver)
	mismatched := false
	for _, p := range e.Server.Plugins() {
		if p.Name == e.Server.Name() || !strings.HasPrefix(strings.ToLower(p.Name), "essentials") {
			continue
		}
		if p.Version != ver {
			mismatched = true
			ctx.Send("versionOutputWarn", p.Name, p.Version)
		} else {
			ctx.Send("versionOutputFine", p.Name, p.Version)
		}
	}
	ctx.Send("versionOutputEconLayer", "Essentials")
	if mismatched {
		ctx.Send("versionMismatchAll")
	}
	return nil
}

// Essentials: essentials.DebugCommand
func runDebug(ctx *core.Ctx, args []string) error {
	debug := !ctx.Ess.Settings.Debug()
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "on", "true":
			debug = true
		case "off", "false":
			debug = false
		}
	}
	ctx.Ess.Config.Set("debug", debug)
	state := "disabled"
	if ctx.Ess.Settings.Debug() {
		state = "enabled"
	}
	ctx.SendText(plugin.Plain("Essentials " + version(ctx.Ess) + " debug mode " + state))
	return nil
}

// runCommandMap lists the commands Essentials relays to other plugins:
// its names (and aliases) that another plugin holds, which Essentials then
// leaves to it. Vanilla commands Essentials overrides are not listed, as in
// Essentials. Essentials: essentials.CommandMapCommand
func runCommandMap(ctx *core.Ctx, _ []string) error {
	relayed := relayedCommands(ctx.Ess)
	if len(relayed) == 0 {
		ctx.Send("blockListEmpty")
		return nil
	}
	ctx.Send("blockList")
	for _, r := range relayed {
		ctx.SendText(plugin.Plain(r))
	}
	return nil
}

// relayedCommands answers "name => plugin:command" for each name in
// Essentials' plugin.yml that a plugin other than Essentials and its
// modules holds, sorted. Essentials: AlternativeCommandsHandler
func relayedCommands(e *core.Essentials) []string {
	var yml struct {
		Commands map[string]struct {
			Aliases []string `yaml:"aliases"`
		} `yaml:"commands"`
	}
	if err := yaml.Unmarshal(core.Resource("plugin.yml"), &yml); err != nil {
		return nil
	}
	owner := map[string]string{}
	for _, c := range e.Server.Commands().All() {
		if c.Plugin == "" || strings.HasPrefix(strings.ToLower(c.Plugin), "essentials") {
			continue
		}
		for _, n := range append([]string{c.Name}, c.Aliases...) {
			if _, ok := owner[n]; !ok {
				owner[n] = c.Plugin + ":" + c.Name
			}
		}
	}
	var out []string
	seen := map[string]bool{}
	for name, c := range yml.Commands {
		for _, n := range append([]string{name}, c.Aliases...) {
			n = strings.ToLower(n)
			if alt, ok := owner[n]; ok && !seen[n] {
				seen[n] = true
				out = append(out, n+" => "+alt)
			}
		}
	}
	slices.Sort(out)
	return out
}

var notDigits = regexp.MustCompile(`[^0-9.]`)

// Essentials: essentials.CleanupCommand
func runCleanup(ctx *core.Ctx, args []string) error {
	if len(args) < 1 || !core.IsInt(args[0]) {
		ctx.SendText(plugin.Plain("This sub-command will delete users who haven't logged in in the last <days> days."))
		ctx.SendText(plugin.Plain("Optional parameters define the minimum amount required to prevent deletion."))
		ctx.SendText(plugin.Plain("Unless you define larger default values, this command will ignore people who have more than 0 money/homes."))
		return errors.New("/" + ctx.Label + " cleanup <days> [money] [homes]")
	}
	ctx.Send("cleaning")
	days, _ := strconv.Atoi(args[0])
	money := 0.0
	if len(args) >= 2 {
		money, _ = strconv.ParseFloat(notDigits.ReplaceAllString(args[1], ""), 64)
	}
	homes := 0
	if len(args) >= 3 && core.IsInt(args[2]) {
		homes, _ = strconv.Atoi(args[2])
	}
	n := ctx.Ess.Users.Cleanup(days, money, homes)
	if ctx.Ess.Settings.Debug() {
		ctx.Ess.Log.Info("cleaned up users", "count", n)
	}
	ctx.Send("cleaned")
	return nil
}

const homesUsage = "/<command> homes (fix | delete [world])"

// Essentials: essentials.HomesCommand
func runHomes(ctx *core.Ctx, args []string) error {
	if len(args) < 1 {
		for _, l := range []string{
			"This sub-command provides a utility to mass-delete homes based on user options:",
			`Use "fix" to delete all homes inside non-existent or unloaded worlds.`,
			`Use "delete" to delete all existing homes.`,
			`Use "delete <worldname>" to delete all homes inside a specific world.`,
		} {
			ctx.SendText(plugin.Plain(l))
		}
		return errors.New(homesUsage)
	}
	e := ctx.Ess
	eachHome := func(del func(core.LazyLocation) bool) {
		for _, id := range e.Users.All() {
			u := e.Users.Get(id)
			for _, name := range u.HomeNames() {
				if l, ok := u.Home(name); ok && del(l) {
					u.DelHome(name)
				}
			}
		}
	}
	switch args[0] {
	case "fix":
		ctx.Send("fixingHomes")
		eachHome(func(l core.LazyLocation) bool {
			_, ok := l.Resolve(e.Server)
			return !ok
		})
		ctx.Send("fixedHomes")
	case "delete":
		if len(args) < 2 {
			ctx.Send("deletingHomes")
			eachHome(func(core.LazyLocation) bool { return true })
			ctx.Send("deletedHomes")
			return nil
		}
		w, ok := core.WorldByName(e.Server, args[1])
		if !ok {
			return core.Tlf("invalidWorld")
		}
		ctx.Send("deletingHomesWorld", args[1])
		eachHome(func(l core.LazyLocation) bool {
			loc, ok := l.Resolve(e.Server)
			return ok && loc.World.Key() == w.Key()
		})
		ctx.Send("deletedHomesWorld", args[1])
	default:
		return errors.New(homesUsage)
	}
	return nil
}

// Essentials: essentials.UsermapCommand
func runUsermap(ctx *core.Ctx, args []string) error {
	if !ctx.Has("essentials.usermap") {
		return nil
	}
	e := ctx.Ess
	all := e.Users.All()
	names := e.Users.Names()
	ctx.Send("usermapSize", len(e.Server.Players()), len(all), e.Config.Int("max-user-cache-count", 20000))
	if len(args) == 0 {
		return nil
	}
	idOf := func(name string) (plugin.UUID, bool) {
		u, ok := e.Users.ByName(name)
		if !ok {
			return plugin.UUID{}, false
		}
		return u.ID, true
	}
	switch {
	case args[0] == "full":
		for _, n := range names {
			if id, ok := idOf(n); ok {
				ctx.Send("usermapEntry", n, id.String())
			}
		}
	case args[0] == "purge":
		destroy := len(args) > 1 && args[1] == "iknowwhatimdoing"
		ctx.Send("usermapPurge", strconv.FormatBool(destroy))
		known := map[plugin.UUID]bool{}
		for _, id := range all {
			known[id] = true
		}
		dir := e.DataPath("userdata")
		backup := e.DataPath(fmt.Sprintf("userdata-npc-backup-boogaloo-%d", time.Now().UnixMilli()))
		log := e.Log
		e.Server.Scheduler().RunAsync(func() { purgeUserdata(log, dir, backup, known, destroy) })
	case strings.EqualFold(args[0], "cache"):
		ctx.Send("usermapKnown", len(all), len(names))
	default:
		if id, err := uuid.Parse(args[0]); err == nil {
			for _, n := range names {
				if got, ok := idOf(n); ok && got == id {
					ctx.Send("usermapEntry", n, args[0])
				}
			}
			return nil
		}
		name := strings.ToLower(args[0])
		id, ok := idOf(name)
		if !ok {
			return core.ErrPlayerNotFound
		}
		ctx.Send("usermapEntry", name, id.String())
	}
	return nil
}

// purgeUserdata logs (and with destroy moves to backup) the userdata files
// no known user has.
func purgeUserdata(log *plugin.Logger, dir, backup string, known map[plugin.UUID]bool, destroy bool) {
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		log.Warn("Missing userdata folder, aborting usermap purge.")
		return
	}
	if destroy {
		if err := os.Mkdir(backup, 0o755); err != nil {
			log.Warn("Unable to create backup folder, aborting usermap purge.")
			return
		}
	}
	entries, _ := os.ReadDir(dir)
	total := 0
	for _, ent := range entries {
		name := ent.Name()
		if !strings.HasSuffix(name, ".yml") {
			continue
		}
		id, err := uuid.Parse(strings.TrimSuffix(name, ".yml"))
		if err != nil || known[id] {
			continue
		}
		total++
		log.Warn("Found orphaned userdata file: " + name)
		if destroy {
			if err := os.Rename(filepath.Join(dir, name), filepath.Join(backup, name)); err != nil {
				log.Warn("Unable to move orphaned userdata file: "+name, "err", err)
			}
		}
	}
	log.Info(fmt.Sprintf("Found %d orphaned userdata files.", total))
}

var (
	consoleMoo = []string{"         (__)", "         (oo)", "   /------\\/", "  / |    ||", " *  /\\---/\\", "    ~~   ~~", "....\"Have you mooed today?\"..."}
	playerMoo  = []string{"            (__)", "            (oo)", "   /------\\/", "  /  |      | |", " *  /\\---/\\", "    ~~    ~~", "....\"Have you mooed today?\"..."}
)

// moo moos at p.
func moo(e *core.Essentials, p plugin.Player) {
	for _, l := range playerMoo {
		p.SendMessage(plugin.Plain(l))
	}
	if s, ok := e.Server.Registries().Sound(plugin.MinecraftKey("entity.cow.milk")); ok {
		p.PlaySound(p.Location().Pos, s, plugin.SoundMaster, 1, 1)
	}
}

// Essentials: essentials.MooCommand
func runMoo(ctx *core.Ctx, args []string) error {
	e := ctx.Ess
	if len(args) == 1 && args[0] == "moo" {
		for _, l := range consoleMoo {
			e.Log.Info(l)
		}
		for _, p := range e.Server.Players() {
			moo(e, p)
		}
		return nil
	}
	if ctx.Player != nil {
		moo(e, ctx.Player)
		return nil
	}
	for _, l := range consoleMoo {
		ctx.SendText(plugin.Plain(l))
	}
	return nil
}

const nyanTune = "1D#,1E,2F#,,2A#,1E,1D#,1E,2F#,2B,2D#,2E,2D#,2A#,2B,,2F#,,1D#,1E,2F#,2B,2C#,2A#,2B,2C#,2E,2D#,2E,2C#,,2F#,,2G#,,1D,1D#,,1C#,1D,1C#,1B,,1B,,1C#,,1D,,1D,1C#,1B,1C#,1D#,2F#,2G#,1D#,2F#,1C#,1D#,1B,1C#,1B,1D#,,2F#,,2G#,1D#,2F#,1C#,1D#,1B,1D,1D#,1D,1C#,1B,1C#,1D,,1B,1C#,1D#,2F#,1C#,1D,1C#,1B,1C#,,1B,,1C#,,2F#,,2G#,,1D,1D#,,1C#,1D,1C#,1B,,1B,,1C#,,1D,,1D,1C#,1B,1C#,1D#,2F#,2G#,1D#,2F#,1C#,1D#,1B,1C#,1B,1D#,,2F#,,2G#,1D#,2F#,1C#,1D#,1B,1D,1D#,1D,1C#,1B,1C#,1D,,1B,1C#,1D#,2F#,1C#,1D,1C#,1B,1C#,,1B,,1B,,1B,,1F#,1G#,1B,,1F#,1G#,1B,1C#,1D#,1B,1E,1D#,1E,2F#,1B,,1B,,1F#,1G#,1B,1E,1D#,1C#,1B,,,,1F#,1B,,1F#,1G#,1B,,1F#,1G#,1B,1B,1C#,1D#,1B,1F#,1G#,1F#,1B,,1B,1A#,1B,1F#,1G#,1B,1E,1D#,1E,2F#,1B,,1A#,,1B,,1F#,1G#,1B,,1F#,1G#,1B,1C#,1D#,1B,1E,1D#,1E,2F#,1B,,1B,,1F#,1G#,1B,1F#,1E,1D#,1C#,1B,,,,1F#,1B,,1F#,1G#,1B,,1F#,1G#,1B,1B,1C#,1D#,1B,1F#,1G#,1F#,1B,,1B,1A#,1B,1F#,1G#,1B,1E,1D#,1E,2F#,1B,,1A#,,1B,,1F#,1G#,1B,,1F#,1G#,1B,1C#,1D#,1B,1E,1D#,1E,2F#,1B,,1B,,1F#,1G#,1B,1F#,1E,1D#,1C#,1B,,,,1F#,1B,,1F#,1G#,1B,,1F#,1G#,1B,1B,1C#,1D#,1B,1F#,1G#,1F#,1B,,1B,1A#,1B,1F#,1G#,1B,1E,1D#,1E,2F#,1B,,1A#,,1B,,1F#,1G#,1B,,1F#,1G#,1B,1C#,1D#,1B,1E,1D#,1E,2F#,1B,,1B,,1F#,1G#,1B,1F#,1E,1D#,1C#,1B,,,,1F#,1B,,1F#,1G#,1B,,1F#,1G#,1B,1B,1C#,1D#,1B,1F#,1G#,1F#,1B,,1B,1A#,1B,1F#,1G#,1B,1E,1D#,1E,2F#,1B,,1A#,,1B,,1F#,1G#,1B,,1F#,1G#,1B,1C#,1D#,1B,1E,1D#,1E,2F#,1B,,1B,,1F#,1G#,1B,1F#,1E,1D#,1C#,1B,,,,1F#,1B,,1F#,1G#,1B,,1F#,1G#,1B,1B,1C#,1D#,1B,1F#,1G#,1F#,1B,,1B,1A#,1B,1F#,1G#,1B,1E,1D#,1E,2F#,1B,,1B,,"

var notePitch = map[string]float32{
	"1F#": 0.5, "1G": 0.53, "1G#": 0.56, "1A": 0.6, "1A#": 0.63, "1B": 0.67, "1C": 0.7, "1C#": 0.76,
	"1D": 0.8, "1D#": 0.84, "1E": 0.9, "1F": 0.94, "2F#": 1.0, "2G": 1.06, "2G#": 1.12, "2A": 1.18,
	"2A#": 1.26, "2B": 1.34, "2C": 1.42, "2C#": 1.5, "2D": 1.6, "2D#": 1.68, "2E": 1.78, "2F": 1.88,
}

// nyanTask is the tune playing, cancelled by the next /essentials nyan.
var nyanTask plugin.Task

// Essentials: essentials.NyanCommand
func runNyan(ctx *core.Ctx, _ []string) error {
	e := ctx.Ess
	if nyanTask != nil {
		nyanTask.Cancel()
	}
	harp, ok := e.Server.Registries().Sound(plugin.MinecraftKey("block.note_block.harp"))
	if !ok {
		return nil
	}
	tune := strings.Split(nyanTune, ",")
	i := 0
	var task plugin.Task
	task = e.Server.Scheduler().RunTimer(20, 2, func() {
		note := tune[i]
		i++
		if i >= len(tune) {
			task.Cancel()
		}
		if note == "" {
			return
		}
		for _, p := range e.Server.Players() {
			p.PlaySound(p.Location().Pos, harp, plugin.SoundMaster, 1, notePitch[note])
		}
	})
	nyanTask = task
	return nil
}
