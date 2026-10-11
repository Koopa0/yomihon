package judge_test

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
)

type agreementDiagnosticP1Declaration struct {
	Family, Name, Body, Private, Target, SelectedDiagnostic string
	Active                                                  int
	Page                                                    agreementHTML
	Targets, PrivateTargets                                 []string
	Before, After, BeforeFull, AfterFull                    map[string]int
}

func agreementDiagnosticP1Pools(signed map[string]int) map[string]int {
	out := map[string]int{}
	for key, n := range signed {
		if strings.HasPrefix(key, "P1/") {
			out[key] = n
		}
	}
	return out
}
func agreementDiagnosticP1Signature(target string, n int) string {
	f := agreementFailure{Property: "P1", Identity: "citation-occurrences", Tuple: agreementCitation{Target: target}, Direction: "judge-only", Multiplicity: n}
	return agreementSignature(&f)
}
func agreementDiagnosticP1Declarations() []agreementDiagnosticP1Declaration {
	var out []agreementDiagnosticP1Declaration
	{
		ws := agreementCanonicalDiagnosticSources()
		for wi := range ws {
			w := &ws[wi]
			for vi := range w.Variants {
				v := &w.Variants[vi]
				selected := ""
				rs := agreementCanonicalDiagnosticWitnesses()
				for ri := range rs {
					r := &rs[ri]
					if r.Body == w.Body && r.Target == v.Target {
						selected = r.Selected
					}
				}
				out = append(out, agreementDiagnosticP1Declaration{Family: "canonical", Name: w.Name, Body: w.Body, Private: v.Body, Target: v.Target, SelectedDiagnostic: selected, Active: agreementCanonicalDiagnosticActive(v.Owners, &w.Check), Page: w.Page, Targets: w.Targets, PrivateTargets: v.Targets, Before: agreementDiagnosticP1Pools(w.Signatures), After: agreementDiagnosticP1Pools(v.Signatures), BeforeFull: w.Signatures, AfterFull: v.Signatures})
			}
		}
	}
	{
		ws := agreementRemovedAliasDiagnosticSources()
		for wi := range ws {
			w := &ws[wi]
			for vi := range w.Variants {
				v := &w.Variants[vi]
				selected := ""
				rs := agreementRemovedAliasDiagnosticWitnesses()
				for ri := range rs {
					r := &rs[ri]
					if r.Body == w.Body && r.Target == v.Target {
						selected = r.Selected
					}
				}
				out = append(out, agreementDiagnosticP1Declaration{Family: "alias", Name: w.Name, Body: w.Body, Private: v.Body, Target: v.Target, SelectedDiagnostic: selected, Active: len(v.Owners), Page: w.Page, Targets: w.Targets, PrivateTargets: v.Targets, Before: agreementDiagnosticP1Pools(w.Signatures), After: agreementDiagnosticP1Pools(v.Signatures), BeforeFull: w.Signatures, AfterFull: v.Signatures})
			}
		}
	}
	{
		ws := agreementInfoCombinedDiagnosticSources()
		for wi := range ws {
			w := &ws[wi]
			for vi := range w.Variants {
				v := &w.Variants[vi]
				selected := ""
				rs := agreementInfoCombinedDiagnosticWitnesses()
				for ri := range rs {
					r := &rs[ri]
					if r.Body == w.Body && r.Target == v.Target {
						selected = r.Selected
					}
				}
				out = append(out, agreementDiagnosticP1Declaration{Family: "info", Name: w.Name, Body: w.Body, Private: v.Body, Target: v.Target, SelectedDiagnostic: selected, Active: agreementInfoCombinedDiagnosticActive(v.Owners, &w.Check), Page: w.Page, Targets: w.Targets, PrivateTargets: v.Targets, Before: agreementDiagnosticP1Pools(w.Signatures), After: agreementDiagnosticP1Pools(v.Signatures), BeforeFull: w.Signatures, AfterFull: v.Signatures})
			}
		}
	}
	{
		ws := agreementInactiveParagraphDiagnosticSources()
		for wi := range ws {
			w := &ws[wi]
			for vi := range w.Variants {
				v := &w.Variants[vi]
				selected := ""
				rs := agreementInactiveParagraphDiagnosticWitnesses()
				for ri := range rs {
					r := &rs[ri]
					if r.Body == w.Body && r.Target == v.Target {
						selected = r.Selected
					}
				}
				out = append(out, agreementDiagnosticP1Declaration{Family: "inactive", Name: w.Name, Body: w.Body, Private: v.Body, Target: v.Target, SelectedDiagnostic: selected, Active: agreementInactiveParagraphDiagnosticActive(v.Owners, &w.Check), Page: w.Page, Targets: w.Targets, PrivateTargets: v.Targets, Before: agreementDiagnosticP1Pools(w.Signatures), After: agreementDiagnosticP1Pools(v.Signatures), BeforeFull: w.Signatures, AfterFull: v.Signatures})
			}
		}
	}
	{
		ws := agreementConsumedWordDiagnosticSources()
		for wi := range ws {
			w := &ws[wi]
			for vi := range w.Variants {
				v := &w.Variants[vi]
				selected := ""
				rs := agreementConsumedWordDiagnosticWitnesses()
				for ri := range rs {
					r := &rs[ri]
					if r.Body == w.Body && r.Target == v.Target {
						selected = r.Selected
					}
				}
				out = append(out, agreementDiagnosticP1Declaration{Family: "word", Name: w.Name, Body: w.Body, Private: v.Body, Target: v.Target, SelectedDiagnostic: selected, Active: agreementConsumedWordDiagnosticActive(v.Owners, &w.Check), Page: w.Page, Targets: w.Targets, PrivateTargets: v.Targets, Before: agreementDiagnosticP1Pools(w.Signatures), After: agreementDiagnosticP1Pools(v.Signatures), BeforeFull: w.Signatures, AfterFull: v.Signatures})
			}
		}
	}
	{
		ws := agreementCutDiagnosticSources()
		for wi := range ws {
			w := &ws[wi]
			for vi := range w.Variants {
				v := &w.Variants[vi]
				selected := ""
				rs := agreementCutDiagnosticWitnesses()
				for ri := range rs {
					r := &rs[ri]
					if r.Body == w.Body && r.Target == v.Target {
						selected = r.Selected
					}
				}
				out = append(out, agreementDiagnosticP1Declaration{Family: "cut", Name: w.Name, Body: w.Body, Private: v.Body, Target: v.Target, SelectedDiagnostic: selected, Active: agreementCutDiagnosticActive(v.Owners, &w.Check), Page: w.Page, Targets: w.Targets, PrivateTargets: v.Targets, Before: agreementDiagnosticP1Pools(w.Signatures), After: agreementDiagnosticP1Pools(v.Signatures), BeforeFull: w.Signatures, AfterFull: v.Signatures})
			}
		}
	}
	return out
}
func agreementDiagnosticP1Eligible(r *agreementDiagnosticP1Declaration) bool {
	if r.Active < 1 {
		return false
	}
	target := r.Target
	replacement := strings.Repeat("q", len(target))
	counts := map[string]int{}
	visible := map[string]int{}
	for _, name := range r.Targets {
		counts[name]++
	}
	for _, c := range r.Page.Citations {
		if c.SourceRole == "" && c.Target != "" {
			visible[c.Target]++
		}
	}
	if counts[target]-visible[target] != r.Active || counts[replacement] != 0 || visible[replacement] != 0 {
		return false
	}
	selected := agreementDiagnosticP1Signature(target, r.Active)
	if r.Before[selected] != 1 {
		return false
	}
	want := maps.Clone(r.Before)
	delete(want, selected)
	want[agreementDiagnosticP1Signature(replacement, r.Active)]++
	return maps.Equal(want, r.After)
}
func agreementDiagnosticP1Validate(c agreementCase, f *agreementFailure, raw string, page *agreementHTML, diagnostics []render.Diagnostic, failures []agreementFailure, family string) (kind, authority, wrong string) {
	switch family {
	case "canonical":
		return agreementCanonicalDiagnosticDifference(c, f, raw, page, diagnostics, failures)
	case "alias":
		return agreementRemovedAliasDiagnosticDifference(c, f, raw, page, diagnostics, failures)
	case "info":
		return agreementInfoCombinedDiagnosticDifference(c, f, raw, page, diagnostics, failures)
	case "inactive":
		return agreementInactiveParagraphDiagnosticDifference(c, f, raw, page, diagnostics, failures)
	case "word":
		return agreementConsumedWordDiagnosticDifference(c, f, raw, page, diagnostics, failures)
	case "cut":
		return agreementCutDiagnosticDifference(c, f, raw, page, diagnostics, failures)
	}
	return "", "", ""
}
func agreementDiagnosticP1Difference(c agreementCase, f *agreementFailure, raw string, page *agreementHTML, diagnostics []render.Diagnostic, failures []agreementFailure) (kind, authority, wrong string) {
	if f == nil || page == nil || f.Property != "P1" || f.Identity != "citation-occurrences" || f.Direction != "judge-only" || f.Multiplicity < 1 || f.Tuple.SourceRole != "" || f.Tuple.Section != "" || f.Tuple.State != "" || f.Fragment != "" || f.Cut != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound {
		return "", "", ""
	}
	rs := agreementDiagnosticP1Declarations()
	for ri := range rs {
		r := &rs[ri]
		if r.Body != c.Body || r.Target != f.Tuple.Target || !agreementDiagnosticP1Eligible(r) || agreementSignature(f) != agreementDiagnosticP1Signature(r.Target, r.Active) {
			continue
		}
		var diagnostic *agreementFailure
		for i := range failures {
			if agreementSignature(&failures[i]) == r.SelectedDiagnostic {
				if diagnostic != nil {
					return "", "", ""
				}
				diagnostic = &failures[i]
			}
		}
		if diagnostic == nil {
			return "", "", ""
		}
		k, _, w := agreementDiagnosticP1Validate(c, diagnostic, raw, page, diagnostics, failures, r.Family)
		if k != "debt" || w != "page-diagnostic" {
			return "", "", ""
		}
		authority = "#1011 stage 5"
		if r.Family == "info" {
			authority = "#1011 stage 4"
		}
		return "debt", authority, "judge"
	}
	return "", "", ""
}

