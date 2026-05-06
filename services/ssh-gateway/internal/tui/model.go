package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sip-happens/services/ssh-gateway/internal/api"
)

type screen int

const (
	screenWelcome screen = iota
	screenCatalog
	screenError
)

type Model struct {
	api *api.Client

	screen screen
	err    error

	catalog []api.CatalogItem
	cursor  int

	width  int
	height int
}

func NewRootModel(client *api.Client) Model {
	return Model{
		api:    client,
		screen: screenWelcome,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

		switch m.screen {
		case screenWelcome:
			switch msg.String() {
			case "enter", "1":
				m.screen = screenCatalog
				return m, m.loadCatalogCmd()
			}
		case screenCatalog:
			switch msg.String() {
			case "esc":
				m.screen = screenWelcome
				return m, nil
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
				return m, nil
			case "down", "j":
				if m.cursor < len(m.catalog)-1 {
					m.cursor++
				}
				return m, nil
			case "r":
				return m, m.loadCatalogCmd()
			}
		}
	case catalogLoadedMsg:
		m.catalog = msg.items
		if m.cursor >= len(m.catalog) {
			m.cursor = 0
		}
		return m, nil
	case catalogErrorMsg:
		m.err = msg.err
		m.screen = screenError
		return m, nil
	}

	return m, nil
}

func (m Model) View() string {
	switch m.screen {
	case screenWelcome:
		return m.viewWelcome()
	case screenCatalog:
		return m.viewCatalog()
	case screenError:
		return m.viewError()
	default:
		return ""
	}
}

func (m Model) viewWelcome() string {
	title := lipgloss.NewStyle().Bold(true).Render("Sip-Happens")
	warn := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Prohibited sale of alcohol to minors.")
	help := "Press Enter to browse catalog. Press q to quit."
	return fmt.Sprintf("%s\n%s\n\n%s\n", title, warn, help)
}

func (m Model) viewCatalog() string {
	header := lipgloss.NewStyle().Bold(true).Render("Catalog (press r to refresh, esc to go back, q to quit)")
	if len(m.catalog) == 0 {
		return header + "\n\n(Empty catalog)\n"
	}

	var out string
	out += header + "\n\n"
	for i, it := range m.catalog {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		line := fmt.Sprintf("%s%-18s  %s  %s %d\n",
			cursor,
			it.Product.SKU,
			it.Product.Name,
			it.Price.Currency,
			it.Price.AmountMinorUnits,
		)
		out += line
	}
	return out
}

func (m Model) viewError() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9")).Render("Error")
	return fmt.Sprintf("%s\n\n%v\n\nPress q to quit.\n", title, m.err)
}

type catalogLoadedMsg struct{ items []api.CatalogItem }
type catalogErrorMsg struct{ err error }

func (m Model) loadCatalogCmd() tea.Cmd {
	return func() tea.Msg {
		if m.api == nil {
			return catalogErrorMsg{err: fmt.Errorf("missing api client")}
		}
		items, err := m.api.GetCatalog(context.Background())
		if err != nil {
			return catalogErrorMsg{err: err}
		}
		return catalogLoadedMsg{items: items}
	}
}

