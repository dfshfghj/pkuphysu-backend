package db

import (
	"time"

	"pkuphysu-backend/internal/model"
)

type ForumReportGroup struct {
	TargetType       string
	TargetID         uint
	ReportCount      int64
	LatestReportedAt time.Time
}

func CreateForumReport(report *model.ForumReport) error {
	if report.Status == "" {
		report.Status = model.ForumReportStatusPending
	}
	return db.Create(report).Error
}

func HasPendingForumReportByUser(reporterID uint, targetType string, targetID uint) (bool, error) {
	var count int64
	err := db.Model(&model.ForumReport{}).
		Where("reporter_id = ? AND target_type = ? AND target_id = ? AND status = ?", reporterID, targetType, targetID, model.ForumReportStatusPending).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func CountPendingForumReports(targetType string, targetID uint) (int64, error) {
	var count int64
	err := db.Model(&model.ForumReport{}).
		Where("target_type = ? AND target_id = ? AND status = ?", targetType, targetID, model.ForumReportStatusPending).
		Count(&count).Error
	return count, err
}

func ListPendingForumReportGroups(limit int) ([]ForumReportGroup, error) {
	var groups []ForumReportGroup
	err := db.Model(&model.ForumReport{}).
		Select("target_type, target_id, COUNT(*) AS report_count, MAX(created_at) AS latest_reported_at").
		Where("status = ?", model.ForumReportStatusPending).
		Group("target_type, target_id").
		Order("report_count DESC, latest_reported_at DESC").
		Limit(limit).
		Scan(&groups).Error
	return groups, err
}

func GetForumReportsByTarget(targetType string, targetID uint, limit int) ([]model.ForumReport, error) {
	var reports []model.ForumReport
	query := db.Preload("Reporter").
		Where("target_type = ? AND target_id = ? AND status = ?", targetType, targetID, model.ForumReportStatusPending).
		Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&reports).Error
	return reports, err
}

func ResolveForumReports(targetType string, targetID uint, reviewerID uint) error {
	now := time.Now()
	return db.Model(&model.ForumReport{}).
		Where("target_type = ? AND target_id = ? AND status = ?", targetType, targetID, model.ForumReportStatusPending).
		Updates(map[string]interface{}{
			"status":      model.ForumReportStatusResolved,
			"reviewed_by": reviewerID,
			"reviewed_at": now,
		}).Error
}
