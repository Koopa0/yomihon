// Package archlock holds the checks that keep the module's shape from drifting
// while every individual change looks reasonable. Each states a property a
// reader would otherwise re-derive by following imports, and fails with the
// import chain that broke it. The checks are tests, so this file exists to
// carry the package comment.
package archlock
