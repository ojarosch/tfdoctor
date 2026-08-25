# tfdoctor

> [!NOTE]
> **macOS users:** tfdoctor is not signed with an Apple Developer certificate
> and therefore not notarized. The Homebrew cask removes the quarantine flag
> automatically, but if you download a binary manually, macOS Gatekeeper may
> block it on first run. Unblock it with:
>
> ```bash
> xattr -dr com.apple.quarantine ./tfdoctor
> ```

**tfdoctor** inspects a Terraform or OpenTofu repository and answers one question:

> Is this Infrastructure-as-Code repository itself engineered well?

It is inspired by tools like `brew doctor`: run it from the root of your
infrastructure repository and get an honest, opinionated health report about
repository hygiene, reproducibility, and execution conventions.

```bash
tfdoctor
tfdoctor ./path/to/repository
```

## What is tfdoctor?

Infrastructure teams have excellent tooling for checking *what* their code
deploys. tfdoctor checks whether the **repository itself** would be pleasant
and safe to inherit:

- Can this repository be reproduced reliably?
- Is the Terraform/OpenTofu version controlled?
- Are provider and module dependencies pinned?
- Is state handled safely?
- Are generated artifacts kept out of version control?
- Does CI execute Terraform/OpenTofu predictably?

tfdoctor is deterministic, fast, offline, and safe to run on arbitrary
repositories. It exits non-zero when failures are present, so you can drop it
straight into CI.

## What tfdoctor is not

tfdoctor does not replace:

- Checkov
- Trivy
- tfsec
- TFLint
- Terraform validate
- OpenTofu validate

Those tools answer different questions. tfdoctor does **not** scan cloud
resources for security problems, detect secrets, execute Terraform, or talk to
cloud APIs. It focuses on the engineering quality of the repository and its
execution conventions.

## Installation

Homebrew (via the tap, published automatically on each release):

```bash
brew tap ojarosch/tap https://github.com/ojarosch/homebrew-tap
brew install tfdoctor
```

Or with `go install`:

```bash
go install github.com/ojarosch/tfdoctor/cmd/tfdoctor@latest
```

