package linkdetect

import "testing"

func TestDetectFindsURLsMailtoAndEmails(t *testing.T) {
	links := Detect("Failed run: https://example.test/run/1. Contact ops@example.test or mailto:oncall@example.test. Read https://example.test/run/1")

	want := []Link{
		{Kind: KindURL, Label: "https://example.test/run/1", Target: "https://example.test/run/1", Start: 12, End: 38},
		{Kind: KindEmail, Label: "ops@example.test", Target: "mailto:ops@example.test", Start: 48, End: 64},
		{Kind: KindEmail, Label: "oncall@example.test", Target: "mailto:oncall@example.test", Start: 68, End: 94},
	}
	if len(links) != len(want) {
		t.Fatalf("links = %#v", links)
	}
	for index := range want {
		if links[index] != want[index] {
			t.Fatalf("links[%d] = %#v, want %#v", index, links[index], want[index])
		}
	}
}

func TestDetectFindsBareDomains(t *testing.T) {
	links := Detect("See example.test/path and http://example.test/path.")

	want := []Link{
		{Kind: KindURL, Label: "example.test/path", Target: "example.test/path", Start: 4, End: 21},
		{Kind: KindURL, Label: "http://example.test/path", Target: "http://example.test/path", Start: 26, End: 50},
	}
	if len(links) != len(want) {
		t.Fatalf("links = %#v", links)
	}
	for index := range want {
		if links[index] != want[index] {
			t.Fatalf("links[%d] = %#v, want %#v", index, links[index], want[index])
		}
	}
}
