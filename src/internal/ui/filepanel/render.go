package filepanel

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"

	variable "github.com/yorukot/superfile/src/config"
	"github.com/yorukot/superfile/src/config/icon"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui"
	"github.com/yorukot/superfile/src/internal/ui/rendering"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
	"github.com/yorukot/superfile/src/pkg/utils"
)

/*
- TODO: Write File Panel Specific unit test
  - Individual panel resizes
  - Footer content of filepanel changes due to resizing
  - i Only mode icons remains on smaller
  - ii Other things that change too
  - Other panels like clipboard and metadata's content changes too on resize
*/
func (m *Model) Render(focused bool) string {
	r := ui.FilePanelRenderer(m.height, m.width, focused)

	m.renderTopBar(r)
	m.renderSearchBar(r)
	m.renderFooter(r, m.SelectedCount())
	if m.NeedRenderHeaders() {
		m.renderColumnHeaders(r)
	}
	m.renderFileEntries(r)
	return r.Render()
}

func (m *Model) renderTopBar(r *rendering.Renderer) {
	// TODO - Add ansitruncate left in renderer and remove truncation here
	width := m.GetContentWidth() - common.InnerPadding
	if m.gitViewActive() {
		// Git view uses two lines: the branch, and the path relative to the repo root.
		branch := m.gitMarker()
		if m.gitBranch != "" {
			branch += icon.Space + m.gitBranch
		}
		relPath := "."
		if rel, err := filepath.Rel(m.gitRoot, m.Location); err == nil {
			relPath = rel
		}
		r.AddLines(
			common.FilePanelTopDirectoryIcon+common.FilePanelTopPathStyle.Render(
				common.TruncateText(branch, width, "...")),
			common.FilePanelTopPathStyle.Render(" "+common.TruncateTextBeginning(relPath, width, "...")),
		)
		r.AddSection()
		return
	}

	path := utils.ContractHome(m.Location, variable.HomeDir)
	suffix := ""
	if m.gitRoot != "" {
		suffix = " " + m.gitMarker()
	}
	truncatedPath := common.TruncateTextBeginning(path, width-lipgloss.Width(suffix), "...")
	r.AddLines(common.FilePanelTopDirectoryIcon + common.FilePanelTopPathStyle.Render(truncatedPath) + suffix)
	r.AddSection()
}

// gitMarker is the short indicator that the panel is inside a git repo.
func (m *Model) gitMarker() string {
	if common.Config.Nerdfont {
		return icon.GitBranch
	}
	return "(git)"
}

func (m *Model) renderSearchBar(r *rendering.Renderer) {
	r.AddLines(" " + m.SearchBar.View())
}

// TODO : Unit test this
func (m *Model) renderFooter(r *rendering.Renderer, selectedCount uint) {
	sortLabel, sortIcon := m.getSortInfo()
	modeLabel, modeIcon := m.getPanelModeInfo(selectedCount)
	cursorStr := m.getCursorString()

	if common.Config.Nerdfont {
		sortLabel = sortIcon + icon.Space + sortLabel
		modeLabel = modeIcon + icon.Space + modeLabel
	} else {
		// TODO : Figure out if we can set icon.Space to " " if nerdfont is false
		// That would simplify code
		sortLabel = sortIcon + " " + sortLabel
	}

	if common.Config.ShowPanelFooterInfo {
		r.SetBorderInfoItems(sortLabel, modeLabel, cursorStr)
		if r.AreInfoItemsTruncated() {
			r.SetBorderInfoItems(sortIcon, modeIcon, cursorStr)
		}
	} else {
		r.SetBorderInfoItems(cursorStr)
	}
}

func (m *Model) renderColumnHeaders(r *rendering.Renderer) {
	var builder strings.Builder
	for _, column := range m.columns {
		builder.WriteString(column.RenderHeader())
	}
	r.AddLines(builder.String())
}

func (m *Model) renderFileEntries(r *rendering.Renderer) {
	if m.Empty() {
		r.AddLines(common.FilePanelNoneText)
		return
	}
	end := min(m.renderIndex+m.PanelElementHeight(), m.ElemCount())

	for itemIndex := m.renderIndex; itemIndex < end; itemIndex++ {
		if m.Renaming && itemIndex == m.GetCursor() {
			r.AddLines(m.Rename.View())
			continue
		}
		var builder strings.Builder
		for _, column := range m.columns {
			colData := column.Render(itemIndex)
			builder.WriteString(colData)
		}
		r.AddLines(builder.String())
	}
}

func (m *Model) getSortInfo() (string, string) {
	iconStr := icon.SortAsc
	if m.SortReversed {
		iconStr = icon.SortDesc
	}
	return sortmodel.SortOptionsShortStr[m.SortKind], iconStr
}

func (m *Model) getPanelModeInfo(selectedCount uint) (string, string) {
	switch m.PanelMode {
	case BrowserMode:
		return "Browser", icon.Browser
	case SelectMode:
		return "Select" + icon.Space + fmt.Sprintf("(%d)", selectedCount), icon.Select
	default:
		return "", ""
	}
}

func (m *Model) getCursorString() string {
	cursor := m.GetCursor()
	if !m.Empty() {
		cursor++ // Convert to 1-based
	}
	return fmt.Sprintf("%d/%d", cursor, m.ElemCount())
}

func (m *Model) renderSelectBox(isSelected bool) string {
	if !common.Config.ShowSelectIcons || !common.Config.Nerdfont || m.PanelMode != SelectMode {
		return ""
	}

	if m.IsFocused {
		if isSelected {
			return common.CheckboxCheckedFocused
		}
		return common.CheckboxEmptyFocused
	}
	if isSelected {
		return common.CheckboxChecked
	}
	return common.CheckboxEmpty
}

// Checks whether a panel needs re-render due to being invalid or due to directory change
func (m *Model) NeedsReRender() bool {
	if !m.EmptyOrInvalid() {
		return filepath.Dir(m.GetFirstElement().Location) != m.Location
	}
	return true
}
