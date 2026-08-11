package provider

import "testing"

// TestFronteggOAuthURLs pins the derived URL shape. These paths come from portal and bootstrap
// behavior rather than a documented contract, so if Frontegg moves them this is the test that
// should fail rather than users silently getting stale URLs.
func TestFronteggOAuthURLs(t *testing.T) {
	appURL, loginURL := fronteggOAuthURLs("abc123def456.frontegg.com")

	if want := "https://abc123def456.frontegg.com/oauth/portal"; appURL != want {
		t.Errorf("app URL: want %q, got %q", want, appURL)
	}
	if want := "https://abc123def456.frontegg.com/oauth"; loginURL != want {
		t.Errorf("login URL: want %q, got %q", want, loginURL)
	}
}

// TestValueOr covers the per-field fallback: a supplied URL always wins, including an explicit
// empty string, and only a genuinely absent value falls back.
func TestValueOr(t *testing.T) {
	supplied := "https://storefront.example.com"
	empty := ""

	tests := []struct {
		name     string
		supplied *string
		fallback string
		want     string
	}{
		{"supplied value wins", &supplied, "https://derived.example.com", supplied},
		{"absent value falls back", nil, "https://derived.example.com", "https://derived.example.com"},
		{"explicit empty string is not a fallback", &empty, "https://derived.example.com", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := valueOr(test.supplied, test.fallback); got != test.want {
				t.Errorf("want %q, got %q", test.want, got)
			}
		})
	}
}
