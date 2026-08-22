package textutil

import "testing"

func TestSnakeCase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"FirstName", "first_name"},
		{"firstName", "first_name"},
		{"HTTPServer", "http_server"},
		{"HTTPStatus", "http_status"},
		{"XMLHttpRequest", "xml_http_request"},
		{"userID", "user_id"},
		{"ALLCAPS", "allcaps"},
		{"ID", "id"},
		// Digit groups: a number attached to the previous word stays with it,
		// trailing uppercase starts a new word.
		{"HTTP20xOK", "http_20x_ok"},
		{"Bld4Floor", "bld4_floor"},
		{"ProductV2", "product_v2"},
		{"V2Product", "v2_product"},
		{"H2O", "h2_o"},
		// Documented divergence from ToDatabaseColumnName on acronym+digit runs:
		// SnakeCase splits "I"+"pv4", DB columns keep "IPv4Address" together.
		{"IPv4Address", "i_pv4_address"},
		// Separators normalize to the connector rune.
		{"user_id", "user_id"},
		{"kebab-case-name", "kebab_case_name"},
		{"already snake_case", "already_snake_case"},
		{"__leading", "__leading"},
		{"double__under", "double__under"},
		// Unicode and CJK non-word characters.
		{"CJK汉字Test", "cjk_汉字_test"},
	}
	for _, c := range cases {
		if got := SnakeCase(c.in); got != c.want {
			t.Errorf("SnakeCase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestKebabCase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"FirstName", "first-name"},
		{"HTTPServer", "http-server"},
		{"HTTP20xOK", "http-20x-ok"},
		{"Bld4Floor", "bld4-floor"},
		{"IPv4Address", "i-pv4-address"},
		{"user_id", "user-id"},
		{"already snake_case", "already-snake-case"},
		{"__leading", "--leading"},
	}
	for _, c := range cases {
		if got := KebabCase(c.in); got != c.want {
			t.Errorf("KebabCase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestToDatabaseColumnName pins the canonical DB column algorithm so the
// template `snakecase` (which diverges on acronym+digit runs, e.g.
// "IPv4Address" -> "i_pv4_address") never silently replaces it.
func TestToDatabaseColumnName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"firstName", "first_name"},
		{"FirstName", "first_name"},
		{"HTTPPort", "http_port"},
		{"userID", "user_id"},
		{"user_id", "user_id"},
		{"kebab-case", "kebab_case"},
		{"spaced name", "spaced_name"},
		{"ALLCAPS", "allcaps"},
		{"ProductV2", "product_v2"},
		{"myURLParser", "my_url_parser"},
		// Acronym+digit run stays glued ("i_pv4address") — see CLAUDE.md;
		// this is why bridges must print DatabaseColumnName instead of
		// re-deriving it with `snakecase`.
		{"IPv4Address", "i_pv4address"},
		// Empty parts from leading/trailing separators are dropped.
		{"__leading", "leading"},
		{"trailing_", "trailing"},
		{"double__under", "double_under"},
	}
	for _, c := range cases {
		if got := ToDatabaseColumnName(c.in); got != c.want {
			t.Errorf("ToDatabaseColumnName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
