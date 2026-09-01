package conversations

import (
	"testing"
	"time"
)

func TestParseMarkdownTranscript(t *testing.T) {
	c, err := ParseTranscript(`# Weekend plans

**User:** Should I take the train?

**Gemini:** Yes, the 9:10 is quieter.
`)
	if err != nil {
		t.Fatal(err)
	}
	if c.Title != "Weekend plans" {
		t.Fatalf("title %q", c.Title)
	}
	if len(c.Messages) != 2 || c.Messages[0].Role != "user" || c.Messages[1].Role != "assistant" {
		t.Fatalf("messages %#v", c.Messages)
	}
	if c.Messages[1].Text != "Yes, the 9:10 is quieter." {
		t.Fatalf("assistant text %q", c.Messages[1].Text)
	}
}

func TestParseJSONTranscript(t *testing.T) {
	c, err := ParseTranscript(`{"title":"JSON chat","messages":[{"role":"user","text":"hi"},{"role":"assistant","content":"hello"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if c.Title != "JSON chat" || len(c.Messages) != 2 {
		t.Fatalf("json object %#v", c)
	}
	c, err = ParseTranscript(`[{"role":"user","text":"hi"},{"role":"model","text":"hello"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Messages) != 2 || c.Messages[1].Role != "assistant" {
		t.Fatalf("%#v", c.Messages)
	}
}

func TestParseBarePaste(t *testing.T) {
	c, err := ParseTranscript("just some notes from spark")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Messages) != 1 || c.Messages[0].Role != "user" {
		t.Fatalf("%#v", c.Messages)
	}
}

func TestParseJSONTimestamps(t *testing.T) {
	c, err := ParseTranscript(`{
		"title":"When",
		"messages":[
			{"role":"user","content":"hi","timestamp":"2026-09-01T20:15:30+10:00"},
			{"role":"assistant","text":"yo","time":"2026-09-01T10:16:00Z"}
		]
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if c.Messages[0].Timestamp == nil || c.Messages[0].Timestamp.UTC().Format("2006-01-02T15:04:05Z") != "2026-09-01T10:15:30Z" {
		t.Fatalf("user ts %#v", c.Messages[0].Timestamp)
	}
	if c.Messages[1].Timestamp == nil || c.Messages[1].Timestamp.UTC().Format(time.RFC3339) != "2026-09-01T10:16:00Z" {
		t.Fatalf("assistant ts %#v", c.Messages[1].Timestamp)
	}
}

func TestIsGeminiShareURL(t *testing.T) {
	ok := []string{
		"https://g.co/gemini/share/abc",
		"g.co/gemini/share/abc",
		"https://gemini.google.com/share/xyz",
	}
	for _, u := range ok {
		if !IsGeminiShareURL(u) {
			t.Fatalf("expected share url %s", u)
		}
	}
	if IsGeminiShareURL("https://example.com/share") {
		t.Fatal("false positive")
	}
}
