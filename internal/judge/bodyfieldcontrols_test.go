package judge_test

func bodyFieldMutations() []agreementMutation {
	const file = "internal/graph/bodystructure.go"
	modes := []agreementMutation{
		{Name: "f3-list-id", Property: "F3", Identity: "list-id", File: file, Function: "collectStructureNode", Needle: "BodyList{ID: id}", Fault: "BodyList{ID: id + 1}"},
		{Name: "f3-list-parent", Property: "F3", Identity: "list-parent", File: file, Function: "collectListMembership", Needle: "o.structure.lists[id-1].ParentRowID = index.rows[node.Parent()]", Fault: "o.structure.lists[id-1].ParentRowID = 0"},
		{Name: "f3-row-id", Property: "F3", Identity: "row-id", File: file, Function: "collectStructureNode", Needle: "BodyRow{ID: id}", Fault: "BodyRow{ID: id + 1}"},
		{Name: "f3-row-list", Property: "F3", Identity: "row-list", File: file, Function: "collectRow", Needle: "row.ListID = index.lists[node.Parent()]", Fault: "row.ListID = 0"},
		{Name: "f3-row-fallback", Property: "F3", Identity: "row-fallback", File: file, Function: "collectRow", Needle: "row.Fallback, row.HasFallback = bodyLinesRange(node)", Fault: "row.Fallback, row.HasFallback = bodyLinesRange(node); row.Fallback.Start++"},
		{Name: "f3-row-presence", Property: "F3", Identity: "row-presence", File: file, Function: "collectRow", Needle: "row.Fallback, row.HasFallback = bodyLinesRange(node)", Fault: "row.Fallback, row.HasFallback = bodyLinesRange(node); row.HasFallback = !row.HasFallback"},
		{Name: "f3-row-first-child", Property: "F3", Identity: "row-first-child", File: file, Function: "collectRow", Needle: "_, row.FirstBlockHasLines = bodyLinesRange(c)", Fault: "_, row.FirstBlockHasLines = bodyLinesRange(node)"},
		{Name: "f3-outline-list", Property: "F3", Identity: "outline-list", File: file, Function: "collectOutline", Needle: "BodyOutline{ListID: id}", Fault: "BodyOutline{ListID: id + 1}"},
		{Name: "f3-outline-stray", Property: "F3", Identity: "outline-stray", File: file, Function: "collectOutline", Needle: "BodyOutline{StrayID: len(o.structure.strays)}", Fault: "BodyOutline{StrayID: len(o.structure.strays) + 1}"},
		{Name: "f3-inline-text-span", Property: "F3", Identity: "inline-text-span", File: file, Function: "bodyInlineParts", Needle: "Start: node.Segment.Start, Stop: node.Segment.Stop", Fault: "Start: node.Segment.Start + 1, Stop: node.Segment.Stop"},
		{Name: "f3-inline-code-span", Property: "F3", Identity: "inline-code-span", File: file, Function: "bodyInlineParts", Needle: "BodyInlinePart{Span: span, Code: true}", Fault: "BodyInlinePart{Span: Span{Start: span.Start + 1, Stop: span.Stop}, Code: true}"},
		{Name: "f3-inline-code-kind", Property: "F3", Identity: "inline-code-kind", File: file, Function: "bodyInlineParts", Needle: "Code: true", Fault: "Code: false"},
		{Name: "f3-accessor-outline", Property: "F3", Identity: "accessor-outline", File: file, Function: "Outline", Needle: "bodyValues(f.data.structure.outline)", Fault: "bodyValues(f.data.structure.outline[:0])"},
		{Name: "f3-accessor-list", Property: "F3", Identity: "accessor-list", File: file, Function: "List", Needle: "return f.data.structure.lists[id-1], true", Fault: "return f.data.structure.lists[id-1], false"},
		{Name: "f3-accessor-row", Property: "F3", Identity: "accessor-row", File: file, Function: "Row", Needle: "return f.data.structure.rows[id-1], true", Fault: "return f.data.structure.rows[id-1], false"},
		{Name: "f3-accessor-list-rows", Property: "F3", Identity: "accessor-list-rows", File: file, Function: "ListRows", Needle: "yield(f.data.structure.rows[rowID-1])", Fault: "yield(BodyRow{ID: rowID})"},
		{Name: "f3-accessor-row-blocks", Property: "F3", Identity: "accessor-row-blocks", File: file, Function: "RowBlocks", Needle: "bodyValues(f.data.structure.blocks[id-1])", Fault: "bodyValues(f.data.structure.blocks[id-1][:0])"},
		{Name: "f3-accessor-row-parts", Property: "F3", Identity: "accessor-row-parts", File: file, Function: "RowInlineParts", Needle: "bodyValues(f.data.structure.parts[id-1])", Fault: "bodyValues(f.data.structure.parts[id-1][:0])"},
		{Name: "f3-accessor-stray", Property: "F3", Identity: "accessor-stray", File: file, Function: "StrayBlocks", Needle: "bodyValues(f.data.structure.strays[id-1])", Fault: "bodyValues(f.data.structure.strays[id-1][:0])"},
		{Name: "f3-accessor-rich", Property: "F3", Identity: "accessor-rich", File: file, Function: "RichHeadings", Needle: "bodyValues(f.data.richHeadings)", Fault: "bodyValues(f.data.richHeadings[:0])"},
		{Name: "f3-accessor-rich-origin", Property: "F3", Identity: "accessor-rich-origin", File: file, Function: "RichHeadingOrigins", Needle: "bodyValues(f.data.richOrigins[id-1])", Fault: "bodyValues(f.data.richOrigins[id-1][:0])"},
		{Name: "f3-accessor-paired", Property: "F3", Identity: "accessor-paired", File: file, Function: "PairedEmphasisOpeners", Needle: "bodyValues(f.data.structure.openers)", Fault: "bodyValues(f.data.structure.openers[:0])"},
	}
	return bodyNativeMutations(append(modes, bodyGroupMutations()...))
}

