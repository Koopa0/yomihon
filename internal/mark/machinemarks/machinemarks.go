// Package machinemarks is how the reader's marks are kept in the vault: one
// file of lines for each machine, which is what git carries between computers,
// read together as one set of marks, and the changes this machine makes to its
// own.
//
// A mark is one thing in one of three states, a highlight, a thought or a
// question, and a line of a file holds the whole of it. A change of any kind,
// of state, of words, of a question resolved, of a deletion, is a new line of
// the same mark, written by the machine that changed it, in its own file and in
// no other. So two computers never write one file, and what they write is
// merged when it is read, not when it is written.
//
// # Which line counts
//
// A line is written by a writer, which is a machine and one of its processes,
// and has a rev, which only goes up for one writer. A line of a mark replaces
// another when the same writer wrote it later, or when it says, in its seen,
// that its writer's page had read the other's writer at the other's rev or
// later. Lines nothing replaces are the mark's values. One is the mark's value;
// several are a conflict, and none of them is merged into another: the latest
// that is not a deletion counts, so that a deletion by a machine that had not
// read the words does not hide them, and the others are shown beside it.
// Nothing is decided by a clock: the time on a line only orders what is shown.
//
// The rule is one a reader without Yomihon can follow: read-marks.jq is a
// program for jq that does, and the tests hold it to what this package reads.
//
// # This machine's own lines
//
// What git can do to a working tree it can do to this machine's file: take
// lines away, bring an older file back, leave its conflict markers in it, and an
// agent can change a line. So this machine's own lines are read from the file
// and from the ledger together, which is outside the vault, and a change is
// recorded in the ledger before the file is written: the line recorded is the
// commit, and a file that was not written, or was taken away, is written again
// from the ledger by Restore or by the next change. A file that is replaced
// while it holds anything the machine did not write is kept whole in the ledger
// first.
//
// # Who is who
//
// A machine has an id, 8 hexadecimal digits made once, and each process of it a
// code of the same shape, written on every line it writes. A line under this
// machine's id whose process this machine never ran was written by another
// computer that has a copy of this one's configuration. The machine that finds
// one leaves its id for a new one, and says so to the log. The lines it
// wrote under the old id stay its own and are written into the file of the new
// id, since the file of the old one is another machine's now; the lines it
// did not write are never taken for its own. So a file can hold lines whose m
// is not the id of its name: the name says who wrote the file, a line's m says
// who wrote the line, and a reader never takes the one for the other.
//
// A write is made under the vault's lock, machine-file.lock in the vault's
// directory under the configuration directory, and takes the ledger's lock
// inside it. Reading takes no lock and writes nothing, not even a folder. This
// machine's own file that cannot be read is counted and the marks are still
// shown, from the ledger; a write refuses, since it would replace what it did
// not read.
//
// Every call that gives a mark out or changes one takes the privacy gate, and a
// write to a mark the gate withholds is answered as a write to one that is not
// there. A line is held to the shape the file of marks before this one held a
// mark to, on the way in and on the way out.
//
// The routes of package mark call it: it is what keeps the reader's marks, and
// nothing else in the process writes them into the vault.
package machinemarks
