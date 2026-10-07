package model

import "time"

// PlanRemeasure models 现场复测 as an append-once subordinate record of an
// irrigation plan. The approved plan metric stays untouched; pages read the
// derived effective metric resolved from the newest remeasure per 分区.
type PlanRemeasure struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	PlanID     uint      `json:"planId" gorm:"uniqueIndex;not null"`
	Facility   string    `json:"facility" gorm:"size:120;index;not null"`
	MeasuredAt time.Time `json:"measuredAt" gorm:"not null"`
	Moisture   float64   `json:"moisture" gorm:"not null"`
	MetricUnit string    `json:"metricUnit" gorm:"size:24"`
	CreatedBy  string    `json:"createdBy" gorm:"size:80"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (item PlanRemeasure) TableName() string { return "plan_remeasures" }
