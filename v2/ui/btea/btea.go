// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package btea

import (
	"fmt"
	"log/slog"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"github.com/ironcore-dev/ironcore-image/v2/ui/observable"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type Model struct {
	width    int
	height   int
	pushes   pushes
	stopping bool
	events   <-chan any
	log      *slog.Logger
}

type ModelOptions struct {
	TitleFunc func(desc ocispec.Descriptor) string
	Verb      string
	Logger    *slog.Logger
}

func NewModel(events <-chan any, opts ModelOptions) *Model {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Model{
		events: events,
		pushes: newPushes(opts.TitleFunc, opts.Verb),
		log:    logger,
	}
}

type doneEvent struct{}

func (m *Model) waitForEvent() tea.Cmd {
	return func() tea.Msg {
		e, ok := <-m.events
		if !ok {
			return doneEvent{}
		}
		return e
	}
}

func (m *Model) Init() tea.Cmd {
	return m.waitForEvent()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case observable.PushEvent:
		m.log.Info("Push Event", "Type", fmt.Sprintf("%T", msg), "MediaType", msg.GetDescriptor().MediaType)
		var cmd tea.Cmd
		m.pushes, cmd = m.pushes.Update(msg)
		if m.stopping && m.pushes.Len() == 0 {
			return m, tea.Sequence(cmd, tea.Quit)
		}
		return m, tea.Sequence(cmd, m.waitForEvent())
	case PushItemFrame:
		var cmd tea.Cmd
		m.pushes, cmd = m.pushes.Update(msg)
		return m, cmd
	case progress.FrameMsg:
		var cmd tea.Cmd
		m.pushes, cmd = m.pushes.Update(msg)
		return m, cmd
	case TagEvent:
		if msg.Error != nil {
			return m, tea.Printf("error tagging %s: %v", msg.Tag, msg.Error)
		}
		return m, tea.Sequence(tea.Printf("tagged %s", msg.Tag), m.waitForEvent())
	case doneEvent:
		m.stopping = true
		if m.pushes.items.Len() == 0 {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) View() tea.View {
	pushes := m.pushes.View()
	return tea.NewView(pushes)
}

type UI struct {
	log     *slog.Logger
	events  chan any
	program *tea.Program
}

func (ui *UI) Stop() {
	defer ui.program.Wait()
	close(ui.events)
}

func (ui *UI) Wait() {
	ui.program.Wait()
}

func (ui *UI) MonitorPusher(p observable.Pusher) {
	p.AddListener(observable.PushListenerFunc(func(event observable.PushEvent) {
		ui.log.Info("Sending event", "Type", fmt.Sprintf("%T", event))
		ui.events <- event
	}))
}

func (ui *UI) PushEvent(event observable.PushEvent) {
	ui.log.Info("Sending event", "Type", fmt.Sprintf("%T", event))
	ui.events <- event
}

type TagEvent struct {
	Tag   string
	Error error
}

func (ui *UI) TagEvent(event TagEvent) {
	ui.events <- event
}

type Options struct {
	Logger    *slog.Logger
	TitleFunc func(desc ocispec.Descriptor) string
	Verb      string
}

func New(opts Options) *UI {
	log := opts.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}

	events := make(chan any, 1024)

	pg := tea.NewProgram(NewModel(events, ModelOptions{
		TitleFunc: opts.TitleFunc,
		Logger:    log,
		Verb:      opts.Verb,
	}))
	go func() {
		_, err := pg.Run()
		if err != nil {
			log.Error("Error running program", "error", err)
		}
	}()

	ui := &UI{
		program: pg,
		events:  events,
		log:     log,
	}

	return ui
}
