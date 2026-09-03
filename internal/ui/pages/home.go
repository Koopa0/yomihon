package pages

import (
	"github.com/koopa0/yomihon/internal/wording"
)

// homeRecentTitle picks the recent block's heading: the recency one where the
// recorded times order the list, the plain one where they separate nothing.
func homeRecentTitle(v *HomeView, lang wording.Lang) string {
	if v.RecentOrdered {
		return wording.HomeRecentTitle.In(lang)
	}
	return wording.HomeTiedTitle.In(lang)
}

// homeRecentLede picks the sentence under that heading — ordered or tied, and
// naming the knowledge layer exactly when the list is scoped to one.
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
