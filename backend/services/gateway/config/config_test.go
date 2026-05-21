//go:build unit

package config

import (
	"testing"
)

func TestParseAuthRule_None(t *testing.T) {
	r := ParseAuthRule("none")
	if r.Required || len(r.Roles) > 0 {
		t.Error("expected empty auth rule for 'none'")
	}
}

func TestParseAuthRule_Required(t *testing.T) {
	r := ParseAuthRule("required")
	if !r.Required {
		t.Error("expected Required=true")
	}
	if len(r.Roles) > 0 {
		t.Error("expected no roles")
	}
}

func TestParseAuthRule_RequiredRoles(t *testing.T) {
	tests := []struct {
		input string
		roles []string
	}{
		{`"required_roles:[user]"`, []string{"user"}},
		{`"required_roles:[driver]"`, []string{"driver"}},
		{`"required_roles:[driver,admin]"`, []string{"driver", "admin"}},
		{`required_roles:[user,driver]`, []string{"user", "driver"}},
	}

	for _, tc := range tests {
		r := ParseAuthRule(tc.input)
		if !r.Required {
			t.Errorf("%q: expected Required=true", tc.input)
		}
		if len(r.Roles) != len(tc.roles) {
			t.Errorf("%q: expected %d roles, got %d", tc.input, len(tc.roles), len(r.Roles))
			continue
		}
		for i, role := range tc.roles {
			if r.Roles[i] != role {
				t.Errorf("%q: role[%d] expected %q, got %q", tc.input, i, role, r.Roles[i])
			}
		}
	}
}

func TestParseAuthRule_Empty(t *testing.T) {
	r := ParseAuthRule("")
	if r.Required {
		t.Error("empty string should not require auth")
	}
}
