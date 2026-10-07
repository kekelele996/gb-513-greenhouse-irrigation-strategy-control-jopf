package model

import "time"

// PlanRemeasure 是灌溉计划审批后现场补登的一次含水率复测。审批指标保留在
// IrrigationPlan.MetricValue 上不动；复测独立留痕，页面读取的“生效含水率”
// 由服务层按分区和复测时间另行派生。
type PlanRemeasure struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	PlanID       uint      `json:"planId" gorm:"uniqueIndex;not null"`
	ZoneCode     string    `json:"zoneCode" gorm:"size:64;index;not null"`
	MeasuredAt   time.Time `json:"measuredAt" gorm:"index;not null"`
	Moisture     float64   `json:"moisture"`
	RegisteredBy string    `json:"registeredBy" gorm:"size:80;index"`
	Evidence     string    `json:"evidence" gorm:"size:2000"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (PlanRemeasure) TableName() string { return "plan_remeasures" }
