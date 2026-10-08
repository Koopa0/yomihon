package judge_test

// These faults use the existing production-overlay driver and real public
// Check/RunCheck controls. Removing individual GFM members leaves other grammar
// extensions present, so each mode targets one recognition boundary.
func stage4Mutations() []agreementMutation {
	const grammar = "goldmark.WithExtensions(extension.GFM, bodyFootnotes{prefix: prefix})"
	return []agreementMutation{
		{Name: "stage4-fence-info", Property: "S4", Identity: "fence-info", File: "internal/judge/extract.go", Function: "structureFrom", Needle: "codeZones = append(codeZones, code.Span)", Fault: "if code.Kind != graph.CodeFence { codeZones = append(codeZones, code.Span) }", Package: "./internal/judge", ControlTest: "TestAgreementStage4FenceInfo"},
		{Name: "stage4-footnote-paragraph", Property: "S4", Identity: "footnote-paragraph", File: "internal/graph/bodygrammar.go", Function: "newBodyMarkdown", Needle: grammar, Fault: "goldmark.WithExtensions(extension.GFM)", Package: "./internal/judge", ControlTest: "TestAgreementStage4FootnoteParagraph"},
		{Name: "stage4-task-path", Property: "S4", Identity: "task-path", File: "internal/graph/bodygrammar.go", Function: "newBodyMarkdown", Needle: grammar, Fault: "goldmark.WithExtensions(extension.Linkify, extension.Table, extension.Strikethrough, bodyFootnotes{prefix: prefix})", Package: "./internal/judge", ControlTest: "TestAgreementStage4TaskPath"},
		{Name: "stage4-linkify-code", Property: "S4", Identity: "linkify-code", File: "internal/graph/bodygrammar.go", Function: "newBodyMarkdown", Needle: grammar, Fault: "goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.TaskList, bodyFootnotes{prefix: prefix})", Package: "./internal/judge", ControlTest: "TestAgreementStage4LinkifyCode"},
		{Name: "stage4-unused-footnote", Property: "S4", Identity: "unused-footnote", File: "internal/judge/extract.go", Function: "structureFrom", Needle: "if !definition.Emitted", Fault: "if !definition.Emitted && false", Package: "./internal/judge", ControlTest: "TestAgreementStage4UnusedFootnote"},
	}
}
