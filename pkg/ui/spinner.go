package ui

import (
	"fmt"
	"sync"
	"time"
)

type Spinner struct {
	mu       sync.Mutex
	stop     chan struct{}
	text     string
	active   bool
	frames   []string
	interval time.Duration
}

func NewSpinner(text string) *Spinner {
	return &Spinner{
		stop:     make(chan struct{}),
		text:     text,
		frames:   []string{"⣾", "⣷", "⣯", "⣟", "⡿", "⢿", "⣻", "⣽"},
		interval: 100 * time.Millisecond,
	}
}

func (s *Spinner) Start() {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}
	s.active = true
	s.stop = make(chan struct{})
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-s.stop:
				fmt.Printf("Done!   \n")
				return
			case <-ticker.C:
				fmt.Printf("\r%s %s... ", s.frames[i%len(s.frames)], s.text)
				i++
			}
		}
	}()
}

func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return
	}
	s.active = false
	close(s.stop)
}
