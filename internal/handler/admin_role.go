// Admin role management handlers.
//
// Provides CRUD for admin_role table and role-menu permission management.
// System built-in roles (is_system=1) cannot be deleted.
// Only super_admin can create/update/delete roles.
package handler

import (
        "errors"
        "strconv"
        "time"

        "github.com/gofiber/fiber/v2"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/middleware"
        "github.com/rocyu-tech/rockgame/internal/model"
        "github.com/rocyu-tech/rockgame/pkg/database"
        "github.com/rocyu-tech/rockgame/pkg/logger"
        "gorm.io/gorm"
)

// allMenuKeys is the complete list of menu keys matching the frontend Sidebar config.
// Parent keys and child keys are both listed; parent keys control visibility of
// the submenu group, child keys control individual menu items.
var allMenuKeys = []string{
        "/dashboard",
        "/users",
        "/games",
        "/games/list",
        "/games/vendors",
        "/games/categories",
        "/activities",
        "/orders",
        "/orders/recharge",
        "/orders/withdraw",
        "/ops",
        "/ops/banners",
        "/ops/mail",
        "/ops/tasks",
        "/system",
        "/system/payment",
        "/system/vip",
        "/system/admins",
        "/system/roles",
        "/audit-log",
        "/reports",
}

// parentMenuKeys maps child keys to their parent group keys.
var parentMenuKeys = map[string]string{
        "/games/list":       "/games",
        "/games/vendors":    "/games",
        "/games/categories": "/games",
        "/orders/recharge":  "/orders",
        "/orders/withdraw":  "/orders",
        "/ops/banners":      "/ops",
        "/ops/mail":         "/ops",
        "/ops/tasks":        "/ops",
        "/system/payment":   "/system",
        "/system/vip":       "/system",
        "/system/admins":    "/system",
        "/system/roles":     "/system",
}

// isValidMenuKey checks if a menu key is in the known menu set.
func isValidMenuKey(key string) bool {
        for _, k := range allMenuKeys {
                if k == key {
                        return true
                }
        }
        return false
}

// GetRoleMenus returns the menu keys assigned to a specific role.
//
// GET /api/v1/admin/system/roles/:id/menus
func GetRoleMenus(c *fiber.Ctx) error {
        roleID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || roleID <= 0 {
                return bizerr.ErrInvalidParams
        }

        db := database.DB()
        if db == nil {
                return bizerr.ErrInternal
        }

        // Verify role exists
        var role model.AdminRole
        if err := db.Select("id, code").First(&role, roleID).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.New(bizerr.CodeNotFound, "role not found")
                }
                return bizerr.ErrInternal
        }

        var menuKeys []string
        if err := db.Table("admin_role_menu").
                Where("role_id = ?", roleID).
                Pluck("menu_key", &menuKeys).Error; err != nil {
                middleware.LogError(c, "GetRoleMenus.Pluck", err)
                return bizerr.ErrInternal
        }

        if menuKeys == nil {
                menuKeys = []string{}
        }

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "role_id":   roleID,
                "role_code": role.Code,
                "menus":     menuKeys,
        }))
}

// SaveRoleMenus replaces the menu permissions for a role.
// Accepts a JSON body: {"menus": ["/dashboard", "/users", ...]}.
//
// PUT /api/v1/admin/system/roles/:id/menus
func SaveRoleMenus(c *fiber.Ctx) error {
        roleID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || roleID <= 0 {
                return bizerr.ErrInvalidParams
        }

        var req struct {
                Menus []string `json:"menus"`
        }
        if err := c.BodyParser(&req); err != nil {
                return bizerr.ErrInvalidParams
        }

        db := database.DB()
        if db == nil {
                return bizerr.ErrInternal
        }

        // Verify role exists
        var role model.AdminRole
        if err := db.Select("id, code").First(&role, roleID).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.New(bizerr.CodeNotFound, "role not found")
                }
                return bizerr.ErrInternal
        }

        // Validate all menu keys
        validMenus := make([]string, 0, len(req.Menus))
        for _, key := range req.Menus {
                if isValidMenuKey(key) {
                        validMenus = append(validMenus, key)
                }
        }

        // Use a transaction to atomically replace all menu permissions
        err = db.Transaction(func(tx *gorm.DB) error {
                // Delete existing entries
                if err := tx.Where("role_id = ?", roleID).Delete(&model.AdminRoleMenu{}).Error; err != nil {
                        return err
                }

                // Insert new entries
                if len(validMenus) > 0 {
                        records := make([]model.AdminRoleMenu, len(validMenus))
                        for i, key := range validMenus {
                                records[i] = model.AdminRoleMenu{
                                        RoleID:  roleID,
                                        MenuKey: key,
                                }
                        }
                        if err := tx.Create(&records).Error; err != nil {
                                return err
                        }
                }
                return nil
        })

        if err != nil {
                middleware.LogError(c, "SaveRoleMenus.Transaction", err)
                return bizerr.ErrInternal
        }

        // Audit log (capture IP before goroutine — c is recycled after handler returns)
        adminID := middleware.GetUserID(c)
        clientIP := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "role.save_menus", "admin_role", strconv.FormatInt(roleID, 10),
                        "save role menus: "+role.Code+" ("+strconv.Itoa(len(validMenus))+" items)", clientIP)
        }()

        logger.Infof("[SaveRoleMenus] success: role_id=%d code=%s menu_count=%d", roleID, role.Code, len(validMenus))
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "role_id":    roleID,
                "menu_count": len(validMenus),
        }))
}

