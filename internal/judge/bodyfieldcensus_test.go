package judge_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Every field declared by the migrated structure surface has a registered
// compiling fault. A new field or accessor must name its observation too.
func TestBodyStructureFieldCensus(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	declarations := make(map[string]string)
	for _, file := range []string{"internal/graph/bodystructure.go", "internal/sequence/sequence.go"} {
		parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, file), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range parsed.Decls {
			switch declaration := declaration.(type) {
			case *ast.GenDecl:
				for _, definition := range declaration.Specs {
					typ, ok := definition.(*ast.TypeSpec)
					if !ok || !typ.Name.IsExported() {
						continue
					}
					structure, ok := typ.Type.(*ast.StructType)
					if !ok {
						continue
					}
					for _, field := range structure.Fields.List {
						if len(field.Names) == 0 {
							t.Fatalf("unowned embedded field in %s", typ.Name.Name)
						}
						for _, name := range field.Names {
							declarations[typ.Name.Name+"."+name.Name] = ""
						}
					}
				}
			case *ast.FuncDecl:
				if declaration.Recv == nil {
					continue
				}
				receiver, ok := declaration.Recv.List[0].Type.(*ast.Ident)
				if ok && receiver.Name == "BodyFacts" && declaration.Name.IsExported() {
					declarations["BodyFacts."+declaration.Name.Name+"()"] = ""
				}
			}
		}
	}
	owners := bodyStructureFieldOwners()
	actual, expected := slices.Sorted(maps.Keys(declarations)), slices.Sorted(maps.Keys(owners))
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Fatalf("structure declarations lack exact fault ownership (-want +got):\n%s", diff)
	}
	modes := make(map[string]agreementMutation)
	for _, mode := range agreementMutations() {
		if _, duplicate := modes[mode.Name]; duplicate {
			t.Fatalf("duplicate mutation %s", mode.Name)
		}
		modes[mode.Name] = mode
	}
	for field, names := range owners {
		for _, name := range names {
			mode, found := modes[name]
			if !found || mode.Property != "F3" || mode.Needle == "" || mode.Fault == "" {
				t.Errorf("%s lacks registered value fault %s", field, name)
			}
		}
	}
}

func bodyStructureFieldOwners() map[string][]string {
	return map[string][]string{
		"BodyOutline.Heading":               {"f3-heading-range"},
		"BodyOutline.ListID":                {"f3-outline-list"},
		"BodyOutline.StrayID":               {"f3-outline-stray"},
		"BodyOutlineHeading.Span":           {"f3-heading-range"},
		"BodyOutlineHeading.Level":          {"f3-heading-level"},
		"BodyList.ID":                       {"f3-list-id"},
		"BodyList.ParentRowID":              {"f3-list-parent"},
		"BodyRow.ID":                        {"f3-row-id"},
		"BodyRow.ListID":                    {"f3-row-list"},
		"BodyRow.ParentRowID":               {"f3-parent"},
		"BodyRow.ChildListID":               {"f3-direct-child"},
		"BodyRow.Fallback":                  {"f3-row-fallback"},
		"BodyRow.HasFallback":               {"f3-row-presence"},
		"BodyRow.FirstBlockHasLines":        {"f3-row-first-child"},
		"BodyInlinePart.Span":               {"f3-inline-text-span", "f3-inline-code-span"},
		"BodyInlinePart.Code":               {"f3-inline-code-kind"},
		"BodyInlinePart.SoftLineBreak":      {"f3-soft-break"},
		"BodyInlinePart.HardLineBreak":      {"f3-hard-break"},
		"RichBodyHeading.ID":                {"f3-rich-id"},
		"RichBodyHeading.Level":             {"f3-rich-level"},
		"RichBodyHeading.Raw":               {"f3-rich-raw"},
		"RichBodyHeading.Span":              {"f3-rich-position"},
		"RichBodyHeading.Original":          {"f3-rich-original"},
		"BodyOrigin.Presentation":           {"f3-origin-position"},
		"BodyOrigin.Original":               {"f3-rich-origin"},
		"BodyOrigin.Synthetic":              {"f3-origin-synthetic"},
		"BodyFacts.Outline()":               {"f3-accessor-outline"},
		"BodyFacts.List()":                  {"f3-accessor-list"},
		"BodyFacts.Row()":                   {"f3-accessor-row"},
		"BodyFacts.ListRows()":              {"f3-accessor-list-rows"},
		"BodyFacts.RowBlocks()":             {"f3-accessor-row-blocks"},
		"BodyFacts.RowInlineParts()":        {"f3-accessor-row-parts"},
		"BodyFacts.StrayBlocks()":           {"f3-accessor-stray"},
		"BodyFacts.PairedEmphasisOpeners()": {"f3-accessor-paired"},
		"BodyFacts.RichHeadings()":          {"f3-accessor-rich"},
		"BodyFacts.RichHeadingOrigins()":    {"f3-accessor-rich-origin"},
		"Link.Target":                       {"f3-link-target"},
		"Link.Display":                      {"f3-link-display"},
		"Link.Aliased":                      {"f3-link-aliased"},
		"Link.Fragment":                     {"f3-link-fragment"},
		"Link.Span":                         {"f3-link-span"},
		"Candidate.Text":                    {"f3-candidate-text"},
		"Candidate.Target":                  {"f3-candidate-target"},
		"Candidate.Aliased":                 {"f3-candidate-aliased"},
		"Candidate.Fragment":                {"f3-candidate-fragment"},
		"Candidate.Line":                    {"f3-candidate-line"},
		"Candidate.Span":                    {"f3-row-span"},
		"Candidate.Gloss":                   {"f3-gloss-text", "f3-gloss-code"},
		"Candidate.TargetSpan":              {"f3-target-span"},
		"Candidate.State":                   {"f3-candidate-state"},
		"Group.Name":                        {"f3-group-heading-name", "f3-container-name"},
		"Group.Level":                       {"f3-group-heading-level"},
		"Group.Line":                        {"f3-group-heading-line", "f3-container-line"},
		"Group.Role":                        {"f3-group-heading-role", "f3-container-role"},
		"Group.Container":                   {"f3-container-container"},
		"Group.AnchorTarget":                {"f3-container-anchortarget"},
		"Group.AnchorSpan":                  {"f3-container-anchorspan"},
		"Group.Invalid":                     {"f3-container-invalid"},
		"Group.Items":                       {"f3-entry-item", "f3-branch-item"},
		"Item.Entry":                        {"f3-entry-item"},
		"Item.Branch":                       {"f3-branch-item"},
		"Diagnostic.Rule":                   {"f3-diagnostic-rule"},
		"Diagnostic.Line":                   {"f3-diagnostic-line"},
		"Diagnostic.Message":                {"f3-diagnostic-message"},
		"Diagnostic.Evidence":               {"f3-diagnostic-evidence"},
		"Document.Groups":                   {"f3-document-groups"},
		"Document.Diagnostics":              {"f3-document-diagnostics"},
	}
}

