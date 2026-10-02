package layouts

import (
	"fmt"

	"github.com/a-h/templ"
)

// speculationReads is the condition a link must meet before the browser may
// fetch its page ahead of a click. It is an allowlist of reading addresses,
// so a route added later is left alone until it is named here.
//
// Every address named is a plain read, and the two exclusions cover what the
// allowlist's own patterns would otherwise reach:
//
//   - a report's raw body sits under the same prefix as the report page and is
//     served as a file rather than read as a page;
//   - a note address carrying "from" is where the redirect after a status
//     change lands, and reading it spends the one-time receipt the landing page
//     states. Fetching it ahead of the reader would leave the arrival with
//     nothing to say. No link on any page carries it, so this closes a door
//     that is shut rather than one that is open.
//
// Left out on purpose: the raw, compare, freshness and preview endpoints,
// search, read-aloud, the preferences page, thought composition, and the
// health view, which reads every note in the vault to answer. The desk and
// the open-thoughts shelf read live status and are left until a measurement
// says the wait is worth the work.
const speculationReads = `{"or":[` +
	`{"href_matches":"/notes/*"},` +
	`{"href_matches":"/syllabus/*"},` +
	`{"href_matches":"/folders"},` +
	`{"href_matches":"/folders/*"},` +
	`{"href_matches":"/reports"},` +
	`{"href_matches":"/reports/*"},` +
	`{"href_matches":"/paths"},` +
	`{"href_matches":"/maps"},` +
	`{"href_matches":"/journal"}` +
	`]},` +
	`{"not":{"href_matches":"/reports/*/raw"}},` +
	`{"not":{"href_matches":{"pathname":"/notes/*","search":"*from=*"}}}`

// speculationSteps picks out the previous and next links at the foot of a
// note and of a journal month, which are the two places a page names where
// the reader is most likely to go next.
const speculationSteps = `{"selector_matches":"a[rel~=prev], a[rel~=next]"}`

// speculationRules is the JSON a speculationrules script carries. It asks for
// prefetch and never prerender: a prerendered page runs its scripts before the
// reader has gone there, which would start the freshness polling of a page
// nobody is on and leave its staleness checks to guess when the reader
// arrived.
//
// Two rules share one condition. Hovering a link for a moment fetches its page
// ("moderate"), and the previous and next links are fetched as soon as the
// page loads ("immediate"), because a reader who has reached the foot of a
// note is about to press one of them. Both rules carry the whole condition, so
// the one that fetches on load cannot reach past the allowlist.
var speculationRules = fmt.Sprintf(
	`{"prefetch":[{"where":{"and":[%[1]s]},"eagerness":"moderate"},`+
		`{"where":{"and":[%[1]s,%[2]s]},"eagerness":"immediate"}]}`,
	speculationReads, speculationSteps)

// speculationScript is the element that carries speculationRules. It is
// written here rather than in the template because the template writes the
// body of a script element as literal text, with no way to put a value in it.
// The nonce is what lets a page that forbids inline script accept it.
func speculationScript(nonce string) templ.Component {
	return templ.Raw(`<script type="speculationrules" nonce="` + templ.EscapeString(nonce) + `">` + speculationRules + `</script>`)
}
