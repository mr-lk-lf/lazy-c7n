package c7n

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PolicyRun is what c7n wrote for one policy (in one region) into an output
// dir given with -s (SPEC §3 "c7n output layout").
type PolicyRun struct {
	Policy    string
	Region    string // from the directory layout or metadata
	Dir       string // <out>/[<region>/]<policy>
	Resource  string
	Mode      string
	Version   string // c7n version
	DryRun    bool
	AccountID string
	Start     time.Time
	End       time.Time
	Duration  float64 // seconds

	// ResourceCount is the number of matched resources, or -1 when c7n did
	// not write resources.json (policy error, or a live non-pull run).
	ResourceCount int
	APICalls      map[string]int
	ActionFiles   []string        // action-<name> files, without the prefix
	LogHasError   bool            // custodian-run.log mentions an error
	Spec          json.RawMessage // the policy as c7n loaded it (metadata.json "policy")
	Actions       []string        // action types of the policy, in order
}

// Status is a one-word summary for lists.
func (r PolicyRun) Status() string {
	switch {
	case r.ResourceCount >= 0 && !r.LogHasError:
		return "ok"
	case r.ResourceCount < 0 && !r.DryRun && r.Mode != "" && r.Mode != "pull" && !r.LogHasError:
		return "deployed"
	}
	return "error"
}

// ResourcesPath is resources.json of this run (it may not exist).
func (r PolicyRun) ResourcesPath() string { return filepath.Join(r.Dir, "resources.json") }

// LogPath is custodian-run.log of this run.
func (r PolicyRun) LogPath() string { return filepath.Join(r.Dir, "custodian-run.log") }

// metadata is the subset of metadata.json we read. Unknown fields are ignored.
type metadata struct {
	Policy struct {
		Name     string `json:"name"`
		Resource string `json:"resource"`
		Mode     struct {
			Type string `json:"type"`
		} `json:"mode"`
	} `json:"policy"`
	Version   string `json:"version"`
	Execution struct {
		Start    float64 `json:"start"`
		EndTime  float64 `json:"end_time"`
		Duration float64 `json:"duration"`
	} `json:"execution"`
	Config struct {
		Region    string `json:"region"`
		DryRun    bool   `json:"dryrun"`
		AccountID string `json:"account_id"`
	} `json:"config"`
	APIStats map[string]int `json:"api-stats"`
	Metrics  []struct {
		MetricName string  `json:"MetricName"`
		Value      float64 `json:"Value"`
	} `json:"metrics"`
}

// ReadOutputDir reads every policy result under dir, in both layouts:
// <dir>/<policy>/ and <dir>/<region>/<policy>/. Results are sorted by
// policy, then region.
func ReadOutputDir(dir string) ([]PolicyRun, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var runs []PolicyRun
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(dir, e.Name())
		if fileExists(filepath.Join(sub, "metadata.json")) {
			runs = append(runs, readPolicyDir(sub, ""))
			continue
		}
		// Multi-region layout: sub is a region.
		inner, err := os.ReadDir(sub)
		if err != nil {
			continue
		}
		for _, p := range inner {
			pdir := filepath.Join(sub, p.Name())
			if p.IsDir() && fileExists(filepath.Join(pdir, "metadata.json")) {
				runs = append(runs, readPolicyDir(pdir, e.Name()))
			}
		}
	}
	if len(runs) == 0 {
		return nil, ErrNoOutput
	}
	sort.SliceStable(runs, func(i, j int) bool {
		if runs[i].Policy != runs[j].Policy {
			return runs[i].Policy < runs[j].Policy
		}
		return runs[i].Region < runs[j].Region
	})
	return runs, nil
}

