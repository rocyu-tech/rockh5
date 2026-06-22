// Package handler contains HTTP handler functions for the RockGame API.
//
// This file (auth.go) implements user authentication and account management handlers:
//   - Register:       new user registration with email/password
//   - Login:          user authentication with credential verification
//   - RefreshToken:   JWT access/refresh token rotation
//   - Logout:         token revocation and cookie clearing
//   - GetProfile / UpdateProfile:  user profile retrieval and modification
//   - ChangePassword: authenticated password change with re-login enforcement
//   - DeleteAccount:  account deactivation (soft delete) with password confirmation
//   - UploadAvatar:   avatar image upload with content-type validation
//   - GetAssets:      user wallet balance query
//
// All handlers return structured JSON responses via the bizerr package.
// Tokens are delivered both in httpOnly cookies (primary) and response body (backward compat).
package handler

import (
        "errors"
        "fmt"
        "net/mail"
        "os"
        "path/filepath"
        "regexp"
        "time"
        "unicode/utf8"

        "github.com/gofiber/fiber/v2"
        "github.com/rocyu-tech/rockgame/internal/config"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/middleware"
        "github.com/rocyu-tech/rockgame/internal/model"
        "github.com/rocyu-tech/rockgame/pkg/auth"
        "github.com/rocyu-tech/rockgame/pkg/cache"
        "github.com/rocyu-tech/rockgame/pkg/database"
        "github.com/rocyu-tech/rockgame/pkg/logger"
        "gorm.io/gorm"
)

// setAuthCookies sets httpOnly, SameSite=Lax cookies for access_token and refresh_token.
// This provides XSS protection (httpOnly) and CSRF protection (SameSite=Lax blocks cross-site POSTs).
// Secure flag is enabled only in production to allow HTTP in dev/test environments.
func setAuthCookies(c *fiber.Ctx, accessToken, refreshToken string, accessTTLSeconds, refreshTTLSeconds int) {
        secure := config.C().App.Env == "prod"
        c.Cookie(&fiber.Cookie{
                Name:     "access_token",
                Value:    accessToken,
                Path:     "/",
                HTTPOnly: true,
                Secure:   secure,
                SameSite: "Lax",
                MaxAge:   accessTTLSeconds,
        })
        c.Cookie(&fiber.Cookie{
                Name:     "refresh_token",
                Value:    refreshToken,
                Path:     "/",
                HTTPOnly: true,
                Secure:   secure,
                SameSite: "Lax",
                MaxAge:   refreshTTLSeconds,
        })
}

// ── Request types ──

// RegisterRequest defines the registration request body
type RegisterRequest struct {
        Email    string `json:"email"`
        Password string `json:"password"`
        Nickname string `json:"nickname"`
}

// LoginRequest defines the login request body
type LoginRequest struct {
        Email    string `json:"email"`
        Password string `json:"password"`
}

// RefreshRequest defines the token refresh request body
type RefreshRequest struct {
        RefreshToken string `json:"refresh_token"`
}

// ── Validation helpers ──

// emailRegex matches standard email format
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// isValidEmail validates email format using both net/mail.ParseAddress and regex.
// The dual check provides defense-in-depth: ParseAddress handles RFC 5322 edge cases
// while the regex enforces a simpler pattern suitable for user-facing validation.
func isValidEmail(email string) bool {
        if len(email) == 0 || len(email) > 128 {
                return false
        }
        _, err := mail.ParseAddress(email)
        return err == nil && emailRegex.MatchString(email)
}

// isPasswordValid checks password complexity requirements.
// Requires: 8-64 chars, at least one letter and one digit.
func isPasswordValid(password string) bool {
        length := utf8.RuneCountInString(password)
        if length < 8 || length > 64 {
                return false
        }
        var hasLetter, hasDigit bool
        for _, r := range password {
                if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
                        hasLetter = true
                }
                if r >= '0' && r <= '9' {
                        hasDigit = true
                }
                if hasLetter && hasDigit {
                        return true
                }
        }
        return false
}

// ── Auth Handlers ──

