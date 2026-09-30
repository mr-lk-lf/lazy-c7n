// Package c7n holds lazyc7n's view of Cloud Custodian data: policies, run
// output and the safety classification of actions. It never evaluates
// policies itself; that is always custodian's job.
package c7n

import (
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Policy is the part of a c7n policy that lazyc7n needs to show and to
// decide how careful a run must be. Unknown policy fields are ignored.
type Policy struct {
	Name        string
	Resource    string // e.g. "aws.ec2"
	Mode        string // mode.type; "" means pull
	Description string
	Filters     []string // one-line summaries, in policy order
	Actions     []string // action types, in policy order
	File        string   // policy file it was read from
	Line        int      // first line of the policy in File (1-based)
	EndLine     int      // last line of the policy in File
}

// IsPull reports whether the policy runs in pull mode. Every other mode is
// serverless: a live run deploys a Lambda instead of evaluating resources
// (SPEC §6.5). An unknown mode is treated as serverless (fail closed).
func (p Policy) IsPull() bool {
	return p.Mode == "" || p.Mode == "pull"
}

// DestructiveActions returns the policy's actions classified as destructive,
// in policy order.
func (p Policy) DestructiveActions() []string {
	var out []string
	for _, a := range p.Actions {
		if ClassifyAction(a) == ActionDestructive {
			out = append(out, a)
		}
	}
	return out
}

// PolicyFile is one YAML file with a top-level `policies:` list.
type PolicyFile struct {
	Path     string
	Rel      string // Path relative to the directory it was found in, for display
	Policies []Policy
	Lines    []string // file content, for the YAML view
	Err      error    // the file looked like a policy file but could not be read
}

// maxPolicyFileSize guards against opening huge unrelated YAML files.
const maxPolicyFileSize = 5 << 20

// ReadPolicyFile parses path. ok is false when the file is not a c7n policy
// file (no top-level `policies:` key), which is not an error.
func ReadPolicyFile(path string) (f PolicyFile, ok bool) {
	f.Path = path
	info, err := os.Stat(path)
	if err != nil {
		f.Err = err
		return f, true
	}
	if info.Size() > maxPolicyFileSize {
		return f, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		f.Err = err
		return f, true
	}
	return ParsePolicies(path, data)
}

// ParsePolicies parses the content of a policy file. See ReadPolicyFile.
func ParsePolicies(path string, data []byte) (f PolicyFile, ok bool) {
	f.Path = path
	f.Lines = strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		// Only report broken files that were meant to be policy files.
		if hasPoliciesLine(f.Lines) {
			f.Err = fmt.Errorf("invalid YAML: %w", err)
			return f, true
		}
		return f, false
	}
	if len(doc.Content) == 0 {
		return f, false
	}
	root := resolve(doc.Content[0])
	if root.Kind != yaml.MappingNode {
		return f, false
	}
	list := mapValue(root, "policies")
	if list == nil {
		return f, false
	}
	list = resolve(list)
	if list.Kind != yaml.SequenceNode {
		f.Err = fmt.Errorf("`policies` is not a list")
		return f, true
	}

	for _, item := range list.Content {
		p := parsePolicy(resolve(item))
		p.File = path
		p.Line = item.Line
		f.Policies = append(f.Policies, p)
	}
	// A policy ends where the next one starts (or at the end of the file),
	// minus trailing blank and comment lines.
	for i := range f.Policies {
		end := len(f.Lines)
		if i+1 < len(f.Policies) {
			end = f.Policies[i+1].Line - 1
		}
		for end > f.Policies[i].Line {
			t := strings.TrimSpace(f.Lines[end-1])
			if t != "" && !strings.HasPrefix(t, "#") {
				break
			}
			end--
		}
		f.Policies[i].EndLine = end
	}
	return f, true
}

// PolicyText returns the lines of the file that hold p.
func (f PolicyFile) PolicyText(p Policy) []string {
	if p.Line < 1 || p.EndLine > len(f.Lines) || p.EndLine < p.Line {
		return nil
	}
	return f.Lines[p.Line-1 : p.EndLine]
}

func hasPoliciesLine(lines []string) bool {
	for _, l := range lines {
		if strings.HasPrefix(l, "policies:") {
			return true
		}
	}
	return false
}

func parsePolicy(n *yaml.Node) Policy {
	var p Policy
	if n.Kind != yaml.MappingNode {
		return p
	}
	p.Name = scalar(mapValue(n, "name"))
	p.Resource = scalar(mapValue(n, "resource"))
	p.Description = strings.TrimSpace(scalar(mapValue(n, "description")))
	if mode := resolve(mapValue(n, "mode")); mode != nil && mode.Kind == yaml.MappingNode {
		p.Mode = scalar(mapValue(mode, "type"))
	}
	if filters := resolve(mapValue(n, "filters")); filters != nil && filters.Kind == yaml.SequenceNode {
		for _, f := range filters.Content {
			p.Filters = append(p.Filters, summarize(resolve(f)))
		}
	}
	if actions := resolve(mapValue(n, "actions")); actions != nil && actions.Kind == yaml.SequenceNode {
		for _, a := range actions.Content {
			a = resolve(a)
			switch a.Kind {
			case yaml.ScalarNode:
				p.Actions = append(p.Actions, a.Value)
			case yaml.MappingNode:
				p.Actions = append(p.Actions, scalar(mapValue(a, "type")))
			case yaml.DocumentNode, yaml.SequenceNode, yaml.AliasNode:
				p.Actions = append(p.Actions, "?")
			}
		}
	}
	return p
}

// summarize turns one filter into a short line: "tag:owner: absent",
// "type: value", "or (2)".
func summarize(n *yaml.Node) string {
	switch n.Kind {
	case yaml.ScalarNode:
		return n.Value
	case yaml.MappingNode:
		if t := scalar(mapValue(n, "type")); t != "" {
			if k := scalar(mapValue(n, "key")); k != "" {
				return t + " " + k + oneLine(mapValue(n, "value"), " = ")
			}
			return t
		}
		if len(n.Content) >= 2 {
			k := n.Content[0].Value
			v := resolve(n.Content[1])
			if v.Kind == yaml.SequenceNode {
				return fmt.Sprintf("%s (%d)", k, len(v.Content))
			}
			return k + oneLine(v, ": ")
		}
	case yaml.DocumentNode, yaml.SequenceNode, yaml.AliasNode:
	}
	return "?"
}

func oneLine(n *yaml.Node, sep string) string {
	n = resolve(n)
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return sep + n.Value
}

// mapValue returns the value for key in a mapping node, or nil.
func mapValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func scalar(n *yaml.Node) string {
	n = resolve(n)
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

// resolve follows YAML aliases (*anchor).
func resolve(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}
