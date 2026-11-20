# Contributing

## Setup

Go is pinned in `mise.toml`. With [mise](https://mise.jdx.dev/) installed,
run Go through it so you get that version:

```bash
mise install
mise exec -- go build ./...
mise exec -- go test ./commands/items/ -run TestHat
```

`PORTING.md` explains the layout and how a command is ported from
EssentialsX's Java code.

## Before you open a pull request

```bash
mise run fmt            # gofmt
mise run fix            # go fix
mise run tidy           # go mod tidy
mise run build
mise run lint
mise run test:changed   # repeats the tests your branch adds or changes
mise run check          # everything above plus the whole suite (about a minute)
```

CI runs the same checks on every pull request.

## Commit messages and releases

Pull requests are squash-merged with their title as the commit message, and
the title must be a [conventional commit](https://www.conventionalcommits.org/):

```
feat(spawn): add per-world spawn points
fix: keep the hat when /hat is refused
docs: explain the launcher setup
feat!: rename the economy service
```

Use the scope for the part you changed: `core`, `admin`, `economy`, `items`,
`messaging`, `player`, `teleport`, `spawn`, `chat`, `protect` or
`antibuild`. A check on each pull request rejects a title that isn't a
conventional commit.

Releases are made by [release-please](https://github.com/googleapis/release-please).
It keeps a release pull request open that bumps the version and updates
`CHANGELOG.md` from the commits on `main`. Merging it tags the release
(`vX.Y.Z`), which is the version servers put in `launcher.json`. Before
1.0.0, `feat` raises the minor version and `fix` the patch version; a
breaking change (`!`) raises the minor version.
