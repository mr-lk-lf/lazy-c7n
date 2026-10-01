package app

import (
	"net/url"
	"strings"
)

// cloudContext says, from the environment custodian will inherit, which
// account and region it is about to act on: AWS profile (or env keys),
// region, a custom endpoint (an emulator), Azure subscription, GCP
// project. Only names and ids are read; credential values never are.
// region is the configured default region, which lazyc7n passes as -r.
func cloudContext(environ []string, region string) string {
	env := map[string]string{}
	for _, kv := range environ {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env[k] = v
		}
	}
	first := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(env[k]); v != "" {
				return v
			}
		}
		return ""
	}

	var aws []string
	switch {
	case first("AWS_PROFILE", "AWS_DEFAULT_PROFILE") != "":
		aws = append(aws, "profile "+first("AWS_PROFILE", "AWS_DEFAULT_PROFILE"))
	case env["AWS_ACCESS_KEY_ID"] != "":
		aws = append(aws, "env keys")
	default:
		aws = append(aws, "default credentials")
	}
	if region == "" {
		region = first("AWS_REGION", "AWS_DEFAULT_REGION")
	}
	if region != "" {
		aws = append(aws, region)
	}
	if ep := first("AWS_ENDPOINT_URL"); ep != "" {
		host := ep
		if u, err := url.Parse(ep); err == nil && u.Host != "" {
			host = u.Host
		}
		aws = append(aws, "endpoint "+host)
	}
	parts := []string{"aws " + strings.Join(aws, " · ")}

	if sub := first("AZURE_SUBSCRIPTION_ID"); sub != "" {
		parts = append(parts, "azure "+sub)
	}
	if project := first("GOOGLE_CLOUD_PROJECT", "CLOUDSDK_CORE_PROJECT"); project != "" {
		parts = append(parts, "gcp "+project)
	}
	return strings.Join(parts, " | ")
}
