package core

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
	"gopkg.in/yaml.v3"
)

// Cmd is an Essentials command: a Bukkit-style executor over the words the
// sender typed. Its name is the key of plugin.yml's commands, which gives its
// aliases, and its permission is essentials.<name>. Essentials:
// EssentialsCommand
type Cmd struct {
	Name string
	// Run runs the command. A *User sender has ctx.User set; the console
	// and command blocks have it nil.
	Run func(ctx *Ctx) error
	// PlayerOnly refuses the console with onlyPlayers.
	PlayerOnly bool
	// Complete answers the words that may complete ctx.Args' last element
	// (the partial word, possibly empty). Nil completes player names.
	Complete func(ctx *Ctx) []string
	// Permission overrides essentials.<name>; "-" for none.
	Permission string
}

// Ctx is one run (or completion) of a command.
type Ctx struct {
	Ess    *Essentials
	Sender plugin.CommandSender
	// User and Player are the sender's when a player sends it; nil otherwise.
	User   *User
	Player plugin.Player
	// Label is the name the sender typed; Args the words after it.
	Label string
	Args  []string
	// Charge is the command's cost from command-costs, charged after Run
	// unless Run answers ErrNoCharge.
	Charge *Trade
	cmd    *Cmd
	pc     *plugin.CommandContext
}

// TlError is an error shown as a translated message. Tlf makes one.
type TlError struct {
	Key  string
	Args []any
	// Plain shows the message as it is rather than through errorWithMessage.
	Plain bool
}

func (e *TlError) Error() string { return e.Key }

// Tlf answers an error that shows the message key, as Essentials'
// TranslatableException does: wrapped in errorWithMessage.
func Tlf(key string, args ...any) error { return &TlError{Key: key, Args: args} }

// TlPlain answers an error that shows the message key unwrapped.
func TlPlain(key string, args ...any) error { return &TlError{Key: key, Args: args, Plain: true} }

var (
	// ErrNotEnoughArgs shows the command's usage.
	ErrNotEnoughArgs = errors.New("not enough arguments")
	// ErrNoCharge ends the command without charging its cost.
	ErrNoCharge = errors.New("no charge")
	// ErrQuiet ends the command silently, without charging.
	ErrQuiet = errors.New("quiet abort")
	// ErrPlayerNotFound shows playerNotFound.
	ErrPlayerNotFound = &TlError{Key: "playerNotFound", Plain: true}
)

// Send sends the sender the message key.
func (c *Ctx) Send(key string, args ...any) { c.Ess.Send(c.Sender, key, args...) }

// SendText sends the sender msg.
func (c *Ctx) SendText(msg plugin.Text) { c.Sender.SendMessage(msg) }

// Has reports whether the sender has node; the console has everything.
func (c *Ctx) Has(node string) bool {
	if c.Player == nil {
		return true
	}
	return c.Player.HasPermission(node)
}

// IsPlayer reports whether a player sent the command.
func (c *Ctx) IsPlayer() bool { return c.User != nil }

// Arg answers argument i, or "".
func (c *Ctx) Arg(i int) string {
	if i < len(c.Args) {
		return c.Args[i]
	}
	return ""
}

// FinalArg answers the arguments from start on, joined with spaces.
// Essentials: getFinalArg
func (c *Ctx) FinalArg(start int) string {
	if start >= len(c.Args) {
		return ""
	}
	return strings.Join(c.Args[start:], " ")
}

// SenderName answers the sender's display name: the user's nickname, or
// Console.
func (c *Ctx) SenderName() string {
	if c.User != nil {
		return c.User.DisplayName()
	}
	return c.Ess.TlString("consoleName")
}

// RequirePlayer refuses the console with onlyPlayers.
func (c *Ctx) RequirePlayer() error {
	if c.User == nil {
		return TlPlain("onlyPlayers", c.Label)
	}
	return nil
}

// GetPlayer answers the online user args[pos] names. Essentials: getPlayer
func (c *Ctx) GetPlayer(pos int, offline bool) (*User, error) {
	if pos >= len(c.Args) || c.Args[pos] == "" {
		return nil, ErrNotEnoughArgs
	}
	return c.Ess.MatchUser(c.User, c.Args[pos], false, offline)
}

