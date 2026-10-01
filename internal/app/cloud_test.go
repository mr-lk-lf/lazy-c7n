package app

import (
	"strings"
	"testing"
)

func TestCloudContext(t *testing.T) {
	cases := []struct {
		env    []string
		region string
		want   string
	}{
		{nil, "", "aws default credentials"},
		{[]string{"AWS_PROFILE=prod", "AWS_REGION=eu-west-1"}, "", "aws profile prod · eu-west-1"},
		{[]string{"AWS_DEFAULT_PROFILE=dev", "AWS_DEFAULT_REGION=us-east-1"}, "eu-west-3", "aws profile dev · eu-west-3"},
		{[]string{"AWS_ACCESS_KEY_ID=AKIAEXAMPLE", "AWS_SECRET_ACCESS_KEY=s3cr3t", "AWS_ENDPOINT_URL=http://localhost:4566"}, "",
			"aws env keys · endpoint localhost:4566"},
		{[]string{"AZURE_SUBSCRIPTION_ID=sub-1", "GOOGLE_CLOUD_PROJECT=proj"}, "", "aws default credentials | azure sub-1 | gcp proj"},
	}
	for _, c := range cases {
		got := cloudContext(c.env, c.region)
		if got != c.want {
			t.Errorf("%v: %q, want %q", c.env, got, c.want)
		}
		if strings.Contains(got, "AKIA") || strings.Contains(got, "s3cr3t") {
			t.Errorf("credential shown: %q", got)
		}
	}
}
