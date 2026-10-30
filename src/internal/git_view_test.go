package internal

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestToggleGitView(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	sub := filepath.Join(repo, "sub")
	plain := filepath.Join(tmp, "plain")
	utils.SetupDirectories(t, filepath.Join(repo, ".git"), sub, plain)
	utils.SetupFilesWithData(t, []byte("ref: refs/heads/mybranch\n"), filepath.Join(repo, ".git", "HEAD"))

	origNerdfont := common.Config.Nerdfont
	common.Config.Nerdfont = false
	t.Cleanup(func() { common.Config.Nerdfont = origNerdfont })

	view := func(m *model) string {
		TeaUpdate(m, tea.WindowSizeMsg{Width: 100, Height: 30})
		return m.View().Content
	}
	toggle := utils.TeaRuneKeyMsg(common.Hotkeys.ToggleGitView[0])

	t.Run("in repo: plain view has marker only, alt view has branch and relative path", func(t *testing.T) {
		m := defaultTestModel(sub)
		out := view(m)
		assert.Contains(t, out, "(git)")
		assert.NotContains(t, out, "mybranch")

		TeaUpdate(m, toggle)
		out = view(m)
		assert.Contains(t, out, "mybranch")
		assert.NotContains(t, out, tmp)

		TeaUpdate(m, toggle)
		assert.Contains(t, view(m), "(git)")
	})

	t.Run("outside repo: no marker and toggle does nothing", func(t *testing.T) {
		m := defaultTestModel(plain)
		TeaUpdate(m, toggle)
		out := view(m)
		assert.NotContains(t, out, "(git)")
		assert.NotContains(t, out, "mybranch")
	})
}
