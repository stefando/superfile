package gitinfo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestFindRootAndRelToRoot(t *testing.T) {
	tmp := t.TempDir()

	plain := filepath.Join(tmp, "plain")
	plainSub := filepath.Join(plain, "sub", "deep")
	worktree := filepath.Join(tmp, "worktree")
	worktreeSub := filepath.Join(worktree, "a")
	outer := filepath.Join(tmp, "outer")
	submodule := filepath.Join(outer, "mod")
	submoduleSub := filepath.Join(submodule, "src")
	outside := filepath.Join(tmp, "outside")

	utils.SetupDirectories(t, plainSub, worktreeSub, submoduleSub, outside,
		filepath.Join(plain, ".git"), filepath.Join(outer, ".git"))
	utils.SetupFiles(t, filepath.Join(worktree, ".git"), filepath.Join(submodule, ".git"))

	tests := []struct {
		name     string
		path     string
		wantRoot string
		wantRel  string
		wantOk   bool
	}{
		{"plain repo root", plain, plain, ".", true},
		{"plain repo subdir", plainSub, plain, filepath.Join("sub", "deep"), true},
		{"linked worktree (.git file)", worktreeSub, worktree, "a", true},
		{"submodule wins over outer repo", submoduleSub, submodule, "src", true},
		{"outer repo outside submodule", outer, outer, ".", true},
		{"not in a repo", outside, "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, ok := FindRoot(tt.path)
			assert.Equal(t, tt.wantOk, ok)
			assert.Equal(t, tt.wantRoot, root)

			rel, ok := RelToRoot(tt.path)
			assert.Equal(t, tt.wantOk, ok)
			assert.Equal(t, tt.wantRel, rel)
		})
	}
}

func TestReadMetadata(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantOK  bool
	}{
		{"empty", "", true},
		{"normal", "ref: refs/heads/main\n", true},
		{"at size limit", strings.Repeat("a", maxMetadataSize), true},
		{"over size limit", strings.Repeat("a", maxMetadataSize+1), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "metadata")
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0600))
			data, ok := readMetadata(path)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.content, string(data))
			} else {
				assert.Nil(t, data)
			}
		})
	}

	t.Run("directory", func(t *testing.T) {
		_, ok := readMetadata(t.TempDir())
		assert.False(t, ok)
	})
	t.Run("missing", func(t *testing.T) {
		_, ok := readMetadata(filepath.Join(t.TempDir(), "missing"))
		assert.False(t, ok)
	})
}

func TestBranchMetadataReadLimits(t *testing.T) {
	for _, metadata := range []string{".git", "HEAD"} {
		t.Run(metadata, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "repo")
			gitDir := filepath.Join(root, ".git")
			if metadata == ".git" {
				gitDir = filepath.Join(t.TempDir(), "gitdir")
				utils.SetupDirectories(t, root)
				utils.SetupFilesWithData(t,
					[]byte("gitdir: "+gitDir+strings.Repeat("\n", maxMetadataSize)), filepath.Join(root, ".git"))
			}
			utils.SetupDirectories(t, gitDir)
			head := "ref: refs/heads/main\n"
			if metadata == "HEAD" {
				head += strings.Repeat("\n", maxMetadataSize)
			}
			utils.SetupFilesWithData(t, []byte(head), filepath.Join(gitDir, "HEAD"))

			name, detached, ok := Branch(root)
			assert.Empty(t, name)
			assert.False(t, detached)
			assert.False(t, ok)
		})
	}
}

func TestBranch(t *testing.T) {
	tmp := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		utils.SetupDirectories(t, filepath.Dir(path))
		utils.SetupFilesWithData(t, []byte(content), path)
	}

	normal := filepath.Join(tmp, "normal")
	write(filepath.Join(normal, ".git", "HEAD"), "ref: refs/heads/feat/x\n")

	detached := filepath.Join(tmp, "detached")
	write(filepath.Join(detached, ".git", "HEAD"), "0123456789abcdef0123456789abcdef01234567\n")

	// Linked worktree: .git file with absolute gitdir, and one with a relative gitdir
	wtGitDir := filepath.Join(tmp, "main", ".git", "worktrees", "wt")
	write(filepath.Join(wtGitDir, "HEAD"), "ref: refs/heads/wt-branch\n")
	worktree := filepath.Join(tmp, "worktree")
	write(filepath.Join(worktree, ".git"), "gitdir: "+wtGitDir+"\n")
	relWorktree := filepath.Join(tmp, "relwt")
	write(filepath.Join(relWorktree, ".git"), "gitdir: ../main/.git/worktrees/wt\n")

	garbled := filepath.Join(tmp, "garbled")
	write(filepath.Join(garbled, ".git", "HEAD"), "xx")
	noHead := filepath.Join(tmp, "nohead")
	utils.SetupDirectories(t, filepath.Join(noHead, ".git"))
	badFile := filepath.Join(tmp, "badfile")
	write(filepath.Join(badFile, ".git"), "not a gitdir pointer")
	outside := filepath.Join(tmp, "outside")
	utils.SetupDirectories(t, outside)

	tests := []struct {
		name         string
		path         string
		wantName     string
		wantDetached bool
		wantOk       bool
	}{
		{"normal branch", normal, "feat/x", false, true},
		{"detached HEAD", detached, "0123456", true, true},
		{"worktree absolute gitdir", worktree, "wt-branch", false, true},
		{"worktree relative gitdir", relWorktree, "wt-branch", false, true},
		{"garbled HEAD", garbled, "", false, false},
		{"missing HEAD", noHead, "", false, false},
		{".git file without gitdir", badFile, "", false, false},
		{"not in a repo", outside, "", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, detached, ok := Branch(tt.path)
			assert.Equal(t, tt.wantOk, ok)
			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantDetached, detached)
		})
	}
}