func bodyGroupMutations() []agreementMutation {
	return []agreementMutation{
		{Name: "f3-group-heading-name", Property: "F3", Identity: "group-heading-name", File: "internal/sequence/sequence.go", Function: "heading", Needle: "Name: strings.TrimSpace(name)", Fault: "Name: strings.TrimSpace(name) + \"wrong\""},
		{Name: "f3-group-heading-level", Property: "F3", Identity: "group-heading-level", File: "internal/sequence/sequence.go", Function: "heading", Needle: "Level: node.Level", Fault: "Level: node.Level + 1"},
		{Name: "f3-group-heading-line", Property: "F3", Identity: "group-heading-line", File: "internal/sequence/sequence.go", Function: "heading", Needle: "Line: line", Fault: "Line: line + 1"},
		{Name: "f3-group-heading-role", Property: "F3", Identity: "group-heading-role", File: "internal/sequence/sequence.go", Function: "heading", Needle: "Role: role", Fault: "Role: role + 1"},
		{Name: "f3-container-name", Property: "F3", Identity: "container-name", File: "internal/sequence/sequence.go", Function: "container", Needle: "Name:         strings.TrimSpace(name)", Fault: "Name: strings.TrimSpace(name) + \"wrong\""},
		{Name: "f3-container-line", Property: "F3", Identity: "container-line", File: "internal/sequence/sequence.go", Function: "container", Needle: "Line:         line", Fault: "Line: line + 1"},
		{Name: "f3-container-role", Property: "F3", Identity: "container-role", File: "internal/sequence/sequence.go", Function: "container", Needle: "Role:         role", Fault: "Role: role + 1"},
		{Name: "f3-container-container", Property: "F3", Identity: "container-container", File: "internal/sequence/sequence.go", Function: "container", Needle: "Container:    true", Fault: "Container: false"},
		{Name: "f3-container-anchortarget", Property: "F3", Identity: "container-anchortarget", File: "internal/sequence/sequence.go", Function: "container", Needle: "AnchorTarget: anchor", Fault: "AnchorTarget: anchor + \"wrong\""},
		{Name: "f3-container-anchorspan", Property: "F3", Identity: "container-anchorspan", File: "internal/sequence/sequence.go", Function: "container", Needle: "AnchorSpan:   anchorSpan", Fault: "AnchorSpan: Span{Start: anchorSpan.Start + 1, Stop: anchorSpan.Stop}"},
		{Name: "f3-container-invalid", Property: "F3", Identity: "container-invalid", File: "internal/sequence/sequence.go", Function: "container", Needle: "Invalid:      invalid", Fault: "Invalid: !invalid"},
		{Name: "f3-entry-item", Property: "F3", Identity: "entry-item", File: "internal/sequence/sequence.go", Function: "plainRow", Needle: "group.Items = append(group.Items, Item{Entry: entry})", Fault: "_ = group; _ = entry"},
		{Name: "f3-branch-item", Property: "F3", Identity: "branch-item", File: "internal/sequence/sequence.go", Function: "container", Needle: "group.Items = append(group.Items, Item{Branch: opened})", Fault: "_ = group; _ = opened"},
		{Name: "f3-document-groups", Property: "F3", Identity: "document-groups", File: "internal/sequence/sequence.go", Function: "ParseFacts", Needle: "Groups: p.roots", Fault: "Groups: nil"},
		{Name: "f3-document-diagnostics", Property: "F3", Identity: "document-diagnostics", File: "internal/sequence/sequence.go", Function: "ParseFacts", Needle: "Diagnostics: p.diagnostics", Fault: "Diagnostics: nil"},
		{Name: "f3-link-target", Property: "F3", Identity: "link-target", File: "internal/sequence/sequence.go", Function: "LiveScanFacts", Needle: "Target: h.target", Fault: "Target: \"wrong\""},
		{Name: "f3-link-display", Property: "F3", Identity: "link-display", File: "internal/sequence/sequence.go", Function: "LiveScanFacts", Needle: "Display: h.display", Fault: "Display: \"wrong\""},
		{Name: "f3-link-aliased", Property: "F3", Identity: "link-aliased", File: "internal/sequence/sequence.go", Function: "LiveScanFacts", Needle: "Aliased: h.aliased", Fault: "Aliased: false"},
		{Name: "f3-link-fragment", Property: "F3", Identity: "link-fragment", File: "internal/sequence/sequence.go", Function: "LiveScanFacts", Needle: "Fragment: h.fragment", Fault: "Fragment: false"},
		{Name: "f3-link-span", Property: "F3", Identity: "link-span", File: "internal/sequence/sequence.go", Function: "LiveScanFacts", Needle: "Span: h.span()", Fault: "Span: Span{}"},
	}
}