// MatchUser finds the user term names: @p/@s for the source, a UUID, an
// exact online name, a name prefix, a display name, then (with offline)
// a saved user. Vanished players are found only with getHidden or by a
// source that may see them. Essentials: matchUser
func (e *Essentials) MatchUser(source *User, term string, getHidden, offline bool) (*User, error) {
	if source != nil && (term == "@p" || term == "@s") {
		return source, nil
	}
	visible := func(u *User) bool { return getHidden || source == nil || source.CanSee(u) }
	var found *User
	if p, ok := e.Server.PlayerNamed(term); ok {
		found = e.Users.Of(p)
	} else if u, ok := e.Users.ByName(term); ok && !u.Online() {
		found = u
	}
	if found != nil {
		if !offline && !found.Online() {
			return nil, ErrPlayerNotFound
		}
		if visible(found) || (offline && strings.EqualFold(found.Name(), term)) {
			return found, nil
		}
		return nil, ErrPlayerNotFound
	}
	low := strings.ToLower(term)
	for _, u := range e.OnlineUsers() {
		if strings.HasPrefix(strings.ToLower(u.Name()), low) && visible(u) {
			return u, nil
		}
	}
	for _, u := range e.OnlineUsers() {
		if strings.Contains(strings.ToLower(StripLegacy(u.DisplayName())), low) && visible(u) {
			return u, nil
		}
	}
	return nil, ErrPlayerNotFound
}

// PlayerNames answers the names of the online players the source may see,
// for completion. Essentials: getPlayers
func (e *Essentials) PlayerNames(source *User) []string {
	var out []string
	for _, u := range e.OnlineUsers() {
		if source == nil || source.CanSee(u) {
			out = append(out, u.Name())
		}
	}
	return out
}

// pluginYML is the part of Essentials' plugin.yml the port reads.
type pluginYML struct {
	Commands map[string]struct {
		Description string   `yaml:"description"`
		Usage       string   `yaml:"usage"`
		Aliases     []string `yaml:"aliases"`
	} `yaml:"commands"`
	Permissions map[string]struct {
		Default     any             `yaml:"default"`
		Description string          `yaml:"description"`
		Children    map[string]bool `yaml:"children"`
	} `yaml:"permissions"`
}

func parsePluginYML(b []byte) (*pluginYML, error) {
	var p pluginYML
	return &p, yaml.Unmarshal(b, &p)
}

// registerPermissions registers plugin.yml's permissions with Bukkit's
// defaults: op (any operator, here level 1), true, false, not op.
func (e *Essentials) registerPermissions() { registerPermissions(e.Server, e.meta) }

func registerPermissions(s plugin.Server, meta *pluginYML) {
	for name, p := range meta.Permissions {
		level := 1
		switch fmt.Sprint(p.Default) {
		case "true":
			level = 0
		case "false":
			level = 5
		}
		s.Permissions().Register(plugin.Permission{Name: name, Description: p.Description,
			Default: plugin.PermissionDefault{OpLevel: level}, Children: p.Children})
	}
}

// registered is a command as registered: its Cmd and the names it got.
type registered struct {
	cmd   *Cmd
	names []string
}

// RegisterCommand registers commands. A name or alias a vanilla command (or
// another plugin) already has is dropped, since golem lets no plugin
// replace one; the command keeps its other names and essentials:<name>.
// A command listed in disabled-commands is not registered.
func (e *Essentials) RegisterCommand(cmds ...Cmd) { e.registerOn(e.Server, e.meta, cmds) }

// RegisterModuleCommands registers a module's commands under the module's
// own plugin handle, with the aliases its plugin.yml lists.
func (e *Essentials) RegisterModuleCommands(s plugin.Server, pluginYML []byte, cmds ...Cmd) error {
	meta, err := parsePluginYML(pluginYML)
	if err != nil {
		return err
	}
	e.registerOn(s, meta, cmds)
	return nil
}

func (e *Essentials) registerOn(s plugin.Server, metas *pluginYML, cmds []Cmd) {
	for i := range cmds {
		c := &cmds[i]
		if slices.ContainsFunc(e.Settings.DisabledCommands(), func(d string) bool { return strings.EqualFold(d, c.Name) }) {
			continue
		}
		meta := metas.Commands[c.Name]
		// plugin.yml repeats some names among their own aliases (tpoffline);
		// the server refuses a command whose names repeat.
		var names []string
		for _, n := range append([]string{c.Name}, meta.Aliases...) {
			if n = strings.ToLower(n); !slices.Contains(names, n) {
				names = append(names, n)
			}
		}
		// Override takes vanilla's names, as Essentials does on Bukkit
		// (disabled-commands gives one back). A name another plugin holds
		// is registered by golem only as plugin:name and reported with
		// ErrNameTaken; the command keeps its other names.
		pc := plugin.Command{Name: names[0], Aliases: names[1:], Description: meta.Description,
			Usage: meta.Usage, Override: true, Node: e.commandNode(c)}
		var got []string
		switch err := s.Commands().Register(pc); {
		case err == nil:
			got = names
		case errors.Is(err, plugin.ErrNameTaken):
			list := strings.TrimPrefix(err.Error(), plugin.ErrNameTaken.Error()+": ")
			skipped := strings.Split(list, ", ")
			got = slices.DeleteFunc(slices.Clone(names), func(n string) bool { return slices.Contains(skipped, n) })
			e.Log.Warn("command names taken by another command are skipped", "command", c.Name, "names", skipped)
		default:
			e.Log.Error("cannot register command", "command", c.Name, "err", err)
		}
		names = got
		if len(names) > 0 {
			e.commands[c.Name] = &registered{cmd: c, names: names}
		}
		if len(names) == 0 {
			e.Log.Warn("every name of the command is taken by another command", "command", c.Name)
		}
	}
}