Or download a prebuilt binary from [GitHub Releases](https://github.com/ojarosch/tfdoctor/releases)
(linux/darwin/windows, amd64/arm64).

Build from source:

```bash
git clone https://github.com/ojarosch/tfdoctor
cd tfdoctor && go build -o tfdoctor ./cmd/tfdoctor
```

## Usage

```bash
tfdoctor [path]            # analyze a repository (default: current directory)
tfdoctor --format json     # machine-readable output
tfdoctor --format text     # human-readable output (default)
tfdoctor --check-s3-backend  # also inspect the live S3 state bucket (opt-in, needs AWS credentials)
tfdoctor --version
tfdoctor --help
```

Exit codes:

| Code | Meaning                                    |
|------|--------------------------------------------|
| 0    | no failures                                |
| 1    | one or more FAIL diagnostics               |
| 2    | tfdoctor could not execute correctly       |

Warnings never cause a non-zero exit by themselves.

## Example output

```text
tfdoctor ./infra

Runtime
✓ OpenTofu version pinned: tofu 1.11.2 (.opentofu-version)
✓ required_version defined: >= 1.10, < 2.0

Providers
✓ .terraform.lock.hcl present
✓ All 3 provider(s) have version constraints

Modules
✓ 4 registry module(s) pinned

Repository
✓ .terraform/ ignored
✗ State files are not ignored: Add *.tfstate and *.tfstate.* to .gitignore

CI
✓ Interactive input disabled for plan/apply

──────────────────────────

12 passed
1 warnings
1 failures
2 info
```

## CI usage

```yaml
# GitHub Actions
- uses: actions/setup-go@v5
  with:
    go-version: stable
- run: go install github.com/ojarosch/tfdoctor/cmd/tfdoctor@latest
- run: tfdoctor .
```

GitLab CI works the same way; any environment with Go (or a downloaded binary)
can run tfdoctor.

## Rule reference

### Runtime

| Rule ID                   | Severity | Check                                                            |
|---------------------------|----------|------------------------------------------------------------------|
| `runtime.detect`          | info     | Detects Terraform / OpenTofu / ambiguous / none                  |
| `runtime.version-pinned`  | warn     | `.terraform-version`, `.opentofu-version`, `.tool-versions`, mise.toml pin present (`latest` does not count) |
| `runtime.required-version`| warn     | `required_version` defined in a root `terraform` block           |

### Providers

| Rule ID                         | Severity | Check                                                        |
|---------------------------------|----------|--------------------------------------------------------------|
| `providers.lockfile-present`    | warn     | `.terraform.lock.hcl` exists when providers are referenced   |
| `providers.version-constraints` | warn     | Every provider has a `version` constraint (one result each)  |
| `providers.source-explicit`     | warn     | Every non-builtin provider declares an explicit `source`     |

### Modules

| Rule ID                       | Severity | Check                                                       |
|-------------------------------|----------|-------------------------------------------------------------|
| `modules.remote-version-pinned` | warn   | Registry modules declare a `version`                        |
| `modules.git-ref-pinned`        | warn   | Git sources use a tag or commit SHA `?ref=` (not a branch)  |

### Repository

| Rule ID                                  | Severity | Check                                              |
|------------------------------------------|----------|----------------------------------------------------|
| `repository.terraform-directory-ignored` | warn     | `.terraform/` appears in `.gitignore`              |
| `repository.state-files-ignored`         | fail     | `*.tfstate` / `*.tfstate.*` patterns ignored       |
| `repository.state-file-present`          | fail     | No state files exist in the working tree           |
| `repository.tfvars-sensitive-files`      | warn     | tfvars files that are not ignored are flagged      |

### Backend

| Rule ID         | Severity | Check                                        |
|-----------------|----------|----------------------------------------------|
| `backend.detect`| info     | Reports configured backend type (local/default if none) |

With `--check-s3-backend`, tfdoctor additionally queries the live S3 bucket from
the `s3` backend block (uses the standard AWS credential chain; the backend's
literal `region` attribute is honored). Unreachable buckets or missing
permissions degrade to warnings, never failures.

| Rule ID                        | Severity | Check                                              |
|--------------------------------|----------|----------------------------------------------------|
| `backend.s3-versioning`        | fail     | Bucket versioning is enabled                       |
| `backend.s3-encryption`        | fail     | A default server-side encryption rule exists       |
| `backend.s3-public-access-block` | fail   | All four Block Public Access settings are enabled  |
| `backend.s3-tls-only`          | warn     | Bucket policy denies requests without TLS          |
| `backend.s3-inspect`           | info/warn| Emitted when the bucket cannot be inspected        |

### IAM

Scans `.tf` files for IAM trust policies federated against
`token.actions.githubusercontent.com` (GitHub OIDC). GitHub now embeds owner/repo
IDs in the subject (`repo:owner@12345/repo@67890:environment:prod`), so policies
matching only the legacy `repo:owner/repo:...` format silently stop authorizing.

| Rule ID                          | Severity | Check                                                     |
|----------------------------------|----------|-----------------------------------------------------------|
| `iam.github-oidc-legacy-subject` | warn     | `:sub` conditions use only legacy subjects without wildcards |

Remediation: add ID-embedded subject variants alongside the legacy ones, or
switch the condition to `StringLike` with a wildcard. To find out the real
subject GitHub mints for you, inspect CloudTrail:

```bash
aws cloudtrail lookup-events \
  --lookup-attributes AttributeKey=EventName,AttributeValue=AssumeRoleWithWebIdentity
```

The `Username` field of a denied call shows the exact subject string.

### CI

Supports `.github/workflows/*.yml|.yaml` and `.gitlab-ci.yml`.

| Rule ID                 | Severity | Check                                                          |
|-------------------------|----------|----------------------------------------------------------------|
| `ci.input-disabled`     | warn     | plan/apply in CI use `-input=false` or `TF_INPUT=false`        |
| `ci.automation-env`     | warn     | `TF_IN_AUTOMATION=true` set when TF runs in CI                 |
| `ci.apply-auto-approve` | info     | `-auto-approve` apply is surfaced for explicit awareness       |

## Development

```bash
go test ./...
go vet ./...
go build ./cmd/tfdoctor
./tfdoctor testdata/healthy-opentofu   # try it on a fixture
```

Fixture repositories live in `testdata/`. Tests do not require network access.
Rules operate on a pre-discovered repository model (`internal/analyze`), so
adding a rule means adding one function and registering it in `internal/rules`.

Releases are cut by pushing a `v*` tag; [.goreleaser.yaml](.goreleaser.yaml)
builds all platforms, publishes the GitHub Release, and updates the
[ojarosch/homebrew-tap](https://github.com/ojarosch/homebrew-tap) cask. The
workflow needs a `TAP_GITHUB_TOKEN` secret (a fine-grained PAT with push access
to the tap repository). Test releases locally with:

```bash
goreleaser release --snapshot --clean --skip=publish
```

## License

[Apache License 2.0](LICENSE)
