package core

import (
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// Theme holds the colours <primary> and <secondary> stand for in messages
// (config.yml message-colors).
type Theme struct{ Primary, Secondary string }

// DefaultTheme is EssentialsX's: gold and red.
var DefaultTheme = Theme{Primary: "#ffaa00", Secondary: "#ff5555"}

var namedColors = map[string]string{
	"black": "black", "dark_blue": "dark_blue", "dark_green": "dark_green", "dark_aqua": "dark_aqua",
	"dark_red": "dark_red", "dark_purple": "dark_purple", "gold": "gold", "gray": "gray", "grey": "gray",
	"dark_gray": "dark_gray", "dark_grey": "dark_gray", "blue": "blue", "green": "green", "aqua": "aqua",
	"red": "red", "light_purple": "light_purple", "yellow": "yellow", "white": "white",
}

// legacyCodes maps the legacy § and & formatting codes to MiniMessage tags.
var legacyCodes = map[byte]string{
	'0': "black", '1': "dark_blue", '2': "dark_green", '3': "dark_aqua", '4': "dark_red",
	'5': "dark_purple", '6': "gold", '7': "gray", '8': "dark_gray", '9': "blue", 'a': "green",
	'b': "aqua", 'c': "red", 'd': "light_purple", 'e': "yellow", 'f': "white",
	'k': "obfuscated", 'l': "bold", 'm': "strikethrough", 'n': "underlined", 'o': "italic", 'r': "reset",
}

// isLegacyColor reports whether a legacy code is a colour, which resets the
// decorations before it as the legacy format does.
func isLegacyColor(c byte) bool { return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || c == 'r' }

// LegacyToMini converts legacy & and § codes, and &#rrggbb, to MiniMessage
// tags. vanilla's legacy format resets decorations at every colour code, so a
// colour code closes everything open with <reset> first.
func LegacyToMini(s string) string { return legacyConvert(s, true) }

// legacyConvert converts § codes, and & codes too when amp is set.
func legacyConvert(s string, amp bool) string {
	if !strings.Contains(s, "§") && (!amp || !strings.Contains(s, "&")) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		marker := c == '&' && amp
		literal := string(c)
		if c == 0xC2 && i+1 < len(s) && s[i+1] == 0xA7 { // § in UTF-8
			marker = true
			literal = "§"
			i++
		}
		if !marker || i+1 >= len(s) {
			b.WriteString(literal)
			continue
		}
		next := s[i+1] | 0x20
		if next == 'x' {
			if hex, n := bukkitHex(s[i+2:], amp); n > 0 {
				b.WriteString("<reset><#" + hex + ">")
				i += 1 + n
				continue
			}
		}
		if next == '#' && i+7 < len(s) && isHex(s[i+2:i+8]) {
			b.WriteString("<reset><#" + strings.ToLower(s[i+2:i+8]) + ">")
			i += 7
			continue
		}
		if tag, ok := legacyCodes[next]; ok {
			if isLegacyColor(next) && tag != "reset" {
				b.WriteString("<reset>")
			}
			b.WriteString("<" + tag + ">")
			i++
			continue
		}
		b.WriteString(literal)
	}
	return b.String()
}

// bukkitHex reads the six marker-digit pairs after Bukkit's §x (or &x),
// "§r§r§g§g§b§b", answering the hex colour and the bytes read; 0 when s does
// not start with them.
func bukkitHex(s string, amp bool) (string, int) {
	var hex []byte
	i := 0
	for range 6 {
		switch {
		case strings.HasPrefix(s[i:], "§"):
			i += len("§")
		case amp && strings.HasPrefix(s[i:], "&"):
			i++
		default:
			return "", 0
		}
		if i >= len(s) || !isHex(s[i:i+1]) {
			return "", 0
		}
		hex = append(hex, s[i]|0x20)
		i++
	}
	return string(hex), i
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i] | 0x20
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return len(s) > 0
}

// StripLegacy removes legacy & and § codes from s.
func StripLegacy(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if (s[i] == '&') && i+1 < len(s) {
			if _, ok := legacyCodes[s[i+1]|0x20]; ok {
				i++
				continue
			}
		}
		if s[i] == 0xC2 && i+2 < len(s) && s[i+1] == 0xA7 {
			i += 2
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// EscapeTags escapes s so that MiniMessage shows it as typed: a player's
// message or name never opens a tag.
func EscapeTags(s string) string {
	if !strings.ContainsAny(s, "<\\") {
		return s
	}
	return strings.NewReplacer("\\", "\\\\", "<", "\\<").Replace(s)
}

// tagState is one open MiniMessage tag and the style it set.
type tagState struct {
	name  string
	style plugin.Style
}

// ParseMini parses the subset of MiniMessage EssentialsX's messages use:
// colours (named, #rrggbb, <color:..>, <primary>, <secondary>), decorations
// and their negations, <reset>, <click:..>, <hover:show_text:..>,
// <insert:..>, <newline>. Unknown tags stay as text, as MiniMessage leaves
// them, which keeps usage strings like <player> readable.
func ParseMini(s string, theme Theme) plugin.Text {
	root := plugin.Text{}
	var stack []tagState
	var cur strings.Builder
	style := func() plugin.Style {
		if len(stack) == 0 {
			return plugin.Style{}
		}
		return stack[len(stack)-1].style
	}
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		root.Children = append(root.Children, plugin.Text{Literal: cur.String(), Style: style()})
		cur.Reset()
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) && (s[i+1] == '<' || s[i+1] == '\\') {
			cur.WriteByte(s[i+1])
			i++
			continue
		}
		if c != '<' {
			cur.WriteByte(c)
			continue
		}
		end := tagEnd(s, i+1)
		if end < 0 {
			cur.WriteByte(c)
			continue
		}
		raw := s[i+1 : end]
		if strings.HasPrefix(raw, "/") {
			name := strings.ToLower(strings.SplitN(raw[1:], ":", 2)[0])
			if known(name, theme) {
				flush()
				for j, s := range slices.Backward(stack) {
					if s.name == name || name == "" {
						stack = stack[:j]
						break
					}
				}
				i = end
				continue
			}
			cur.WriteString(s[i : end+1])
			i = end
			continue
		}
		parts := splitTagArgs(raw)
		name := strings.ToLower(parts[0])
		switch name {
		case "reset":
			flush()
			stack = stack[:0]
			i = end
			continue
		case "newline", "br":
			cur.WriteByte('\n')
			i = end
			continue
		}
		st, ok := applyTag(style(), name, parts[1:], theme)
		if !ok {
			cur.WriteString(s[i : end+1])
			i = end
			continue
		}
		flush()
		stack = append(stack, tagState{name: strings.TrimPrefix(name, "!"), style: st})
		i = end
	}
	flush()
	if len(root.Children) == 1 {
		return root.Children[0]
	}
	return root
}

