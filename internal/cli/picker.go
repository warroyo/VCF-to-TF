package cli

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/warroyo/VCF-to-TF/internal/tf"
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
	list      list.Model
	baseTitle string
	opts      tf.Options // render options, toggled live with c / o / r
	chosen    *pickItem
	aborted   bool
}

func (m pickerModel) Init() tea.Cmd { return nil }

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// titleWithState renders the list title plus the live toggle hints.
func (m pickerModel) titleWithState() string {
	return fmt.Sprintf("%s  ·  [c]omments:%s  [o]ptional-tags:%s  [r]equired-only:%s",
		m.baseTitle, onOff(m.opts.Comments), onOff(m.opts.MarkOptional), onOff(m.opts.RequiredOnly))
}

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
		case "c":
			m.opts.Comments = !m.opts.Comments
			m.list.Title = m.titleWithState()
			return m, nil
		case "o":
			m.opts.MarkOptional = !m.opts.MarkOptional
			m.list.Title = m.titleWithState()
			return m, nil
		case "r":
			m.opts.RequiredOnly = !m.opts.RequiredOnly
			m.list.Title = m.titleWithState()
			return m, nil
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
func runPicker(title, statusNoun string, items []list.Item, opts tf.Options) (interface{}, tf.Options, error) {
	// Compact, one-line-per-item delegate: no description line, no inter-item
	// spacing. Keeps long lists scannable.
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)
	delegate.SetHeight(1)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("170")).BorderForeground(lipgloss.Color("170"))

	l := list.New(items, delegate, 0, 0)
	l.SetStatusBarItemName(statusNoun, statusNoun+"s")
	l.SetShowHelp(true)

	model := pickerModel{list: l, baseTitle: title, opts: opts}
	model.list.Title = model.titleWithState()

	p := tea.NewProgram(
		model,
		tea.WithOutput(os.Stderr),
		tea.WithAltScreen(),
	)

	res, err := p.Run()
	if err != nil {
		return nil, opts, err
	}

	m := res.(pickerModel)
	if m.aborted || m.chosen == nil {
		return nil, opts, errAborted
	}
	return m.chosen.value, m.opts, nil
}