// Register handles user registration.
// Flow: validate input → check duplicate → hash password → create user → generate JWT pair → set cookies.
// Uses database-level unique constraint as a race-condition safeguard for duplicate email detection.
func Register(c *fiber.Ctx) error {
        var req RegisterRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "Register.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        logger.Infof("[Register] request received: email=%s", req.Email)

        // Validate email format
        if !isValidEmail(req.Email) {
                middleware.LogWarn(c, "Register.ValidateEmail", "invalid email format: "+req.Email)
                return bizerr.New(bizerr.CodeInvalidParams, "invalid email format")
        }

        // Validate password complexity
        if !isPasswordValid(req.Password) {
                middleware.LogWarn(c, "Register.ValidatePassword", "password length out of range")
                return bizerr.New(bizerr.CodeInvalidParams, "password must be 8-64 characters")
        }

        cfg := config.C()
        db := database.DB()
        if db == nil {
                middleware.LogError(c, "Register.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Check if user already exists (application-level check; DB constraint handles race conditions)
        var existing model.User
        if err := db.Where("email = ?", req.Email).First(&existing).Error; err == nil {
                middleware.LogWarn(c, "Register.DuplicateCheck", "user already exists: "+req.Email)
                return bizerr.ErrUserExists
        } else if !errors.Is(err, gorm.ErrRecordNotFound) {
                middleware.LogError(c, "Register.DuplicateCheck", err)
                return bizerr.ErrInternal
        }

        // Hash password using bcrypt (cost factor configured in auth package)
        hash, err := auth.HashPassword(req.Password)
        if err != nil {
                middleware.LogError(c, "Register.HashPassword", err)
                return bizerr.ErrInternal
        }

        // Default nickname to email if not provided
        nickname := req.Nickname
        if nickname == "" {
                nickname = req.Email
        }

        // Create user (use Create with error handling for race condition)
        user := model.User{
                Email:        req.Email,
                PasswordHash: hash,
                Nickname:     nickname,
                Status:       model.StatusActive,
                Language:     cfg.App.Language,
        }

        if err := db.Create(&user).Error; err != nil {
                // Handle duplicate key error from database (race condition protection)
                if isDuplicateKeyError(err) {
                        middleware.LogWarn(c, "Register.Create", "duplicate key on insert (race condition): "+req.Email)
                        return bizerr.ErrUserExists
                }
                middleware.LogError(c, "Register.Create", err)
                return bizerr.ErrInternal
        }

        // Generate tokens
        accessToken, err := auth.GenerateToken(cfg.JWT.ActiveSecrets(), user.ID, "", cfg.JWT.AccessTTL)
        if err != nil {
                middleware.LogError(c, "Register.GenerateToken", err)
                return bizerr.ErrInternal
        }
        refreshToken, err := auth.GenerateRefreshToken(cfg.JWT.ActiveSecrets(), user.ID, cfg.JWT.RefreshTTL)
        if err != nil {
                middleware.LogError(c, "Register.GenerateRefreshToken", err)
                return bizerr.ErrInternal
        }

        // Set httpOnly cookies for XSS/CSRF protection (tokens also returned in body for backward compat)
        setAuthCookies(c, accessToken, refreshToken, cfg.JWT.AccessTTL*60, cfg.JWT.RefreshTTL*86400)

        logger.Infof("[Register] registration succeeded: user_id=%d email=%s", user.ID, user.Email)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "user_id":       user.ID,
                "email":         user.Email,
                "nickname":      user.Nickname,
                "access_token":  accessToken,
                "refresh_token": refreshToken,
                "token_type":    "Bearer",
                "expires_in":    cfg.JWT.AccessTTL * 60,
        }))
}

