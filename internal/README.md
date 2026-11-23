# internal

One package per feature. See `docs/design.md#packages` for what each owns and
the rules between them. The short version: `proxy` is the only package that
makes outbound HTTP calls, packages depend on interfaces, and `cmd/mcpdeck`
does the wiring.
