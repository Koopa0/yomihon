package pages

import "net/url"

func sourceLocationHref(relPath, fragment string) string {
	href := notesHref(relPath)
	if fragment != "" {
		href += "#" + url.PathEscape(fragment)
	}
	return href
}
