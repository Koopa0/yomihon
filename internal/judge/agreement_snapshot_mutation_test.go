package judge_test

// Every borrowed product and the savings in complete body reads are observed
// through the snapshot's complete result, not an isolated helper return.
func snapshotBodyMutations() []agreementMutation {
	return []agreementMutation{
		{Name: "f3-snapshot-sharing", Property: "F3", Identity: "snapshot-body-sharing", File: "internal/snapshot/snapshot.go", Function: "deriveNote", Needle: "body := graph.ReadBody(read.parsed.Body)", Fault: "body := graph.ReadBody(read.parsed.Body); body = graph.ReadBody(read.parsed.Body)", Package: "./internal/snapshot", ControlTest: "TestSnapshotBodyProductsShareRecognition"},
		{Name: "f3-snapshot-document", Property: "F3", Identity: "snapshot-body-sharing", File: "internal/lexical/lexical.go", Function: "DocumentFromFacts", Needle: "projection := render.PlainProjectionFacts(facts)", Fault: "projection := render.PlainProjectionFacts(facts); projection.Text = \"\"", Package: "./internal/snapshot", ControlTest: "TestSnapshotBodyProductsShareRecognition"},
		{Name: "f3-snapshot-planned", Property: "F3", Identity: "snapshot-body-sharing", File: "internal/judge/planned.go", Function: "NewPlannedFacts", Needle: "set.add(extractPlannedNamesFrom(body.Source(), marks, &facts))", Fault: "set.add(extractPlannedNamesFrom(body.Source(), marks, &facts)[:0])", Package: "./internal/snapshot", ControlTest: "TestSnapshotBodyProductsShareRecognition"},
		{Name: "f3-snapshot-links", Property: "F3", Identity: "snapshot-body-sharing", File: "internal/judge/planned.go", Function: "LinkTargetsFacts", Needle: "return targets", Fault: "return targets[:0]", Package: "./internal/snapshot", ControlTest: "TestSnapshotBodyProductsShareRecognition"},
		{Name: "f3-snapshot-title", Property: "F3", Identity: "snapshot-body-sharing", File: "internal/render/render.go", Function: "DropsTitleHeadingFacts", Needle: "return dropped >= 0", Fault: "return dropped < 0", Package: "./internal/snapshot", ControlTest: "TestSnapshotBodyProductsShareRecognition"},
	}
}
