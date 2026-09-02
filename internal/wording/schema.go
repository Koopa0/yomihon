package wording

// SchemaPart is one piece of what a page says about a schema finding: either
// words in the reader's own language, or one of the note's own words, shown
// the way the note wrote it.
//
// The sentence arrives in pieces because the note's words are the reader's
// evidence and have to survive as text rather than be folded into a sentence
// built elsewhere. Code says which kind of piece this is; the text is the same
// field either way, so there is no piece carrying two texts and none carrying
// none — a shape a page could only render as silence.
type SchemaPart struct {
	Text string
	Code bool
}

// SchemaSentence is what a page says about one schema finding, given the rule
// that fired and whatever that rule named: the frontmatter field at fault, the
// value it carried, and the folder the note sits in.
//
// The finding carries a sentence of its own, and this is deliberately not it.
// That one is written once, in one language, for a format other programs read;
// repeating it on a page would put an English sentence in front of a reader
// who chose otherwise, and would tie what the page says to bytes that are
// frozen for somebody else's benefit. What the page shares with the command is
// the verdict, not its wording.
//
// A rule this does not know still says something, and says which rule it was,
// because a page that goes quiet about a fault is the fault this whole surface
// exists to end.
func SchemaSentence(lang Lang, ruleID, field, target, folder string) []SchemaPart {
	code := func(s string) SchemaPart { return SchemaPart{Text: s, Code: true} }
	text := func(p Phrase) SchemaPart { return SchemaPart{Text: p.In(lang)} }

	switch ruleID {
	case "schema.enum":
		return []SchemaPart{code(field), text(schemaWrittenAs), code(target), text(schemaNotInList)}
	case "schema.language":
		return []SchemaPart{code(field), text(schemaWrittenAs), code(target), text(schemaNotALanguageTag)}
	case "schema.slug":
		return []SchemaPart{code(field), text(schemaWrittenAs), code(target), text(schemaNotASlug)}
	case "schema.domain_folder":
		return []SchemaPart{
			code(field), text(schemaWrittenAs), code(target),
			text(schemaFolderMismatch), code(folder), text(schemaFolderMismatchEnd),
		}
	case "schema.legacy_tag":
		return []SchemaPart{code(target), text(schemaLegacyTagIn), code(field), text(schemaLegacyTagEnd)}
	case "schema.required":
		return []SchemaPart{text(schemaRequiredStart), code(field), text(schemaRequiredEnd)}
	case "schema.unknown_key":
		return []SchemaPart{code(target), text(schemaUnknownKey)}
	case "schema.provenance":
		return []SchemaPart{
			text(schemaProvenanceStart), code("based_on"),
			text(schemaProvenanceMiddle), code("source_locator"), text(schemaProvenanceEnd),
		}
	}
	return []SchemaPart{text(schemaUnknownRuleStart), code(ruleID), text(schemaUnknownRuleEnd)}
}

// The pieces the sentences above are made of. They are fragments rather than
// whole sentences because the note's own words sit between them, and a
// fragment still has to read as part of one sentence in both languages.
var (
	schemaWrittenAs = both(" 寫的 ", " is written as ")

	schemaNotInList = both("不在 schema 的允許清單裡。", ", which is not in the schema's list.")

	schemaNotALanguageTag = both(
		"不是合法的語言標籤(BCP 47)。",
		", which is not a valid BCP 47 language tag.")

	schemaNotASlug = both(
		"不符合 schema 給 slug 的格式。",
		", which is not the shape the schema gives a slug.")

	schemaFolderMismatch    = both("與這篇所在的資料夾 ", ", which does not match the folder this note sits in, ")
	schemaFolderMismatchEnd = both(" 不一致。", ".")

	schemaLegacyTagIn  = both(" 寫在 ", " in ")
	schemaLegacyTagEnd = both(" 裡,是舊式標籤,schema 要的是一個欄位。", " is a legacy tag; the schema asks for a field.")

	schemaRequiredStart = both("schema 要求這種筆記寫 ", "The schema requires ")
	schemaRequiredEnd   = both(",這篇沒有。", " on a note of this kind, and this one has none.")

	schemaUnknownKey = both(" 不是 schema 認得的欄位。", " is not a field the schema knows.")

	schemaProvenanceStart  = both("這篇 concept 既沒寫 ", "This concept has neither ")
	schemaProvenanceMiddle = both(" 也沒寫 ", " nor ")
	schemaProvenanceEnd    = both("。", ".")

	schemaUnknownRuleStart = both("schema 對這篇報了一項 ", "The schema reported ")
	schemaUnknownRuleEnd   = both(",而這個頁面還沒有它的說法。", " about this note, and this page has no words for it yet.")
)

// The two things the health page says about frontmatter. They are separate
// sections because a reader does something different about each: a note whose
// frontmatter cannot be read needs its YAML repaired before anything else can
// be judged, while a note the schema disagrees with has a specific field to
// change.
var (
	HealthFrontmatterTitle = both(
		"frontmatter 讀不出來的筆記",
		"Notes whose frontmatter could not be read")

	HealthFrontmatterLede = both(
		"這些筆記的 frontmatter 不是合法的 YAML。內文照常閱讀、連結照常解析,但它們宣告的每一項都無法判斷;在依狀態分組裡,它們只出現在「無法判讀」一格。",
		"These notes' frontmatter is not valid YAML. The body still reads and the links still resolve, but nothing they declare could be judged; in the grouping by status they appear only in the cell for what could not be read.")

	HealthSchemaTitle = both(
		"schema 有話說的筆記",
		"Notes the schema has something to say about")

	HealthSchemaLede = both(
		"這些筆記的 frontmatter 讀得出來,但有 schema 不接受的地方。每一篇的細節在它自己的頁面上。",
		"These notes' frontmatter reads, and something in it is not what the schema accepts. Each note's own page says which.")
)