// Login handles user login.
// Flow: validate input → find user → check account status → verify password → update last login → generate JWT pair → set cookies.
// Password verification uses bcrypt's constant-time comparison to prevent timing attacks.
func Login(c *fiber.Ctx) error {
        var req LoginRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "Login.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        logger.Infof("[Login] request received: email=%s", req.Email)

        if req.Email == "" || req.Password == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "email and password are required")
        }

        cfg := config.C()
        db := database.DB()
        if db == nil {
                middleware.LogError(c, "Login.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Find user by email
        var user model.User
        if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        middleware.LogWarn(c, "Login.FindUser", "user not found: "+req.Email)
                        return bizerr.ErrUserNotFound
                }
                middleware.LogError(c, "Login.FindUser", err)
                return bizerr.ErrInternal
        }

        // Check account status — disabled accounts cannot log in
        if user.Status == model.StatusInactive {
                middleware.LogWarn(c, "Login.StatusCheck", "account disabled")
                return bizerr.ErrAccountDisabled
        }

        // Verify password (constant-time comparison via bcrypt)
        if !auth.CheckPassword(req.Password, user.PasswordHash) {
                middleware.LogWarn(c, "Login.CheckPassword", "invalid password attempt")
                return bizerr.ErrInvalidPassword
        }

        // Update last login (fire-and-forget, don't block response)
        now := time.Now()
        clientIP := c.IP()
        go func() {
                if err := database.DB().Model(&user).Updates(map[string]interface{}{
                        "last_login_at": now,
                        "last_login_ip": clientIP,
                }).Error; err != nil {
                        logger.Warnf("[WARN] Login.UpdateLastLogin failed: user_id=%d err=%v", user.ID, err)
                }
        }()

        // Generate tokens
        accessToken, err := auth.GenerateToken(cfg.JWT.ActiveSecrets(), user.ID, "", cfg.JWT.AccessTTL)
        if err != nil {
                middleware.LogError(c, "Login.GenerateToken", err)
                return bizerr.ErrInternal
        }
        refreshToken, err := auth.GenerateRefreshToken(cfg.JWT.ActiveSecrets(), user.ID, cfg.JWT.RefreshTTL)
        if err != nil {
                middleware.LogError(c, "Login.GenerateRefreshToken", err)
                return bizerr.ErrInternal
        }

        // Set httpOnly cookies for XSS/CSRF protection (tokens also returned in body for backward compat)
        setAuthCookies(c, accessToken, refreshToken, cfg.JWT.AccessTTL*60, cfg.JWT.RefreshTTL*86400)

        logger.Infof("[Login] login succeeded: user_id=%d email=%s ip=%s", user.ID, user.Email, clientIP)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "user_id":       user.ID,
                "email":         user.Email,
                "nickname":      user.Nickname,
                "avatar":        user.Avatar,
                "access_token":  accessToken,
                "refresh_token": refreshToken,
                "token_type":    "Bearer",
                "expires_in":    cfg.JWT.AccessTTL * 60,
        }))
}

// RefreshToken handles token refresh using RefreshClaims struct (consistent with GenerateRefreshToken).
// Flow: parse and validate refresh token → generate new access + refresh token pair (rotation).
// Token rotation prevents refresh token reuse: old refresh tokens remain valid until expiry
// but the client should always use the latest one.
func RefreshToken(c *fiber.Ctx) error {
        logger.Infof("[RefreshToken] request received")

        var req RefreshRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "RefreshToken.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        if req.RefreshToken == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "refresh_token is required")
        }

        cfg := config.C()

        // Parse refresh token using RefreshClaims struct (supports key rotation)
        parsedToken, err := auth.ParseRefreshToken(req.RefreshToken, cfg.JWT.ActiveSecrets())
        if err != nil {
                if auth.IsTokenExpiredError(err) {
                        middleware.LogWarn(c, "RefreshToken.Parse", "refresh token expired")
                        return bizerr.ErrTokenExpired
                }
                middleware.LogError(c, "RefreshToken.Parse", err)
                return bizerr.ErrInvalidToken
        }
        claims, ok := parsedToken.Claims.(*auth.RefreshClaims)
        if !parsedToken.Valid || !ok || claims.Type != "refresh" {
                middleware.LogWarn(c, "RefreshToken.Validate", "invalid or non-refresh token")
                return bizerr.New(bizerr.CodeInvalidToken, "not a refresh token")
        }

        // Generate new access token and refresh token (rotation)
        accessToken, err := auth.GenerateToken(cfg.JWT.ActiveSecrets(), claims.UserID, "", cfg.JWT.AccessTTL)
        if err != nil {
                middleware.LogError(c, "RefreshToken.GenerateToken", err)
                return bizerr.ErrInternal
        }
        newRefreshToken, err := auth.GenerateRefreshToken(cfg.JWT.ActiveSecrets(), claims.UserID, cfg.JWT.RefreshTTL)
        if err != nil {
                middleware.LogError(c, "RefreshToken.GenerateRefreshToken", err)
                return bizerr.ErrInternal
        }

        // Set httpOnly cookies for XSS/CSRF protection
        setAuthCookies(c, accessToken, newRefreshToken, cfg.JWT.AccessTTL*60, cfg.JWT.RefreshTTL*86400)

        logger.Infof("[RefreshToken] token refreshed successfully: user_id=%d", claims.UserID)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "access_token":  accessToken,
                "refresh_token": newRefreshToken,
                "token_type":    "Bearer",
                "expires_in":    cfg.JWT.AccessTTL * 60,
        }))
}

