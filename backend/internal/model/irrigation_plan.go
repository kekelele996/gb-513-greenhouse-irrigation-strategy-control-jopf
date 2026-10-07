package model

import "time"

// IrrigationPlan models 灌溉计划 as an independently versioned aggregate. The fields
// cover ownership, operational context, evidence and measured risk so later
// changes naturally span persistence, service and UI layers.
type IrrigationPlan struct {
	BaseModel
	Facility    string  `json:"facility" gorm:"size:120;index"`
	Owner       string  `json:"owner" gorm:"size:120;index"`
	Category    string  `json:"category" gorm:"size:80;index"`
	RiskLevel   string  `json:"riskLevel" gorm:"size:32;index"`
	MetricValue float64 `json:"metricValue"`
	MetricUnit  string  `json:"metricUnit" gorm:"size:24"`
	// ApprovedMetricValue 在审批通过那一刻钉住当时的含水率，普通计划编辑不触碰；
	// 页面生效含水率的“审批回落值”读它而不是可编辑的 MetricValue。
	ApprovedMetricValue *float64  `json:"approvedMetricValue"`
	EffectiveAt         time.Time `json:"effectiveAt"`
	Evidence            string    `json:"evidence" gorm:"size:2000"`
	RelatedCode         string    `json:"relatedCode" gorm:"size:64;index"`

	// 生效含水率由服务层按分区复测派生，不入库；审批回落值始终保留可追溯。
	EffectiveMoisture     *float64   `json:"effectiveMoisture" gorm:"-"`
	EffectiveMoistureFrom string     `json:"effectiveMoistureFrom" gorm:"-"`
	RemeasuredAt          *time.Time `json:"remeasuredAt" gorm:"-"`
	RemeasureID           *uint      `json:"remeasureId" gorm:"-"`
	RemeasureRegistered   bool       `json:"remeasureRegistered" gorm:"-"`
}

func (item *IrrigationPlan) GetBase() *BaseModel { return &item.BaseModel }

func (item IrrigationPlan) TableName() string { return "irrigation_plans" }

var IrrigationPlanInitialStatus = "draft"
