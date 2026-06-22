package model

import "time"

// AdminUser represents the admin_user table
type AdminUser struct {
        ID           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
        Username     string     `gorm:"size:64;not null;uniqueIndex" json:"username"`
        PasswordHash string     `gorm:"size:128;not null;default:''" json:"-"`
        RealName     string     `gorm:"size:64;not null;default:''" json:"real_name"`
        Email        string     `gorm:"size:128;not null;default:''" json:"email"`
        Role         string     `gorm:"size:32;not null;default:'operator'" json:"role"`
        Status       int8       `gorm:"type:tinyint;not null;default:1" json:"status"`
        LastLoginAt  *time.Time `gorm:"column:last_login_at" json:"last_login_at"`
        LastLoginIP  string     `gorm:"column:last_login_ip;size:45;not null;default:''" json:"last_login_ip"`
        CreatedAt    time.Time  `json:"created_at"`
        UpdatedAt    time.Time  `json:"updated_at"`
}

func (AdminUser) TableName() string { return "admin_user" }

// AdminAuditLog represents the admin_audit_log table
type AdminAuditLog struct {
        ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
        AdminID    int64     `gorm:"not null" json:"admin_id"`
        AdminName  string    `gorm:"size:64;not null;default:''" json:"admin_name"`
        Action     string    `gorm:"size:64;not null" json:"action"`
        TargetType string    `gorm:"size:32;not null;default:''" json:"target_type"`
        TargetID   string    `gorm:"size:64;not null;default:''" json:"target_id"`
        Detail     string    `gorm:"type:text" json:"detail"`
        IP         string    `gorm:"size:45;not null;default:''" json:"ip"`
        CreatedAt  time.Time `json:"created_at"`
}

func (AdminAuditLog) TableName() string { return "admin_audit_log" }

// AdminRole represents the admin_role table
type AdminRole struct {
        ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
        Code        string    `gorm:"size:32;not null;uniqueIndex" json:"code"`
        Name        string    `gorm:"size:64;not null" json:"name"`
        Level       int       `gorm:"not null;default:10" json:"level"`
        Description string    `gorm:"size:256;not null;default:''" json:"description"`
        IsSystem    int8      `gorm:"type:tinyint;not null;default:0" json:"is_system"`
        Status      int8      `gorm:"type:tinyint;not null;default:1" json:"status"`
        SortOrder   int       `gorm:"not null;default:0" json:"sort_order"`
        CreatedAt   time.Time `json:"created_at"`
        UpdatedAt   time.Time `json:"updated_at"`
}

func (AdminRole) TableName() string { return "admin_role" }

// AdminRoleMenu represents the admin_role_menu table
type AdminRoleMenu struct {
        ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
        RoleID    int64     `gorm:"not null;uniqueIndex:uk_role_menu" json:"role_id"`
        MenuKey   string    `gorm:"size:64;not null;uniqueIndex:uk_role_menu" json:"menu_key"`
        CreatedAt time.Time `json:"created_at"`
}

func (AdminRoleMenu) TableName() string { return "admin_role_menu" }