// RegisterModulePermissions registers the permissions a module's
// plugin.yml lists, under the module's handle.
func (e *Essentials) RegisterModulePermissions(s plugin.Server, pluginYML []byte) error {
	meta, err := parsePluginYML(pluginYML)
	if err != nil {
		return err
	}
	registerPermissions(s, meta)
	return nil
}

// CommandNames answers the names and aliases a command got, false when it is
// not registered.
func (e *Essentials) CommandNames(name string) ([]string, bool) {
	r, ok := e.commands[name]
	if !ok {
		return nil, false
	}
	return r.names, true
}

// node is the permission the command needs: essentials.<name> unless the Cmd
// names another, or none for "-". Essentials checks it itself, as on Bukkit,
// where its plugin.yml commands carry no permission: every player sees every
// command, and one without the node is told so. Essentials: isAuthorized(cmd)
func (c *Cmd) node() string {
	switch c.Permission {
	case "-":
		return ""
	case "":
		return "essentials." + c.Name
	}
	return c.Permission
}

// authorized reports whether the sender may run c; the console and command
// blocks always may.
func (c *Cmd) authorized(u *User) bool {
	n := c.node()
	return u == nil || n == "" || u.HasPermission(n)
}

// commandNode is a command's brigadier tree: the bare name, or the name and
// a greedy string of words.
func (e *Essentials) commandNode(c *Cmd) plugin.Node {
	run := func(pc *plugin.CommandContext) (int, error) { return e.execute(c, pc), nil }
	return plugin.Node{Run: run, Children: []plugin.Node{{
		Arg: plugin.GreedyArg("args"),
		Run: run,
		Suggest: func(pc *plugin.CommandContext, partial string) []string {
			return e.complete(c, pc, partial)
		},
	}}}
}

func (e *Essentials) newCtx(c *Cmd, pc *plugin.CommandContext, args []string) *Ctx {
	ctx := &Ctx{Ess: e, Sender: pc.Sender, Args: args, cmd: c, pc: pc}
	ctx.Label, _, _ = strings.Cut(strings.TrimPrefix(pc.Input, "/"), " ")
	if _, after, ok := strings.Cut(ctx.Label, ":"); ok {
		ctx.Label = after
	}
	if p, ok := pc.Sender.(plugin.Player); ok {
		ctx.Player = p
		ctx.User = e.Users.Of(p)
	}
	return ctx
}

// SplitArgs splits a command line's arguments on spaces, as Bukkit does,
// dropping empty words.
func SplitArgs(s string) []string {
	return strings.Fields(s)
}

