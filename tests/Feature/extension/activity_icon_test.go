package extension_test

import (
	"encoding/xml"
	"os"
	"testing"
)

type svgIcon struct {
	Width               string    `xml:"width,attr"`
	Height              string    `xml:"height,attr"`
	ViewBox             string    `xml:"viewBox,attr"`
	Fill                string    `xml:"fill,attr"`
	PreserveAspectRatio string    `xml:"preserveAspectRatio,attr"`
	Paths               []svgPath `xml:"path"`
}

type svgPath struct {
	Data     string `xml:"d,attr"`
	Fill     string `xml:"fill,attr"`
	FillRule string `xml:"fill-rule,attr"`
	ClipRule string `xml:"clip-rule,attr"`
}

func TestTheActivityBarUsesTheFourPathAruMark(t *testing.T) {
	activity := readSVGIcon(t, "images/aru.svg")
	if got, want := len(activity.Paths), 4; got != want {
		t.Fatalf("Activity Bar Aru mark paths = %d, want %d", got, want)
	}
	for index, path := range activity.Paths {
		if path.Data == "" {
			t.Fatalf("Activity Bar Aru path %d is empty", index+1)
		}
	}
}

func TestTheActivityBarIconFollowsTheThemeAndFitsTwentyFourPixels(t *testing.T) {
	activity := readSVGIcon(t, "images/aru.svg")

	if activity.Width != "24" || activity.Height != "24" {
		t.Fatalf("Activity Bar icon size = %sx%s, want 24x24", activity.Width, activity.Height)
	}
	if got, want := activity.ViewBox, "0 0 292 260"; got != want {
		t.Fatalf("Activity Bar icon viewBox = %q, want %q", got, want)
	}
	if got, want := activity.Fill, "currentColor"; got != want {
		t.Fatalf("Activity Bar icon fill = %q, want %q", got, want)
	}
	for index, path := range activity.Paths {
		if path.Fill != "" && path.Fill != "currentColor" {
			t.Fatalf("Activity Bar path %d overrides the themed fill with %q", index+1, path.Fill)
		}
	}
}

func readSVGIcon(t *testing.T, name string) svgIcon {
	t.Helper()
	raw, err := os.ReadFile(rootPath(t, name))
	if err != nil {
		t.Fatal(err)
	}
	var icon svgIcon
	if err := xml.Unmarshal(raw, &icon); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return icon
}
