# Judge rules

Use this inventory to locate a `yomihon check` finding by its `rule_id`.
`SourceRule` is the literal `source_rule` authority in the finding; the source
link names the Go file that constructs it. The inventory describes existing
rules and does not define new behavior.

## Judge rule inventory

| RuleID | SourceRule | Finding source file |
|---|---|---|
| `collision.alias` | `yomihon` | [`internal/judge/graphrules.go`](../internal/judge/graphrules.go) |
| `collision.name` | `yomihon` | [`internal/judge/graphrules.go`](../internal/judge/graphrules.go) |
| `callout.title_markup` | `yomihon` | [`internal/judge/callout.go`](../internal/judge/callout.go) |
| `embed.block_missing` | `yomihon` | [`internal/judge/fragment.go`](../internal/judge/fragment.go) |
| `embed.section_missing` | `yomihon` | [`internal/judge/fragment.go`](../internal/judge/fragment.go) |
| `link.block_missing` | `yomihon` | [`internal/judge/fragment.go`](../internal/judge/fragment.go) |
| `link.broken` | `yomihon` | [`internal/judge/graphrules.go`](../internal/judge/graphrules.go) |
| `link.broken.path` | `yomihon` | [`internal/judge/diskref.go`](../internal/judge/diskref.go) |
| `link.section_missing` | `yomihon` | [`internal/judge/fragment.go`](../internal/judge/fragment.go) |
| `link.title_not_alias` | `yomihon` | [`internal/judge/graphrules.go`](../internal/judge/graphrules.go) |
| `map.disk_mismatch` | `yomihon` | [`internal/judge/graphrules.go`](../internal/judge/graphrules.go) |
| `map.disk_unlisted` | `yomihon` | [`internal/judge/graphrules.go`](../internal/judge/graphrules.go) |
| `path.entry_multi_target` | `yomihon` | [`internal/judge/pathrules.go`](../internal/judge/pathrules.go) |
| `path.entry_noncanonical` | `yomihon` | [`internal/judge/pathrules.go`](../internal/judge/pathrules.go) |
| `path.entry_outside_branch` | `yomihon` | [`internal/judge/pathrules.go`](../internal/judge/pathrules.go) |
| `path.local_orphan` | `yomihon` | [`internal/judge/pathrules.go`](../internal/judge/pathrules.go) |
| `path.nesting_too_deep` | `yomihon` | [`internal/judge/pathrules.go`](../internal/judge/pathrules.go) |
| `path.role_conflict` | `yomihon` | [`internal/judge/pathrules.go`](../internal/judge/pathrules.go) |
| `path.role_duplicate` | `yomihon` | [`internal/judge/pathrules.go`](../internal/judge/pathrules.go) |
| `path.role_invalid` | `yomihon` | [`internal/judge/pathrules.go`](../internal/judge/pathrules.go) |
| `path.role_misplaced` | `yomihon` | [`internal/judge/pathrules.go`](../internal/judge/pathrules.go) |
| `path.role_missing` | `yomihon` | [`internal/judge/pathrules.go`](../internal/judge/pathrules.go) |
| `path.role_nested_primary` | `yomihon` | [`internal/judge/pathrules.go`](../internal/judge/pathrules.go) |
| `path.role_on_entry` | `yomihon` | [`internal/judge/pathrules.go`](../internal/judge/pathrules.go) |
| `provenance.unresolved` | `vault-schema.toml#supersession` | [`internal/judge/graphrules.go`](../internal/judge/graphrules.go) |
| `provenance.unresolved` | `yomihon` | [`internal/judge/graphrules.go`](../internal/judge/graphrules.go) |
| `scan.skipped` | `yomihon` | [`internal/judge/skipped.go`](../internal/judge/skipped.go) |
| `schema.domain_folder` | `vault-schema.toml#rules` | [`internal/judge/schema.go`](../internal/judge/schema.go) |
| `schema.enum` | `vault-schema.toml` | [`internal/judge/schema.go`](../internal/judge/schema.go) |
| `schema.frontmatter` | `vault-schema.toml` | [`internal/judge/schema.go`](../internal/judge/schema.go) |
| `schema.language` | `vault-schema.toml` | [`internal/judge/schema.go`](../internal/judge/schema.go) |
| `schema.legacy_tag` | `vault-schema.toml#rules` | [`internal/judge/schema.go`](../internal/judge/schema.go) |
| `schema.provenance` | `vault-schema.toml#rules` | [`internal/judge/schema.go`](../internal/judge/schema.go) |
| `schema.required` | `vault-schema.toml` | [`internal/judge/schema.go`](../internal/judge/schema.go) |
| `schema.slug` | `vault-schema.toml#rules` | [`internal/judge/schema.go`](../internal/judge/schema.go) |
| `schema.unknown_key` | `vault-schema.toml` | [`internal/judge/schema.go`](../internal/judge/schema.go) |
| `schema.unmatched_knowledge_dir` | `vault-schema.toml#scan` | [`internal/judge/schema.go`](../internal/judge/schema.go) |
| `supersession.archived_navigation_target` | `vault-schema.toml#supersession` | [`internal/judge/supersession.go`](../internal/judge/supersession.go) |
| `supersession.predecessor_not_archived` | `vault-schema.toml#supersession` | [`internal/judge/supersession.go`](../internal/judge/supersession.go) |

`provenance.unresolved` cites `yomihon` for `based_on` and `related`, and
`vault-schema.toml#supersession` for contract-configured replacement fields.
If a configured field is also named `based_on` or `related`,
`appendConfiguredReferences` preserves that field's `yomihon` authority.
A RuleID prefix alone does not determine authority.

Study-path predicates and IDs belong to
[`internal/sequence/sequence.go`](../internal/sequence/sequence.go).
[`internal/judge/pathrules.go`](../internal/judge/pathrules.go) carries those
diagnostics through `pathFinding` into the judge's finding format; it does
not own the study-path grammar.

## Maintaining the inventory

When a rule, authority, or source file changes, reconcile this table with the
finding's construction path, the registry in
[`internal/judge/command.go`](../internal/judge/command.go), and the frozen
[`JSONL goldens`](../internal/judge/testdata/golden).
Keep one row per distinct `(RuleID, SourceRule)` pair.
A new rule needs a fixture under `internal/judge/testdata/` that emits it
before its row can be added.

[`TestJudgeRuleInventory`](../internal/judge/sourcerule_test.go) checks the
actual table's format, complete registered ID set, exact authority-pair set
from the literal JSONL goldens, and existing Go source-file links whose labels
match their target paths. Lines beginning with `|` after trimming whitespace
are allowed only in the inventory table. Run it with:

```sh
go test ./internal/judge -run '^TestJudgeRuleInventory$' -count=1
```

Source-file existence is the mechanical limit: a reviewer must follow each
finding's construction path to confirm that the linked file owns it, including
conditional authorities and the sequence-to-judge boundary. The test does not
infer emission ownership from Go source or authorize changing golden bytes.