// GetRoleMenusByCode returns the menu keys for a role identified by its code.
// Used by the login flow to return the admin's menu permissions.
func GetRoleMenusByCode(roleCode string) []string {
        db := database.DB()
        if db == nil {
                return nil
        }

        var roleID int64
        if err := db.Table("admin_role").Where("code = ? AND status = 1", roleCode).Select("id").Scan(&roleID).Error; err != nil || roleID == 0 {
                return nil
        }

        var menuKeys []string
        if err := db.Table("admin_role_menu").
                Where("role_id = ?", roleID).
                Pluck("menu_key", &menuKeys).Error; err != nil {
                return nil
        }

        return menuKeys
}

// RoleListItem is the DTO for role list responses.
type RoleListItem struct {
        ID          int64  `json:"id"`
        Code        string `json:"code"`
        Name        string `json:"name"`
        Level       int    `json:"level"`
        Description string `json:"description"`
        IsSystem    int8   `json:"is_system"`
        Status      int8   `json:"status"`
        SortOrder   int    `json:"sort_order"`
        CreatedAt   string `json:"created_at"`
}

// CreateRoleRequest is the request body for creating a role.
type CreateRoleRequest struct {
        Code        string `json:"code"`
        Name        string `json:"name"`
        Level       int    `json:"level"`
        Description string `json:"description"`
        SortOrder   int    `json:"sort_order"`
}

// UpdateRoleRequest is the request body for updating a role.
type UpdateRoleRequest struct {
        Name        *string `json:"name"`
        Level       *int    `json:"level"`
        Description *string `json:"description"`
        SortOrder   *int    `json:"sort_order"`
}

// GetAdminRoles returns all roles ordered by sort_order, then id.
//
// GET /api/v1/admin/system/roles
func GetAdminRoles(c *fiber.Ctx) error {
        logger.Infof("[GetAdminRoles] start")

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "GetAdminRoles.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var roles []RoleListItem
        if err := db.Table("admin_role").
                Select("id, code, name, level, description, is_system, status, sort_order, created_at").
                Order("sort_order ASC, id ASC").
                Find(&roles).Error; err != nil {
                middleware.LogError(c, "GetAdminRoles.Find", err)
                return bizerr.ErrInternal
        }

        if roles == nil {
                roles = []RoleListItem{}
        }

        logger.Infof("[GetAdminRoles] success: count=%d", len(roles))
        return c.JSON(bizerr.SuccessResponse(roles))
}

// GetActiveRoles returns only active roles (for dropdown selects).
//
// GET /api/v1/admin/system/roles/active
func GetActiveRoles(c *fiber.Ctx) error {
        db := database.DB()
        if db == nil {
                return bizerr.ErrInternal
        }

        var roles []RoleListItem
        if err := db.Table("admin_role").
                Select("id, code, name, level, description, is_system, status, sort_order, created_at").
                Where("status = 1").
                Order("sort_order ASC, id ASC").
                Find(&roles).Error; err != nil {
                middleware.LogError(c, "GetActiveRoles.Find", err)
                return bizerr.ErrInternal
        }

        if roles == nil {
                roles = []RoleListItem{}
        }

        return c.JSON(bizerr.SuccessResponse(roles))
}

// CreateAdminRole creates a new custom role.
//
// POST /api/v1/admin/system/roles
func CreateAdminRole(c *fiber.Ctx) error {
        logger.Infof("[CreateAdminRole] start")

        var req CreateRoleRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "CreateAdminRole.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        if req.Code == "" || req.Name == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "code and name are required")
        }
        if req.Level <= 0 {
                return bizerr.New(bizerr.CodeInvalidParams, "level must be a positive integer")
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "CreateAdminRole.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Check code uniqueness
        var count int64
        if err := db.Table("admin_role").Where("code = ?", req.Code).Count(&count).Error; err != nil {
                middleware.LogError(c, "CreateAdminRole.CheckCode", err)
                return bizerr.ErrInternal
        }
        if count > 0 {
                return bizerr.New(bizerr.CodeUserExists, "role code already exists")
        }

        role := model.AdminRole{
                Code:        req.Code,
                Name:        req.Name,
                Level:       req.Level,
                Description: req.Description,
                IsSystem:    0,
                Status:      1,
                SortOrder:   req.SortOrder,
        }

        if err := db.Create(&role).Error; err != nil {
                middleware.LogError(c, "CreateAdminRole.Create", err)
                return bizerr.ErrInternal
        }

        // Audit log
        adminID := middleware.GetUserID(c)
        go func() {
                RecordAuditLog(adminID, "", "role.create", "admin_role", req.Code,
                        "create role: "+req.Name, c.IP())
        }()

        logger.Infof("[CreateAdminRole] success: code=%s name=%s", req.Code, req.Name)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id":   role.ID,
                "code": role.Code,
        }))
}

