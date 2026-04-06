package main

import "testing"

func TestSummarizeI2PConsoleHTML(t *testing.T) {
	t.Parallel()

	html := `
<html>
  <head><title>I2P Router Console - Network Database</title></head>
  <body>
    <div>Peers: 123</div>
    <div>Firewalled</div>
    <div>Clock Skew of 2m</div>
  </body>
</html>`

	report := summarizeI2PConsoleHTML(html)
	if report.Title != "I2P Router Console - Network Database" {
		t.Fatalf("Title = %q", report.Title)
	}
	required := []string{
		"clock-skew warning visible",
		"firewalled status mentioned",
		"network database section present",
		"peer information present",
	}
	for _, want := range required {
		found := false
		for _, got := range report.Indicators {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing indicator %q in %#v", want, report.Indicators)
		}
	}
}
