package pages

import (
	"github.com/koopa0/yomihon/internal/wording"
)

// homeRecentTitle picks the recent block's heading: the recency heading when
// the recorded times actually order the list, and the tie notice's plain one
// when they separate nothing.
func homeRecentTitle(v *HomeView, lang wording.Lang) string {
	if v.RecentOrdered {
		return wording.HomeRecentTitle.In(lang)
	}
	return wording.HomeTiedTitle.In(lang)
}

// homeRecentLede picks the sentence under that heading. Each of the four is
// the one whose every clause is true of the page it sits on: ordered or
// tied, and naming the knowledge layer exactly when the list is scoped to
// one.
func homeRecentLede(v *HomeView, lang wording.Lang) string {
	switch {
	case v.RecentOrdered && v.RecentScoped:
		return wording.HomeRecentLedeScoped.In(lang)
	case v.RecentOrdered:
		return wording.HomeRecentLede.In(lang)
	case v.RecentScoped:
		return wording.HomeTiedLedeScoped.In(lang)
	default:
		return wording.HomeTiedLede.In(lang)
	}
}
