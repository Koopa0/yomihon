package pages

// preferenceGroup is a run of choices under one heading, or the choices ahead
// of every heading where Title is empty.
type preferenceGroup struct {
	Title  string
	Fields []PreferenceField
}

// groups gathers the choices into the runs the page heads, in their order: a
// new run starts wherever a choice's group is not the one before it.
func (v *PreferencesView) groups() []preferenceGroup {
	var out []preferenceGroup
	for i := range v.Fields {
		f := &v.Fields[i]
		if len(out) == 0 || out[len(out)-1].Title != f.Group {
			out = append(out, preferenceGroup{Title: f.Group})
		}
		out[len(out)-1].Fields = append(out[len(out)-1].Fields, *f)
	}
	return out
}
