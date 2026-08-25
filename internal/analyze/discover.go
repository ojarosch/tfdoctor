// Package analyze discovers and models a Terraform/OpenTofu repository so
// that rules operate on a pre-computed view instead of touching the
// filesystem themselves.
package analyze

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SkipDirs is the centralized list of directories excluded from all
// recursive scans.
var SkipDirs = map[string]bool{
	".git":         true,
	".terraform":   true,
	"node_modules": true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
}

// Status is the severity of a diagnostic result.
type Status string

const (
	Pass Status = "pass"
	Warn Status = "warn"
	Fail Status = "fail"
	Info Status = "info"
)

// Result is a single diagnostic outcome.
type Result struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Status      Status `json:"status"`
	Title       string `json:"title"`
	Description string `json:"description"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
}

// Context is what every rule receives: an already-discovered repository.
type Context struct {
	Repo *Repo
}

// ProviderRef is a required_providers entry.
type ProviderRef struct {
	Name    string
	Source  string // empty if omitted
	Version string // empty if no constraint
	File    string
	Line    int
}

// ModuleRef is a module block in a root module.
type ModuleRef struct {
	Name    string
	Source  string
	Version string
	File    string
	Line    int
}

// Backend is a configured state backend.
type Backend struct {
	Type   string
	Bucket string // literal bucket attr (s3), empty when dynamic
	Region string // literal region attr (s3)
	File   string
	Line   int
}

// TFFile is one parsed .tf file.
type TFFile struct {
	Path                string // repo-relative, slash-separated
	Raw                 string // full file content, for text-level rules
	RequiredVersion     string
	RequiredVersionLine int
	Providers           []ProviderRef
	Modules             []ModuleRef
	Backends            []Backend
}

// PinInfo is a detected runtime version pin.
type PinInfo struct {
	Tool          string // terraform | tofu
	Version       string
	File          string
	Deterministic bool // false for e.g. "latest"
}

// FoundFile is a file found on disk with optional git tracking info.
type FoundFile struct {
	Path    string // repo-relative
	Tracked bool   // meaningful only when GitAvailable
}

// TextFile is a CI or other config file kept as raw text.
type TextFile struct {
	Path    string
	Content string
}

// Repo is the discovered model of a repository.
type Repo struct {
	Path          string // absolute
	Rel           string // as given on the command line
	TFFiles       []*TFFile
	Pins          []PinInfo
	Gitignore     []string // non-comment lines from repo-root and scan-root .gitignore
	GitAvailable  bool
	Tracked       map[string]bool
	Lockfile      string // repo-relative path if .terraform.lock.hcl exists
	StateFiles    []FoundFile
	TfvarsFiles   []FoundFile
	CIFiles       []TextFile
}

func (r *Repo) hasTerraformSignal() bool {
	for _, p := range r.Pins {
		if p.Tool == "terraform" {
			return true
		}
	}
	for _, f := range r.TFFiles {
		if f.RequiredVersion != "" || len(f.Backends) > 0 {
			return true
		}
	}
	return false
}

func (r *Repo) hasTofuSignal() bool {
	for _, p := range r.Pins {
		if p.Tool == "tofu" {
			return true
		}
	}
	return false
}

// Discover walks the repository at path and builds the Repo model.
func Discover(path string) (*Repo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot access %s: %w", path, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	repo := &Repo{Path: abs, Rel: filepath.Clean(path)}
	if gr := findGitRoot(abs); gr != "" && gr != abs {
		readLines(filepath.Join(gr, ".gitignore"), &repo.Gitignore)
	}

	var tfPaths []string
	err = filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != abs && SkipDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(abs, p)
		rel = filepath.ToSlash(rel)
		switch {
		case strings.HasSuffix(name, ".tf"):
			tfPaths = append(tfPaths, rel)
		case name == ".terraform.lock.hcl" && repo.Lockfile == "":
			repo.Lockfile = rel
		case name == ".gitignore" && rel == ".gitignore":
			readLines(p, &repo.Gitignore)
		case name == ".terraform-version" || name == ".opentofu-version" ||
			name == ".tool-versions" || name == "mise.toml" || name == "mise.local.toml":
			collectPin(rel, p, &repo.Pins)
		case isStateFile(name):
			repo.StateFiles = append(repo.StateFiles, FoundFile{Path: rel})
		case strings.HasSuffix(name, ".tfvars"):
			repo.TfvarsFiles = append(repo.TfvarsFiles, FoundFile{Path: rel})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, rel := range tfPaths {
		f, err := parseTF(abs, rel)
		if err != nil {
			continue // unparsable files are skipped; rules work with the rest
		}
		repo.TFFiles = append(repo.TFFiles, f)
	}

	repo.Tracked, repo.GitAvailable = lsFiles(abs)

	markTracked(repo.StateFiles, repo)
	markTracked(repo.TfvarsFiles, repo)

	repo.CIFiles = discoverCI(abs)
	return repo, nil
}

// findGitRoot walks up from dir to the closest ancestor containing .git.
// Returns "" when dir is not inside a git repository.
func findGitRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func readLines(p string, out *[]string) {
	data, err := os.ReadFile(p)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		*out = append(*out, line)
	}
}

func markTracked(files []FoundFile, repo *Repo) {
	for i := range files {
		files[i].Tracked = repo.Tracked[files[i].Path]
	}
}

func isStateFile(name string) bool {
	return strings.HasSuffix(name, ".tfstate") || strings.Contains(name, ".tfstate.")
}
