package cli

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// errAborted is returned when the user quits the picker without choosing.
var errAborted = fmt.Errorf("selection aborted")

// pickItem is a single selectable row. value carries the caller's data.
type pickItem struct {
	title string
	desc  string
	value interface{}
}

func (i pickItem) Title() string       { return i.title }
func (i pickItem) Description() string { return i.desc }
func (i pickItem) FilterValue() string { return i.title + " " + i.desc }

type pickerModel struct {
	list    list.Model
	chosen  *pickItem
	aborted bool
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h := msg.Height - 2
		if h < 5 {
			h = 5
		}
		m.list.SetSize(msg.Width, h)
		return m, nil

	case tea.KeyMsg:
		// While filtering, let the list consume keys (typing the filter).
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.aborted = true
			return m, tea.Quit
		case "enter":
			if it, ok := m.list.SelectedItem().(pickItem); ok {
				m.chosen = &it
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m pickerModel) View() string { return m.list.View() }

// runPicker shows a filterable, keyboard-driven list titled `title` and returns
// the chosen item's value. The UI renders to stderr so stdout stays reserved
// for the generated HCL. Returns errAborted if the user quits.
func runPicker(title, statusNoun string, items []list.Item) (interface{}, error) {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("170")).BorderForeground(lipgloss.Color("170"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("170")).BorderForeground(lipgloss.Color("170"))

	l := list.New(items, delegate, 0, 0)
	l.Title = title
	l.SetStatusBarItemName(statusNoun, statusNoun+"s")
	l.SetShowHelp(true)

	p := tea.NewProgram(
		pickerModel{list: l},
		tea.WithOutput(os.Stderr),
		tea.WithAltScreen(),
	)

	res, err := p.Run()
	if err != nil {
		return nil, err
	}

	m := res.(pickerModel)
	if m.aborted || m.chosen == nil {
		return nil, errAborted
	}
	return m.chosen.value, nil
}
