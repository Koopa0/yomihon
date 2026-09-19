package pages

import "strconv"

// PageNumber is which stretch of a long listing one request asked for. Pages
// are counted from one; AllPages is the whole listing on a single page, which
// is what a sheet of paper gets. The zero value reads as the first page, so a
// listing whose request said nothing still answers with one.
type PageNumber int

// AllPages asks for the listing undivided.
const AllPages PageNumber = -1

// allPagesWord is how a request spells AllPages, and how a link writes it back.
const allPagesWord = "all"

// maxPageNumber is the largest page this interface will read out of a request.
// It is here so a page number can be multiplied by a page size without
// overflowing, and a folder of notes holding more rows than this has other
// problems: a listing that deep is not one a reader reached, it is a number
// somebody typed.
const maxPageNumber = 1_000_000

// pagerReach is how many pages either side of the current one the strip names.
// Far enough that the pages a reader is about to want are already there, near
// enough that the strip stays one line at a phone's width; the way to the end
// of a long listing is the whole-listing link, not a hundred numbers.
const pagerReach = 2

// ParsePageNumber reads the stretch a request asked a listing for. The set is
// closed the way the findings table's ordering is: a value outside it — no
// number, a zero, a negative, a number past the ceiling above — leaves the
// reader on the first page rather than on an argument about it. Which stretch
// of a listing is on screen is how the page is laid out, not what it is about,
// so a request that cannot be honoured is answered with the page.
//
// A page past the end of the listing is outside the set too, but only the
// listing knows where its end is, so NewPager makes that fall back rather than
// this.
func ParsePageNumber(value string) PageNumber {
	if value == allPagesWord {
		return AllPages
	}
	asked, err := strconv.Atoi(value)
	if err != nil || asked < 1 || asked > maxPageNumber {
		return 1
	}
	return PageNumber(asked)
}

// String is how a request spells this page, in a link and in the address bar
// alike. There is one spelling, so a link and the parse that reads it back
// cannot disagree about what a page is called.
func (n PageNumber) String() string {
	if n == AllPages {
		return allPagesWord
	}
	return strconv.Itoa(int(n))
}

// Pager is one listing's division into pages, as the strip under it draws
// them: which rows this page holds, how many rows there are in all, and the
// address of each page reachable from here. It describes one request's answer
// and is kept by nobody.
//
// Number is greater than zero exactly when the listing is really divided, so
// one field answers both "is there a strip" and "which page is this". An
// undivided listing — one page's worth of rows, or the whole listing asked for
// at once — carries the whole stretch and no addresses at all.
type Pager struct {
	// First and Last are the half-open stretch of the listing this page holds,
	// counted from zero.
	First, Last int
	// Total is every row the listing has. The count sentence and every tally
	// beside the rows are answerable to this number, never to the page.
	Total int
	// Number is the page on screen, counted from one, and zero where the
	// listing is undivided.
	Number int
	// Count is how many pages the listing comes to.
	Count int
	// Previous and Next are the addresses either side of this page, empty at
	// each end: a control that leads nowhere is worse than no control.
	Previous, Next string
	// Numbers are the pages near this one, this one among them.
	Numbers []PagerPage
	// Whole is the address of the undivided listing, which is what prints as
	// the whole report.
	Whole string
}

// PagerPage is one page the strip names: its number, where it is, and whether
// it is the one being read.
type PagerPage struct {
	Number  int
	Href    string
	Current bool
}

// NewPager divides a listing of total rows into pages of size rows each and
// describes the one asked for. size must be at least one.
//
// address writes one page's own link, which is the only thing two listings
// using this strip do differently. It is called once per named page and never
// kept, because how a link is spelled is no part of a division's identity.
//
// A page the listing does not have falls back to the first, for the reason
// ParsePageNumber gives.
func NewPager(asked PageNumber, size, total int, address func(PageNumber) string) Pager {
	undivided := Pager{Last: total, Total: total}
	count := (total + size - 1) / size
	if asked == AllPages || count <= 1 {
		return undivided
	}
	number := int(asked)
	if number < 1 || number > count {
		number = 1
	}
	divided := Pager{
		First:   (number - 1) * size,
		Last:    min(number*size, total),
		Total:   total,
		Number:  number,
		Count:   count,
		Numbers: make([]PagerPage, 0, 2*pagerReach+1),
		Whole:   address(AllPages),
	}
	if number > 1 {
		divided.Previous = address(PageNumber(number - 1))
	}
	if number < count {
		divided.Next = address(PageNumber(number + 1))
	}
	for near := max(1, number-pagerReach); near <= min(count, number+pagerReach); near++ {
		divided.Numbers = append(divided.Numbers, PagerPage{
			Number:  near,
			Href:    address(PageNumber(near)),
			Current: near == number,
		})
	}
	return divided
}
