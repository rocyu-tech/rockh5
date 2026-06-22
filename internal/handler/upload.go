package handler

// upload.go — Image upload and deletion handlers for the admin panel.
//
// Endpoints:
//   - UploadImage: multipart image upload with content-type sniffing
//   - DeleteImage: delete a previously uploaded image by URL path

import (
        "crypto/rand"
        "encoding/hex"
        "fmt"
        "io"
        "net/http"
        "os"
        "path/filepath"
        "strings"
        "time"

        "github.com/gofiber/fiber/v2"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/middleware"
        "github.com/rocyu-tech/rockgame/pkg/logger"
)

// UploadConfig holds upload configuration
type UploadConfig struct {
        // BaseDir is the root directory for uploaded files (default: "uploads")
        BaseDir string
        // MaxSize is the max file size in bytes (default: 5MB)
        MaxSize int64
        // AllowedTypes maps allowed MIME types to file extensions
        AllowedTypes map[string]string
}

// DefaultUploadConfig returns the default upload configuration for admin panel images.
// Restricts uploads to 5MB and common image formats (jpeg, png, gif, webp, svg).
func DefaultUploadConfig() UploadConfig {
        return UploadConfig{
                BaseDir: "uploads/admin",
                MaxSize: 5 * 1024 * 1024, // 5MB
                AllowedTypes: map[string]string{
                        "image/jpeg":    ".jpg",
                        "image/png":     ".png",
                        "image/gif":     ".gif",
                        "image/webp":    ".webp",
                        "image/svg+xml": ".svg",
                },
        }
}

// UploadImage handles generic image upload for admin panel (vendor logo, category icon, etc.)
//
// WHAT: Accepts a multipart form file upload, validates size and content type,
// persists to disk under a date-partitioned directory, and returns the public URL.
//
// Security considerations:
//   - File type is detected by sniffing the first 512 bytes via http.DetectContentType
//     rather than trusting the client-supplied Content-Type header.
//   - Filenames are randomized (timestamp + crypto/rand hex) to prevent collisions
//     and eliminate path-injection risks from user-provided names.
//
// POST /api/v1/admin/upload/image
// Form field: "file"
// Returns: { code: 0, data: { url: "/uploads/admin/xxx.png" } }
func UploadImage(c *fiber.Ctx) error {
        logger.Infof("[UploadImage] start: admin_id=%d", middleware.GetUserID(c))

        file, err := c.FormFile("file")
        if err != nil {
                middleware.LogWarn(c, "UploadImage.FormFile", err.Error())
                return bizerr.New(bizerr.CodeInvalidParams, "file is required")
        }

        cfg := DefaultUploadConfig()

        // Validate file size
        if file.Size > cfg.MaxSize {
                middleware.LogWarn(c, "UploadImage.TooLarge", fmt.Sprintf("file size %d exceeds limit %d", file.Size, cfg.MaxSize))
                return bizerr.New(bizerr.CodeInvalidParams, fmt.Sprintf("file must be under %dMB", cfg.MaxSize/1024/1024))
        }

        // H5: validate file type by sniffing content (http.DetectContentType) instead of
        // trusting the client-supplied Content-Type header which can be spoofed.
        src, err := file.Open()
        if err != nil {
                middleware.LogError(c, "UploadImage.Open", err)
                return bizerr.ErrInternal
        }

        // Read first 512 bytes for content sniffing
        sniffBuf := make([]byte, 512)
        n, err := src.Read(sniffBuf)
        if err != nil {
                src.Close()
                middleware.LogError(c, "UploadImage.ReadSniff", err)
                return bizerr.ErrInternal
        }
        actualType := http.DetectContentType(sniffBuf[:n])

        // Map detected MIME types to allowed extensions
        var ext string
        switch actualType {
        case "image/jpeg":
                ext = ".jpg"
        case "image/png":
                ext = ".png"
        case "image/gif":
                ext = ".gif"
        case "image/webp":
                ext = ".webp"
        case "image/svg+xml":
                ext = ".svg"
        default:
                src.Close()
                middleware.LogWarn(c, "UploadImage.DetectedType", actualType)
                return bizerr.New(bizerr.CodeInvalidParams, "unsupported file type (detected: "+actualType+"), allowed: jpeg, png, gif, webp, svg")
        }

        // Generate unique filename: {date}/{timestamp}_{random}.{ext}
        // Using timestamp + random hex avoids filename collisions without exposing user-provided names.
        dateDir := time.Now().Format("20060102")
        filename := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), randomString(8), ext)
        relPath := filepath.Join(cfg.BaseDir, dateDir, filename)
        savePath := relPath

        // Create directory
        dir := filepath.Dir(savePath)
        if err := os.MkdirAll(dir, 0755); err != nil {
                middleware.LogError(c, "UploadImage.Mkdir", err)
                return bizerr.ErrInternal
        }

        dst, err := os.Create(savePath)
        if err != nil {
                src.Close()
                middleware.LogError(c, "UploadImage.Create", err)
                return bizerr.ErrInternal
        }
        defer dst.Close()

        // Write the already-read sniff bytes first to preserve the full file content
        if _, err := dst.Write(sniffBuf[:n]); err != nil {
                src.Close()
                middleware.LogError(c, "UploadImage.WriteHead", err)
                os.Remove(savePath)
                return bizerr.ErrInternal
        }

        // Copy the remaining bytes from the source reader
        if _, err := io.Copy(dst, src); err != nil {
                src.Close()
                middleware.LogError(c, "UploadImage.Copy", err)
                os.Remove(savePath)
                return bizerr.ErrInternal
        }
        src.Close()

        // Build URL path (always use forward slashes, starting with /)
        url := "/" + strings.ReplaceAll(relPath, "\\", "/")

        logger.Infof("[UploadImage] completed: admin_id=%d file=%s size=%d url=%s",
                middleware.GetUserID(c), file.Filename, file.Size, url)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "url": url,
        }))
}

