package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is a YAML file read as a tree, with getters by dotted path
// ("chat.format"). Missing keys fall back to the bundled default file, so a
// config.yml from an older version keeps working. Essentials:
// EssentialsConfiguration
type Config struct {
	Path     string
	root     map[string]any
	defaults map[string]any
}

// LoadConfig reads path, first writing the bundled resource def there when
// it does not exist (def may be empty for no defaults).
func LoadConfig(path, def string) (*Config, error) {
	c := &Config{Path: path, root: map[string]any{}, defaults: map[string]any{}}
	if def != "" {
		_ = yaml.Unmarshal(Resource(def), &c.defaults)
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(path, Resource(def), 0o644); err != nil {
				return nil, err
			}
		}
	}
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if len(b) > 0 {
		if err := yaml.Unmarshal(b, &c.root); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if c.root == nil {
			c.root = map[string]any{}
		}
	}
	return c, nil
}

// NewMemoryConfig answers a config backed by m, for sections and tests.
func NewMemoryConfig(m map[string]any) *Config {
	if m == nil {
		m = map[string]any{}
	}
	return &Config{root: m, defaults: map[string]any{}}
}

// Save writes the tree back to Path. Comments of the original file are lost,
// so only data files (warps, userdata) are saved, never config.yml.
func (c *Config) Save() error {
	b, err := yaml.Marshal(c.root)
	if err != nil {
		return err
	}
	return WriteFileAtomic(c.Path, b)
}

// WriteFileAtomic writes b to a temporary file beside path and renames it
// over path, so a crash never leaves half a file.
func WriteFileAtomic(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func lookup(m map[string]any, path string) (any, bool) {
	var cur any = m
	for part := range strings.SplitSeq(path, ".") {
		mm, ok := asMap(cur)
		if !ok {
			return nil, false
		}
		cur, ok = mm[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func asMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, v := range m {
			out[fmt.Sprint(k)] = v
		}
		return out, true
	}
	return nil, false
}

// Get answers the raw value at path, from the file or else the defaults.
func (c *Config) Get(path string) (any, bool) {
	if v, ok := lookup(c.root, path); ok {
		return v, true
	}
	return lookup(c.defaults, path)
}

// IsSet reports whether the file itself (not the defaults) has path.
func (c *Config) IsSet(path string) bool {
	_, ok := lookup(c.root, path)
	return ok
}

// Set sets path, creating the sections on the way; nil removes it.
func (c *Config) Set(path string, v any) {
	parts := strings.Split(path, ".")
	m := c.root
	for _, p := range parts[:len(parts)-1] {
		next, ok := asMap(m[p])
		if !ok {
			next = map[string]any{}
		}
		m[p] = next
		m = next
	}
	if v == nil {
		delete(m, parts[len(parts)-1])
		return
	}
	m[parts[len(parts)-1]] = v
}

func (c *Config) String(path, def string) string {
	v, ok := c.Get(path)
	if !ok || v == nil {
		return def
	}
	return fmt.Sprint(v)
}

func (c *Config) Bool(path string, def bool) bool {
	v, ok := c.Get(path)
	if !ok {
		return def
	}
	switch b := v.(type) {
	case bool:
		return b
	case string:
		if p, err := strconv.ParseBool(b); err == nil {
			return p
		}
	}
	return def
}

func (c *Config) Int(path string, def int) int {
	v, ok := c.Get(path)
	if !ok {
		return def
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		if p, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return p
		}
	}
	return def
}

func (c *Config) Float(path string, def float64) float64 {
	v, ok := c.Get(path)
	if !ok {
		return def
	}
	switch n := v.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case float64:
		return n
	case string:
		if p, err := strconv.ParseFloat(strings.TrimSpace(n), 64); err == nil {
			return p
		}
	}
	return def
}

// Seconds answers a number of seconds as a duration.
func (c *Config) Seconds(path string, def float64) time.Duration {
	return time.Duration(c.Float(path, def) * float64(time.Second))
}

// StringList answers a list of strings; a single string is a list of one.
func (c *Config) StringList(path string) []string {
	v, ok := c.Get(path)
	if !ok || v == nil {
		return nil
	}
	switch l := v.(type) {
	case []any:
		out := make([]string, 0, len(l))
		for _, e := range l {
			out = append(out, fmt.Sprint(e))
		}
		return out
	case []string:
		return l
	case string:
		return []string{l}
	}
	return nil
}

// Section answers the subtree at path as its own Config (sharing the
// file's maps, so a Set on it changes the file), with the defaults' subtree.
func (c *Config) Section(path string) (*Config, bool) {
	v, ok := lookup(c.root, path)
	m, isMap := asMap(v)
	d, _ := lookup(c.defaults, path)
	dm, _ := asMap(d)
	if dm == nil {
		dm = map[string]any{}
	}
	if !ok || !isMap {
		if len(dm) == 0 {
			return nil, false
		}
		m = map[string]any{}
	}
	return &Config{Path: c.Path, root: m, defaults: dm}, true
}

// Keys answers the keys of the section at path ("" for the root), from the
// file, or the defaults when the file has no such section.
func (c *Config) Keys(path string) []string {
	src := c.root
	if path != "" {
		v, ok := lookup(c.root, path)
		if !ok {
			v, _ = lookup(c.defaults, path)
		}
		src, _ = asMap(v)
	}
	keys := make([]string, 0, len(src))
	for k := range src {
		keys = append(keys, k)
	}
	return keys
}

// Map answers the section at path as a map, nil when there is none.
func (c *Config) Map(path string) map[string]any {
	v, _ := c.Get(path)
	m, _ := asMap(v)
	return m
}

// Root answers the file's tree.
func (c *Config) Root() map[string]any { return c.root }

// ReadYAML decodes the YAML file at path into v; a missing file leaves v
// untouched and is not an error.
func ReadYAML(path string, v any) error {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// WriteYAML encodes v as YAML and writes it to path atomically.
func WriteYAML(path string, v any) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, b)
}
