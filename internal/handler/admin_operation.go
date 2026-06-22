// Package handler provides admin HTTP handlers for operational content management.
// This file contains handlers for listing banners, promotional activities, and task definitions.
package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	bizerr "github.com/rocyu-tech/rockgame/internal/errors"
	"github.com/rocyu-tech/rockgame/internal/middleware"
	"github.com/rocyu-tech/rockgame/pkg/database"
	"github.com/rocyu-tech/rockgame/pkg/logger"
)

// BannerListItem represents a banner in the admin list
type BannerListItem struct {
	ID         int64  `json:"id"`
	LobbyID    int64  `json:"lobby_id"`
	ImageURL   string `json:"image_url"`
	LinkURL    string `json:"link_url"`
	Weight     int    `json:"weight"`
	TargetLang string `json:"target_lang"`
	Status     int8   `json:"status"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	CreatedAt  string `json:"created_at"`
}

// GetAdminBanners returns a paginated, filterable list of banners.
// Supports filtering by status. Ordered by ID descending (newest first).
func GetAdminBanners(c *fiber.Ctx) error {
	logger.Infof("[GetAdminBanners] start: status=%s page=%s page_size=%s",
		c.Query("status", ""), c.Query("page", "1"), c.Query("page_size", "20"))

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "GetAdminBanners.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	page, pageSize, offset := ParsePagination(c)

	status := c.Query("status", "")

	countQuery := db.Table("banner")
	if status != "" {
		countQuery = countQuery.Where("status = ?", status)
	}

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		middleware.LogError(c, "GetAdminBanners.Count", err)
		return bizerr.ErrInternal
	}

	var banners []BannerListItem
	query := db.Table("banner").Select("id, lobby_id, image_url, link_url, weight, target_lang, status, start_time, end_time, created_at")
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&banners).Error; err != nil {
		middleware.LogError(c, "GetAdminBanners.Find", err)
		return bizerr.ErrInternal
	}

	if banners == nil {
		banners = []BannerListItem{}
	}

	logger.Infof("[GetAdminBanners] success: total=%d page=%d page_size=%d returned=%d", total, page, pageSize, len(banners))
	return c.JSON(bizerr.SuccessResponse(&bizerr.PagedData{
		List:     banners,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  int64(page*pageSize) < total,
	}))
}

// ActivityListItem represents an activity in the admin list
type ActivityListItem struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	HandlerName string `json:"handler_name"`
	Status      int8   `json:"status"`
	Priority    int    `json:"priority"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	CreatedAt   string `json:"created_at"`
}

// GetAdminActivities returns a paginated, filterable list of promotional activity definitions.
// Supports filtering by status and type. Ordered by ID descending (newest first).
func GetAdminActivities(c *fiber.Ctx) error {
	logger.Infof("[GetAdminActivities] start: status=%s type=%s page=%s page_size=%s",
		c.Query("status", ""), c.Query("type", ""), c.Query("page", "1"), c.Query("page_size", "20"))

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "GetAdminActivities.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	page, pageSize, offset := ParsePagination(c)

	status := c.Query("status", "")
	actType := c.Query("type", "")

	countQuery := db.Table("activity_define")
	if status != "" {
		countQuery = countQuery.Where("status = ?", status)
	}
	if actType != "" {
		countQuery = countQuery.Where("type = ?", actType)
	}

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		middleware.LogError(c, "GetAdminActivities.Count", err)
		return bizerr.ErrInternal
	}

	var activities []ActivityListItem
	query := db.Table("activity_define").Select("id, name, type, handler_name, status, priority, start_time, end_time, created_at")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if actType != "" {
		query = query.Where("type = ?", actType)
	}

	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&activities).Error; err != nil {
		middleware.LogError(c, "GetAdminActivities.Find", err)
		return bizerr.ErrInternal
	}

	if activities == nil {
		activities = []ActivityListItem{}
	}

	logger.Infof("[GetAdminActivities] success: total=%d page=%d page_size=%d returned=%d", total, page, pageSize, len(activities))
	return c.JSON(bizerr.SuccessResponse(&bizerr.PagedData{
		List:     activities,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  int64(page*pageSize) < total,
	}))
}

// TaskListItem represents a task in the admin list
type TaskListItem struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Cycle       int    `json:"cycle"`
	TargetKey   string `json:"target_key"`
	TargetValue int    `json:"target_value"`
	RewardType  string `json:"reward_type"`
	RewardValue string `json:"reward_value"`
	SortOrder   int    `json:"sort_order"`
	Status      int8   `json:"status"`
	CreatedAt   string `json:"created_at"`
}

// GetAdminTasks returns a paginated, filterable list of task definitions.
// Supports filtering by status and type. Ordered by sort_order ASC then ID DESC,
// so manually-ordered tasks appear first, with newer tasks of the same sort_order appearing last.
func GetAdminTasks(c *fiber.Ctx) error {
	logger.Infof("[GetAdminTasks] start: status=%s type=%s page=%s page_size=%s",
		c.Query("status", ""), c.Query("type", ""), c.Query("page", "1"), c.Query("page_size", "20"))

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "GetAdminTasks.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	page, pageSize, offset := ParsePagination(c)

	status := c.Query("status", "")
	taskType := c.Query("type", "")

	countQuery := db.Table("task_define")
	if status != "" {
		countQuery = countQuery.Where("status = ?", status)
	}
	if taskType != "" {
		countQuery = countQuery.Where("type = ?", taskType)
	}

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		middleware.LogError(c, "GetAdminTasks.Count", err)
		return bizerr.ErrInternal
	}

	var tasks []TaskListItem
	query := db.Table("task_define").Select("id, name, type, cycle, target_key, target_value, reward_type, reward_value, sort_order, status, created_at")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if taskType != "" {
		query = query.Where("type = ?", taskType)
	}

	if err := query.Order("sort_order ASC, id DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error; err != nil {
		middleware.LogError(c, "GetAdminTasks.Find", err)
		return bizerr.ErrInternal
	}

	if tasks == nil {
		tasks = []TaskListItem{}
	}

	logger.Infof("[GetAdminTasks] success: total=%d page=%d page_size=%d returned=%d", total, page, pageSize, len(tasks))
	return c.JSON(bizerr.SuccessResponse(&bizerr.PagedData{
		List:     tasks,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  int64(page*pageSize) < total,
	}))
}
