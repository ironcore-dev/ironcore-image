// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package btea

import (
	"errors"
	"fmt"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/errdef"
)

type push struct {
	title    string
	verb     string
	expected ocispec.Descriptor
	spinner  spinner.Model
	progress progress.Model
}

func newPush(title, verb string, expected ocispec.Descriptor) *push {
	if title == "" {
		title = expected.MediaType
	}
	if verb == "" {
		verb = "pushed"
	}
	return &push{
		title:    title,
		verb:     verb,
		expected: expected,
		spinner:  spinner.New(),
		progress: progress.New(),
	}
}

func (p *push) View() string {
	return fmt.Sprintf("%s %s %s", p.spinner.View(), p.title, p.progress.View())
}

func (p *push) Tick() tea.Msg {
	return p.spinner.Tick()
}

func (p *push) Progress(readBytes int64) tea.Cmd {
	percentage := float64(readBytes) / float64(p.expected.Size)
	return p.progress.SetPercent(percentage)
}

func (p *push) Result(err error) tea.Cmd {
	if err != nil {
		if !errors.Is(err, errdef.ErrAlreadyExists) {
			return tea.Printf("%s error: %v", p.title, err)
		}
		return tea.Printf("%s cached (%s)", p.title, p.expected.Digest)
	}
	return tea.Printf("%s %s (%s)", p.title, p.verb, p.expected.Digest)
}

func (p *push) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		p.spinner, cmd = p.spinner.Update(msg)
		return cmd
	case progress.FrameMsg:
		var cmd tea.Cmd
		p.progress, cmd = p.progress.Update(msg)
		return cmd
	}
	return nil
}
