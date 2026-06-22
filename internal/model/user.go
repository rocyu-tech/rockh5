package model

import "time"

// User represents the users table (不分表)
type User struct {
        ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
        Email        string    `gorm:"size:128;not null;default:'';uniqueIndex" json:"email"`
        Phone        string    `gorm:"size:32;not null;default:''" json:"phone"`
        PhoneCode    string    `gorm:"size:8;not null;default:''" json:"phone_code"`
        PasswordHash string    `gorm:"size:128;not null;default:''" json:"-"`
        Nickname     string    `gorm:"size:64;not null;default:''" json:"nickname"`
        Avatar       string    `gorm:"size:512;not null;default:''" json:"avatar"`
        Status       int8      `gorm:"type:tinyint;not null;default:1" json:"status"` // 0=disabled 1=active
        KYCStatus    int8      `gorm:"type:tinyint;not null;default:0" json:"kyc_status"`
        KYCLevel     int8      `gorm:"type:tinyint;not null;default:0" json:"kyc_level"`
        Language     string    `gorm:"size:10;not null;default:'en'" json:"language"`
        Timezone     string    `gorm:"size:32;not null;default:'UTC'" json:"timezone"`
        LastLoginAt  *time.Time `json:"last_login_at"`
        LastLoginIP  string    `gorm:"size:45;not null;default:''" json:"last_login_ip"`
        CreatedAt    time.Time  `json:"created_at"`
        UpdatedAt    time.Time  `json:"updated_at"`
}

func (User) TableName() string {
        return "users"
}
