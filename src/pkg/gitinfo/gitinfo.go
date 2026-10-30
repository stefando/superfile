// Package gitinfo detects git repositories without running the git binary.
package gitinfo

import (
	"os"
	"path/filepath"
	"strings"
)

// FindRoot walks up from path and returns the first directory that contains a ".git" entry.
// The entry can be a directory (regular repo) or a file (linked worktree or submodule).
// The walk is lexical, symlinks are not resolved, so results stay consistent with panel locations.
func FindRoot(path string) (string, bool) {
	dir := filepath.Clean(path)
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// RelToRoot returns path relative to the root of the git repo that contains it.
// The repo root itself gives ".". If path is not in a repo, ok is false.
func RelToRoot(path string) (string, bool) {
	root, ok := FindRoot(path)
	if !ok {
		return "", false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", false
	}
	return rel, true
}

const (
	headRefPrefix = "ref: refs/heads/"
	gitDirPrefix  = "gitdir:"
	shortHashLen  = 7
)

// Branch returns the current branch name of the repo that contains path.
// Results are the name, whether HEAD is detached, and ok. A detached HEAD gives the short commit hash.
// ok is false if path is not in a repo or HEAD can't be read.
func Branch(path string) (string, bool, bool) {
	root, found := FindRoot(path)
	if !found {
		return "", false, false
	}
	gitDir, found := resolveGitDir(root)
	if !found {
		return "", false, false
	}
	data, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return "", false, false
	}
	head := strings.TrimSpace(string(data))
	if branch, isRef := strings.CutPrefix(head, headRefPrefix); isRef {
		return branch, false, branch != ""
	}
	if len(head) < shortHashLen {
		return "", false, false
	}
	return head[:shortHashLen], true, true
}

// resolveGitDir returns the real git directory for a repo root.
// ".git" is either the directory itself or a file "gitdir: <path>" (linked worktree, submodule).
func resolveGitDir(root string) (string, bool) {
	dotGit := filepath.Join(root, ".git")
	info, err := os.Stat(dotGit)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		return dotGit, true
	}
	data, err := os.ReadFile(dotGit)
	if err != nil {
		return "", false
	}
	target, isGitDir := strings.CutPrefix(strings.TrimSpace(string(data)), gitDirPrefix)
	if !isGitDir {
		return "", false
	}
	target = strings.TrimSpace(target)
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	return target, true
}
