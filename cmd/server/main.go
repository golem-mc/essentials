// Command server is the golem server with EssentialsX built in: each
// module's init registers it before the server starts.
package main

import (
	_ "github.com/golem-mc/essentials"
	_ "github.com/golem-mc/essentials/antibuild"
	_ "github.com/golem-mc/essentials/chat"
	_ "github.com/golem-mc/essentials/protect"
	_ "github.com/golem-mc/essentials/spawn"
	"github.com/golem-mc/golem/app"
)

func main() { app.Main() }
