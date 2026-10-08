package judge_test

// Each fault enters the existing compiler-overlay driver and reads the actual
// page and complete public check output, with its unchanged prose controls.
func stage5Mutations() []agreementMutation {
	return []agreementMutation{
		{Name: "stage5-code-boundary", Property: "S5", Identity: "code-boundary", File: "internal/render/wikilink.go", Function: "replaceOutside", Needle: "withinAny(skip, loc[0], loc[0]+1)", Fault: "withinAny(skip, loc[0], loc[1])", Package: "./internal/judge", ControlTest: "TestAgreementStage5CodeBoundary"},
	}
}
