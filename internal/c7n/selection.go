package c7n

import (
	"fmt"
	"path"
	"strings"
)

// Selection is what to pass to `custodian run` for a set of chosen
// policies: the files that hold them and one -p per policy name.
type Selection struct {
	Files []string
	Names []string
}

// Select builds the custodian arguments for the chosen policies and checks
// that custodian would run exactly those. custodian applies every -p (a
// glob) to every file given, so a name repeated in another file, or a
// duplicate name, would run more than was chosen; that is refused.
func Select(all []PolicyFile, chosen []Policy) (Selection, error) {
	var sel Selection
	if len(chosen) == 0 {
		return sel, fmt.Errorf("no policy selected")
	}
	fileSeen := map[string]bool{}
	nameSeen := map[string]bool{}
	want := map[string]bool{} // file:line of chosen policies
	for _, p := range chosen {
		if p.Name == "" {
			return sel, fmt.Errorf("a policy in %s has no name", p.File)
		}
		if !fileSeen[p.File] {
			fileSeen[p.File] = true
			sel.Files = append(sel.Files, p.File)
		}
		if !nameSeen[p.Name] {
			nameSeen[p.Name] = true
			sel.Names = append(sel.Names, p.Name)
		}
		want[policyKey(p)] = true
	}

	// What custodian would actually pick.
	var extra []string
	for _, f := range all {
		if !fileSeen[f.Path] {
			continue
		}
		for _, p := range f.Policies {
			if matchesAny(p.Name, sel.Names) && !want[policyKey(p)] {
				extra = append(extra, fmt.Sprintf("%s (%s:%d)", p.Name, p.File, p.Line))
			}
		}
	}
	if len(extra) > 0 {
		return sel, fmt.Errorf("custodian would also run %s: policy names must be unique across the selected files; select from one file at a time or rename",
			strings.Join(extra, ", "))
	}
	return sel, nil
}

func policyKey(p Policy) string { return fmt.Sprintf("%s:%d", p.File, p.Line) }

// matchesAny mirrors custodian's -p matching (Python fnmatch). A pattern
// that path.Match cannot parse is compared literally.
func matchesAny(name string, patterns []string) bool {
	for _, pat := range patterns {
		ok, err := path.Match(pat, name)
		if err != nil {
			ok = pat == name
		}
		if ok {
			return true
		}
	}
	return false
}
