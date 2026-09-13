package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/jgabor/spela/internal/nav"
)

func (p *resourcePaneModel) profileDetail() *DetailModel {
	if p.State().Scope.Kind == nav.ScopeGlobal {
		return &p.defaultsDetail
	}
	return &p.content.detail
}

func (p resourcePaneModel) UpdateAction(action KeyAction) (resourcePaneModel, tea.Cmd) {
	var command tea.Cmd
	switch p.State().Destination {
	case nav.DestinationSettings:
		p.settings, command = p.settings.UpdateAction(action)
	case nav.DestinationDLLCatalog:
		if !p.dllsResource.HasModalOpen() && (action == ActionDetailPreviousItem || action == ActionDetailNextItem) {
			p.dllsResource = p.dllsResource.ScrollDetail(action, p.State().DLLCatalogSection)
		} else {
			p.dllsResource, command = p.dllsResource.UpdateAction(action)
		}
	case nav.DestinationLibrary:
		if p.content.HasModalOpen() || p.State().Aspect == nav.AspectDLLs {
			p.content, command = p.content.UpdateDLLAction(action)
		} else if p.State().Aspect == nav.AspectProfile {
			save, _ := p.profileDetail().UpdateAction(action)
			if save {
				if p.State().Scope.Kind == nav.ScopeGlobal {
					command = p.saveDefaultProfile()
				} else {
					command = p.content.saveResolvedProfile()
				}
			}
		} else {
			p.scrollDetail(action)
		}
	case nav.DestinationMonitor:
		p.scrollDetail(action)
	}
	return p, command
}

func (p resourcePaneModel) UpdateEditorInput(key tea.KeyPressMsg) (resourcePaneModel, tea.Cmd) {
	if p.State().Destination == nav.DestinationSettings {
		var command tea.Cmd
		p.settings, command = p.settings.UpdateEditorInput(key)
		return p, command
	}
	p.profileDetail().UpdateEditorInput(key)
	return p, nil
}

func (p *resourcePaneModel) scrollDetail(action KeyAction) {
	delta := 0
	if action == ActionDetailPreviousItem {
		delta = -1
	}
	if action == ActionDetailNextItem {
		delta = 1
	}
	maximum := p.detailScrollMaximum()
	p.scrollOffset = min(max(p.scrollOffset+delta, 0), maximum)
}

func (p resourcePaneModel) detailScrollMaximum() int {
	return max(len(strings.Split(strings.TrimRight(p.readOnlyContent(), "\n"), "\n"))-max(p.height-2, 1), 0)
}

func (p resourcePaneModel) readOnlyContent() string {
	switch p.State().Destination {
	case nav.DestinationMonitor:
		metrics := p.metricsView
		metrics.height = 0
		return metrics.View(true, p.State().MonitorSection)
	case nav.DestinationLibrary:
		if p.State().Aspect == nav.AspectOverview {
			overview := p.overview
			overview.height = 100000
			overview.offset = 0
			return overview.View()
		}
	}
	return ""
}

func (p resourcePaneModel) readOnlyView() string {
	lines := strings.Split(strings.TrimRight(p.readOnlyContent(), "\n"), "\n")
	visible := max(p.height-2, 1)
	offset := min(p.scrollOffset, max(len(lines)-visible, 0))
	return strings.Join(lines[offset:min(offset+visible, len(lines))], "\n")
}