type agreementDiagnosticP1Receipt struct {
	Family, Name, Target, BodySHA, Selected string
	Active                                  int
	Eligible                                bool
}

func agreementDiagnosticP1Receipts() []agreementDiagnosticP1Receipt {
	return []agreementDiagnosticP1Receipt{
		{Family: "canonical", Name: "5f0d30a203b4", Target: "A", BodySHA: "5f0d30a203b44272bcc324c0b6f5a76c4890a4e6b3b89d2752c022e8bd965bdf", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=3 cut=\"\" page=false judge=false excerpt=false", Active: 3, Eligible: true},
		{Family: "canonical", Name: "45125be6862b", Target: "A", BodySHA: "45125be6862b7415fe07a54bf52d3210e3afdb7e956cc26ce41b253480cdbeea", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "canonical", Name: "3c485a377ef0", Target: "A", BodySHA: "3c485a377ef052895511726325b0c63d0182583a8947af7e699ff96c0a4fb4ca", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: false},
		{Family: "canonical", Name: "8deb2f36428c", Target: "A", BodySHA: "8deb2f36428cf2349208fcd677d88e58cc2837f6c63cdc4397f298c8d0c8d025", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: false},
		{Family: "canonical", Name: "bb783f4f8ef7", Target: "A", BodySHA: "bb783f4f8ef7f98c7fbfbddd39c8d65587f7452c18542a527f99e2f78a05d6fc", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "canonical", Name: "3486c1969915", Target: "A", BodySHA: "3486c196991505c9720d227ebafb5cb682593b1c02160a40247b2aaf2b173444", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: false},
		{Family: "canonical", Name: "1206e9dd8d0e", Target: "A", BodySHA: "1206e9dd8d0e225c681f804dae2ac46957255a55eb877913566b944871fd39c7", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "canonical", Name: "db20059fac3f", Target: "A", BodySHA: "db20059fac3f88fba046928efaa658d59691474f1c60e069683ca722de9b22ed", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "canonical", Name: "0af16098b14c", Target: "A", BodySHA: "0af16098b14ce384202e33e25f26729103f22eafff1b4c6d9cb1dc4bdc9e6317", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: false},
		{Family: "alias", Name: "6815c332b79f", Target: "B", BodySHA: "6815c332b79f3fd720c0db2429578589211619cdcda805a21b9a50a25fbdd2e0", Selected: "P1/citation-occurrences tuple={SourceRole: Target:B Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: true},
		{Family: "alias", Name: "1dc283f2f9a6", Target: "B", BodySHA: "1dc283f2f9a6a138586fab6a41a7318a2fe472ffafeba1bfdf7341a3b48fcc1b", Selected: "P1/citation-occurrences tuple={SourceRole: Target:B Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: true},
		{Family: "info", Name: "30216ff168eb", Target: "A", BodySHA: "30216ff168eb09151dfe546c56ee91aedac5f8a8bb48975c92a95e93ebef3ad1", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "info", Name: "226feda5f982", Target: "A", BodySHA: "226feda5f982c0064e882714c16dfb2d3587b932c12cb1d85c066d2da25df3f3", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "info", Name: "a81cbb395cb6", Target: "A", BodySHA: "a81cbb395cb638c5f46bfd7160296128a887779082a9f8be334127153d4637d0", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: false},
		{Family: "info", Name: "af89a08567c6", Target: "A", BodySHA: "af89a08567c6b3b4dbe9baf9039a0c00997c213cf892bf61b0be309f8bc1133b", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "info", Name: "370a14bc927e", Target: "A", BodySHA: "370a14bc927ee88819503f967ddda41b81c5aad9da2504620a22bbeee8891788", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: false},
		{Family: "info", Name: "d2082db610d8", Target: "A", BodySHA: "d2082db610d8e49ff7157383545507af6280b06b6acfa913e85d513e8a50bb28", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "info", Name: "1d3c318e6098", Target: "A", BodySHA: "1d3c318e60986c4ba9b356e10a3fa7afd9ad2f242a411347dd492b4c02e99c1c", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "info", Name: "9a8fcf2ab488", Target: "A", BodySHA: "9a8fcf2ab488f639c2d828174c39aab306952618dd3d4cb7d0cfa3695a4cda07", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: false},
		{Family: "info", Name: "621f29fa59a4", Target: "A", BodySHA: "621f29fa59a418b463c90e12cf41cc5d42e5501d853ffe5ada6ecef6f2ccc79d", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "info", Name: "bf1e28282bb7", Target: "A", BodySHA: "bf1e28282bb7d25de38eaf64df83bccaf0d4c2b5101acc8376661d0a71aa1502", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "info", Name: "49f2ff4db004", Target: "A", BodySHA: "49f2ff4db0049815939c2d8747fcfd38a0da901c8e4d31842bdec6298550e83f", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "info", Name: "49a0e48ecbb4", Target: "A", BodySHA: "49a0e48ecbb4afc5de9dfc9da20d93d743df32ad0549f6dbe423fea35d41f0a6", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: true},
		{Family: "info", Name: "af811d6093ff", Target: "A", BodySHA: "af811d6093ffd0bee0e1e92ef4956b6239bc44624d4c446dd536872a3816617c", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "info", Name: "54e7dd1b9099", Target: "A", BodySHA: "54e7dd1b90994f2a476e866db2839851f890a9719538fbac4d85341f1b8f979b", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "info", Name: "c83104105a89", Target: "A", BodySHA: "c83104105a89d04a6bf7985510a2e9c7e53770541cf97fd65df748f533e2c8ac", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "info", Name: "d60a732d33be", Target: "A", BodySHA: "d60a732d33bea167987a93fb01b779d1d3a95d66a0356cb6619234f4e989aaa1", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "info", Name: "77dcc34ef5c9", Target: "A", BodySHA: "77dcc34ef5c93d1baf638991430eb1c268b95c088ad19646293a18e9c2b1c6e9", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: false},
		{Family: "info", Name: "0c61b4fbb652", Target: "A", BodySHA: "0c61b4fbb652e23cab38d6f58fc3e1c8791cfdb0ed5e2e5fce9b2653978ff9fb", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: true},
		{Family: "inactive", Name: "669130014e22", Target: "A", BodySHA: "669130014e2286d9b7f45f49072a51abcb71ebb4292161e8b742624a6ea094d0", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: false},
		{Family: "inactive", Name: "cd696dcf7fac", Target: "A", BodySHA: "cd696dcf7fac4ffe28336402832eed67dcfe2ea8caf7fb23ca0e67a3dafc3a41", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: false},
		{Family: "inactive", Name: "cd696dcf7fac", Target: "B", BodySHA: "cd696dcf7fac4ffe28336402832eed67dcfe2ea8caf7fb23ca0e67a3dafc3a41", Selected: "P1/citation-occurrences tuple={SourceRole: Target:B Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: true},
		{Family: "inactive", Name: "32310ac531c2", Target: "A", BodySHA: "32310ac531c24b69b90f2f62804fe34656d20c2747415631ff510ee13451a11c", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: true},
		{Family: "inactive", Name: "6c93770d1e96", Target: "A", BodySHA: "6c93770d1e963b5e0af528a269f9a57ccbe103fd3deeee9b096246588ba44155", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "inactive", Name: "a51af9873eea", Target: "A", BodySHA: "a51af9873eea51c4e157f9e6fa9c918c8ca422d2dfcee03ba4b2e56f5f277fe7", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: true},
		{Family: "inactive", Name: "a51af9873eea", Target: "B", BodySHA: "a51af9873eea51c4e157f9e6fa9c918c8ca422d2dfcee03ba4b2e56f5f277fe7", Selected: "P1/citation-occurrences tuple={SourceRole: Target:B Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: true},
		{Family: "inactive", Name: "e10bf50b61e9", Target: "A", BodySHA: "e10bf50b61e91026076b07e947a2be5e9746f4b125856d2d0a63ab75cbb7b793", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: true},
		{Family: "inactive", Name: "747f3eae8f97", Target: "A", BodySHA: "747f3eae8f975ed894f6879a556ffc4e83c75ebeb9b679adcb9b1563a46a1087", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: false},
		{Family: "word", Name: "17460cebbfd2", Target: "A", BodySHA: "17460cebbfd25dc9c106720ceda17d7df2d0bfcec9daf78723e63954d4d3c927", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: false},
		{Family: "word", Name: "17460cebbfd2", Target: "B", BodySHA: "17460cebbfd25dc9c106720ceda17d7df2d0bfcec9daf78723e63954d4d3c927", Selected: "P1/citation-occurrences tuple={SourceRole: Target:B Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "word", Name: "03d5a52b02d1", Target: "A", BodySHA: "03d5a52b02d101f6c7dd9e52f2828474cda04dc36deb9b09e42cb4f48832c464", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: true},
		{Family: "word", Name: "52426dc6ac6b", Target: "A", BodySHA: "52426dc6ac6bcea26e1886c05c1bd7202a0e80005710d499bc3cd1ab704e2600", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: false},
		{Family: "word", Name: "52426dc6ac6b", Target: "B", BodySHA: "52426dc6ac6bcea26e1886c05c1bd7202a0e80005710d499bc3cd1ab704e2600", Selected: "P1/citation-occurrences tuple={SourceRole: Target:B Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: true},
		{Family: "word", Name: "ebb51b3c45a9", Target: "A", BodySHA: "ebb51b3c45a92634fc977bbb35c76e2eabb9e65641697ab61a957aa89b39e1c8", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: true},
		{Family: "word", Name: "34b52925a8e4", Target: "A", BodySHA: "34b52925a8e476ac6d2013ddd6e70ac323dc9ce56ec699be08073f6b56e43869", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: true},
		{Family: "cut", Name: "e64b6bec1fdf", Target: "A", BodySHA: "e64b6bec1fdfde3c90cbb838d96f96b74fae50ef4cfdf57a1854161ff1441eb9", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=3 cut=\"\" page=false judge=false excerpt=false", Active: 3, Eligible: true},
		{Family: "cut", Name: "c41d987b2fa8", Target: "A", BodySHA: "c41d987b2fa852495b83518d37a80537c37660bf32bd557e23d4ba0a9231ca74", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=3 cut=\"\" page=false judge=false excerpt=false", Active: 3, Eligible: false},
		{Family: "cut", Name: "c41d987b2fa8", Target: "image.png", BodySHA: "c41d987b2fa852495b83518d37a80537c37660bf32bd557e23d4ba0a9231ca74", Selected: "P1/citation-occurrences tuple={SourceRole: Target:image.png Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: true},
		{Family: "cut", Name: "a088e1a52503", Target: "A", BodySHA: "a088e1a52503945d22f08be390f7ba61e29d6278a11766e62af9071eceaa30dc", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=3 cut=\"\" page=false judge=false excerpt=false", Active: 3, Eligible: true},
		{Family: "cut", Name: "be9a8d76f524", Target: "A", BodySHA: "be9a8d76f5249da582e32d8453b0162e145578b1cb7c72cc44a10febad1c2bbb", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: false},
		{Family: "cut", Name: "1bfcb354be0b", Target: "A", BodySHA: "1bfcb354be0b5896eff890d562d76c19a9685108c4c106450afe818dda4e620b", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=6 cut=\"\" page=false judge=false excerpt=false", Active: 6, Eligible: true},
		{Family: "cut", Name: "5c092cbbf8ea", Target: "A", BodySHA: "5c092cbbf8ea25e5e03e1125e2fb576c658f6b5abbe8ba8ff0f81af6e4602ae3", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: false},
		{Family: "cut", Name: "f384368092b4", Target: "A", BodySHA: "f384368092b40e9ece6ae72f8dee184eadc62ec456dc2173733778482a55b163", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Active: 1, Eligible: true},
		{Family: "cut", Name: "79c70437dcef", Target: "A", BodySHA: "79c70437dcef877c6076006b3aef376c8ebca4345541f4828d507d78e358d9f3", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=4 cut=\"\" page=false judge=false excerpt=false", Active: 4, Eligible: true},
		{Family: "cut", Name: "390229c2ec81", Target: "A", BodySHA: "390229c2ec81642cbef2dfa2d595957d6d0791808597d455c8001c32a4cbf0e4", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Active: 2, Eligible: false},
		{Family: "cut", Name: "224a7ffd511d", Target: "A", BodySHA: "224a7ffd511d901df94d240835a722ccbcafc1e0001bcdbfd896e4a23adc1d18", Selected: "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=4 cut=\"\" page=false judge=false excerpt=false", Active: 4, Eligible: false},
	}
}
func agreementDiagnosticP1Observe(t *testing.T, body string) (render.Result, agreementHTML, []agreementFailure) {
	t.Helper()
	raw, page := agreementIsolatedPage(t, agreementCase{Body: body})
	fs := agreementPageFailures(body, &raw, &page)
	fs = append(fs, agreementFragmentFailures(t, []agreementCase{{Body: body}}, []agreementHTML{page})[0]...)
	return raw, page, fs
}
func TestAgreementDiagnosticP1OwnedWholeSet(t *testing.T) {
	t.Parallel()
	rs := agreementDiagnosticP1Declarations()
	want := agreementDiagnosticP1Receipts()
	if len(rs) != len(want) {
		t.Fatal("caught: complete diagnostic P1 declaration set")
	}
	eligible := 0
	for i := range rs {
		r := &rs[i]
		w := &want[i]
		sha := fmt.Sprintf("%x", sha256.Sum256([]byte(r.Body)))
		if r.Family != w.Family || r.Name != w.Name || r.Target != w.Target || sha != w.BodySHA || r.Active != w.Active || agreementDiagnosticP1Signature(r.Target, r.Active) != w.Selected || agreementDiagnosticP1Eligible(r) != w.Eligible {
			t.Fatal("caught: complete diagnostic P1 source and eligibility census")
		}
		if w.Eligible {
			eligible++
		}
	}
	if eligible != 20 {
		t.Fatal("caught: complete eligible diagnostic P1 source set")
	}
}
func TestAgreementDiagnosticP1OwnedPublic(t *testing.T) {
	t.Parallel()
	rs := agreementDiagnosticP1Declarations()
	for ri := range rs {
		r := &rs[ri]
		if !agreementDiagnosticP1Receipts()[ri].Eligible {
			continue
		}
		t.Run(r.Family+"/"+r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			raw, page, fs := agreementDiagnosticP1Observe(t, r.Body)
			pr, pp, pfs := agreementDiagnosticP1Observe(t, r.Private)
			f := agreementFailure{Property: "P1", Identity: "citation-occurrences", Tuple: agreementCitation{Target: r.Target}, Direction: "judge-only", Multiplicity: r.Active}
			k, a, w := agreementDiagnosticP1Difference(agreementCase{Body: r.Body}, &f, raw.HTML, &page, raw.Diagnostics, fs)
			authority := "#1011 stage 5"
			if r.Family == "info" {
				authority = "#1011 stage 4"
			}
			if k != "debt" || a != authority || w != "judge" {
				t.Fatal("caught: complete diagnostic P1 owned source")
			}
			if !maps.Equal(agreementMixedCurrent(fs), r.BeforeFull) || !maps.Equal(agreementMixedCurrent(pfs), r.AfterFull) || !maps.Equal(agreementDiagnosticP1Pools(agreementMixedCurrent(fs)), r.Before) || !maps.Equal(agreementDiagnosticP1Pools(agreementMixedCurrent(pfs)), r.After) || !cmp.Equal(page, r.Page) || !cmp.Equal(page, pp) || raw.HTML != pr.HTML || !cmp.Equal(judge.LinkTargets(r.Private), r.PrivateTargets) {
				t.Fatal("caught: complete original and private diagnostic P1 public ledgers")
			}
		})
	}
}
func TestAgreementDiagnosticP1OwnedRefusals(t *testing.T) {
	t.Parallel()
	rs := agreementDiagnosticP1Declarations()
	for ri := range rs {
		r := &rs[ri]
		if !agreementDiagnosticP1Receipts()[ri].Eligible {
			continue
		}
		t.Run(r.Family+"/"+r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			raw, page, fs := agreementDiagnosticP1Observe(t, r.Body)
			base := agreementFailure{Property: "P1", Identity: "citation-occurrences", Tuple: agreementCitation{Target: r.Target}, Direction: "judge-only", Multiplicity: r.Active}
			changed := []agreementFailure{base, base, base, base, base, base, base, base, base, base, base, base}
			changed[0].Multiplicity++
			changed[1].Direction = "page-only"
			changed[2].Property = "P0"
			changed[3].Identity = "diagnostic-html"
			changed[4].Tuple.SourceRole = "embed"
			changed[5].Tuple.Section = "A"
			changed[6].Tuple.State = "wikilink-broken"
			changed[7].Fragment = "A"
			changed[8].Cut = "x"
			changed[9].PagePresent = true
			changed[10].JudgeAccepted = true
			changed[11].ExcerptFound = true
			for i := range changed {
				k, _, _ := agreementDiagnosticP1Difference(agreementCase{Body: r.Body}, &changed[i], raw.HTML, &page, raw.Diagnostics, fs)
				if k != "" {
					t.Fatal("caught: altered diagnostic P1 selected receipt refuses")
				}
			}
			contexts := []agreementCase{{Body: r.Body, Title: "A"}, {Body: r.Body, Companions: map[string]string{"A": "x"}}, {Body: r.Body + " "}}
			for _, c := range contexts {
				k, _, _ := agreementDiagnosticP1Difference(c, &base, raw.HTML, &page, raw.Diagnostics, fs)
				if k != "" {
					t.Fatal("caught: changed diagnostic P1 source context refuses")
				}
			}
			badPage := page
			badPage.Citations = append(append([]agreementCitation(nil), page.Citations...), agreementCitation{Target: "counterfeit"})
			k, _, _ := agreementDiagnosticP1Difference(agreementCase{Body: r.Body}, &base, raw.HTML, &badPage, raw.Diagnostics, fs)
			if k != "" {
				t.Fatal("caught: changed diagnostic P1 page ledger refuses")
			}
			k, _, _ = agreementDiagnosticP1Difference(agreementCase{Body: r.Body}, &base, raw.HTML+"<pre><code>counterfeit</code></pre>", &page, raw.Diagnostics, fs)
			if k != "" {
				t.Fatal("caught: changed diagnostic P1 raw HTML refuses")
			}
			k, _, _ = agreementDiagnosticP1Difference(agreementCase{Body: r.Body}, &base, raw.HTML, &page, nil, fs)
			if k != "" {
				t.Fatal("caught: changed diagnostic P1 diagnostic ledger refuses")
			}
			empty := append([]agreementFailure(nil), fs...)
			for i := range empty {
				if agreementSignature(&empty[i]) == r.SelectedDiagnostic {
					empty = append(empty[:i], empty[i+1:]...)
					break
				}
			}
			k, _, _ = agreementDiagnosticP1Difference(agreementCase{Body: r.Body}, &base, raw.HTML, &page, raw.Diagnostics, empty)
			if k != "" {
				t.Fatal("caught: missing complete diagnostic P1 source pool refuses")
			}
		})
	}
}

func TestAgreementDiagnosticP1OwnedSignedRefusals(t *testing.T) {
	t.Parallel()
	rs := agreementDiagnosticP1Declarations()
	for ri := range rs {
		r := &rs[ri]
		if !agreementDiagnosticP1Receipts()[ri].Eligible {
			continue
		}
		t.Run(r.Family+"/"+r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			replacement := strings.Repeat("q", len(r.Target))
			variants := []agreementDiagnosticP1Declaration{*r, *r, *r, *r, *r, *r, *r}
			variants[0].Active = 0
			variants[1].Active++
			variants[2].Targets = append(append([]string(nil), r.Targets...), replacement)
			variants[3].Before = maps.Clone(r.Before)
			delete(variants[3].Before, agreementDiagnosticP1Signature(r.Target, r.Active))
			variants[4].After = maps.Clone(r.After)
			delete(variants[4].After, agreementDiagnosticP1Signature(replacement, r.Active))
			variants[4].After[agreementDiagnosticP1Signature(replacement, r.Active+1)]++
			variants[5].After = maps.Clone(r.After)
			variants[5].After[agreementDiagnosticP1Signature("counterfeit", 1)]++
			variants[6].Page = r.Page
			variants[6].Page.Citations = append(append([]agreementCitation(nil), r.Page.Citations...), agreementCitation{Target: r.Target})
			for i := range variants {
				if agreementDiagnosticP1Eligible(&variants[i]) {
					t.Fatal("caught: complete diagnostic P1 producer and signed transfer conservation")
				}
			}
		})
	}
}
