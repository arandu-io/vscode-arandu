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

func TestTheActivityBarIconUsesEveryCanonicalPath(t *testing.T) {
	canonical := readSVGIcon(t, "images/favicon.svg")
	activity := readSVGIcon(t, "images/activity.svg")

	if got, want := len(canonical.Paths), 44; got != want {
		t.Fatalf("canonical Arandu icon paths = %d, want %d", got, want)
	}
	if got, want := len(activity.Paths), len(canonical.Paths); got != want {
		t.Fatalf("Activity Bar icon paths = %d, want all %d canonical paths", got, want)
	}
	for index, want := range canonical.Paths {
		got := activity.Paths[index]
		got.Fill = ""
		want.Fill = ""
		if got != want {
			t.Fatalf("Activity Bar path %d = %#v, want canonical %#v", index+1, got, want)
		}
	}
}

func TestTheActivityBarIconFollowsTheThemeAndFitsTwentyFourPixels(t *testing.T) {
	canonical := readSVGIcon(t, "images/favicon.svg")
	activity := readSVGIcon(t, "images/activity.svg")

	if activity.Width != "24" || activity.Height != "24" {
		t.Fatalf("Activity Bar icon size = %sx%s, want 24x24", activity.Width, activity.Height)
	}
	if got, want := activity.ViewBox, "0 0 631 515"; got != want || got != canonical.ViewBox {
		t.Fatalf("Activity Bar icon viewBox = %q, want canonical %q", got, want)
	}
	if got, want := activity.PreserveAspectRatio, "xMidYMid meet"; got != want {
		t.Fatalf("Activity Bar icon aspect ratio = %q, want %q", got, want)
	}
	if got, want := activity.Fill, "currentColor"; got != want {
		t.Fatalf("Activity Bar icon fill = %q, want %q", got, want)
	}
	for index, path := range activity.Paths {
		if path.Fill != "" {
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
