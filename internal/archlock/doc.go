// Package archlock holds the checks that keep the module's shape from drifting
// while every individual change looks reasonable. Each one states a property a
// reader would otherwise have to re-derive by following imports, and fails with
// the import chain that broke it.
//
// The checks are tests because a property about this repository is not
// something the program does, so there is nothing to link them into; running
// under the ordinary test command is what makes them a gate rather than a
// document. That is also why this package declares nothing: this file carries
// the reason so that go doc can answer why the directory exists.
package archlock
