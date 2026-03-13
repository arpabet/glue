# Gluegen

`gluegen` is Glue's code-generation tool for package-local component scanning.

Go does not support Spring-style runtime package scanning. Instead, `gluegen` scans source packages and generates `glue_gen.go` files with package-local scanners.

## Install

```bash
go install go.arpabet.com/glue/cmd/gluegen@latest
```

## Generate

Scan the current module:

```bash
gluegen ./...
```

This generates `glue_gen.go` in each package that contains Glue components.

Generated public API:

```go
func GlueGen() glue.Scanner
```

Usage:

```go
ctx, err := glue.New(
    myapp.GlueGen(),
    mydb.GlueGen(),
    myhttp.GlueGen(),
)
```

## Inclusion Rules

`gluegen` includes package-local struct types when at least one of the following is true:

* the type has a `//glue:component`, `//glue:factory`, or `//glue:scanner` comment
* the struct has a field with an `inject:"..."` tag
* the struct has a field with a `value:"..."` tag
* the struct embeds `glue.FactoryBean`
* the struct embeds `glue.ContextFactoryBean`
* the struct embeds `glue.Scanner`

The generated scanner registers the struct types themselves. Factory-produced objects still follow normal `FactoryBean` runtime behavior.

## Check Mode

Use `--check` in CI to fail when generated files are stale:

```bash
gluegen --check ./...
```

## Notes

* generation is package-local; there is no global root scanner by default
* package composition stays explicit in application wiring
* `gluegen` generates scanners, not runtime package discovery
