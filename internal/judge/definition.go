package judge

// The definition rule reports a link reference definition, "[label]: address",
// that no link or image in its own note resolves through. The page shows such a
// definition nowhere, so what its author meant to cite is invisible: the label
// was renamed on one side only, or two markers were written together as
// "[E1][E2]", which the parser reads as one link whose text is E1 and whose
// label is E2. The parser's own resolution says which label each link used, so
// check and the page agree on what is used. Two definitions of one label are
// judged as one, because the parser answers every reference from the first.

// checkUnusedDefinitions reports each note's unused link reference definitions.
func checkUnusedDefinitions(notes []note, authority scanAuthority) []Finding {
	var out []Finding
	for i := range notes {
		n := &notes[i]
		if !authority.egressAllowed(n.path) {
			continue
		}
		for _, def := range n.unusedDefinitions {
			out = append(out, unusedDefinition(n, def))
		}
	}
	return out
}

// unusedDefinition is a link reference definition nothing in its note uses.
func unusedDefinition(n *note, def linkDefinition) Finding {
	return Finding{
		RuleID:          "link.unused.definition",
		Severity:        SeverityWarn,
		Path:            n.path,
		Line:            new(def.line),
		Message:         "link reference definition [" + def.label + "] is never used",
		Evidence:        "no link or image in this note refers to the label [" + def.label + "]",
		SuggestedAction: "use the definition, remove it, or put a space between adjacent references such as [E1] [E2]",
		SourceRule:      sourceYomihon,
		Target:          new(def.label),
		Fingerprint:     fingerprint("link.unused.definition", n.path, def.label),
	}
}
