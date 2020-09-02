package gps

import (
	"fmt"
	"os"
	"time"

	"github.com/ernestrc/blue/gps"
	"github.com/olekukonko/tablewriter"
)

func renderTable(poss []gps.Coordinates) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Device", "Lat/Lng", "Altitude", "Last Updated"})
	defer table.Render()

	for _, pos := range poss {
		table.Append([]string{
			pos.DeviceID,
			fmt.Sprintf("%f,%f", pos.Latitude, pos.Longitude),
			fmt.Sprintf("%f m", pos.Altitude),
			fmt.Sprintf("%s ago", time.Since(time.Unix(pos.UnixTime, 0)).String()),
		})
	}
}
