package c7n

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Report is the output of `custodian report --format csv`: the columns c7n
// considers useful for the resource type (its default report fields), one
// row per matched resource.
type Report struct {
	Columns []string
	Rows    [][]string
}

// ParseReportCSV reads custodian's CSV report. Short or long rows are kept
// as they are.
func ParseReportCSV(data []byte) (Report, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil {
		return Report{}, err
	}
	if len(records) == 0 {
		return Report{}, errors.New("empty report")
	}
	return Report{Columns: records[0], Rows: records[1:]}, nil
}

// RowFor returns the report row of a resource: the row that contains its
// id, or else the row at the same position. ok is false if there is none.
func (r Report) RowFor(res Resource, index int) ([]string, bool) {
	for _, row := range r.Rows {
		for _, cell := range row {
			if cell != "" && cell == res.ID {
				return row, true
			}
		}
	}
	if index >= 0 && index < len(r.Rows) {
		return r.Rows[index], true
	}
	return nil, false
}

// Index maps each resource to its report row (as RowFor does), in one
// pass, for long lists. -1 means no row.
func (r Report) Index(resources []Resource) []int {
	byCell := map[string]int{}
	for i, row := range r.Rows {
		for _, cell := range row {
			if _, seen := byCell[cell]; cell != "" && !seen {
				byCell[cell] = i
			}
		}
	}
	out := make([]int, len(resources))
	for i, res := range resources {
		row, ok := byCell[res.ID]
		switch {
		case ok:
			out[i] = row
		case i < len(r.Rows):
			out[i] = i
		default:
			out[i] = -1
		}
	}
	return out
}

// WriteReportPolicy writes the policy recorded in pr's metadata.json as a
// policy file in dir, for `custodian report`, which needs the policy (any
// output dir can be reported this way, even without its original file).
// The caller removes the file.
func WriteReportPolicy(pr PolicyRun, dir string) (string, error) {
	if len(pr.Spec) == 0 {
		return "", errors.New("no policy in metadata.json")
	}
	data, err := json.Marshal(map[string][]json.RawMessage{"policies": {pr.Spec}})
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "report-*.json")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return "", err
	}
	return filepath.Clean(f.Name()), f.Close()
}
