package hello

import "testing"

func TestGreet(t *testing.T) {
	t.Run("normal input", func(t *testing.T) {
		got := Greet("World")
		want := "Hello, World!"
		if got != want {
			t.Errorf("Greet(%q) = %q, want %q", "World", got, want)
		}
	})

	t.Run("empty string", func(t *testing.T) {
		got := Greet("")
		want := "Hello, !"
		if got != want {
			t.Errorf("Greet(%q) = %q, want %q", "", got, want)
		}
	})
}