// Logout handles user logout by revoking both access and refresh tokens.
// POST /api/v1/auth/logout
// Optional body: {"refresh_token": "..."} — if provided, the refresh token is also blacklisted.
// Always clears auth cookies regardless of whether token revocation succeeds.
func Logout(c *fiber.Ctx) error {
        logger.Infof("[Logout] request received")

        cfg := config.C()

        // Revoke the current access token
        middleware.RevokeCurrentToken(c, cfg.JWT.ActiveSecrets())

        // Optionally revoke the refresh token if provided in request body
        var req struct {
                RefreshToken string `json:"refresh_token"`
        }
        if len(c.Body()) > 0 {
                if err := c.BodyParser(&req); err == nil && req.RefreshToken != "" {
                        rdb := cache.Client()
                        if rdb != nil {
                                // Parse refresh token to determine remaining TTL
                                parsedToken, err := auth.ParseRefreshToken(req.RefreshToken, cfg.JWT.ActiveSecrets())
                                if err == nil && parsedToken.Valid {
                                        if claims, ok := parsedToken.Claims.(*auth.RefreshClaims); ok && claims.ExpiresAt != nil {
                                                remaining := time.Until(claims.ExpiresAt.Time)
                                                if remaining > 0 {
                                                        if err := auth.RevokeToken(c.Context(), rdb, req.RefreshToken, remaining); err != nil {
                                                                logger.Warnf("[Logout] failed to revoke refresh token: %v", err)
                                                        }
                                                }
                                        }
                                }
                        }
                }
        }

        // Clear httpOnly cookies — ensures client-side token removal even if revocation fails
        c.ClearCookie("access_token", "refresh_token")

        logger.Infof("[Logout] logout completed successfully")

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "logged_out": true,
        }))
}

// ── Account Handlers ──

// GetProfile returns the current user's profile.
// Requires authentication (user_id extracted from JWT via middleware).
func GetProfile(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                middleware.LogError(c, "GetProfile", errors.New("user_id not found in context"))
                return bizerr.ErrUnauthorized
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "GetProfile.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var user model.User
        if err := db.First(&user, userID).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        middleware.LogWarn(c, "GetProfile.FindUser", "user not found in db")
                        return bizerr.ErrUserNotFound
                }
                middleware.LogError(c, "GetProfile.FindUser", err)
                return bizerr.ErrInternal
        }

        // Check user status — disabled accounts should not be accessible
        if user.Status == model.StatusInactive {
                middleware.LogWarn(c, "GetProfile.StatusCheck", "account disabled")
                return bizerr.ErrAccountDisabled
        }

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "user_id":       user.ID,
                "email":         user.Email,
                "phone":         user.Phone,
                "nickname":      user.Nickname,
                "avatar":        user.Avatar,
                "status":        user.Status,
                "language":      user.Language,
                "timezone":      user.Timezone,
                "kyc_status":    user.KYCStatus,
                "last_login_at": user.LastLoginAt,
                "created_at":    user.CreatedAt,
        }))
}

// UpdateProfile updates the current user's profile (nickname, avatar, language, timezone).
// Uses partial update semantics: only non-nil fields in the request body are modified.
func UpdateProfile(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                middleware.LogError(c, "UpdateProfile", errors.New("user_id not found in context"))
                return bizerr.ErrUnauthorized
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "UpdateProfile.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Parse request body (partial update — only non-empty fields are updated)
        var input struct {
                Nickname *string `json:"nickname"`
                Avatar   *string `json:"avatar"`
                Language *string `json:"language"`
                Timezone *string `json:"timezone"`
        }
        if err := c.BodyParser(&input); err != nil {
                middleware.LogWarn(c, "UpdateProfile.Parse", err.Error())
                return bizerr.ErrInvalidParams
        }

        // Build updates map (only update provided fields)
        updates := make(map[string]interface{})
        if input.Nickname != nil {
                updates["nickname"] = *input.Nickname
        }
        if input.Avatar != nil {
                updates["avatar"] = *input.Avatar
        }
        if input.Language != nil {
                updates["language"] = *input.Language
        }
        if input.Timezone != nil {
                updates["timezone"] = *input.Timezone
        }

        if len(updates) == 0 {
                return bizerr.ErrInvalidParams
        }

        result := db.Model(&model.User{}).Where("id = ?", userID).Updates(updates)
        if result.Error != nil {
                middleware.LogError(c, "UpdateProfile.Update", result.Error)
                return bizerr.ErrInternal
        }
        if result.RowsAffected == 0 {
                middleware.LogWarn(c, "UpdateProfile.NotFound", "user not found")
                return bizerr.ErrUserNotFound
        }

        logger.Infof("[UpdateProfile] profile updated successfully: user_id=%d fields=%d", userID, len(updates))

        return c.JSON(bizerr.SuccessResponse(nil))
}

