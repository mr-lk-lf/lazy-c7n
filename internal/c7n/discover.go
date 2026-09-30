package c7n

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Discover finds policy files (SPEC §9.4): every *.yml / *.yaml under each
// path (recursively) that has a top-level `policies:` key. A path may also
// be a single file. Hidden directories and node_modules are skipped.
// Missing paths are reported as a PolicyFile with Err set.
func Discover(paths []string) []PolicyFile {
	var files []PolicyFile
	seen := map[string]bool{}
	add := func(path, rel string) {
		abs, err := filepath.Abs(path)
		if err == nil && seen[abs] {
			return
		}
		seen[abs] = true
		if f, ok := ReadPolicyFile(path); ok {
			f.Rel = filepath.ToSlash(rel)
			files = append(files, f)
		}
	}

	for _, root := range paths {
		info, err := os.Stat(root)
		if err != nil {
			files = append(files, PolicyFile{Path: root, Rel: root, Err: err})
			continue
		}
		if !info.IsDir() {
			add(root, filepath.Base(root))
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // unreadable entries are skipped, not fatal
			}
			name := d.Name()
			if d.IsDir() {
				if path != root && (strings.HasPrefix(name, ".") || name == "node_modules") {
					return filepath.SkipDir
				}
				return nil
			}
			if ext := strings.ToLower(filepath.Ext(name)); ext == ".yml" || ext == ".yaml" {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					rel = path
				}
				add(path, rel)
			}
			return nil
		})
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}
