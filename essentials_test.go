package essentials_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/essentials/internal/esstest"
	"gopkg.in/yaml.v3"

	_ "github.com/golem-mc/essentials"
)

// TestEveryNameRegisters starts the plugin and checks that each command it
// registers keeps every name plugin.yml lists, and that no name is listed
// for two commands (/head belongs to skull, not hat).
func TestEveryNameRegisters(t *testing.T) {
	env := esstest.Start(t)
	var yml struct {
		Commands map[string]struct {
			Aliases []string `yaml:"aliases"`
		} `yaml:"commands"`
	}
	if err := yaml.Unmarshal(core.Resource("plugin.yml"), &yml); err != nil {
		t.Fatal(err)
	}
	listed := map[string]string{}
	for cmd, c := range yml.Commands {
		for _, n := range append([]string{cmd}, c.Aliases...) {
			n = strings.ToLower(n)
			if o, ok := listed[n]; ok && o != cmd {
				t.Errorf("plugin.yml lists %q for both %s and %s", n, o, cmd)
			}
			listed[n] = cmd
		}
	}
	n := 0
	for cmd, c := range yml.Commands {
		got, ok := env.Ess.CommandNames(cmd)
		if !ok {
			continue
		}
		n++
		for _, name := range append([]string{cmd}, c.Aliases...) {
			if !slices.Contains(got, strings.ToLower(name)) {
				t.Errorf("/%s lost the name %q; registered %v", cmd, name, got)
			}
		}
	}
	if n == 0 {
		t.Fatal("no command registered")
	}
}
