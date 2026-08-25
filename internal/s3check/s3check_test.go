package s3check

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ojarosch/tfdoctor/internal/analyze"
)

func TestVersioningResult(t *testing.T) {
	if r := versioningResult(&s3.GetBucketVersioningOutput{
		Status: types.BucketVersioningStatusEnabled}); r.Status != analyze.Pass {
		t.Errorf("enabled versioning should pass, got %s", r.Status)
	}
	for _, status := range []types.BucketVersioningStatus{types.BucketVersioningStatusSuspended, ""} {
		if r := versioningResult(&s3.GetBucketVersioningOutput{Status: status}); r.Status != analyze.Fail {
			t.Errorf("status %q should fail, got %s", status, r.Status)
		}
	}
}

func TestEncryptionResult(t *testing.T) {
	noRules := &s3.GetBucketEncryptionOutput{}
	if r := encryptionResult(noRules); r.Status != analyze.Fail {
		t.Errorf("no rules should fail, got %s", r.Status)
	}
	withRules := &s3.GetBucketEncryptionOutput{
		ServerSideEncryptionConfiguration: &types.ServerSideEncryptionConfiguration{
			Rules: []types.ServerSideEncryptionRule{{}},
		},
	}
	if r := encryptionResult(withRules); r.Status != analyze.Pass {
		t.Errorf("rules present should pass, got %s", r.Status)
	}
}

func TestPublicAccessBlockResult(t *testing.T) {
	all := types.PublicAccessBlockConfiguration{
		BlockPublicAcls: aws.Bool(true), BlockPublicPolicy: aws.Bool(true),
		IgnorePublicAcls: aws.Bool(true), RestrictPublicBuckets: aws.Bool(true),
	}
	if r := publicAccessBlockResult(&s3.GetPublicAccessBlockOutput{
		PublicAccessBlockConfiguration: &all}); r.Status != analyze.Pass {
		t.Errorf("full PAB should pass, got %s", r.Status)
	}
	partial := all
	partial.BlockPublicPolicy = aws.Bool(false)
	if r := publicAccessBlockResult(&s3.GetPublicAccessBlockOutput{
		PublicAccessBlockConfiguration: &partial}); r.Status != analyze.Fail {
		t.Errorf("partial PAB should fail, got %s", r.Status)
	}
}

func TestTLSOnlyResult(t *testing.T) {
	good := &s3.GetBucketPolicyOutput{Policy: aws.String(
		`{"Statement":[{"Effect":"Deny","Condition":{"Bool":{"aws:SecureTransport":"false"}}}]}`)}
	if r := tlsOnlyResult(good); r.Status != analyze.Pass {
		t.Errorf("deny policy should pass, got %s", r.Status)
	}
	bad := &s3.GetBucketPolicyOutput{}
	if r := tlsOnlyResult(bad); r.Status != analyze.Warn {
		t.Errorf("missing policy should warn, got %s", r.Status)
	}
	allowOnly := &s3.GetBucketPolicyOutput{Policy: aws.String(
		`{"Statement":[{"Effect":"Allow","Action":"s3:*"}]}`)}
	if r := tlsOnlyResult(allowOnly); r.Status != analyze.Warn {
		t.Errorf("policy without deny should warn, got %s", r.Status)
	}
}

func TestCheckWithoutS3Backend(t *testing.T) {
	repo := &analyze.Repo{}
	out := Check(t.Context(), repo)
	if len(out) != 1 || out[0].Status != analyze.Info {
		t.Fatalf("expected single info result, got %+v", out)
	}
}