// ChangePassword changes the current user's password.
// Flow: validate new password rules → verify old password → hash new password → update DB → revoke current token.
// The current access token is revoked after password change to force re-login,
// ensuring any compromised sessions cannot continue with the old password.
func ChangePassword(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                middleware.LogError(c, "ChangePassword", errors.New("user_id not found in context"))
                return bizerr.ErrUnauthorized
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "ChangePassword.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var input struct {
                OldPassword    string `json:"old_password" validate:"required"`
                NewPassword    string `json:"new_password" validate:"required"`
                ConfirmPassword string `json:"confirm_password" validate:"required"`
        }
        if err := c.BodyParser(&input); err != nil {
                middleware.LogWarn(c, "ChangePassword.Parse", err.Error())
                return bizerr.ErrInvalidParams
        }

        // Validate input
        if input.OldPassword == "" || input.NewPassword == "" || input.ConfirmPassword == "" {
                return bizerr.ErrInvalidParams
        }
        if input.NewPassword != input.ConfirmPassword {
                middleware.LogWarn(c, "ChangePassword.Mismatch", "new password and confirm password do not match")
                return bizerr.New(bizerr.CodeInvalidParams, "passwords do not match")
        }
        if utf8.RuneCountInString(input.NewPassword) < 8 {
                middleware.LogWarn(c, "ChangePassword.TooShort", "new password too short")
                return bizerr.New(bizerr.CodeInvalidParams, "password must be at least 8 characters")
        }
        if !isPasswordValid(input.NewPassword) {
                middleware.LogWarn(c, "ChangePassword.Complexity", "new password lacks required complexity")
                return bizerr.New(bizerr.CodeInvalidParams, "password must contain at least one letter and one digit")
        }
        if input.OldPassword == input.NewPassword {
                middleware.LogWarn(c, "ChangePassword.SameAsOld", "new password same as old password")
                return bizerr.New(bizerr.CodeInvalidParams, "new password must be different from old password")
        }

        // Fetch current password hash (select only needed columns to avoid loading sensitive data)
        var user model.User
        if err := db.Select("id, password_hash").First(&user, userID).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.ErrUserNotFound
                }
                middleware.LogError(c, "ChangePassword.FindUser", err)
                return bizerr.ErrInternal
        }

        // Verify old password before allowing change
        if !auth.CheckPassword(input.OldPassword, user.PasswordHash) {
                middleware.LogWarn(c, "ChangePassword.WrongPassword", "old password incorrect")
                return bizerr.ErrInvalidPassword
        }

        // Hash and update new password
        newHash, err := auth.HashPassword(input.NewPassword)
        if err != nil {
                middleware.LogError(c, "ChangePassword.Hash", err)
                return bizerr.ErrInternal
        }

        if err := db.Model(&user).Update("password_hash", newHash).Error; err != nil {
                middleware.LogError(c, "ChangePassword.Update", err)
                return bizerr.ErrInternal
        }

        // Revoke current access token to force re-login with new password
        cfg := config.C()
        middleware.RevokeCurrentToken(c, cfg.JWT.ActiveSecrets())

        logger.Infof("[ChangePassword] password changed successfully: user_id=%d", userID)

        return c.JSON(bizerr.SuccessResponse(nil))
}

