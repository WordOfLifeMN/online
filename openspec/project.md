# Project Conventions: WOLM `online`

## What this is

A Go CLI (`online`) that supports the Word of Life Ministries media workflow.
Run locally by the media operator on Windows; not a server, not a service.

## Layout

| Package | Role |
|---------|------|
| `cmd/` | Cobra commands. One file per command, `<noun>_<verb>.go` for subcommands. |
| `catalog/` | Domain model: `Catalog`, `CatalogSeri`, `CatalogMessage`, `OnlineResource`, plus the `Ministry` / `View` / `MessageType` enums. No I/O beyond JSON. |
| `gclient/` | Google API access (Sheets today). Translates sheet rows into `catalog` types. |
| `util/` | Small helpers: hashing, path normalization, stopwatch, indenting report. |

## Conventions

- Config via viper: `./online-config.yaml`, then `~/.wolm/online-config.yaml`.
  Every flag is bound with `viper.BindPFlag` so config file and flag are interchangeable.
- Credentials live in `~/.wolm/`. Never commit them, never log them.
- `initLogging()` at the top of every `RunE`. `log.Printf` is silent unless `--verbose`;
  user-facing output goes to `fmt.Printf` on stdout.
- Model types follow a `New…` / `Initialize()` / `Normalize()` lifecycle. `Initialize()`
  must be idempotent, guarded by an `initialized` field.
- External tools are invoked with `exec.Command`, not cgo bindings: `ffmpeg`,
  `faster-whisper-xxl`, `aws`.
- Tests are table-driven with `testify`; suites where a fixture is shared.
  Fixtures live in `testdata/`.

## Quality gate

```
make test     # go test ./...
go vet ./...
make build
```

## Vocabulary

- **seri** — the singular of "series", used to avoid the collision where a list of
  series is also "series".
- **message** — one recorded teaching. A message may belong to zero or more series.
- **ministry** — which arm of the church a message belongs to (`wol`, `core`, `tbo`,
  `ask-pastor`, `faith-freedom`, and CORE sub-ministries).
- **view / visibility** — `public`, `partner`, `private`, `raw`.
