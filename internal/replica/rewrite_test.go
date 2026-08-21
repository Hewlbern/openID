package replica

import "testing"

func TestRewriteOrigin(t *testing.T) {
	in := []byte(`<http://localhost:4000/mike/profile/card#me> a <http://xmlns.com/foaf/0.1/Person> .`)
	out := RewriteOrigin(in, DefaultLocalBases, "https://pod.example")
	want := `<https://pod.example/mike/profile/card#me> a <http://xmlns.com/foaf/0.1/Person> .`
	if string(out) != want {
		t.Fatalf("got %s", out)
	}
}
