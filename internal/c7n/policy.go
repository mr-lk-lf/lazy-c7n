// Package c7n holds lazyc7n's view of Cloud Custodian data: policies, run
// output and the safety classification of actions. It never evaluates
// policies itself; that is always custodian's job.
package c7n

// Policy is the part of a c7n policy that lazyc7n needs to show and to
// decide how careful a run must be. Unknown policy fields are ignored.
type Policy struct {
	Name     string
	Resource string // e.g. "aws.ec2"
	Mode     string // mode.type; "" means pull
	Actions  []string
	File     string // policy file it was read from
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
