package bot

import "testing"

func TestCommandMenu(t *testing.T) {
	commands := CommandMenu()
	if len(commands) != 8 {
		t.Fatalf("CommandMenu() returned %d commands, want 8", len(commands))
	}

	want := map[string]bool{
		"start":    true,
		"sales":    true,
		"orders":   true,
		"cards":    true,
		"fulfill":  true,
		"pfulfill": true,
		"stock":    true,
		"cancel":   true,
	}
	for _, command := range commands {
		if !want[command.Text] {
			t.Errorf("CommandMenu() returned unexpected command %q", command.Text)
		}
		if command.Description == "" {
			t.Errorf("CommandMenu() command %q has empty description", command.Text)
		}
		delete(want, command.Text)
	}
	for command := range want {
		t.Errorf("CommandMenu() missing command %q", command)
	}
}