// UpdateAdminRole updates a custom role. System roles can only have name/description updated.
//
// PUT /api/v1/admin/system/roles/:id
func UpdateAdminRole(c *fiber.Ctx) error {
        targetID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || targetID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[UpdateAdminRole] start: id=%d", targetID)

        var req UpdateRoleRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "UpdateAdminRole.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "UpdateAdminRole.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var role model.AdminRole
        if err := db.First(&role, targetID).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.New(bizerr.CodeNotFound, "role not found")
                }
                middleware.LogError(c, "UpdateAdminRole.Find", err)
                return bizerr.ErrInternal
        }

        updates := map[string]interface{}{"updated_at": time.Now()}
        if req.Name != nil {
                updates["name"] = *req.Name
        }
        if req.Description != nil {
                updates["description"] = *req.Description
        }
        if req.SortOrder != nil {
                updates["sort_order"] = *req.SortOrder
        }

        // Only allow level change for non-system roles
        if req.Level != nil && role.IsSystem == 0 {
                if *req.Level <= 0 {
                        return bizerr.New(bizerr.CodeInvalidParams, "level must be a positive integer")
                }
                updates["level"] = *req.Level
        }

        if err := db.Model(&role).Updates(updates).Error; err != nil {
                middleware.LogError(c, "UpdateAdminRole.Update", err)
                return bizerr.ErrInternal
        }

        // Audit log
        adminID := middleware.GetUserID(c)
        go func() {
                RecordAuditLog(adminID, "", "role.update", "admin_role", strconv.FormatInt(targetID, 10),
                        "update role: "+role.Code, c.IP())
        }()

        logger.Infof("[UpdateAdminRole] success: id=%d", targetID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{"id": targetID}))
}

// ToggleAdminRoleStatus flips a role's status (0/1). System roles cannot be disabled.
//
// PUT /api/v1/admin/system/roles/:id/status
func ToggleAdminRoleStatus(c *fiber.Ctx) error {
        targetID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || targetID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[ToggleAdminRoleStatus] start: id=%d", targetID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "ToggleAdminRoleStatus.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var role model.AdminRole
        if err := db.First(&role, targetID).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.New(bizerr.CodeNotFound, "role not found")
                }
                middleware.LogError(c, "ToggleAdminRoleStatus.Find", err)
                return bizerr.ErrInternal
        }

        if role.IsSystem == 1 {
                return bizerr.New(bizerr.CodeForbidden, "cannot disable system built-in roles")
        }

        newStatus := int8(1)
        if role.Status == 1 {
                newStatus = 0
        }

        if err := db.Model(&role).Update("status", newStatus).Error; err != nil {
                middleware.LogError(c, "ToggleAdminRoleStatus.Update", err)
                return bizerr.ErrInternal
        }

        logger.Infof("[ToggleAdminRoleStatus] success: id=%d new_status=%d", targetID, newStatus)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{"id": targetID, "status": newStatus}))
}

// DeleteAdminRole deletes a custom role. System roles and roles in use cannot be deleted.
//
// DELETE /api/v1/admin/system/roles/:id
func DeleteAdminRole(c *fiber.Ctx) error {
        targetID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || targetID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[DeleteAdminRole] start: id=%d", targetID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "DeleteAdminRole.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var role model.AdminRole
        if err := db.First(&role, targetID).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.New(bizerr.CodeNotFound, "role not found")
                }
                middleware.LogError(c, "DeleteAdminRole.Find", err)
                return bizerr.ErrInternal
        }

        if role.IsSystem == 1 {
                return bizerr.New(bizerr.CodeForbidden, "cannot delete system built-in roles")
        }

        // Check if any admin user is using this role
        var userCount int64
        if err := db.Table("admin_user").Where("role = ?", role.Code).Count(&userCount).Error; err != nil {
                middleware.LogError(c, "DeleteAdminRole.CheckUsers", err)
                return bizerr.ErrInternal
        }
        if userCount > 0 {
                return bizerr.New(bizerr.CodeInvalidParams, "cannot delete role: %d admin user(s) are using this role")
        }

        if err := db.Delete(&role).Error; err != nil {
                middleware.LogError(c, "DeleteAdminRole.Delete", err)
                return bizerr.ErrInternal
        }

        // Audit log
        adminID := middleware.GetUserID(c)
        go func() {
                RecordAuditLog(adminID, "", "role.delete", "admin_role", strconv.FormatInt(targetID, 10),
                        "delete role: "+role.Code, c.IP())
        }()

        logger.Infof("[DeleteAdminRole] success: id=%d code=%s", targetID, role.Code)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{"id": targetID}))
}