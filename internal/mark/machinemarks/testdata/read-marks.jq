# Reads the machine files of a vault with no help from Yomihon:
#
#   jq -c -n -R -f read-marks.jq reader-notes/.yomihon/marks/*.jsonl
#
# It prints one object for each mark: the value that counts (mark), and the
# other values of a conflict (conflict), each a whole value. A mark whose every
# value is a deletion has mark.deleted set.
#
# The rule it follows:
#  1. Every line of every file is read. A line that is not JSON, which a git
#     conflict marker is not, is skipped, and so is one whose fmt is not 1.
#  2. Lines are grouped by mark.id. A writer is machine "/" process (m, s).
#  3. A line is replaced by another of its group when that is of the same writer
#     and has a larger rev, or is of another writer and says in seen that it had
#     read this line's writer at this line's rev or later. A replaced line does
#     not count.
#  4. One line left is the mark's value. Several are a conflict: they are put in
#     the order of t, the latest first, and the first of them that is not a
#     deletion goes to the front, and it is the value.
#
# What it does not do: check the checksum at the end of each line, which jq has
# no SHA-256 to do, and read this machine's ledger, which is outside the vault.
# So a line changed by hand is read by it and not by Yomihon, and what git took
# out of this machine's working tree is not seen.
[inputs | fromjson? | select(type == "object" and .fmt == 1 and (.mark.id | type) == "string")]
| map(. + {w: (.m + "/" + (.s // ""))})
| unique_by([.w, .rev, .mark.id])
| group_by(.mark.id)
| map(
    . as $group
    | [ $group[] | . as $line
        | select(
            any($group[] | . as $other
                | ($other.w == $line.w and $other.rev > $line.rev)
                  or ($other.w != $line.w and (($other.seen // {})[$line.w] // 0) >= $line.rev))
            | not
          )
      ]
    | if length == 0 then $group else . end
    | sort_by([.t, .w, .rev]) | reverse
    | (map(.mark.deleted != true) | index(true) // 0) as $first
    | [.[$first]] + .[:$first] + .[$first + 1:]
    | {id: .[0].mark.id, mark: .[0].mark, conflict: (.[1:] | map(.mark))}
  )
| .[]
