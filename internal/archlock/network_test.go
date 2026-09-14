package archlock

import (
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// outboundClientSurface is net/http's outbound half: the convenience calls, the
// client and transport a caller borrows or composes, and the request
// constructors whose only consumer is a client. Serving is absent from the list
// rather than exempted by name, so Server, ServeMux, Redirect, the handler types
// and the response writer stay legal without this check having to know they
// exist.
var outboundClientSurface = []string{
	"Client", "DefaultClient", "DefaultTransport", "Get", "Head",
	"NewRequest", "NewRequestWithContext", "Post", "PostForm", "Transport",
}

// loopbackListenerFile is the one file that may name a listener, because the
// command binds the server's single socket there. Which address that socket
// carries is a different question from the one below, which reads only whether
// a file spells something that opens one.
const loopbackListenerFile = "cmd/yomihon/main.go"

// opensAConnection reports whether a symbol starts a connection or names the
// thing that would start one. The net families are matched by prefix, so a
// variant the standard library grows is refused the day it appears; Listener is
// the interface a server is handed, and naming one starts nothing.
//
// Turning a name into an address is itself a call to whatever answers for this
// machine, so the whole Lookup family is refused beside the dialers, and so are
// the two names that reach the resolver behind it. Those two are written out
// because they are a pair and not a family: a prefix would claim a shape the
// standard library does not have here.
//
// crypto/tls dials on its own, without the caller spelling anything from net,
// so its Dial family is refused by the same prefix. That prefix also takes in
// Dialer, the type whose methods dial: a call on a value is invisible to a walk
// that resolves import names, but no file can hold a Dialer without spelling
// the type. Client wraps a connection as its outbound side and is written out
// exactly, because its four Client-prefixed siblings are configuration and name
// no connection. Listen binds a socket of its own, the same way net.Listen
// does, so it is refused alongside the dialers with no exemption; NewListener
// wraps a listener that already exists and opens nothing, so it stays absent.
// Server and Config are the rest of the serving half and are absent too.
func opensAConnection(pkg, symbol string) bool {
	switch pkg {
	case "net":
		switch symbol {
		case "Listener":
			return false
		case "DefaultResolver", "Resolver":
			return true
		}
		return strings.HasPrefix(symbol, "Dial") ||
			strings.HasPrefix(symbol, "Listen") ||
			strings.HasPrefix(symbol, "Lookup")
	case "net/http":
		return slices.Contains(outboundClientSurface, symbol)
	case "crypto/tls":
		return symbol == "Client" || symbol == "Listen" || strings.HasPrefix(symbol, "Dial")
	}
	return false
}

// outboundSites reports every connection-opening symbol one file names, and
// counts separately the ones the loopback listener is allowed, so a caller can
// tell a clean tree from a walk that read nothing at all. Import names are
// resolved first: a file that reaches the same package under another name
// writes calls no fixed spelling would recognise, and a file that dot-imports
// it hides them entirely, which is reported rather than audited.
func outboundSites(path string, fset *token.FileSet, file *ast.File) (found []site, permitted int) {
	pkgOf := map[string]string{}
	for _, imp := range file.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil || (p != "net" && p != "net/http" && p != "crypto/tls") {
			continue
		}
		switch {
		case imp.Name == nil:
			pkgOf[p[strings.LastIndexByte(p, '/')+1:]] = p
		case imp.Name.Name == ".":
			found = append(found, site{
				path: path,
				line: fset.Position(imp.Pos()).Line,
				text: p + " is imported into this file's own namespace, so what it opens cannot be read here",
			})
		case imp.Name.Name == "_":
			// Imported for its initialisers; nothing here is callable through it.
		default:
			pkgOf[imp.Name.Name] = p
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		selector, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		qualifier, isIdent := selector.X.(*ast.Ident)
		if !isIdent {
			return true
		}
		pkg, imported := pkgOf[qualifier.Name]
		if !imported || !opensAConnection(pkg, selector.Sel.Name) {
			return true
		}
		if path == loopbackListenerFile && pkg == "net" && selector.Sel.Name == "ListenConfig" {
			permitted++
			return true
		}
		found = append(found, site{
			path: path,
			line: fset.Position(selector.Pos()).Line,
			text: pkg + "." + selector.Sel.Name,
		})
		return true
	})
	return found, permitted
}

// TestTheOnlySocketProductionCodeOpensIsTheLoopbackListener keeps yomihon off
// the network. It reads every shipped Go file for a spelling that opens a
// connection — a dialer, a listener, a name lookup, net/http's client half, or
// crypto/tls's dialers and client side — and permits exactly one: the listener
// the command binds for the server itself.
//
// Serving what this machine asked for is not reaching out. The stylesheet and
// the diagram module under /static/ are same-origin answers written to a
// request already in hand, and the reading page's own content policy is what
// forbids a subresource from anywhere else.
//
// It judges a name where it is written rather than a call, which is why Do is
// not among them: a Do call needs a client, and no file can name a client
// without spelling one of these. A client held in a field or handed over as a
// value would otherwise read as nothing at all.
//
// A tree naming none of them is the state this asks for, so finding nothing is
// where the check finishes rather than a reason to distrust it. What would make
// it meaningless is a walk that reached neither the command nor its listener,
// and that is what the count below guards.
func TestTheOnlySocketProductionCodeOpensIsTheLoopbackListener(t *testing.T) {
	t.Parallel()

	var found []site
	permitted := 0
	forEachProductionFile(t, func(path string, fset *token.FileSet, file *ast.File) {
		sites, allowed := outboundSites(path, fset, file)
		found = append(found, sites...)
		permitted += allowed
	})
	if permitted == 0 {
		t.Fatal("the loopback listener was not seen, so this walk never reached the file that binds the server's socket")
	}
	report(t, "this opens a connection, and yomihon makes no outbound call of any kind", found)
}