// The native declarations and driver must own the same complete set of values.
func TestBodyValueControlCensus(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "../sequence/bodyvaluecontrol_test.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	actual := make(map[string]string)
	matches := 0
	for _, declaration := range file.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, definition := range group.Specs {
			value, ok := definition.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 || value.Names[0].Name != "bodyValueCases" {
				continue
			}
			matches++
			if len(value.Values) != 1 {
				t.Fatal("native body controls need one declaration")
			}
			rows, ok := value.Values[0].(*ast.CompositeLit)
			if !ok {
				t.Fatal("native body controls need literal rows")
			}
			for _, entry := range rows.Elts {
				row, ok := entry.(*ast.CompositeLit)
				if !ok {
					t.Fatal("native body control is not a literal row")
				}
				values := make(map[string]string)
				for _, entry := range row.Elts {
					field, ok := entry.(*ast.KeyValueExpr)
					if !ok {
						t.Fatal("native body control lacks named fields")
					}
					name, ok := field.Key.(*ast.Ident)
					if !ok {
						t.Fatal("native body control field is not a name")
					}
					if name.Name == "Run" {
						continue
					}
					literal, ok := field.Value.(*ast.BasicLit)
					if !ok || literal.Kind != token.STRING {
						t.Fatal("native body control metadata is not literal")
					}
					text, err := strconv.Unquote(literal.Value)
					if err != nil {
						t.Fatal(err)
					}
					values[name.Name] = text
				}
				name, identity := values["Name"], values["Identity"]
				if name == "" || identity == "" || actual[name] != "" {
					t.Fatalf("native body control has empty or duplicate ownership: %+v", values)
				}
				actual[name] = identity
			}
		}
	}
	if matches != 1 {
		t.Fatalf("native body control declaration matches=%d", matches)
	}
	want := make(map[string]string)
	for _, mode := range agreementMutations() {
		if mode.ModeFlag != "body-mode" {
			continue
		}
		if mode.Package != "./internal/sequence" || mode.ControlTest != "TestBodyValueMutationControl" || mode.Property != "F3" || want[mode.Name] != "" {
			t.Fatalf("native body mode lacks exact ownership: %+v", mode)
		}
		want[mode.Name] = mode.Identity
	}
	if diff := cmp.Diff(want, actual); diff != "" {
		t.Fatalf("native body controls differ from the complete registry (-want +got):\n%s", diff)
	}
}
