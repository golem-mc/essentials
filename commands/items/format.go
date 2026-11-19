package items

import (
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

// Legacy codes by kind, for the color, format and magic permissions.
// Essentials: FormatUtil.COLORS, FORMATS, MAGIC
const (
	colorCodes  = "0123456789abcdef"
	formatCodes = "lmnor"
	magicCodes  = "k"
)

// codeNames are the ChatColor names of the codes, for the per-colour nodes
// (permBase.red, permBase.obfuscated).
var codeNames = map[byte]string{
	'0': "black", '1': "dark_blue", '2': "dark_green", '3': "dark_aqua", '4': "dark_red", '5': "dark_purple",
	'6': "gold", '7': "gray", '8': "dark_gray", '9': "blue", 'a': "green", 'b': "aqua", 'c': "red",
	'd': "light_purple", 'e': "yellow", 'f': "white", 'k': "obfuscated", 'l': "bold", 'm': "strikethrough",
	'n': "underline", 'o': "italic", 'r': "reset",
}

// miniTags are the MiniMessage tags of the codes.
var miniTags = map[byte]string{
	'0': "black", '1': "dark_blue", '2': "dark_green", '3': "dark_aqua", '4': "dark_red", '5': "dark_purple",
	'6': "gold", '7': "gray", '8': "dark_gray", '9': "blue", 'a': "green", 'b': "aqua", 'c': "red",
	'd': "light_purple", 'e': "yellow", 'f': "white", 'k': "obfuscated", 'l': "bold", 'm': "strikethrough",
	'n': "underlined", 'o': "italic", 'r': "reset",
}

// supportedCodes answers the codes the user may use under permBase.
// Essentials: FormatUtil.getSupported
func supportedCodes(e *core.Essentials, u *core.User, permBase string) map[byte]bool {
	out := map[byte]bool{}
	add := func(codes string) {
		for i := range len(codes) {
			out[codes[i]] = true
		}
	}
	if u == nil {
		add(colorCodes + formatCodes + magicCodes)
		return out
	}
	if u.HasPermission(permBase + ".color") {
		add(colorCodes)
	}
	if u.HasPermission(permBase + ".format") {
		add(formatCodes)
	}
	if u.HasPermission(permBase + ".magic") {
		add(magicCodes)
	}
	if p, ok := u.Player(); ok {
		for code, name := range codeNames {
			if has, set := permExact(p, permBase+"."+name); set {
				out[code] = has
			}
		}
	}
	return out
}

func rgbAllowed(u *core.User, permBase string) bool {
	return u == nil || u.HasPermission(permBase+".rgb")
}

func isCode(c byte) bool { _, ok := codeNames[c|0x20]; return ok }

func isHexString(s string) bool {
	if len(s) != 6 {
		return false
	}
	for i := range len(s) {
		c := s[i] | 0x20
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// formatString turns the & codes the user may use under permBase into §
// codes (§#rrggbb for &#rrggbb with .rgb) and strips the § codes they may
// not. Essentials: FormatUtil.formatString
func formatString(e *core.Essentials, u *core.User, permBase, s string) string {
	supported := supportedCodes(e, u, permBase)
	rgb := rgbAllowed(u, permBase)
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if strings.HasPrefix(s[i:], "§") && i+2 < len(s) && isCode(s[i+2]) {
			if !supported[s[i+2]|0x20] {
				i += 2
				continue
			}
			b.WriteString(s[i : i+3])
			i += 2
			continue
		}
		if s[i] != '&' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		escaped := s[i+1] == '&'
		j := i + 1
		if escaped {
			j++
		}
		switch {
		case j < len(s) && isCode(s[j]):
			if !escaped && supported[s[j]|0x20] {
				b.WriteString("§" + string(s[j]))
			} else {
				b.WriteString("&" + string(s[j]))
			}
			i = j
		case rgb && j+7 <= len(s) && s[j] == '#' && isHexString(s[j+1:j+7]):
			if escaped {
				b.WriteString("&" + s[j:j+7])
			} else {
				b.WriteString("§" + s[j:j+7])
			}
			i = j + 6
		default:
			b.WriteByte('&')
		}
	}
	return b.String()
}

// stripColor removes § codes. Bukkit: ChatColor.stripColor
func stripColor(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if strings.HasPrefix(s[i:], "§") && i+2 < len(s) {
			if s[i+2] == '#' && i+9 <= len(s) && isHexString(s[i+3:i+9]) {
				i += 8
				continue
			}
			if isCode(s[i+2]) {
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// secToMini converts § codes to MiniMessage, escaping the text between
// them, as Bukkit reads a legacy string; a colour resets the decorations.
func secToMini(s string) string {
	var b strings.Builder
	start := 0
	flush := func(end int) { b.WriteString(core.EscapeTags(s[start:end])) }
	for i := 0; i < len(s); i++ {
		if !strings.HasPrefix(s[i:], "§") || i+2 >= len(s) {
			continue
		}
		c := s[i+2] | 0x20
		switch {
		case c == '#' && i+9 <= len(s) && isHexString(s[i+3:i+9]):
			flush(i)
			b.WriteString("<reset><#" + strings.ToLower(s[i+3:i+9]) + ">")
			i += 8
		case isCode(c):
			flush(i)
			if strings.IndexByte(colorCodes, c) >= 0 {
				b.WriteString("<reset>")
			}
			b.WriteString("<" + miniTags[c] + ">")
			i += 2
		default:
			continue
		}
		start = i + 1
	}
	flush(len(s))
	return b.String()
}

// legacyText answers a § string as a component, as Bukkit's ItemMeta and
// Sign setters take one.
func legacyText(e *core.Essentials, s string) plugin.Text {
	return core.ParseMini(secToMini(s), e.Theme())
}

// codeOf answers the legacy code of a named colour.
var codeOf = func() map[string]byte {
	m := map[string]byte{}
	for c, n := range miniTags {
		if strings.IndexByte(colorCodes, c) >= 0 {
			m[n] = c
		}
	}
	return m
}()

// unformatText writes a component back as & codes, keeping only those the
// user may use under permBase, for completions and /editsign copy.
// Essentials: FormatUtil.unformatString
func unformatText(e *core.Essentials, u *core.User, permBase string, t plugin.Text) string {
	supported := supportedCodes(e, u, permBase)
	rgb := rgbAllowed(u, permBase)
	var b strings.Builder
	var last string
	var walk func(t plugin.Text, st plugin.Style)
	walk = func(t plugin.Text, st plugin.Style) {
		st = inherit(st, t.Style)
		if text := t.Literal; text != "" || t.Translate != "" {
			if text == "" {
				text = t.PlainString()
			}
			codes := styleCodes(st, supported, rgb)
			if codes != last {
				if last != "" && codes == "" {
					if supported['r'] {
						b.WriteString("&r")
					}
				} else {
					b.WriteString(codes)
				}
				last = codes
			}
			b.WriteString(text)
		}
		for _, c := range t.Children {
			walk(c, st)
		}
	}
	walk(t, plugin.Style{})
	return b.String()
}

func inherit(parent, child plugin.Style) plugin.Style {
	if child.Color == "" {
		child.Color = parent.Color
	}
	for _, p := range []struct{ c, p **bool }{{&child.Bold, &parent.Bold}, {&child.Italic, &parent.Italic},
		{&child.Underlined, &parent.Underlined}, {&child.Strikethrough, &parent.Strikethrough},
		{&child.Obfuscated, &parent.Obfuscated}} {
		if *p.c == nil {
			*p.c = *p.p
		}
	}
	return child
}

func styleCodes(st plugin.Style, supported map[byte]bool, rgb bool) string {
	var b strings.Builder
	if c, ok := codeOf[st.Color]; ok && supported[c] {
		b.WriteString("&" + string(c))
	} else if strings.HasPrefix(st.Color, "#") && rgb {
		b.WriteString("&" + st.Color)
	}
	for _, d := range []struct {
		on   *bool
		code byte
	}{{st.Obfuscated, 'k'}, {st.Bold, 'l'}, {st.Strikethrough, 'm'}, {st.Underlined, 'n'}, {st.Italic, 'o'}} {
		if d.on != nil && *d.on && supported[d.code] {
			b.WriteString("&" + string(d.code))
		}
	}
	return b.String()
}