func readPolicyDir(dir, region string) PolicyRun {
	r := PolicyRun{Policy: filepath.Base(dir), Region: region, Dir: dir, ResourceCount: -1}

	var md metadata
	if data, err := os.ReadFile(filepath.Join(dir, "metadata.json")); err == nil {
		var spec struct {
			Policy json.RawMessage `json:"policy"`
		}
		if json.Unmarshal(data, &spec) == nil {
			r.Spec = spec.Policy
			r.Actions = specActions(spec.Policy)
		}
		if json.Unmarshal(data, &md) == nil {
			if md.Policy.Name != "" {
				r.Policy = md.Policy.Name
			}
			r.Resource = md.Policy.Resource
			r.Mode = md.Policy.Mode.Type
			r.Version = md.Version
			r.DryRun = md.Config.DryRun
			r.AccountID = md.Config.AccountID
			if r.Region == "" {
				r.Region = md.Config.Region
			}
			r.Start = epoch(md.Execution.Start)
			r.End = epoch(md.Execution.EndTime)
			r.Duration = md.Execution.Duration
			r.APICalls = md.APIStats
		}
	}

	if fileExists(r.ResourcesPath()) {
		r.ResourceCount = countResources(r.ResourcesPath(), md)
	}
	if log, err := os.ReadFile(r.LogPath()); err == nil {
		r.LogHasError = bytes.Contains(log, []byte(":ERROR ")) || bytes.Contains(log, []byte(" - ERROR - ")) ||
			bytes.Contains(log, []byte("Traceback (most recent call last)"))
	}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if name, ok := strings.CutPrefix(e.Name(), "action-"); ok && !e.IsDir() {
				r.ActionFiles = append(r.ActionFiles, name)
			}
		}
	}
	return r
}

// countResources prefers the ResourceCount metric (no need to read a big
// resources.json) and falls back to counting the array.
func countResources(path string, md metadata) int {
	for _, m := range md.Metrics {
		if m.MetricName == "ResourceCount" {
			return int(m.Value)
		}
	}
	_, total, err := ReadResources(path, "", 0)
	if err != nil {
		return -1
	}
	return total
}

func epoch(sec float64) time.Time {
	if sec <= 0 {
		return time.Time{}
	}
	whole, frac := math.Modf(sec)
	return time.Unix(int64(whole), int64(frac*1e9))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Resource is one matched resource from resources.json.
type Resource struct {
	ID   string
	Raw  json.RawMessage // exactly as c7n wrote it
	Tags []Tag
}

type Tag struct{ Key, Value string }

// Pretty returns the resource as indented JSON, keeping c7n's key order.
func (r Resource) Pretty() string {
	var b bytes.Buffer
	if err := json.Indent(&b, r.Raw, "", "  "); err != nil {
		return string(r.Raw)
	}
	return b.String()
}

// MaxResources is how many resources the Resources screen loads; bigger
// results are counted but not kept in memory.
const MaxResources = 10000

// ReadResources streams resources.json and keeps the first limit
// resources (limit 0: only count them). total is the number in the file.
// resourceType (e.g. "aws.ec2") picks the id field; it may be empty.
func ReadResources(path, resourceType string, limit int) (res []Resource, total int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = f.Close() }()
	fail := func(err error) ([]Resource, int, error) {
		return nil, 0, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}

	dec := json.NewDecoder(bufio.NewReaderSize(f, 1<<20))
	if tok, err := dec.Token(); err != nil {
		return fail(err)
	} else if d, ok := tok.(json.Delim); !ok || d != '[' {
		return fail(errors.New("not a JSON list"))
	}
	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return fail(err)
		}
		total++
		if len(res) < limit {
			res = append(res, newResource(resourceType, raw))
		}
	}
	if _, err := dec.Token(); err != nil { // the closing ]: catches truncated files
		return fail(err)
	}
	return res, total, nil
}

// newResource reads only the top level of a resource: enough for its id
// and tags, cheap even for big resources.
func newResource(resourceType string, raw json.RawMessage) Resource {
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields) // non-objects just get no id
	return Resource{ID: resourceID(resourceType, fields), Raw: raw, Tags: resourceTags(fields)}
}

// str is a field's value if it is a string.
func str(fields map[string]json.RawMessage, key string) string {
	var v string
	if raw, ok := fields[key]; ok && json.Unmarshal(raw, &v) == nil {
		return v
	}
	return ""
}

