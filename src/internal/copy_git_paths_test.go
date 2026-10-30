package internal

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestCopyPathRelativeToGitRootNestedRepositories(t *testing.T) {
	for _, gitFile := range []bool{false, true} {
		name := "nested repository"
		if gitFile {
			name = "submodule"
		}
		t.Run(name, func(t *testing.T) {
			repo := t.TempDir()
			sub := filepath.Join(repo, "sub")
			nested := filepath.Join(sub, "module")
			file := filepath.Join(sub, "file.txt")
			utils.SetupDirectories(t, filepath.Join(repo, ".git"), nested)
			utils.SetupFiles(t, file, filepath.Join(nested, "inside.txt"))
			if gitFile {
				gitDir := filepath.Join(repo, ".git", "modules", "module")
				utils.SetupDirectories(t, gitDir)
				utils.SetupFilesWithData(t, []byte("ref: refs/heads/main\n"), filepath.Join(gitDir, "HEAD"))
				utils.SetupFilesWithData(t, []byte("gitdir: "+gitDir+"\n"), filepath.Join(nested, ".git"))
			} else {
				utils.SetupDirectories(t, filepath.Join(nested, ".git"))
			}

			m := defaultTestModel(sub)
			var copied string
			m.clipboardWriter = func(text string) error {
				copied = text
				return nil
			}
			key := utils.TeaRuneKeyMsg(common.Hotkeys.CopyPathRelativeToGitRoot[0])
			panel := m.getFocusedFilePanel()
			setFilePanelSelectedItemByLocation(t, panel, nested)
			TeaUpdate(m, key)
			assert.Equal(t, filepath.Join("sub", "module"), copied)

			panel.ChangeFilePanelMode()
			panel.SetSelected(nested)
			panel.SetSelected(file)
			TeaUpdate(m, key)
			assert.Equal(t, strings.Join([]string{
				filepath.Join("sub", "module"), filepath.Join("sub", "file.txt"),
			}, "\n"), copied)
		})
	}
}

func TestCopyPathRelativeToGitRootWithNewlines(t *testing.T) {
	for _, repoName := range []string{"repo", "repo\nname"} {
		t.Run(repoName, func(t *testing.T) {
			repo := filepath.Join(t.TempDir(), repoName)
			sub := filepath.Join(repo, "sub\nfolder")
			file1 := filepath.Join(sub, "a\nfile.txt")
			file2 := filepath.Join(sub, "b\nfile.txt")
			utils.SetupDirectories(t, filepath.Join(repo, ".git"), sub)
			utils.SetupFiles(t, file1, file2)

			m := defaultTestModel(sub)
			var copied string
			m.clipboardWriter = func(text string) error {
				copied = text
				return nil
			}
			key := utils.TeaRuneKeyMsg(common.Hotkeys.CopyPathRelativeToGitRoot[0])
			panel := m.getFocusedFilePanel()
			setFilePanelSelectedItemByLocation(t, panel, file1)
			TeaUpdate(m, key)
			assert.Equal(t, filepath.Join("sub\nfolder", "a\nfile.txt"), copied)

			panel.ChangeFilePanelMode()
			panel.SetSelected(file2)
			panel.SetSelected(file1)
			TeaUpdate(m, key)
			assert.Equal(t, strings.Join([]string{
				filepath.Join("sub\nfolder", "a\nfile.txt"), filepath.Join("sub\nfolder", "b\nfile.txt"),
			}, "\n"), copied)
		})
	}
}
