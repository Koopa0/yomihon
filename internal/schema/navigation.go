package schema

import (
	"fmt"
	"maps"
	"slices"
	"sync"
)

// NavigationRoles classifies the note types that make up courses, units,
// indexes and thoughts; its membership sets cannot be changed after loading.
// The zero value is unclaimed, so every set is empty and no note is in one.
type NavigationRoles struct {
	sets  *roleSets
	claim Claim
}

// roleSets holds what the contract declared. They sit behind one pointer so
// that NavigationRoles stays small enough to pass by value.
type roleSets struct {
	courses map[string]struct{}
	indexes map[string]struct{}
	units   map[string]struct{}
	thought string
}

// Claim reports how far the navigation declaration got.
func (r NavigationRoles) Claim() Claim {
	return r.claim
}

// Available reports whether the contract declared a valid navigation role set.
func (r NavigationRoles) Available() bool {
	return r.claim.held()
}

// Trustworthy reports whether the role sets may be projected over: true when
// they were read cleanly or never declared, false when a declaration was made
// and could not be honoured.
func (r NavigationRoles) Trustworthy() bool {
	return r.claim.Trustworthy()
}

// IsPathType reports whether noteType is declared as a course. A declaration
// that could not be honoured holds no sets, so it declares none.
func (r NavigationRoles) IsPathType(noteType string) bool {
	if r.sets == nil {
		return false
	}
	_, ok := r.sets.courses[NormalizeWord(noteType)]
	return ok
}

// IsMapType reports whether noteType is declared as an index.
func (r NavigationRoles) IsMapType(noteType string) bool {
	if r.sets == nil {
		return false
	}
	_, ok := r.sets.indexes[NormalizeWord(noteType)]
	return ok
}

// PathTypes returns every type the contract declares for courses, and MapTypes
// the same for indexes. They are the words a surface names
// when it has to tell a reader what puts a note on one of those two shelves:
// nothing else can spell them, because a second copy of the vault's vocabulary
// goes on saying a word the vault has since renamed. The words are the folded
// spelling every comparison here uses.
//
// The answer is sorted rather than given in the order the contract listed it.
// What the contract declares is membership, so reordering the same words is no
// change at all, and a reader's sentence that rearranged itself for one would
// be reporting something that did not happen.
//
// A declaration nothing made, or one that could not be honoured, names none —
// the same answer IsPathType and IsMapType give about every note. The returned
// slice is the caller's own.
func (r NavigationRoles) PathTypes() []string {
	if r.sets == nil {
		return nil
	}
	return slices.Sorted(maps.Keys(r.sets.courses))
}

// MapTypes returns the types the contract declares as indexes. See PathTypes
// for what the answer means.
func (r NavigationRoles) MapTypes() []string {
	if r.sets == nil {
		return nil
	}
	return slices.Sorted(maps.Keys(r.sets.indexes))
}

// IsLessonType reports whether noteType is the type this vault files a
// course's units as. A course asks it to tell one made of units from one made
// of other notes, so the comparison against that spelling is made here and
// nowhere else. A contract that gives no type the lesson role has none, and so
// does the zero value.
func (r NavigationRoles) IsLessonType(noteType string) bool {
	if r.sets == nil {
		return false
	}
	_, ok := r.sets.units[NormalizeWord(noteType)]
	return ok
}

// AnswerType returns the note type that holds the thought role: the notes a
// reader writes about what they read. A contract that gives no type the role,
// or one whose roles could not be honoured, returns no type.
func (r NavigationRoles) AnswerType() string {
	if r.sets == nil {
		return ""
	}
	return r.sets.thought
}

// deriveNavigationRoles reads the roles the contract gave its types. A role it
// does not know, or a second type holding a role only one may hold, closes
// navigation rather than refusing the contract: the check report says so, and
// the reader still opens everything it can read.
func deriveNavigationRoles(types []typeDefinition) NavigationRoles {
	sets := roleSets{courses: map[string]struct{}{}, indexes: map[string]struct{}{}}
	holders := map[Role]string{}
	for _, def := range types {
		switch def.role {
		case "":
			continue
		case RoleCourse:
			sets.courses[def.name] = struct{}{}
		case RoleMap:
			sets.indexes[def.name] = struct{}{}
		case RoleLesson, RoleThought:
			if holder, taken := holders[def.role]; taken {
				return invalidNavigationRoles("types %q and %q both hold the %s role, which one type holds", holder, def.name, def.role)
			}
			holders[def.role] = def.name
		default:
			return invalidNavigationRoles("type %q has the unknown role %q", def.name, string(def.role))
		}
	}
	if lesson := holders[RoleLesson]; lesson != "" {
		sets.units = map[string]struct{}{lesson: {}}
	}
	sets.thought = holders[RoleThought]
	return NavigationRoles{sets: &sets, claim: heldClaim()}
}

// builtInNavigationRoles are the type names a folder with no rules file is read
// by. A note that calls itself a course, a unit, an index or a thought is one,
// and the older words each of those has — study-path, lesson, moc, map — are
// accepted beside them. A rules file replaces the whole set; it is never
// merged into it.
var builtInNavigationRoles = sync.OnceValue(func() NavigationRoles {
	return NavigationRoles{
		sets: &roleSets{
			courses: map[string]struct{}{"course": {}, "study-path": {}},
			indexes: map[string]struct{}{"index": {}, "moc": {}, "map": {}},
			units:   map[string]struct{}{"unit": {}, "lesson": {}},
			thought: "thought",
		},
		claim: heldClaim(),
	}
})

func invalidNavigationRoles(format string, args ...any) NavigationRoles {
	return NavigationRoles{claim: Rejected(fmt.Sprintf("invalid navigation roles: "+format+"; Paths and Maps disabled", args...))}
}
