package figure

import (
	"regexp"
	"strings"
)

// Report is what a person approving a figure is shown before they decide. It
// is a reading aid and not a boundary: it finds words in the text, and script
// can spell anything without writing the word. What stops a figure is the
// frame's content policy and the refusals Inspect makes, never this.
type Report struct {
	// Size is the number of bytes approved.
	Size int
	// Scripts is how many inline scripts the figure carries.
	Scripts int
	// Addresses are the address-like strings in the text, each once, in the
	// order they first appear.
	Addresses []string
	// Names are the browser facilities the text names that reach outside a
	// page or run text as script, each once, in the order of the list below.
	Names []string
}

// addressPattern finds what reads as an address: a scheme the browser can
// reach something with, or a scheme-relative //host. It is deliberately loose;
// a false match is a line a person reads, and a missed one is what this report
// exists to show.
var addressPattern = regexp.MustCompile(`(?i)(?:\b(?:https?|wss?|data|blob|file|ftp|stun|stuns|turn|turns):[^\s"'<>()\x60\\]*|(?:^|[\s"'(=\x60])//[a-z0-9][a-z0-9.-]*[a-z0-9](?::\d+)?)`)

// reachingNames are the facilities a figure's script could use to reach
// outside its frame, navigate, or turn text into code. Each entry is a
// substring searched for as written.
var reachingNames = []string{
	"RTCPeerConnection", "RTCDataChannel", "fetch", "XMLHttpRequest",
	"WebSocket", "WebTransport", "EventSource", "sendBeacon", "Worker",
	"location", "window.open", "open(", "import(", "importScripts", "eval",
	"Function(", "setTimeout", "setInterval", "srcdoc", "createElement",
	"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write",
	"top.", "parent.", "opener", "postMessage", "form", "submit",
}

// createdElement names an element a script builds that frames, links, or
// loads something, which the plain word createElement does not say.
var createdElement = regexp.MustCompile(`createElement(?:NS)?\(\s*(?:[^,)]*,\s*)?["'\x60](iframe|frame|link|script|object|embed|img|a|form|base|meta)["'\x60]`)

// Describe reports on one inspected figure. It reads the same bytes that were
// inspected and hashed, and takes the script count from that inspection.
func Describe(data []byte, in *Inspection) Report {
	text := string(data)
	report := Report{Size: len(data), Scripts: len(in.Scripts)}
	seen := map[string]bool{}
	for _, match := range addressPattern.FindAllString(text, -1) {
		match = strings.TrimLeft(match, " \t\r\n\"'(=`")
		if !seen[match] {
			seen[match] = true
			report.Addresses = append(report.Addresses, match)
		}
	}
	for _, name := range reachingNames {
		if strings.Contains(text, name) {
			report.Names = append(report.Names, name)
		}
	}
	for _, match := range createdElement.FindAllStringSubmatch(text, -1) {
		name := "createElement(" + match[1] + ")"
		if !seen[name] {
			seen[name] = true
			report.Names = append(report.Names, name)
		}
	}
	return report
}