// execute runs a command and shows its errors. Essentials: onCommandEssentials
func (e *Essentials) execute(c *Cmd, pc *plugin.CommandContext) (result int) {
	args := SplitArgs(plugin.Arg[string](pc, "args"))
	ctx := e.newCtx(c, pc, args)
	defer func() {
		if r := recover(); r != nil {
			e.Log.Error("command failed", "command", c.Name, "panic", r)
			ctx.Send("commandFailed", ctx.Label)
			result = 0
		}
	}()
	if ctx.User == nil && c.PlayerOnly {
		ctx.Send("onlyPlayers", ctx.Label)
		return 0
	}
	if u := ctx.User; u != nil {
		if c.Name != "mail" && u.HasPermission("essentials.mail") {
			e.NotifyOfMail(u)
		}
		if !c.authorized(u) {
			e.Log.Info(e.TlString("deniedAccessCommand", u.Name()))
			u.Send("noAccessCommand")
			return 0
		}
		if u.Data.Jailed && !u.HasPermission("essentials.jail.allow."+c.Name) {
			if u.Data.Timestamps.Jail > 0 {
				u.Send("playerJailedFor", u.Name(), FormatDateDiff(time.UnixMilli(u.Data.Timestamps.Jail)))
			} else {
				u.Send("jailMessage")
			}
			return 0
		}
		cost := e.Settings.CommandCost(c.Name)
		if cost.Sign() > 0 && !u.HasPermission("essentials.nocommandcost."+c.Name) {
			ctx.Charge = &Trade{Money: &cost}
			if err := ctx.Charge.CheckAffordable(u); err != nil {
				e.ShowError(ctx, err)
				return 0
			}
		}
	}
	err := c.Run(ctx)
	switch {
	case err == nil:
		if ctx.Charge != nil && ctx.User != nil {
			if err := ctx.Charge.Charge(ctx.User); err != nil {
				e.ShowError(ctx, err)
			}
		}
		return 1
	case errors.Is(err, ErrNoCharge), errors.Is(err, ErrQuiet):
		return 1
	case errors.Is(err, ErrNotEnoughArgs):
		e.ShowUsage(ctx)
		return 0
	}
	e.ShowError(ctx, err)
	return 0
}

// ShowError shows err: a TlError translated, anything else in
// errorWithMessage. Essentials: showError
func (e *Essentials) ShowError(ctx *Ctx, err error) {
	if tl, ok := errors.AsType[*TlError](err); ok {
		if tl.Plain {
			ctx.Send(tl.Key, tl.Args...)
			return
		}
		ctx.Send("errorWithMessage", Raw(e.I18n.Format(e.locale(ctx.Sender), tl.Key, tl.Args...)))
		return
	}
	ctx.Send("errorWithMessage", err.Error())
}

func (e *Essentials) locale(s plugin.CommandSender) string {
	if p, ok := s.(plugin.Player); ok && e.Settings.PerPlayerLocale() {
		return p.Locale()
	}
	return ""
}

// ShowUsage shows the command's usages from the messages
// (<name>CommandUsageN and its Description).
func (e *Essentials) ShowUsage(ctx *Ctx) {
	name := ctx.cmd.Name
	loc := e.locale(ctx.Sender)
	if !e.Settings.VerboseCommandUsages() || !e.I18n.Has(name+"CommandUsage1") {
		ctx.SendText(e.Mini(e.I18n.Format(loc, name+"CommandDescription")))
		ctx.SendText(plugin.Plain(strings.ReplaceAll(e.I18n.Format(loc, name+"CommandUsage"), "<command>", ctx.Label)))
		return
	}
	ctx.Send("commandHelpLine1", ctx.Label)
	ctx.Send("commandHelpLine2", Raw(EscapeTags(e.I18n.Format(loc, name+"CommandDescription"))))
	ctx.Send("commandHelpLine3")
	for i := 1; ; i++ {
		key := name + "CommandUsage" + strconv.Itoa(i)
		if !e.I18n.Has(key) {
			break
		}
		usage := strings.ReplaceAll(e.I18n.Format(loc, key), "<command>", ctx.Label)
		ctx.Send("commandHelpLineUsage", Raw(EscapeTags(usage)), Raw(EscapeTags(e.I18n.Format(loc, key+"Description"))))
	}
}

// complete answers suggestions for a greedy argument: the whole partial
// line with its last word completed, which is how the server places a
// greedy argument's suggestions.
func (e *Essentials) complete(c *Cmd, pc *plugin.CommandContext, partial string) []string {
	words := strings.Split(partial, " ")
	ctx := e.newCtx(c, pc, words)
	if !c.authorized(ctx.User) || ctx.User != nil && ctx.User.Data.Jailed && !ctx.User.HasPermission("essentials.jail.allow."+c.Name) {
		return nil
	}
	var cands []string
	if c.Complete != nil {
		cands = c.Complete(ctx)
	} else {
		cands = e.PlayerNames(ctx.User)
	}
	prefix := strings.Join(words[:len(words)-1], " ")
	if prefix != "" {
		prefix += " "
	}
	last := strings.ToLower(words[len(words)-1])
	var out []string
	for _, s := range cands {
		if strings.HasPrefix(strings.ToLower(s), last) {
			out = append(out, prefix+s)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// CompletePlayers completes player names visible to the sender.
func (c *Ctx) CompletePlayers() []string { return c.Ess.PlayerNames(c.User) }

// Durations are completions for a time argument. Essentials: COMMON_DATE_DIFFS
var Durations = []string{"1m", "15m", "1h", "3h", "12h", "1d", "1w", "1mo", "1y"}
