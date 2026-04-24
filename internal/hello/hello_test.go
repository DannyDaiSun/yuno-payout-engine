package hello

import "testing"

func TestHello(t *testing.T) {
	got := Hello()
	want := "hello world"
	if got != want {
		t.Errorf("Hello() = %q, want %q", got, want)
	}
}
