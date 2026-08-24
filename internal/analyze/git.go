package analyze

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// lsFiles runs `git ls-files` in dir. Returns a set of repo-relative tracked
// paths. Never modifies git state; degrades gracefully when git is missing
// or dir is not a repository.
func lsFiles(dir string) (map[string]bool, bool) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	tracked := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			tracked[filepath.ToSlash(filepath.Clean(line))] = true
		}
	}
	return tracked, true
}

func (r *Repo) isTracked(rel string) bool {
	if !r.GitAvailable {
		return false
	}
	return r.Tracked[rel]
}
