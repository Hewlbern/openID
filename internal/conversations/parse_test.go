package conversations

import "testing"

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
