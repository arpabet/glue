# Hierarchy and Search

## Parent and Child Containers

`Extend(...)` creates a child container that sees both its own beans and parent-visible beans.
The parent still sees only its own beans.

```go
parent, err := glue.New(new(a))
child, err := parent.Extend(new(b))
```

Destroying the child does not destroy the parent.

## Lazy Children

`glue.Child(name, scan...)` registers a lazily created child container.

Methods:
* `Object()`
* `ObjectWithContext(ctx)`
* `Close()`
* `CloseWithContext(ctx)`

Use `ObjectWithContext(ctx)` when child creation depends on context-aware initialization or context-aware factories.

## Search Levels

Use named search constants instead of raw numbers:

```go
const (
    DefaultSearchLevel         = SearchFallback
    SearchFallback             = 0
    SearchCurrent              = 1
    SearchCurrentAndParent     = 2
    SearchCurrentAndTwoParents = 3
    SearchCurrentAndAllParents = -1
)
```

Meaning:
* `SearchFallback`: current container, otherwise nearest parent
* `SearchCurrent`: current container only
* `SearchCurrentAndParent`: current + direct parent
* `SearchCurrentAndTwoParents`: current + parent + grandparent
* `SearchCurrentAndAllParents`: all visible ancestors

These levels are used by:
* `Container.Bean(...)`
* `Container.Lookup(...)`
* `inject:"...,search=..."`
