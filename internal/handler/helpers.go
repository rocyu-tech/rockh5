package handler

import (
        "errors"
        "strconv"

        "github.com/gofiber/fiber/v2"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/middleware"
        "github.com/rocyu-tech/rockgame/pkg/database"
        "gorm.io/gorm"
)

// ParsePagination extracts page, pageSize, offset from query params.
func ParsePagination(c *fiber.Ctx) (page, pageSize, offset int) {
        page, _ = strconv.Atoi(c.Query("page", "1"))
        pageSize, _ = strconv.Atoi(c.Query("page_size", "20"))
        if page < 1 {
                page = 1
        }
        if pageSize < 1 || pageSize > 100 {
                pageSize = 20
        }
        offset = (page - 1) * pageSize
        return
}

// MustDB returns the database instance or sends an internal error response.
// Returns nil if DB is not available (caller should return immediately).
func MustDB(c *fiber.Ctx, logTag string) *gorm.DB {
        db := database.DB()
        if db == nil {
                middleware.LogError(c, logTag+".DB", errors.New("database not initialized"))
                return nil
        }
        return db
}

// SharedErrorHandler handles both BizError and fiber.Error correctly.
func SharedErrorHandler(c *fiber.Ctx, err error) error {
        if bizErr, ok := err.(*bizerr.BizError); ok {
                return c.Status(bizErr.HTTP).JSON(bizerr.ErrorResponse(bizErr))
        }
        code := fiber.StatusInternalServerError
        if e, ok := err.(*fiber.Error); ok {
                code = e.Code
        }
        return c.Status(code).JSON(bizerr.ErrorResponse(err))
}