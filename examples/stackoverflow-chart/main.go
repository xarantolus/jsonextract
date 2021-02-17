package main

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/wcharczuk/go-chart"
	"github.com/xarantolus/jsonextract"
)

func main() {
	var c = http.Client{
		Timeout: 90 * time.Second,
	}

	// Download a StackOverflow user profile
	// The profile page only contains the chart data if it ends with "?tab=topactivity"
	resp, err := c.Get("https://stackoverflow.com/users/5728357/xarantolus?tab=topactivity")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// There's two lines like this in the page source:
	//     var graphData = [984,984,984,984,...];
	//     StackExchange.user.renderMiniGraph(graphData);

	// That's what we want to extract into this array
	var yValues []float64

	// Now we read the entire page source and extract into our yValues array
	err = jsonextract.Reader(resp.Body, func(b []byte) error {
		// Try to unmarshal
		err := json.Unmarshal(b, &yValues)
		if err == nil && len(yValues) > 0 {
			// If it was successful, we stop parsing
			return jsonextract.ErrStop
		}

		// continue with next JSON object
		return nil
	})
	if err != nil {
		panic("cannot extract JSON objects: " + err.Error())
	}

	// Now we check the integrity of our extracted data.
	// With a struct we should check if all struct fields we want
	// were extracted, but here we just see if we got any numbers
	if len(yValues) == 0 {
		panic("seems like we couldn't get chart data")
	}

	// Create the graph
	graph := chart.Chart{
		Series: []chart.Series{
			chart.ContinuousSeries{
				XValues: generateXValues(len(yValues)),
				YValues: yValues,
				Name:    "Reputation",
			},
		},
	}

	// Create the file where we want to render it
	f, err := os.Create("stackoverflow-chart.png")
	if err != nil {
		panic("creating chart file: " + err.Error())
	}
	defer f.Close()

	// Render this graph
	err = graph.Render(chart.PNG, f)
	if err != nil {
		panic("rendering chart: " + err.Error())
	}
}

func generateXValues(limit int) (out []float64) {
	for i := 0; i < limit; i++ {
		out = append(out, float64(i))
	}

	return
}
