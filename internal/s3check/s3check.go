// Package s3check inspects the live S3 state bucket behind an s3 backend
// for security best practices. It is opt-in via --check-s3-backend and
// degrades to warn/info results when the bucket is unreachable.
package s3check

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ojarosch/tfdoctor/internal/analyze"
)

// Check runs all S3 backend checks against the first s3 backend found in
// the repository. Results use Category "Backend" so they group with
// backend.detect in the report.
func Check(ctx context.Context, repo *analyze.Repo) []analyze.Result {
	var b *analyze.Backend
	for _, f := range repo.TFFiles {
		for i := range f.Backends {
			if f.Backends[i].Type == "s3" {
				b = &f.Backends[i]
				break
			}
		}
	}
	if b == nil {
		return []analyze.Result{res(info("No S3 backend to inspect",
			"--check-s3-backend needs a literal s3 backend block"), "backend.s3-inspect")}
	}
	if b.Bucket == "" {
		return []analyze.Result{res(info("S3 bucket not inspectable",
			fmt.Sprintf("backend block at %s:%d has no literal bucket attribute", b.File, b.Line)),
			"backend.s3-inspect")}
	}

	cfgOpts := []func(*config.LoadOptions) error{}
	if b.Region != "" {
		cfgOpts = append(cfgOpts, config.WithRegion(b.Region))
	}
	cfg, err := config.LoadDefaultConfig(ctx, cfgOpts...)
	if err != nil {
		return []analyze.Result{res(warn("Could not load AWS configuration: "+err.Error(),
			"Check credentials (aws sso login / env vars) and try again"), "backend.s3-inspect")}
	}
	client := s3.NewFromConfig(cfg)

	bucket := aws.String(b.Bucket)
	out := []analyze.Result{}

	v, err := client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: bucket})
	out = append(out, checkResult("backend.s3-versioning", err,
		func() analyze.Result { return versioningResult(v) },
		"bucket versioning"))

	e, err := client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{Bucket: bucket})
	out = append(out, checkResult("backend.s3-encryption", err,
		func() analyze.Result { return encryptionResult(e) },
		"bucket encryption"))

	p, err := client.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: bucket})
	out = append(out, checkResult("backend.s3-public-access-block", err,
		func() analyze.Result { return publicAccessBlockResult(p) },
		"public access block"))

	pol, err := client.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: bucket})
	out = append(out, checkResult("backend.s3-tls-only", err,
		func() analyze.Result { return tlsOnlyResult(pol) },
		"bucket policy"))
	return out
}

// checkResult converts an API error into a warn; otherwise defers to the
// pure evaluation function.
func checkResult(id string, err error, eval func() analyze.Result, what string) analyze.Result {
	if err != nil {
		return res(warn(fmt.Sprintf("%s could not be checked: %s", what, err),
			"The bucket may not exist or the credentials may lack permission"), id)
	}
	r := eval()
	r.ID = id
	r.Category = "Backend"
	return r
}

// versioningResult requires bucket versioning so state history survives
// accidental deletion or corruption.
func versioningResult(v *s3.GetBucketVersioningOutput) analyze.Result {
	if v.Status == types.BucketVersioningStatusEnabled {
		return pass("State bucket versioning enabled", "")
	}
	return fail("State bucket versioning is not enabled",
		"Enable versioning on the state bucket to recover from deleted or corrupted state")
}

// encryptionResult requires a default encryption rule (any SSE algorithm).
func encryptionResult(e *s3.GetBucketEncryptionOutput) analyze.Result {
	if e.ServerSideEncryptionConfiguration != nil && len(e.ServerSideEncryptionConfiguration.Rules) > 0 {
		return pass("State bucket default encryption configured", "")
	}
	return fail("State bucket has no default encryption",
		"Configure a default server-side encryption rule on the state bucket")
}

// publicAccessBlockResult requires all four public access settings enabled.
func publicAccessBlockResult(p *s3.GetPublicAccessBlockOutput) analyze.Result {
	c := p.PublicAccessBlockConfiguration
	if aws.ToBool(c.BlockPublicAcls) && aws.ToBool(c.BlockPublicPolicy) &&
		aws.ToBool(c.IgnorePublicAcls) && aws.ToBool(c.RestrictPublicBuckets) {
		return pass("State bucket blocks all public access", "")
	}
	return fail("State bucket public access block is incomplete",
		"Enable all four Block Public Access settings on the state bucket")
}

// tlsOnlyResult heuristically checks for a deny statement on insecure
// transport. ponytail: substring scan of policy JSON instead of full IAM
// parsing; switch to policy decoding if false positives show up.
func tlsOnlyResult(p *s3.GetBucketPolicyOutput) analyze.Result {
	policy := aws.ToString(p.Policy)
	if strings.Contains(policy, "aws:SecureTransport") && strings.Contains(policy, `"Deny"`) {
		return pass("State bucket denies non-TLS access", "")
	}
	return warn("State bucket policy does not deny non-TLS access",
		`Add a bucket policy statement denying requests where "aws:SecureTransport" is false`)
}

func res(r analyze.Result, id string) analyze.Result {
	r.ID = id
	r.Category = "Backend"
	return r
}

func pass(title, desc string) analyze.Result {
	return analyze.Result{Status: analyze.Pass, Title: title, Description: desc}
}
func warn(title, desc string) analyze.Result {
	return analyze.Result{Status: analyze.Warn, Title: title, Description: desc}
}
func fail(title, desc string) analyze.Result {
	return analyze.Result{Status: analyze.Fail, Title: title, Description: desc}
}
func info(title, desc string) analyze.Result {
	return analyze.Result{Status: analyze.Info, Title: title, Description: desc}
}
