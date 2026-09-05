package extension_test

import (
	"encoding/xml"
	"os"
	"strings"
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

// TestTheActivityBarIconIsDrawnAtTwentyFourPixels checks the size the Activity
// Bar actually renders at.
//
// VS Code draws this icon in a 24-pixel strip, monochrome, and that size is the
// whole design constraint: an emblem carrying dozens of strokes resolves to a
// smudge there, which is why the Activity Bar carries its own drawing rather
// than the one in images/favicon.svg.
func TestTheActivityBarIconIsDrawnAtTwentyFourPixels(t *testing.T) {
	activity := readSVGIcon(t, "images/activity.svg")

	if activity.Width != "24" || activity.Height != "24" {
		t.Fatalf("Activity Bar icon size = %sx%s, want 24x24", activity.Width, activity.Height)
	}
	if activity.ViewBox == "" {
		t.Fatal("Activity Bar icon declares no viewBox, so it does not scale to the strip it is drawn in")
	}
	if len(activity.Paths) == 0 {
		t.Fatal("Activity Bar icon has no paths, so it renders as nothing at all")
	}
	for index, path := range activity.Paths {
		if strings.TrimSpace(path.Data) == "" {
			t.Fatalf("Activity Bar path %d carries no d attribute and draws nothing", index+1)
		}
	}
}

// TestTheActivityBarIconTakesItsColorFromTheTheme is the check that decides
// whether the icon is visible at all.
//
// The Activity Bar paints the icon in the theme's foreground, and it can only do
// that through currentColor. A literal fill survives one theme and disappears in
// the other -- white on the light strip is the failure nobody sees, because the
// icon does not look broken, it looks absent.
//
// Either place carries it: on the <svg>, where every path inherits, or on each
// path. The test accepts both and refuses a literal anywhere, because what
// matters is that no stroke names a color of its own.
//
// This is also the reason images/favicon.svg is not reused here. It fills every
// path with white, so copying it into this slot would satisfy a check that
// compared the two files and would leave the strip empty in a light theme -- a
// green suite over an invisible icon.
func TestTheActivityBarIconTakesItsColorFromTheTheme(t *testing.T) {
	activity := readSVGIcon(t, "images/activity.svg")

	themed := func(fill string) bool { return fill == "" || fill == "currentColor" || fill == "none" }

	if !themed(activity.Fill) {
		t.Fatalf("Activity Bar icon fill = %q, want currentColor or none so the theme decides", activity.Fill)
	}
	painted := 0
	for index, path := range activity.Paths {
		if !themed(path.Fill) {
			t.Fatalf("Activity Bar path %d is filled with %q, which is one theme's color written down:\n"+
				"the Activity Bar paints its icons in the theme foreground, so a literal fill is invisible "+
				"in the theme it was not chosen for.", index+1, path.Fill)
		}
		if path.Fill == "currentColor" {
			painted++
		}
	}
	if painted == 0 && activity.Fill != "currentColor" {
		t.Fatal("nothing in the Activity Bar icon names currentColor, so no stroke takes the theme foreground:\n" +
			"put it on the <svg> for every path to inherit, or on each path.")
	}
}

// TestTheEmblemIsNotWhatTheActivityBarDraws fixes the difference between the two
// icon files, so a later change cannot quietly collapse them into one.
//
// They answer different questions. The emblem identifies the extension in the
// marketplace and the tab, in full color at a readable size. The Activity Bar
// icon is monochrome at 24 pixels. Making one file serve both means either a
// smudge in the strip or a flat silhouette in the marketplace.
func TestTheEmblemIsNotWhatTheActivityBarDraws(t *testing.T) {
	emblem := readSVGIcon(t, "images/favicon.svg")
	activity := readSVGIcon(t, "images/activity.svg")

	if emblem.ViewBox == activity.ViewBox && len(emblem.Paths) == len(activity.Paths) {
		t.Fatal("the Activity Bar icon and the emblem are now the same drawing.\n" +
			"They are separate on purpose: the emblem is full color at a readable size, and the Activity Bar " +
			"is one theme color at 24 pixels. If this is intended, the emblem's literal fills have to go first, " +
			"or the strip renders it invisible in one of the two themes.")
	}

	literal := 0
	for _, path := range emblem.Paths {
		if path.Fill != "" && path.Fill != "currentColor" && path.Fill != "none" {
			literal++
		}
	}
	if literal == 0 {
		t.Skip("the emblem no longer carries literal fills, so the reason recorded here needs rereading")
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
