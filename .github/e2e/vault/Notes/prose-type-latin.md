---
type: concept
lang: en
based_on: "[[Notes/alpha]]"
---

# Prose type in English

This note only exists to measure how a note in a Latin-script language is set: paragraphs, inline code, links, lists, quotations, callouts, tables, code blocks and footnotes, each written once so that every line box in the column can be read at every text size.

## Paragraphs and inline

Most engineers assume that raising PostgreSQL to `Repeatable Read` rules out every concurrency anomaly. Snapshot isolation does stop dirty reads and non-repeatable reads, but two transactions that read the same precondition and then write disjoint rows never collide on a row lock, so the database cannot see the conflict[^skew].

A `40001` is not an incident; it is part of the protocol. Read [[alpha]] before retrying, then [[beta|another account]]. Press <kbd>Ctrl</kbd>+<kbd>K</kbd> to search, with ==a marked phrase==, **bold** and *italic* side by side.

### Third level

The body of a section, one step under a chapter.

#### Fourth level

The body of a subsection, the same size as the text.

##### Fifth level

The body of an item.

###### Sixth level

The body of a clause.

## Lists

- First, never trigger an irreversible external call inside the closure; write it to an outbox table in the same transaction.
- Second, every wait must watch `ctx.Done()` so a caller that hung up long ago does not leave a retry spinning.
  - A nested item, one level in.
- Third, the number of retries has a ceiling.

1. Read the precondition.
2. Write the disjoint rows.
3. Let the database find the dependency cycle at commit.

## Quotations and callouts

> If the business rule reads "count who is on call and, if at least two, take yourself off", the snapshot it read has already expired by the time of the commit.

> [!note] An invariant that spans rows is not guarded by the snapshot alone
> The database has to know which query range your write depends on; otherwise both transactions commit legally and leave a legal but ruined state with nobody on call.

> [!warning] Do not call outside services inside a retried closure
> The transaction reruns; the outside call is never taken back.

> [!example]- A folded example
> The reader has to press to see this line.

## Code and tables

```go
func retry(ctx context.Context, fn func(context.Context) error) error {
	for {
		if err := fn(ctx); !errors.Is(err, errSerialization) {
			return err
		}
	}
}
```

| Isolation level | Dirty read | Write skew |
| --- | --- | --- |
| Read Committed | No | Possible |
| Repeatable Read | No | Possible |
| Serializable | No | No |

---

The last paragraph after the rule. LATIN-END

[^skew]: This is write skew: the precondition is a set query, while the writes are to individual rows.