// DeleteAccount deactivates (soft deletes) the current user's account.
// Requires password confirmation to prevent unauthorized deletion.
// Sets user status to inactive rather than hard-deleting, preserving data for audit/troubleshooting.
// Revokes the current token so the deleted account cannot continue operating.
func DeleteAccount(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                middleware.LogError(c, "DeleteAccount", errors.New("user_id not found in context"))
                return bizerr.ErrUnauthorized
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "DeleteAccount.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var input struct {
                Password string `json:"password" validate:"required"`
        }
        if err := c.BodyParser(&input); err != nil {
                middleware.LogWarn(c, "DeleteAccount.Parse", err.Error())
                return bizerr.ErrInvalidParams
        }

        if input.Password == "" {
                return bizerr.ErrInvalidParams
        }

        // Verify password before deletion — prevents CSRF or token hijacking from deleting account
        var user model.User
        if err := db.Select("id, password_hash, status").First(&user, userID).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.ErrUserNotFound
                }
                middleware.LogError(c, "DeleteAccount.FindUser", err)
                return bizerr.ErrInternal
        }

        if !auth.CheckPassword(input.Password, user.PasswordHash) {
                middleware.LogWarn(c, "DeleteAccount.WrongPassword", "password incorrect")
                return bizerr.ErrInvalidPassword
        }

        // Set status to disabled (0) instead of hard delete
        if err := db.Model(&user).Update("status", model.StatusInactive).Error; err != nil {
                middleware.LogError(c, "DeleteAccount.Update", err)
                return bizerr.ErrInternal
        }

        // Revoke current token so deleted account can't continue operating
        deleteCfg := config.C()
        middleware.RevokeCurrentToken(c, deleteCfg.JWT.ActiveSecrets())

        logger.Infof("[DeleteAccount] account deactivated successfully: user_id=%d", userID)

        return c.JSON(bizerr.SuccessResponse(nil))
}

// UploadAvatar handles avatar upload via multipart form.
// Flow: validate file size (≤2MB) → validate Content-Type → verify magic bytes → save to disk → update DB.
// Uses magic byte verification to prevent Content-Type spoofing attacks.
// File extension is derived from the validated content type, never from the client-supplied filename.
func UploadAvatar(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                middleware.LogError(c, "UploadAvatar", errors.New("user_id not found in context"))
                return bizerr.ErrUnauthorized
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "UploadAvatar.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Get file from multipart form
        file, err := c.FormFile("avatar")
        if err != nil {
                middleware.LogWarn(c, "UploadAvatar.FormFile", err.Error())
                return bizerr.New(bizerr.CodeInvalidParams, "avatar file required")
        }

        // Validate file size (max 2MB)
        if file.Size > 2*1024*1024 {
                middleware.LogWarn(c, "UploadAvatar.TooLarge", "avatar file too large")
                return bizerr.New(bizerr.CodeInvalidParams, "avatar file must be under 2MB")
        }

        // Validate file type via Content-Type header
        allowedTypes := map[string]bool{
                "image/jpeg": true,
                "image/png":  true,
                "image/gif":  true,
                "image/webp": true,
        }
        contentType := file.Header.Get("Content-Type")
        if !allowedTypes[contentType] {
                middleware.LogWarn(c, "UploadAvatar.InvalidType", contentType)
                return bizerr.New(bizerr.CodeInvalidParams, "avatar must be jpeg, png, gif, or webp")
        }

        // Validate file content via magic bytes (prevent Content-Type spoofing)
        // Read first 512 bytes for magic number detection
        srcFile, err := file.Open()
        if err != nil {
                middleware.LogError(c, "UploadAvatar.OpenFile", err)
                return bizerr.ErrInternal
        }
        defer srcFile.Close()

        header := make([]byte, 512)
        n, err := srcFile.Read(header)
        if err != nil {
                middleware.LogError(c, "UploadAvatar.ReadHeader", err)
                return bizerr.ErrInternal
        }
        header = header[:n]

        if !validateImageMagic(header, contentType) {
                middleware.LogWarn(c, "UploadAvatar.MagicMismatch", "file content does not match declared type")
                return bizerr.New(bizerr.CodeInvalidParams, "avatar file content does not match declared type")
        }

        // Save file to uploads directory (placeholder — integrate with OSS/CDN in production)
        uploadDir := "uploads/avatars"
        // Use extension derived from validated content type, NOT from client-supplied filename
        // to prevent extension injection (e.g., uploading "malware.exe" disguised as jpeg)
        ext := contentTypeToExt(contentType)
        filename := fmt.Sprintf("%d_%d%s", userID, time.Now().UnixNano(), ext)
        savePath := filepath.Join(uploadDir, filename)

        if err := os.MkdirAll(uploadDir, 0755); err != nil {
                middleware.LogError(c, "UploadAvatar.Mkdir", err)
                return bizerr.ErrInternal
        }

        if err := c.SaveFile(file, savePath); err != nil {
                middleware.LogError(c, "UploadAvatar.Save", err)
                return bizerr.ErrInternal
        }

        // Update avatar URL in database
        avatarURL := "/" + savePath
        if err := db.Model(&model.User{}).Where("id = ?", userID).Update("avatar", avatarURL).Error; err != nil {
                middleware.LogError(c, "UploadAvatar.UpdateDB", err)
                return bizerr.ErrInternal
        }

        logger.Infof("[UploadAvatar] avatar uploaded successfully: user_id=%d avatar=%s size=%d", userID, avatarURL, file.Size)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "avatar": avatarURL,
        }))
}

