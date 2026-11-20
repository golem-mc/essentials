package messaging

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

// text is a text file's lines and chapters. Essentials: IText, TextInput
type text struct {
	lines    []string
	chapters []string
	// bookmarks are the line of each chapter heading by lower-case title.
	bookmarks map[string]int
}

var invalidFileChars = regexp.MustCompile(`[^a-z0-9-]`)

// errNoFile is TextInput's error for a text file it had to create.
type errNoFile string

func (e errNoFile) Error() string {
	return fmt.Sprintf("File %s.txt does not exist. Creating one for you.", string(e))
}

// readText reads <name>_<player>.txt or <name>.txt from the data directory
// with the keywords filled in for the sender. A missing file is written from
// the bundled one when create is set, and answers an error.
// Essentials: TextInput with KeywordReplacer
func (g *group) readText(ctx *core.Ctx, name string, create bool) (*text, error) {
	path := ""
	if ctx.User != nil {
		p := g.e.DataPath(name + "_" + invalidFileChars.ReplaceAllString(strings.ToLower(ctx.User.Name()), "_") + ".txt")
		if _, err := os.Stat(p); err == nil {
			path = p
		}
	}
	if path == "" {
		path = g.e.DataPath(name + ".txt")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if !create {
			return &text{bookmarks: map[string]int{}}, nil
		}
		if werr := os.WriteFile(path, core.Resource(name+".txt"), 0o644); werr != nil {
			return nil, werr
		}
		return nil, errNoFile(name)
	}
	t := &text{bookmarks: map[string]int{}}
	raw := strings.Split(strings.ReplaceAll(string(b), "\r", ""), "\n")
	if n := len(raw); n > 0 && raw[n-1] == "" {
		raw = raw[:n-1]
	}
	for i, line := range raw {
		if len(line) > 1 && line[0] == '#' {
			titles := strings.Split(strings.ReplaceAll(strings.TrimSpace(line[1:]), " ", "_"), ",")
			t.chapters = append(t.chapters, replaceFormat(titles[0]))
			for _, title := range titles {
				t.bookmarks[stripEssentialsFormat(strings.ToLower(title))] = i
			}
		}
		t.lines = append(t.lines, g.e.ReplaceKeywords(replaceFormat(line), ctx.User))
	}
	return t, nil
}

