package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const tokenFile = "token"

var (
	focusStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	labelStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	titleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	statusStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	successStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	hintStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	checkboxOn      = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render("[x]")
	checkboxOff     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("[ ]")
)

type model struct {
	inputs []textinput.Model
	focus  int
	isDM   bool
	status string
	err    string
}

const (
	fieldToken = iota
	fieldChannel
	fieldAudio
	fieldTimes
)

func initialModel() model {
	token := textinput.New()
	token.Placeholder = "Enter Discord token"
	token.EchoMode = textinput.EchoPassword
	token.EchoCharacter = '•'
	token.Width = 40
	token.Focus()

	channel := textinput.New()
	channel.Placeholder = "Enter channel ID"
	channel.Width = 40

	audio := textinput.New()
	audio.Placeholder = "Enter audio file path"
	audio.Width = 40

	times := textinput.New()
	times.Placeholder = "Number of times to send"
	times.SetValue("1")
	times.Width = 40

	return model{
		inputs: []textinput.Model{token, channel, audio, times},
		focus:  fieldToken,
		status: "Ready",
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyTab:
			m.nextFocus()
		case tea.KeyShiftTab:
			m.prevFocus()
		case tea.KeyEnter:
			return m.handleSubmit()
		case tea.KeyCtrlS:
			return m.saveToken()
		case tea.KeyCtrlD:
			m.toggleDM()
		case tea.KeyCtrlR:
			m.clearStatus()
		default:
			m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
		}
	default:
		m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
	}

	return m, cmd
}

func (m *model) nextFocus() {
	m.inputs[m.focus].Blur()
	m.focus = (m.focus + 1) % len(m.inputs)
	m.inputs[m.focus].Focus()
}

func (m *model) prevFocus() {
	m.inputs[m.focus].Blur()
	m.focus--
	if m.focus < 0 {
		m.focus = len(m.inputs) - 1
	}
	m.inputs[m.focus].Focus()
}

func (m *model) toggleDM() {
	m.isDM = !m.isDM
	if m.isDM {
		m.inputs[fieldChannel].Placeholder = "Enter user ID"
	} else {
		m.inputs[fieldChannel].Placeholder = "Enter channel ID"
	}
	if m.isDM {
		m.status = "DM mode enabled"
	} else {
		m.status = "DM mode disabled"
	}
}

func (m *model) clearStatus() {
	m.status = "Ready"
	m.err = ""
}

// sanitizeID removes any non-digit characters from a Discord ID.
// This lets users paste IDs copied from mentions such as <@123456789> or <#123456789>.
func sanitizeID(id string) string {
	var b strings.Builder
	for _, r := range id {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (m model) handleSubmit() (tea.Model, tea.Cmd) {
	switch m.focus {
	case fieldToken:
		return m.saveToken()
	default:
		return m.send()
	}
}

func (m model) saveToken() (tea.Model, tea.Cmd) {
	token := m.inputs[fieldToken].Value()
	if token == "" {
		m.err = "token cannot be empty"
		m.status = ""
		return m, nil
	}

	if err := os.WriteFile(tokenFile, []byte(token), 0600); err != nil {
		m.err = fmt.Sprintf("could not save token: %s", err)
		m.status = ""
		return m, nil
	}

	m.inputs[fieldToken].SetValue("")
	m.status = "Token saved securely"
	m.err = ""
	return m, nil
}

func (m model) send() (tea.Model, tea.Cmd) {
	// Resolve token from the input field or from the saved file.
	token := m.inputs[fieldToken].Value()
	if token == "" {
		if _, err := os.Stat(tokenFile); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				m.err = "please enter and save a Discord token (Ctrl+S)"
			} else {
				m.err = fmt.Sprintf("could not check saved token: %s", err)
			}
			m.status = ""
			return m, nil
		}

		data, err := os.ReadFile(tokenFile)
		if err != nil {
			m.err = fmt.Sprintf("could not read saved token: %s", err)
			m.status = ""
			return m, nil
		}
		token = string(data)
	}

	if token == "" {
		m.err = "Discord token is empty"
		m.status = ""
		return m, nil
	}

	channel := sanitizeID(m.inputs[fieldChannel].Value())
	m.inputs[fieldChannel].SetValue(channel)
	if channel == "" {
		m.err = "please enter a channel or user ID"
		m.status = ""
		return m, nil
	}

	audioFilePath := m.inputs[fieldAudio].Value()
	if audioFilePath == "" {
		m.err = "please enter an audio file path"
		m.status = ""
		return m, nil
	}

	x, err := strconv.Atoi(m.inputs[fieldTimes].Value())
	if err != nil || x <= 0 {
		m.err = "invalid number of times to send"
		m.status = ""
		return m, nil
	}

	// If DM mode is enabled, resolve the user ID to a DM channel ID.
	if m.isDM {
		m.status = "Creating DM channel..."
		dmChannelID, err := CreateDMChannel(token, channel)
		if err != nil {
			m.err = fmt.Sprintf("could not create DM channel: %s", err)
			m.status = ""
			return m, nil
		}
		channel = dmChannelID
	}

	m.status = fmt.Sprintf("Sending %d voice message(s)...", x)
	m.err = ""

	for i := 0; i < x; i++ {
		file, err := NewFile(audioFilePath)
		if err != nil {
			m.err = fmt.Sprintf("could not load audio file: %s", err)
			m.status = ""
			return m, nil
		}

		if _, err := file.CreateFile(token, channel); err != nil {
			m.err = fmt.Sprintf("create attachment failed: %s", err)
			m.status = ""
			return m, nil
		}

		if err := file.PutFileData(); err != nil {
			m.err = fmt.Sprintf("upload failed: %s", err)
			m.status = ""
			return m, nil
		}

		if err := file.SendFile(token, channel); err != nil {
			m.err = fmt.Sprintf("send failed: %s", err)
			m.status = ""
			return m, nil
		}
	}

	m.status = "Voice messages sent successfully!"
	m.err = ""
	return m, nil
}

func (m model) View() string {
	b := "\n" + titleStyle.Render("Discord Voice Message") + "\n\n"

	labels := []string{"Token", "Channel / User ID", "Audio File", "Times to Send"}
	for i, input := range m.inputs {
		label := labels[i]
		if i == m.focus {
			label = focusStyle.Render(label)
		} else {
			label = labelStyle.Render(label)
		}
		b += label + "\n" + input.View() + "\n\n"
	}

	dmCheck := checkboxOff
	if m.isDM {
		dmCheck = checkboxOn
	}
	b += dmCheck + " " + labelStyle.Render("Is DM") + "\n\n"

	b += hintStyle.Render("tab/shift+tab: move focus  •  enter: send  •  Ctrl+S: save token  •  Ctrl+D: toggle DM  •  Ctrl+R: clear status  •  Ctrl+C: quit") + "\n\n"

	if m.err != "" {
		b += errorStyle.Render("Error: "+m.err) + "\n"
	}
	if m.status != "" {
		b += statusStyle.Render("Status: " + m.status) + "\n"
	}

	return b
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error running program:", err)
		os.Exit(1)
	}
}
