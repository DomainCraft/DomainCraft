package textutil

import "testing"

func TestPluralize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"User", "Users"},
		{"Category", "Categories"},
		{"Status", "Statuses"},
		{"Box", "Boxes"},
		{"Lady", "Ladies"},
		{"Piano", "Pianoes"},
		{"Tax", "Taxes"},
		{"Quiz", "Quizzes"},
		{"Hero", "Heroes"},
		{"Day", "Days"},     // y after vowel -> -s
		{"Key", "Keys"},     // y after vowel -> -s
		{"Dish", "Dishes"},  // -sh -> -es
	}
	for _, tt := range tests {
		got := Pluralize(tt.input)
		if got != tt.want {
			t.Errorf("Pluralize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSingularize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Users", "User"},
		{"Categories", "Category"},
		{"Statuses", "Status"},
		{"Boxes", "Box"},
		{"Quizzes", "Quiz"},
		{"Statuses", "Status"},
		{"Tags", "Tag"},
		{"User", "User"},    // no change
	}
	for _, tt := range tests {
		got := Singularize(tt.input)
		if got != tt.want {
			t.Errorf("Singularize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPascalCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"firstName", "FirstName"},
		{"first_name", "FirstName"},
		{"first-name", "FirstName"},
		{"first name", "FirstName"},
		{"HTML", "Html"},
		{"", ""},
		{"a", "A"},
	}
	for _, tt := range tests {
		got := PascalCase(tt.input)
		if got != tt.want {
			t.Errorf("PascalCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCamelCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"first_name", "firstName"},
		{"FirstName", "firstName"},
		{"", ""},
	}
	for _, tt := range tests {
		got := CamelCase(tt.input)
		if got != tt.want {
			t.Errorf("CamelCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSplitIdentifier(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"firstName", []string{"first", "Name"}},
		{"first_name", []string{"first", "name"}},
		{"HTMLParser", []string{"HTML", "Parser"}},
		{"HTTPSServer", []string{"HTTPS", "Server"}},
		{"simple", []string{"simple"}},
	}
	for _, tt := range tests {
		got := SplitIdentifier(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("SplitIdentifier(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("SplitIdentifier(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}
