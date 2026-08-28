# Contributing

Thanks for your interest. This document covers the setup, the conventions, and the one rule that is not negotiable.

## The one rule

**A command must never print a result it did not compute.**

No placeholder output, no sample data, no simulated success. If a code path cannot produce a real answer, it returns an error saying so.

The reason is specific to this tool. Its output is read by people deciding whether to apply a plan, whether a bucket is public, whether a change is safe. A placeholder result is indistinguishable from a real one at the point of reading, and it will be acted on. An error is not.

There are helpers for this in `internal/cli/runtime.go`:

```go
return notImplemented("Cloud context management", "docs/ROADMAP.md#contexts")
return requiresBackend("infractl policy")
```

A pull request that adds `fmt.Println("Scanning...")` followed by `return nil` will be asked to change.

## Setup

Go 1.25 or later.

```bash
git clone https://github.com/Ashutosh0x/infra-control.git
cd infra-control
make build
make test
```

| Target | Does |
| --- | --- |
| `make build` | Compile all four binaries into `./bin` |
| `make test` | Run the test suite |
| `make lint` | Run golangci-lint |
| `make fmt` | Run gofmt |
| `make check` | Format, lint, test |

Run `make check` before opening a pull request.

## Repository layout

| Path | Contains |
| --- | --- |
| `cmd/` | Entry points: `infractl`, `controller`, `worker`, `mcp-server` |
| `internal/cli/` | Cobra commands. One file per command group |
| `internal/ui/` | Terminal presentation: colour, tables, diffs, spinners, output formats |
| `internal/terraform/` | State and plan parsing, attribute comparison |
| `internal/graph/` | Dependency graph and blast radius |
| `internal/risk/` | Risk scoring engine |
| `internal/store/` | Postgres and Redis persistence |
| `pkg/types/` | Shared types. Importable by external code |
| `docs/` | Documentation |

`internal/` is private to this module. Anything an external consumer needs belongs in `pkg/`.

## Adding a command

1. Put it in the right file under `internal/cli/`, or create one for a new group.
2. Set `GroupID` to `analyse`, `platform`, or `system` so it lands in the right section of `--help`.
3. Write `Long` and `Example`. The examples are what people actually read.
4. Build a `ui.View` with both a `Data` payload and a `Table`, then hand it to `rt.write`. That is what makes every format work at once.
5. Return errors through `failf` with the right exit code, never `os.Exit`.
6. Register completions for any flag with a closed set of values.

```go
var exampleCmd = &cobra.Command{
	Use:     "example <input>",
	Short:   "One line, lowercase, no trailing period",
	GroupID: "analyse",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireFile(args[0], "input file"); err != nil {
			return err
		}

		result, err := doTheWork(args[0])
		if err != nil {
			return failf(ExitError, "%w", err)
		}

		table := ui.NewTable(
			ui.Column{Title: "NAME", MinWidth: 12, Truncatable: true},
			ui.Column{Title: "COUNT", Align: ui.AlignRight},
		)
		for _, item := range result.Items {
			table.StringRow(item.Name, strconv.Itoa(item.Count))
		}

		return rt.write(ui.View{
			Data:  result,
			Table: table,
			Empty: "No items matched those filters.",
		})
	},
}
```

## Output rules

These are enforced by tests in `internal/ui`. Breaking one is a bug.

| Rule | Reason |
| --- | --- |
| Results to stdout, progress to stderr | `-o json \| jq` must work unfiltered |
| Machine formats carry no ANSI | Otherwise the payload is invalid |
| Never call `fmt.Println` directly in a command | Bypasses colour, width, and quiet handling |
| Sensitive values dropped before the payload is built | Masking later still leaks through `-o json` |
| Sort before output | Two runs on unchanged input must be byte-identical |
| No emoji | Renders inconsistently, means nothing to a screen reader, cannot be styled |

## Tests

Every behavioural change needs a test. Table-driven where it fits.

Say what the test protects, not what it does. `TestCompareAttributesIgnoresProviderBookkeeping` with a comment explaining that comparing `arn` would report drift on every resource every run is worth more than `TestCompare2`.

Cases worth covering when you touch parsing or comparison:

- Numeric type mismatches between state and live reads
- Integers past 2^53
- `count` and `for_each` index keys, including string keys
- Module-nested addresses
- Null in state versus absent live
- Determinism across repeated runs

```bash
go test ./...
go test -cover ./internal/...
go test -run TestParsePlan ./internal/terraform/ -v
```

## Commits and pull requests

Conventional Commits:

```
feat(drift): report unmanaged resources by default
fix(ui): measure display width past the CSI parameter bytes
docs(readme): document the live snapshot format
test(terraform): cover for_each string index keys
```

In the pull request, say what changed and why. If it changes output, paste the before and after. If it fixes a bug, describe the input that triggered it.

Checklist:

- [ ] `make check` passes
- [ ] Tests added for the change
- [ ] No placeholder output added
- [ ] Docs updated if behaviour changed
- [ ] No emoji in code, output, or docs

## Style

Standard Go, `gofmt`, and the `golangci-lint` configuration in `.golangci.yml`.

Comments explain why, not what. `// increment i` is noise. `// A dependency on a data source has no node to point at, so it is skipped rather than invented` is the reason someone will need in six months.

Errors are lowercase, no trailing punctuation, and wrap with `%w`:

```go
return fmt.Errorf("parse state file %s: %w", path, err)
```

An error a user will see should say what to do next:

```go
return failf(ExitUsage,
	"no managed resource at address %q.\n"+
		"  Run `infractl state list %s` to see the addresses in this state file.",
	address, path)
```

## Questions

Open a [Discussion](https://github.com/Ashutosh0x/infra-control/discussions) for design proposals or questions. Use issues for bugs and concrete feature requests.
