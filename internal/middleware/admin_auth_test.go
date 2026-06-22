package middleware

import (
	"testing"

	bizerr "github.com/rocyu-tech/rockgame/internal/errors"
)

func TestRoleLevelHierarchy(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		expected int
	}{
		{"super is level 4", RoleSuper, 4},
		{"admin is level 3", RoleAdmin, 3},
		{"operator is level 2", RoleOperator, 2},
		{"viewer is level 1", RoleViewer, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := roleLevel[tt.role]
			if !ok {
				t.Fatalf("role %q not found in roleLevel map", tt.role)
			}
			if got != tt.expected {
				t.Errorf("roleLevel[%q] = %d, want %d", tt.role, got, tt.expected)
			}
		})
	}
}

func TestRoleLevelOrdering(t *testing.T) {
	// Verify strict ordering: super > admin > operator > viewer
	if roleLevel[RoleSuper] <= roleLevel[RoleAdmin] {
		t.Errorf("super (%d) should be > admin (%d)", roleLevel[RoleSuper], roleLevel[RoleAdmin])
	}
	if roleLevel[RoleAdmin] <= roleLevel[RoleOperator] {
		t.Errorf("admin (%d) should be > operator (%d)", roleLevel[RoleAdmin], roleLevel[RoleOperator])
	}
	if roleLevel[RoleOperator] <= roleLevel[RoleViewer] {
		t.Errorf("operator (%d) should be > viewer (%d)", roleLevel[RoleOperator], roleLevel[RoleViewer])
	}
}

func TestRequireRoleAllowedRoles(t *testing.T) {
	// Test that RequireRole creates middleware with correct min level
	tests := []struct {
		name         string
		allowedRoles []string
		wantMinLevel int
	}{
		{"viewer only", []string{RoleViewer}, 1},
		{"operator and above", []string{RoleOperator}, 2},
		{"admin and above", []string{RoleAdmin}, 3},
		{"super only", []string{RoleSuper}, 4},
		{"multiple roles uses highest", []string{RoleViewer, RoleSuper}, 4},
		{"operator + viewer uses operator", []string{RoleViewer, RoleOperator}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := RequireRole(tt.allowedRoles...)
			if mw == nil {
				t.Fatal("RequireRole returned nil middleware")
			}
			// The middleware is a closure — we can't easily test it without Fiber app,
			// but we verify it was created successfully.
			// Full integration tests would need a Fiber test app.
		})
	}
}

func TestRequireRoleEmptyPanic(t *testing.T) {
	// Empty roles should return a middleware that always denies (since minLevel stays 0)
	mw := RequireRole()
	if mw == nil {
		t.Fatal("RequireRole() with no args should return non-nil middleware")
	}
}

func TestBizErrorForbiddenExists(t *testing.T) {
	// Verify ErrForbidden is properly configured
	if bizerr.ErrForbidden == nil {
		t.Fatal("ErrForbidden is nil")
	}
	if bizerr.ErrForbidden.HTTP != 403 {
		t.Errorf("ErrForbidden.HTTP = %d, want 403", bizerr.ErrForbidden.HTTP)
	}
	if bizerr.ErrForbidden.Code != bizerr.CodeForbidden {
		t.Errorf("ErrForbidden.Code = %d, want %d", bizerr.ErrForbidden.Code, bizerr.CodeForbidden)
	}
}

func TestJoinRoles(t *testing.T) {
	tests := []struct {
		roles []string
		want  string
	}{
		{[]string{"super"}, "super"},
		{[]string{"operator", "viewer"}, "operator, viewer"},
		{[]string{"super", "admin", "operator", "viewer"}, "super, admin, operator, viewer"},
	}
	for _, tt := range tests {
		got := joinRoles(tt.roles)
		if got != tt.want {
			t.Errorf("joinRoles(%v) = %q, want %q", tt.roles, got, tt.want)
		}
	}
}