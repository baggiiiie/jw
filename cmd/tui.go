package cmd

import (
	"fmt"
	"log"
	"strings"
	"time"

	"jenkins-monitor/pkg/color"
	"jenkins-monitor/pkg/config"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func runTUI() {
	// Initial check to prevent TUI from starting if there are no jobs.
	initialCfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}
	if len(initialCfg.Jobs) == 0 {
		fmt.Println("No jobs in the watch list. TUI will not start.")
		return
	}

	app := tview.NewApplication()
	table := tview.NewTable().
		SetBorders(true)

	// updateTableContent refreshes the table view with the latest job statuses.
	// It will stop the application if the job list becomes empty.
	updateTableContent := func() {
		// Use Reload() instead of Load() to get fresh data from disk,
		// since the daemon (a separate process) may have modified the config.
		cfg, err := config.Reload()
		if err != nil {
			log.Printf("Error loading config: %v", err)
			return
		}

		// If the job list is empty, stop the TUI.
		if len(cfg.Jobs) == 0 {
			app.Stop()
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
			urlParts := strings.Split(job.URL, "/")
			url := strings.Join(urlParts[len(urlParts)-3:], "/")
			table.SetCell(i, 0, tview.NewTableCell(url))
			table.SetCell(i, 1, tview.NewTableCell(status).SetTextColor(statusColor))
			table.SetCell(i, 2, tview.NewTableCell(formatDuration(duration)))
			i++
		}
	}

	// Initial table population
	updateTableContent()

	// done channel is used to signal the ticker goroutine to stop.
	done := make(chan struct{})

	// Refresh the table every 5 seconds.
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				app.QueueUpdateDraw(func() {
					updateTableContent()
				})
			case <-done:
				return
			}
		}
	}()

	// Run the application.
	if err := app.SetRoot(table, true).Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
	}

	// Signal the ticker to stop.
	fmt.Println(color.GreenText("All watched jobs finished! TUI exited."))
	close(done)
}
