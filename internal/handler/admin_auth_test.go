package handler

import (
	"testing"

	bizerr "github.com/rocyu-tech/rockgame/internal/errors"
)

func TestValidateAdminPassword(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		wantErr   bool
		errMsgLen int
	}{
		{"too short", "Abc1", true, 0},
		{"exactly 10 valid", "Admin12345", false, 0},
		{"missing uppercase", "admin123456", true, 0},
		{"missing lowercase", "ADMIN123456", true, 0},
		{"missing digit", "AdminPasswd", true, 0},
		{"all good mixed", "MyP@ssw0rd!", false, 0},
		{"boundary 9 chars", "Admin1234", true, 0},
		{"empty string", "", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAdminPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAdminPassword(%q) error = %v, wantErr %v", tt.password, err, tt.wantErr)
			}
			if err != nil {
				bizErr, ok := err.(*bizerr.BizError)
				if !ok {
					t.Errorf("expected *BizError, got %T", err)
				}
				if bizErr.Code != bizerr.CodeInvalidParams {
					t.Errorf("expected CodeInvalidParams (%d), got %d", bizerr.CodeInvalidParams, bizErr.Code)
				}
			}
		})
	}
}