package fs

import "testing"

func TestUpsertMarkedBlock(t *testing.T) {
	start := "<!-- start -->"
	end := "<!-- end -->"

	t.Run("creates block on empty content", func(t *testing.T) {
		got := UpsertMarkedBlock("", start, end, "hello")
		want := "<!-- start -->\nhello\n<!-- end -->\n"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("appends block to existing content", func(t *testing.T) {
		got := UpsertMarkedBlock("# Project\n\nRules here.\n", start, end, "hello")
		want := "# Project\n\nRules here.\n\n<!-- start -->\nhello\n<!-- end -->\n"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("replaces existing marked block idempotently", func(t *testing.T) {
		initial := "# Project\n\n<!-- start -->\nold\n<!-- end -->\n\n## Footer\n"
		got := UpsertMarkedBlock(initial, start, end, "new")
		want := "# Project\n\n<!-- start -->\nnew\n<!-- end -->\n\n## Footer\n"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}

		// Re-applying same content must stay identical.
		gotAgain := UpsertMarkedBlock(got, start, end, "new")
		if gotAgain != want {
			t.Fatalf("not idempotent: got %q, want %q", gotAgain, want)
		}
	})
}

func TestAppendMissingLines(t *testing.T) {
	t.Run("creates lines on empty content", func(t *testing.T) {
		got, changed := AppendMissingLines("", []string{".scalable-code/report.json", ".scalable-code/report.md"})
		if !changed {
			t.Fatalf("expected changed=true")
		}
		want := ".scalable-code/report.json\n.scalable-code/report.md\n"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("appends only missing lines", func(t *testing.T) {
		existing := "build/\n.scalable-code/report.json\n"
		got, changed := AppendMissingLines(existing, []string{".scalable-code/report.json", ".scalable-code/report.md"})
		if !changed {
			t.Fatalf("expected changed=true")
		}
		want := "build/\n.scalable-code/report.json\n.scalable-code/report.md\n"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("no changes when all lines exist", func(t *testing.T) {
		existing := ".scalable-code/report.json\n.scalable-code/report.md\n"
		_, changed := AppendMissingLines(existing, []string{".scalable-code/report.json", ".scalable-code/report.md"})
		if changed {
			t.Fatalf("expected changed=false")
		}
	})
}