// idFields maps resource types to the field that identifies them. Types not
// listed fall back to common names (SPEC §7: resource-id extraction is a
// table, nothing may assume AWS).
var idFields = map[string]string{
	"aws.ec2":            "InstanceId",
	"aws.s3":             "Name",
	"aws.ebs":            "VolumeId",
	"aws.ebs-snapshot":   "SnapshotId",
	"aws.ami":            "ImageId",
	"aws.security-group": "GroupId",
	"aws.vpc":            "VpcId",
	"aws.subnet":         "SubnetId",
	"aws.eni":            "NetworkInterfaceId",
	"aws.network-addr":   "AllocationId",
	"aws.iam-role":       "RoleName",
	"aws.iam-user":       "UserName",
	"aws.iam-policy":     "PolicyName",
	"aws.rds":            "DBInstanceIdentifier",
	"aws.rds-snapshot":   "DBSnapshotIdentifier",
	"aws.lambda":         "FunctionName",
	"aws.dynamodb-table": "TableName",
	"aws.sqs":            "QueueUrl",
	"aws.sns":            "TopicArn",
	"aws.kms-key":        "KeyId",
	"aws.log-group":      "logGroupName",
	"aws.asg":            "AutoScalingGroupName",
	"aws.elb":            "LoadBalancerName",
	"aws.app-elb":        "LoadBalancerName",
	"aws.ecr":            "repositoryName",
	"aws.efs":            "FileSystemId",
	"aws.cloudtrail":     "Name",
}

var fallbackIDFields = []string{"id", "Id", "ID", "name", "Name", "Arn", "arn", "selfLink"}

func resourceID(resourceType string, fields map[string]json.RawMessage) string {
	if fields == nil {
		return "?"
	}
	if resourceType != "" && !strings.Contains(resourceType, ".") {
		resourceType = "aws." + resourceType // c7n also accepts "ec2" for "aws.ec2"
	}
	if f, ok := idFields[resourceType]; ok {
		if v := str(fields, f); v != "" {
			return v
		}
	}
	for _, f := range fallbackIDFields {
		if v := str(fields, f); v != "" {
			return v
		}
	}
	// Any field ending in Id, Name or Arn, in stable order.
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, suffix := range []string{"Id", "Name", "Arn"} {
		for _, k := range keys {
			if v := str(fields, k); v != "" && strings.HasSuffix(k, suffix) {
				return v
			}
		}
	}
	return "?"
}

// resourceTags reads AWS-style Tags ([{Key, Value}]) or map-style tags /
// labels (Azure, GCP).
func resourceTags(fields map[string]json.RawMessage) []Tag {
	var tags []Tag
	var list []struct{ Key, Value string }
	if raw, ok := fields["Tags"]; ok && json.Unmarshal(raw, &list) == nil {
		for _, t := range list {
			tags = append(tags, Tag{t.Key, t.Value})
		}
	}
	for _, key := range []string{"tags", "labels"} {
		var m map[string]any
		if raw, ok := fields[key]; ok && json.Unmarshal(raw, &m) == nil {
			for k, v := range m {
				s, _ := v.(string)
				tags = append(tags, Tag{k, s})
			}
		}
	}
	sort.SliceStable(tags, func(i, j int) bool { return tags[i].Key < tags[j].Key })
	return tags
}

// ErrNoOutput is returned when a dir has no c7n output in it.
var ErrNoOutput = errors.New("no c7n output found (expected <dir>/<policy>/metadata.json)")

// specActions lists the action types of a policy spec as JSON. An action
// is either a string ("delete") or an object with a type.
func specActions(spec json.RawMessage) []string {
	var p struct {
		Actions []json.RawMessage `json:"actions"`
	}
	if json.Unmarshal(spec, &p) != nil {
		return nil
	}
	var out []string
	for _, a := range p.Actions {
		var name string
		if json.Unmarshal(a, &name) == nil {
			out = append(out, name)
			continue
		}
		var obj struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(a, &obj) == nil && obj.Type != "" {
			out = append(out, obj.Type)
		} else {
			out = append(out, "?")
		}
	}
	return out
}