// DeleteImage deletes an uploaded image file from disk.
//
// WHAT: Removes the file at the given URL path after validating that the path
// is contained within the "uploads/" directory to prevent arbitrary file deletion.
//
// Security considerations:
//   - The path is stripped of its leading slash, then checked for a "uploads/" prefix.
//   - filepath.Clean is applied to resolve any encoded or redundant separators
//     before a secondary ".." traversal check.
//   - If the file is already missing (os.IsNotExist), the endpoint returns success
//     idempotently rather than exposing whether the file existed.
//
// DELETE /api/v1/admin/upload/image?url=/uploads/admin/xxx.png
func DeleteImage(c *fiber.Ctx) error {
        logger.Infof("[DeleteImage] start: admin_id=%d url=%s", middleware.GetUserID(c), c.Query("url"))

        fileURL := c.Query("url")
        if fileURL == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "url query parameter is required")
        }

        // Security: only allow deleting files under uploads/ directory
        // Remove leading slash
        cleanPath := strings.TrimPrefix(fileURL, "/")
        if cleanPath == "" || !strings.HasPrefix(cleanPath, "uploads/") {
                middleware.LogWarn(c, "DeleteImage.InvalidPath", fileURL)
                return bizerr.New(bizerr.CodeInvalidParams, "invalid file path")
        }

        // H6: use filepath.Clean to normalize the path before traversal check
        cleanPath = filepath.Clean(cleanPath)

        // Prevent directory traversal (check again after normalization)
        if strings.Contains(cleanPath, "..") {
                middleware.LogWarn(c, "DeleteImage.Traversal", fileURL)
                return bizerr.New(bizerr.CodeInvalidParams, "invalid file path")
        }

        if err := os.Remove(cleanPath); err != nil {
                if os.IsNotExist(err) {
                        // File already gone, return success silently
                        return c.JSON(bizerr.SuccessResponse(nil))
                }
                middleware.LogError(c, "DeleteImage.Remove", err)
                return bizerr.ErrInternal
        }

        logger.Infof("[DeleteImage] completed: admin_id=%d url=%s", middleware.GetUserID(c), fileURL)
        return c.JSON(bizerr.SuccessResponse(nil))
}

// randomString generates a random hex string of given length using crypto/rand.
// Used as part of the filename to ensure uniqueness and prevent guessing.
func randomString(length int) string {
        b := make([]byte, length)
        _, _ = rand.Read(b)
        return hex.EncodeToString(b)[:length]
}