// tagEnd answers the index of the > closing the tag starting at i, skipping
// quoted arguments; -1 when there is none.
func tagEnd(s string, i int) int {
	var quote byte
	for j := i; j < len(s); j++ {
		c := s[j]
		switch {
		case quote != 0:
			switch c {
			case '\\':
				j++
			case quote:
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '<':
			return -1
		case c == '>':
			return j
		}
	}
	return -1
}

// splitTagArgs splits a tag's body on colons outside quotes and unquotes
// each part.
func splitTagArgs(raw string) []string {
	var parts []string
	var b strings.Builder
	var quote byte
	for j := 0; j < len(raw); j++ {
		c := raw[j]
		switch {
		case quote != 0:
			if c == '\\' && j+1 < len(raw) {
				b.WriteByte(raw[j+1])
				j++
			} else if c == quote {
				quote = 0
			} else {
				b.WriteByte(c)
			}
		case c == '\'' || c == '"':
			quote = c
		case c == ':':
			parts = append(parts, b.String())
			b.Reset()
		default:
			b.WriteByte(c)
		}
	}
	return append(parts, b.String())
}

// argTags are the tags ParseMini knows that take arguments.
var argTags = map[string]bool{"color": true, "colour": true, "c": true, "click": true, "hover": true, "insert": true, "insertion": true}

// known reports whether a closing tag's name is one ParseMini opens.
func known(name string, theme Theme) bool {
	if argTags[name] || name == "reset" {
		return true
	}
	_, ok := applyTag(plugin.Style{}, name, nil, theme)
	return ok
}

// applyTag answers base with the tag applied, false for a tag ParseMini
// does not know.
func applyTag(base plugin.Style, name string, args []string, theme Theme) (plugin.Style, bool) {
	st := base
	neg := strings.HasPrefix(name, "!")
	name = strings.TrimPrefix(name, "!")
	if c, ok := colorOf(name, theme); ok && !neg {
		st.Color = c
		return st, true
	}
	set := func(p **bool) (plugin.Style, bool) {
		*p = new(!neg)
		return st, true
	}
	switch name {
	case "bold", "b":
		return set(&st.Bold)
	case "italic", "i", "em":
		return set(&st.Italic)
	case "underlined", "u":
		return set(&st.Underlined)
	case "strikethrough", "st":
		return set(&st.Strikethrough)
	case "obfuscated", "obf":
		return set(&st.Obfuscated)
	case "color", "colour", "c":
		if len(args) == 1 {
			if c, ok := colorOf(strings.ToLower(args[0]), theme); ok {
				st.Color = c
				return st, true
			}
		}
	case "click":
		if len(args) >= 2 {
			actions := map[string]plugin.ClickAction{
				"open_url": plugin.OpenURL, "run_command": plugin.RunCommand, "suggest_command": plugin.SuggestCommand,
				"change_page": plugin.ChangePage, "copy_to_clipboard": plugin.CopyToClipboard,
			}
			if a, ok := actions[strings.ToLower(args[0])]; ok {
				st.Click = &plugin.Click{Action: a, Value: strings.Join(args[1:], ":")}
				return st, true
			}
		}
	case "hover":
		if len(args) >= 2 && strings.EqualFold(args[0], "show_text") {
			h := ParseMini(strings.Join(args[1:], ":"), theme)
			st.Hover = &plugin.Hover{Text: &h}
			return st, true
		}
	case "insert", "insertion":
		if len(args) >= 1 {
			st.Insertion = strings.Join(args, ":")
			return st, true
		}
	}
	return base, false
}

func colorOf(name string, theme Theme) (string, bool) {
	switch name {
	case "primary":
		return theme.Primary, true
	case "secondary":
		return theme.Secondary, true
	}
	if c, ok := namedColors[name]; ok {
		return c, true
	}
	if strings.HasPrefix(name, "#") && len(name) == 7 && isHex(name[1:]) {
		return name, true
	}
	return "", false
}

// Mini parses s, which may use legacy codes as well as MiniMessage tags, in
// the server's theme; for config strings such as formats and MOTDs.
func (e *Essentials) Mini(s string) plugin.Text {
	return ParseMini(LegacyToMini(s), e.theme)
}
