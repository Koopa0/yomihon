package shell

// findingLists is how many entries the finding lists of a gathering hold
// between them, so that a test can ask whether it found anything at all. The
// health page counts its own rows from the same lists; this is no count a page
// states.
func findingLists(f *Findings) int {
	total := len(f.Blocked) + len(f.Skipped) + len(f.Unwritten) + len(f.TitleOnly) +
		len(f.Collisions) + len(f.NavigationFaults) + len(f.FrontmatterUnreadable) + len(f.SchemaFaults)
	for _, group := range f.Islands {
		total += len(group.Notes)
	}
	return total
}
