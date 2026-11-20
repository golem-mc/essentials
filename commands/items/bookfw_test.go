package items

import (
	"testing"

	"github.com/golem-mc/golem/nbt"
)

func TestComponentTextCompound(t *testing.T) {
	c := nbt.Compound{
		"text":  nbt.String("a"),
		"extra": nbt.List{Items: []nbt.Tag{nbt.String("b"), nbt.Compound{"text": nbt.String("c")}}},
	}
	if got := componentText(c); got != "abc" {
		t.Fatalf("componentText = %q, want abc", got)
	}
}