// showPage shows a page of t, or a page of one of its chapters; commandName
// names the command in headers, "" for none. Essentials: TextPager.showPage
func (g *group) showPage(s plugin.CommandSender, t *text, pageStr, chapterPageStr, commandName string) {
	send := func(line string) { s.SendMessage(g.e.Mini(line)) }
	lines := t.lines
	if pageStr == "" || isDigits(pageStr) {
		if len(lines) > 0 && strings.HasPrefix(lines[0], "#") {
			g.e.Send(s, "infoChapter")
			s.SendMessage(g.e.Mini(strings.Join(t.chapters, ", ")))
			return
		}
		page := 1
		if pageStr != "" {
			if n, err := strconv.Atoi(pageStr); err == nil {
				page = max(n, 1)
			}
		}
		end := 0
		for end < len(lines) && !strings.HasPrefix(lines[end], "#") {
			end++
		}
		pages := (end + 8) / 9
		if page > pages {
			g.e.Send(s, "infoUnknownChapter")
			return
		}
		if commandName != "" {
			g.e.Send(s, "infoPages", page, pages, capitalTitle(commandName))
		}
		for i := (page - 1) * 9; i < end && i < page*9; i++ {
			send(lines[i])
		}
		if page < pages && commandName != "" {
			g.e.Send(s, "readNextPage", commandName, page+1)
		}
		return
	}
	chapterPage := 0
	if chapterPageStr != "" {
		if n, err := strconv.Atoi(chapterPageStr); err == nil {
			chapterPage = max(n-1, 0)
		}
	}
	at, ok := t.bookmarks[strings.ToLower(pageStr)]
	if !ok {
		g.e.Send(s, "infoUnknownChapter")
		return
	}
	start := at + 1
	end := start
	for end < len(lines) && (len(lines[end]) == 0 || lines[end][0] != '#') {
		end++
	}
	page, pages := chapterPage+1, (end-start+8)/9
	if commandName != "" {
		g.e.Send(s, "infoChapterPages", capitalTitle(commandName)+": "+pageStr, page, pages)
	}
	from := start + chapterPage*9
	for i := from; i < end && i < from+9; i++ {
		send(lines[i])
	}
	if page < pages && commandName != "" {
		g.e.Send(s, "readNextPage", commandName, pageStr+" "+strconv.Itoa(page+1))
	}
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

// textCommand is /motd, /rules and /info: a text file shown by page or
// chapter. Essentials: Commandmotd, Commandrules, Commandinfo
func (g *group) textCommand(name, file string) core.Cmd {
	return core.Cmd{Name: name, Run: func(ctx *core.Ctx) error {
		if ctx.User != nil {
			ctx.User.UpdateDisplayName()
		}
		t, err := g.readText(ctx, file, true)
		if err != nil {
			return err
		}
		g.showPage(ctx.Sender, t, ctx.Arg(0), ctx.Arg(1), ctx.Label)
		return nil
	}, Complete: func(*core.Ctx) []string { return nil }}
}

// customText shows a chapter of custom.txt: the one the label names, as an
// alias of /customtext does, or the first argument's.
func (g *group) customText(ctx *core.Ctx) error {
	if ctx.User != nil {
		ctx.User.UpdateDisplayName()
	}
	t, err := g.readText(ctx, "custom", true)
	if err != nil {
		return err
	}
	chapter, page := ctx.Label, ctx.Arg(0)
	if strings.EqualFold(ctx.Label, "customtext") && len(ctx.Args) > 0 {
		chapter, page = ctx.Args[0], ctx.Arg(1)
	}
	g.showPage(ctx.Sender, t, chapter, page, "")
	return nil
}

func (g *group) registerText() {
	e := g.e
	e.RegisterCommand(
		g.textCommand("motd", "motd"),
		g.textCommand("rules", "rules"),
		g.textCommand("info", "info"),
		core.Cmd{
			// Essentials: Commandcustomtext
			Name:     "customtext",
			Run:      g.customText,
			Complete: func(*core.Ctx) []string { return nil },
		},
		core.Cmd{
			// Essentials: Commandhelp
			Name: "help",
			Run:  g.help,
			Complete: func(ctx *core.Ctx) []string {
				if len(ctx.Args) != 1 {
					return nil
				}
				// Essentials: getCommands and getPlugins
				var out []string
				for _, c := range g.helpCommands() {
					if c.plugin != "" {
						out = append(out, c.label)
					}
				}
				for _, p := range g.pluginNames() {
					out = append(out, pluginDisplayName(p))
				}
				return out
			},
		},
	)
	// golem has no commands.yml: customtext-aliases lists the
	// custom.txt chapters to register as commands, as its aliases there do.
	for _, alias := range e.Config.StringList("customtext-aliases") {
		alias = strings.ToLower(strings.TrimSpace(alias))
		if alias == "" || alias == "customtext" {
			continue
		}
		e.RegisterCommand(core.Cmd{
			Name:       alias,
			Permission: "essentials.customtext",
			Run:        g.customText,
			Complete:   func(*core.Ctx) []string { return nil },
		})
	}
}

// vanillaDescription is what Bukkit describes a vanilla command with.
const vanillaDescription = "A Mojang provided command."

// helpCommand is a registered command as /help lists it, once however many
// names it has.
type helpCommand struct {
	// plugin is the owning plugin's name, "" for vanilla.
	plugin string
	// name is the command's own name: for a command of Essentials or a
	// module, the one its essentials.<name> node and messages use.
	name string
	// label is the name a player types; aliases its other names.
	label       string
	aliases     []string
	description string
	permission  string
	// usage is what the command registered, with <command> for its label.
	usage string
}

// essFamily reports whether plugin is Essentials or one of its modules,
// whose commands /help checks against essentials.<name>. Essentials:
// HelpInput, pluginNameLow.contains("essentials")
func essFamily(plugin string) bool { return strings.Contains(strings.ToLower(plugin), "essentials") }

// pluginDisplayName answers the name Essentials' plugins have on Bukkit,
// for the names this port registers them under.
func pluginDisplayName(name string) string {
	switch strings.ToLower(name) {
	case "essentials":
		return "Essentials"
	case "essentialschat":
		return "EssentialsChat"
	case "essentialsspawn":
		return "EssentialsSpawn"
	case "essentialsprotect":
		return "EssentialsProtect"
	case "essentialsantibuild":
		return "EssentialsAntiBuild"
	}
	return name
}

// helpCommands answers every registered command. An Essentials command is
// listed once, under the names core says it got (its own name may have gone
// to another plugin); a name core added that is no command's own (a
// customtext-aliases entry) is left out, as Bukkit's commands.yml aliases
// are. Essentials: getKnownCommands
func (g *group) helpCommands() []helpCommand {
	var out []helpCommand
	seen := map[string]bool{}
	for _, c := range g.e.Server.Commands().All() {
		switch {
		case c.Plugin == "":
			out = append(out, helpCommand{name: c.Name, label: c.Name, aliases: c.Aliases, description: vanillaDescription})
		case essFamily(c.Plugin):
			name, _ := strings.CutPrefix(c.Permission, "essentials.")
			names, ok := g.e.CommandNames(name)
			if !ok || !slices.Contains(names, c.Name) {
				names, ok = g.e.CommandNames(c.Name)
				name = c.Name
			}
			if !ok || len(names) == 0 || seen[c.Plugin+":"+name] {
				continue
			}
			seen[c.Plugin+":"+name] = true
			out = append(out, helpCommand{plugin: c.Plugin, name: name, label: names[0], aliases: names[1:], description: c.Description, permission: c.Permission, usage: c.Usage})
		default:
			out = append(out, helpCommand{plugin: c.Plugin, name: c.Name, label: c.Name, aliases: c.Aliases, description: c.Description, permission: c.Permission, usage: c.Usage})
		}
	}
	return out
}

// pluginNames answers the plugins in the server's order. Essentials:
// PluginManager.getPlugins
func (g *group) pluginNames() []string {
	var out []string
	for _, p := range g.e.Server.Plugins() {
		out = append(out, p.Name)
	}
	return out
}

// help shows help.txt when there is one, else the commands the player may
// use, matching a search term, or one command's usage.
func (g *group) help(ctx *core.Ctx) error {
	if ctx.User == nil {
		ctx.Send("helpConsole")
		return nil
	}
	pageStr, chapterPageStr := ctx.Arg(0), ctx.Arg(1)
	command := ctx.Label
	t, err := g.readText(ctx, "help", false)
	if err != nil {
		return err
	}
	if len(t.lines) == 0 {
		if cmd, ok := strings.CutPrefix(pageStr, "/"); ok {
			if g.commandHelp(ctx, cmd) {
				return nil
			}
		}
		if pageStr == "" || core.IsInt(pageStr) {
			t = g.helpInput(ctx, "")
		} else {
			if len(pageStr) > 26 {
				pageStr = pageStr[:25]
			}
			t = g.helpInput(ctx, strings.ToLower(pageStr))
			command += " " + pageStr
			pageStr = chapterPageStr
		}
		chapterPageStr = ""
	} else {
		ctx.User.UpdateDisplayName()
	}
	g.showPage(ctx.Sender, t, pageStr, chapterPageStr, command)
	return nil
}

// commandHelp shows the description, aliases and usages of the command
// cmd names, of any plugin or vanilla; false when there is none.
func (g *group) commandHelp(ctx *core.Ctx, cmd string) bool {
	loc := g.locale(ctx.Sender)
	for _, c := range g.helpCommands() {
		prefix := "minecraft:"
		if c.plugin != "" {
			prefix = strings.ToLower(c.plugin) + ":"
		}
		if !slices.ContainsFunc(append([]string{c.label}, c.aliases...), func(n string) bool {
			return strings.EqualFold(n, cmd) || strings.EqualFold(prefix+n, cmd)
		}) {
			continue
		}
		// Only Essentials' own commands have translated descriptions and
		// usages; a module's or another plugin's show what they registered.
		ess := c.plugin == g.e.Server.Name()
		ctx.Send("commandHelpLine1", cmd)
		desc := core.Raw(core.EscapeTags(c.description))
		if ess && g.e.I18n.Has(c.name+"CommandDescription") {
			desc = core.Raw(g.e.I18n.Format(loc, c.name+"CommandDescription"))
		}
		ctx.Send("commandHelpLine2", desc)
		ctx.Send("commandHelpLine4", "["+strings.Join(c.aliases, ", ")+"]")
		ctx.Send("commandHelpLine3")
		usages := 0
		for i := 1; ess; i++ {
			key := c.name + "CommandUsage" + strconv.Itoa(i)
			if !g.e.I18n.Has(key) {
				break
			}
			usages++
			usage := strings.ReplaceAll(g.e.I18n.Format(loc, key), "<command>", cmd)
			ctx.Send("commandHelpLineUsage", core.Raw(core.EscapeTags(usage)), core.Raw(core.EscapeTags(g.e.I18n.Format(loc, key+"Description"))))
		}
		if usages == 0 {
			// The registered usage (plugin.yml's for Essentials), else
			// Bukkit's default "/<name>".
			usage := "/" + c.label
			if c.usage != "" {
				usage = strings.ReplaceAll(c.usage, "<command>", cmd)
			}
			ctx.SendText(plugin.Plain(usage))
		}
		return true
	}
	return false
}

// helpInput lists the commands the player may use, by plugin, matching
// match. Essentials: HelpInput
func (g *group) helpInput(ctx *core.Ctx, match string) *text {
	loc := g.locale(ctx.Sender)
	t := &text{bookmarks: map[string]int{}}
	if match != "" {
		t.lines = append(t.lines, g.e.I18n.Format(loc, "helpMatching", match))
	}
	byPlugin := map[string][]helpCommand{}
	for _, c := range g.helpCommands() {
		if c.plugin != "" {
			byPlugin[c.plugin] = append(byPlugin[c.plugin], c)
		}
	}
	nonEss := g.e.Config.Bool("non-ess-in-help", true)
	hidePermissionless := g.e.Config.Bool("hide-permissionless-help", true)
	var newLines []string
	for _, p := range g.pluginNames() {
		pluginName := pluginDisplayName(p)
		pluginLow := strings.ToLower(pluginName)
		if pluginLow == match {
			t.lines = []string{g.e.I18n.Format(loc, "helpFrom", pluginName)}
			newLines = nil
		}
		whitelisted := ctx.Has("essentials.help." + pluginLow)
		cmds := byPlugin[p]
		slices.SortFunc(cmds, func(a, b helpCommand) int { return strings.Compare(a.label, b.label) })
		var lines []string
		for _, c := range cmds {
			if match != "" && !strings.Contains(pluginLow, match) && !strings.Contains(strings.ToLower(c.label), match) &&
				!strings.Contains(strings.ToLower(c.description), match) {
				continue
			}
			line := g.e.I18n.Format(loc, "helpLine", c.label, c.description)
			switch {
			case essFamily(p):
				if ctx.Has("essentials." + c.name) {
					lines = append(lines, line)
				}
			case !nonEss:
			case whitelisted || ctx.Has("essentials.help."+pluginLow+"."+c.label):
				lines = append(lines, line)
			case c.permission != "":
				if slices.ContainsFunc(strings.Split(c.permission, ";"), ctx.Has) {
					lines = append(lines, line)
				}
			case !hidePermissionless:
				lines = append(lines, line)
			}
		}
		if len(lines) == 0 {
			continue
		}
		newLines = append(newLines, lines...)
		if pluginLow == match {
			break
		}
		if match == "" {
			t.lines = append(t.lines, g.e.I18n.Format(loc, "helpPlugin", pluginName, pluginLow))
		}
	}
	t.lines = append(t.lines, newLines...)
	return t
}
