package main

import (
	"fmt"
	"io"
	"time"

	"github.com/bashkirian/calendar-service/internal/calendar"
)

const layout = "Mon, 02 Jan 2006 15:04 MST"

// printEntry renders e for humans.
func printEntry(w io.Writer, e calendar.Entry) error {
	_, err := fmt.Fprintf(w, `┌─ Calendar entry %s
│ Start     %s
│ Stop      %s
│ Duration  %s
│ Created   %s
└ Updated   %s
`,
		e.ID,
		e.StartDate.Format(layout),
		e.StopDate.Format(layout),
		e.StopDate.Sub(e.StartDate),
		e.CreatedAt.Format(time.RFC3339),
		e.UpdatedAt.Format(time.RFC3339),
	)
	return err
}
