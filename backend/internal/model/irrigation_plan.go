package model

import "time"

// IrrigationPlan models 灌溉计划 as an independently versioned aggregate. The fields
// cover ownership, operational context, evidence and measured risk so later
// changes naturally span persistence, service and UI layers.
type IrrigationPlan struct {
	BaseModel
	Facility    string    `json:"facility" gorm:"size:120;index"`
	Owner       string    `json:"owner" gorm:"size:120;index"`
	Category    string    `json:"category" gorm:"size:80;index"`
	RiskLevel   string    `json:"riskLevel" gorm:"size:32;index"`
	MetricValue float64   `json:"metricValue"`
	MetricUnit  string    `json:"metricUnit" gorm:"size:24"`
	EffectiveAt time.Time `json:"effectiveAt"`
	Evidence    string    `json:"evidence" gorm:"size:2000"`
	RelatedCode string    `json:"relatedCode" gorm:"size:64;index"`

	// 复测与生效指标是读取时推导的展示字段，不落库。审批值 MetricValue 保持不动，
	// 页面统一读 Effective*；Remeasure* 是本计划自己补登的那条复测。
	RemeasureMoisture     *float64   `json:"remeasureMoisture,omitempty" gorm:"-"`
	RemeasuredAt          *time.Time `json:"remeasuredAt,omitempty" gorm:"-"`
	EffectiveMetricValue  float64    `json:"effectiveMetricValue" gorm:"-"`
	EffectiveMetricUnit   string     `json:"effectiveMetricUnit" gorm:"-"`
	EffectiveMetricSource string     `json:"effectiveMetricSource" gorm:"-"`
	EffectiveMeasuredAt   *time.Time `json:"effectiveMeasuredAt,omitempty" gorm:"-"`
}

func (item *IrrigationPlan) GetBase() *BaseModel { return &item.BaseModel }

func (item IrrigationPlan) TableName() string { return "irrigation_plans" }

var IrrigationPlanInitialStatus = "draft"