// GetAssets returns the current user's wallet assets (bonus, coin, total, currency).
// If no wallet record exists for the user, returns zero balances rather than an error,
// allowing new users to see an empty wallet without requiring wallet initialization.
func GetAssets(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                middleware.LogError(c, "GetAssets", errors.New("user_id not found in context"))
                return bizerr.ErrUnauthorized
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "GetAssets.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var wallet struct {
                CashBalance  float64 `gorm:"column:cash_balance"`
                BonusBalance float64 `gorm:"column:bonus_balance"`
        }
        if err := db.Table("user_wallet").
                Select("cash_balance, bonus_balance").
                Where("user_id = ?", userID).
                Scan(&wallet).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        middleware.LogWarn(c, "GetAssets.WalletNotFound", "wallet not found for user")
                        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                                "bonus":    0,
                                "coin":     0,
                                "total":    0,
                                "currency": "USD",
                        }))
                }
                middleware.LogError(c, "GetAssets.Query", err)
                return bizerr.ErrInternal
        }

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "bonus":    wallet.BonusBalance,
                "coin":     wallet.CashBalance,
                "total":    wallet.CashBalance + wallet.BonusBalance,
                "currency": "USD",
        }))
}

// ── Internal helpers ──

// isDuplicateKeyError checks if a database error is a duplicate key violation.
// Works with MySQL (errno 1062) and common drivers.
func isDuplicateKeyError(err error) bool {
        // gorm wraps the original driver error
        if errors.Is(err, gorm.ErrDuplicatedKey) {
                return true
        }
        // Fallback: check error message for MySQL duplicate key
        errMsg := err.Error()
        if len(errMsg) >= 6 && errMsg[len(errMsg)-6:] == "Error 1062" {
                return true
        }
        if len(errMsg) >= 17 && errMsg[len(errMsg)-17:] == "Duplicate entry '" {
                return true
        }
        return false
}

// validateImageMagic checks if the file header bytes match the declared Content-Type.
// This prevents uploading malicious files with spoofed Content-Type headers.
func validateImageMagic(header []byte, contentType string) bool {
        switch contentType {
        case "image/jpeg":
                // JPEG: starts with FF D8 FF
                return len(header) >= 3 && header[0] == 0xFF && header[1] == 0xD8 && header[2] == 0xFF
        case "image/png":
                // PNG: starts with 89 50 4E 47 (0x89504E47)
                return len(header) >= 8 &&
                        header[0] == 0x89 && header[1] == 0x50 && header[2] == 0x4E && header[3] == 0x47 &&
                        header[4] == 0x0D && header[5] == 0x0A && header[6] == 0x1A && header[7] == 0x0A
        case "image/gif":
                // GIF: starts with "GIF87a" or "GIF89a"
                return len(header) >= 6 &&
                        (header[0] == 'G' && header[1] == 'I' && header[2] == 'F') &&
                        (header[3] == '8' && (header[4] == '7' || header[4] == '9') && header[5] == 'a')
        case "image/webp":
                // WebP: RIFF....WEBP
                return len(header) >= 12 &&
                        header[0] == 'R' && header[1] == 'I' && header[2] == 'F' && header[3] == 'F' &&
                        header[8] == 'W' && header[9] == 'E' && header[10] == 'B' && header[11] == 'P'
        default:
                return false
        }
}

// contentTypeToExt maps a validated content type to a safe file extension.
// Used instead of filepath.Ext(clientFilename) to prevent extension injection.
func contentTypeToExt(contentType string) string {
        switch contentType {
        case "image/jpeg":
                return ".jpg"
        case "image/png":
                return ".png"
        case "image/gif":
                return ".gif"
        case "image/webp":
                return ".webp"
        default:
                return ".bin"
        }
}
