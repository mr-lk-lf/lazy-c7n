package c7n

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// Schema is the part of `custodian schema --json` the schema browser uses:
// for each resource type, its actions and filters with their JSON schema.
type Schema struct {
	Resources map[string]SchemaResource
	Modes     []string
}

type SchemaResource struct {
	Actions map[string]json.RawMessage
	Filters map[string]json.RawMessage
}

// ParseSchema reads the output of `custodian schema --json`. Unknown parts
// are ignored.
func ParseSchema(data []byte) (Schema, error) {
	var raw struct {
		Definitions struct {
			Resources map[string]struct {
				Actions map[string]json.RawMessage `json:"actions"`
				Filters map[string]json.RawMessage `json:"filters"`
			} `json:"resources"`
			PolicyMode struct {
				AnyOf []struct {
					Properties struct {
						Type struct {
							Enum []string `json:"enum"`
						} `json:"type"`
					} `json:"properties"`
				} `json:"anyOf"`
			} `json:"policy-mode"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Schema{}, fmt.Errorf("schema: %w", err)
	}
	if len(raw.Definitions.Resources) == 0 {
		return Schema{}, fmt.Errorf("schema: no resources in custodian's output")
	}
	s := Schema{Resources: map[string]SchemaResource{}}
	for name, r := range raw.Definitions.Resources {
		s.Resources[name] = SchemaResource{Actions: r.Actions, Filters: r.Filters}
	}
	for _, m := range raw.Definitions.PolicyMode.AnyOf {
		s.Modes = append(s.Modes, m.Properties.Type.Enum...)
	}
	sort.Strings(s.Modes)
	return s, nil
}

// ResourceTypes returns every resource type, sorted.
func (s Schema) ResourceTypes() []string {
	out := make([]string, 0, len(s.Resources))
	for name := range s.Resources {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Names returns the sorted keys of an actions or filters map.
func Names(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for name := range m {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// PrettyJSON indents raw JSON (or returns it unchanged if it is not JSON).
func PrettyJSON(raw json.RawMessage) string {
	var b bytes.Buffer
	if err := json.Indent(&b, raw, "", "  "); err != nil {
		return string(raw)
	}
	return b.String()
}
