package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type textInputModel struct {
	textInput textinput.Model
	err       error
	output    string
	quitting  bool
	width     int
}

func initialTextInputModel(prompt string) textInputModel {
	ti := textinput.New()
	ti.Placeholder = "Type your message or \\command..."
	ti.Focus()
	ti.CharLimit = 1000
	ti.Width = 60 // Default width
	ti.Prompt = prompt

	return textInputModel{
		textInput: ti,
		err:       nil,
		width:     60,
	}
}

func (m textInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m textInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		// Adjust text input width (subtract border and padding)
		// Border: 2 chars (left + right)
		// Padding: 2 chars (left + right)
		// Prompt: length of prompt
		availableWidth := m.width - 4
		if availableWidth > 0 {
			m.textInput.Width = availableWidth - len(m.textInput.Prompt)
		}
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.output = m.textInput.Value()
			m.quitting = true
			return m, tea.Quit
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m textInputModel) View() string {
	// Define styles
	focusedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")). // Blue border
		Padding(0, 1).
		Width(m.width - 2) // Subtract border width

	return focusedStyle.Render(m.textInput.View()) + "\n"
}

// GetInput runs the bubbletea program to get user input
func GetInput(prompt string) (string, error) {
	p := tea.NewProgram(initialTextInputModel(prompt))
	m, err := p.Run()
	if err != nil {
		return "", err
	}

	if m, ok := m.(textInputModel); ok {
		if m.output == "" && m.quitting {
			// Handle empty input or quit
			return "", nil
		}
		return m.output, nil
	}

	return "", fmt.Errorf("could not assert model")
}
