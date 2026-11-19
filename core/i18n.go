package core

import (
	"bufio"
	"embed"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/golem-mc/golem/plugin"
)

//go:embed resources
var resources embed.FS

// Resource answers a bundled resource file (config.yml, items.json, ...).
func Resource(name string) []byte {
	b, err := resources.ReadFile("resources/" + name)
	if err != nil {
		panic("essentials: missing resource " + name)
	}
	return b
}

// I18n holds the message bundles: the bundled English messages, the bundled
// translation for each locale, and a server's own messages*.properties in
// the data directory, which override the bundled ones. Essentials: I18n
type I18n struct {
	dataDir string
	mu      sync.Mutex
	bundles map[string]map[string]string // locale ("" for the default) to key to message
	locale  string
}

func newI18n(dataDir, locale string) *I18n {
	i := &I18n{dataDir: dataDir, bundles: make(map[string]map[string]string)}
	i.SetLocale(locale)
	return i
}

// SetLocale makes loc (e.g. "de" or "en_GB") the server's locale, dropping
// the cached bundles so edited files are read again.
func (i *I18n) SetLocale(loc string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.locale = normalizeLocale(loc)
	clear(i.bundles)
}

// Locale answers the server's locale.
func (i *I18n) Locale() string {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.locale
}

// normalizeLocale turns "de-DE", "de_de" or "DE" into Java's "de_DE".
func normalizeLocale(loc string) string {
	loc = strings.ReplaceAll(strings.TrimSpace(loc), "-", "_")
	lang, country, _ := strings.Cut(loc, "_")
	lang = strings.ToLower(lang)
	if country == "" {
		return lang
	}
	return lang + "_" + strings.ToUpper(country)
}

// bundle answers the messages of one file name suffix ("" for
// messages.properties), merged with the data directory's copy.
func (i *I18n) bundle(suffix string) map[string]string {
	if b, ok := i.bundles[suffix]; ok {
		return b
	}
	name := "messages.properties"
	if suffix != "" {
		name = "messages_" + suffix + ".properties"
	}
	b := map[string]string{}
	if f, err := resources.Open("resources/" + name); err == nil {
		maps.Copy(b, parseProperties(f))
		f.Close()
	}
	if i.dataDir != "" {
		if f, err := os.Open(filepath.Join(i.dataDir, name)); err == nil {
			maps.Copy(b, parseProperties(f))
			f.Close()
		}
	}
	i.bundles[suffix] = b
	return b
}

// raw answers the message for key in loc: the locale with its country, then
// its language, then the server's locale, then English. A missing key
// answers the key itself, so a typo shows rather than an empty line.
func (i *I18n) raw(loc, key string) string {
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, l := range []string{normalizeLocale(loc), i.locale} {
		if l == "" {
			continue
		}
		if m, ok := i.bundle(l)[key]; ok {
			return m
		}
		if lang, _, ok := strings.Cut(l, "_"); ok {
			if m, ok := i.bundle(lang)[key]; ok {
				return m
			}
		}
	}
	if m, ok := i.bundle("")[key]; ok {
		return m
	}
	return key
}

// Has reports whether any bundle has key.
func (i *I18n) Has(key string) bool { return i.raw("", key) != key }

// Raw is a message argument inserted as MiniMessage rather than escaped:
// a parsed placeholder in Essentials.
type Raw string

// Format answers the message key in loc as MiniMessage, with args filled in
// by Java's MessageFormat rules: {n} is argument n, ” is a quote, and text
// between single quotes is literal. A message without args only has its
// doubled quotes collapsed, as Essentials does. Arguments are escaped, so a
// player's text never opens a tag, and their legacy § codes become tags.
func (i *I18n) Format(loc, key string, args ...any) string {
	msg := i.raw(loc, key)
	if len(args) == 0 {
		return strings.ReplaceAll(msg, "''", "'")
	}
	strs := make([]string, len(args))
	for n, a := range args {
		strs[n] = formatArg(a)
	}
	return MessageFormat(msg, strs)
}

func formatArg(a any) string {
	switch v := a.(type) {
	case Raw:
		return string(v)
	case plugin.Text:
		return EscapeTags(v.PlainString())
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	}
	return sectionToMini(EscapeTags(fmt.Sprint(a)))
}

// sectionToMini converts only § codes, which is what a message argument
// may carry (a coloured nickname); an & a player typed stays an &.
func sectionToMini(s string) string { return legacyConvert(s, false) }

// MessageFormat fills {n} placeholders in pattern with args, following
// java.text.MessageFormat's quoting. A placeholder's format type
// ({0,number}) is ignored; the argument is inserted as given.
func MessageFormat(pattern string, args []string) string {
	var b strings.Builder
	inQuote := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch {
		case c == '\'':
			if i+1 < len(pattern) && pattern[i+1] == '\'' {
				b.WriteByte('\'')
				i++
			} else {
				inQuote = !inQuote
			}
		case inQuote:
			b.WriteByte(c)
		case c == '{':
			end := strings.IndexByte(pattern[i:], '}')
			if end < 0 {
				b.WriteString(pattern[i:])
				return b.String()
			}
			spec := pattern[i+1 : i+end]
			idx, _, _ := strings.Cut(spec, ",")
			n, err := strconv.Atoi(strings.TrimSpace(idx))
			switch {
			case err != nil:
				b.WriteString("{" + spec + "}")
			case n < len(args):
				b.WriteString(args[n])
			default:
				b.WriteString("{" + spec + "}")
			}
			i += end
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// parseProperties reads a Java .properties file (UTF-8, as Essentials'
// UTF8PropertiesControl reads them): key=value or key:value, # and !
// comments, backslash escapes and line continuations.
func parseProperties(r io.Reader) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	var logical strings.Builder
	for sc.Scan() {
		line := sc.Text()
		if logical.Len() == 0 {
			line = strings.TrimLeft(line, " \t\f")
			if line == "" || line[0] == '#' || line[0] == '!' {
				continue
			}
		} else {
			line = strings.TrimLeft(line, " \t\f")
		}
		if trailingBackslashes(line)%2 == 1 {
			logical.WriteString(line[:len(line)-1])
			continue
		}
		logical.WriteString(line)
		key, value := splitProperty(logical.String())
		out[unescapeProperty(key)] = unescapeProperty(value)
		logical.Reset()
	}
	return out
}

func trailingBackslashes(s string) int {
	n := 0
	for i := len(s) - 1; i >= 0 && s[i] == '\\'; i-- {
		n++
	}
	return n
}

func splitProperty(s string) (string, string) {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '=', ':', ' ', '\t':
			key := s[:i]
			rest := strings.TrimLeft(s[i:], " \t")
			if rest != "" && (rest[0] == '=' || rest[0] == ':') {
				rest = strings.TrimLeft(rest[1:], " \t")
			}
			return key, rest
		}
	}
	return s, ""
}

func unescapeProperty(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'f':
			b.WriteByte('\f')
		case 'u':
			if i+4 < len(s) {
				if r, err := strconv.ParseUint(s[i+1:i+5], 16, 32); err == nil {
					b.WriteRune(rune(r))
					i += 4
					continue
				}
			}
			b.WriteByte('u')
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
