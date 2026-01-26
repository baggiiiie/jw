package cmd

import (
	"fmt"
	"jenkins-monitor/pkg/config"
	"log"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func runTUI() {
	app := tview.NewApplication()
	table := tview.NewTable().
		SetBorders(true)

	// Function to update the table data
	updateTableContent := func() {
		cfg, err := config.Load()
		if err != nil {
			// Handle error, maybe show it in the TUI
			log.Printf("Error loading config: %v", err)
			return
		}

		table.Clear()

		// Set table headers
		headerCell := func(text string) *tview.TableCell {
			return tview.NewTableCell(text).
				SetTextColor(tcell.ColorYellow).
				SetSelectable(false)
		}
		table.SetCell(0, 0, headerCell("Job URL"))
		table.SetCell(0, 1, headerCell("Status"))
		table.SetCell(0, 2, headerCell("Monitored For"))

		// Populate table rows
		i := 1
		for _, job := range cfg.Jobs {
			duration := time.Since(job.StartTime)
			status := "OK"
			statusColor := tcell.ColorGreen
			if job.LastCheckFailed {
				status = "Failing"
				statusColor = tcell.ColorRed
			}
			table.SetCell(i, 0, tview.NewTableCell(job.URL))
			table.SetCell(i, 1, tview.NewTableCell(status).SetTextColor(statusColor))
			table.SetCell(i, 2, tview.NewTableCell(formatDuration(duration)))
			i++
		}
	}

	// Initial table update
	updateTableContent()

	// Create a channel to signal the ticker goroutine to stop
	done := make(chan struct{})

	// Set up a ticker to refresh the table every 5 seconds
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		defer ticker.Stop() // Ensure ticker is stopped when goroutine exits
		for {
			select {
			case <-ticker.C:
				app.QueueUpdateDraw(func() {
					updateTableContent()
				})
			case <-done: // Listen for signal to exit
				return
			}
		}
	}()

	// Run the tview application. This call blocks until the application exits.
	if err := app.SetRoot(table, true).Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
	}

	// Signal the ticker goroutine to stop after the tview app has exited
	close(done)
}
