package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/repository"
)

// 生效含水率来源取值：approved 表示沿用审批指标，remeasure 表示采用现场复测。
const (
	MoistureSourceApproved  = "approved"
	MoistureSourceRemeasure = "remeasure"
)

// decoratePlanMoisture 按“同分区、复测时间不早于生效时间、取最新一条”的规则
// 为一批计划填充生效含水率；无合格复测时回落到审批 MetricValue。
func decoratePlanMoisture(ctx context.Context, repo repository.PlanRemeasureRepository, plans []model.IrrigationPlan) error {
	if len(plans) == 0 {
		return nil
	}
	keys := make([]repository.ZoneCutoff, 0, len(plans))
	for _, plan := range plans {
		keys = append(keys, repository.ZoneCutoff{
			Key:      planKey(plan.ID),
			OwnPlan:  plan.ID,
			ZoneCode: plan.RelatedCode,
			Cutoff:   plan.EffectiveAt,
		})
	}
	effective, err := repo.EffectiveFor(ctx, keys)
	if err != nil {
		return fmt.Errorf("derive effective moisture for plans: %w", err)
	}
	registered, err := repo.RegisteredPlanIDs(ctx)
	if err != nil {
		return fmt.Errorf("load registered remeasure plan ids: %w", err)
	}
	for i := range plans {
		plans[i].RemeasureRegistered = registered[plans[i].ID]
		applyToPlan(&plans[i], effective[planKey(plans[i].ID)])
	}
	return nil
}

// decorateExecutionMoisture 阀门页复用同一规则：执行记录按自身分区与生效时间选
// 取合格复测；执行记录不与具体计划绑定，因此不回填 RemeasureRegistered。
func decorateExecutionMoisture(ctx context.Context, repo repository.PlanRemeasureRepository, executions []model.ValveExecution) error {
	if len(executions) == 0 {
		return nil
	}
	keys := make([]repository.ZoneCutoff, 0, len(executions))
	for _, execution := range executions {
		keys = append(keys, repository.ZoneCutoff{
			Key:      "execution-" + strconv.FormatUint(uint64(execution.ID), 10),
			OwnPlan:  0,
			ZoneCode: execution.RelatedCode,
			Cutoff:   execution.EffectiveAt,
		})
	}
	effective, err := repo.EffectiveFor(ctx, keys)
	if err != nil {
		return fmt.Errorf("derive effective moisture for executions: %w", err)
	}
	for i := range executions {
		applyToExecution(&executions[i], effective["execution-"+strconv.FormatUint(uint64(executions[i].ID), 10)])
	}
	return nil
}

func planKey(id uint) string { return "plan-" + strconv.FormatUint(uint64(id), 10) }

func applyToPlan(plan *model.IrrigationPlan, remeasure model.PlanRemeasure) {
	// 审批前没有钉住值时退回当前 MetricValue；审批后一律以钉住值为准，普通编辑
	// 改 MetricValue 不会改变页面显示。
	approved := plan.MetricValue
	if plan.ApprovedMetricValue != nil {
		approved = *plan.ApprovedMetricValue
	}
	plan.EffectiveMoisture = &approved
	plan.EffectiveMoistureFrom = MoistureSourceApproved
	if remeasure.ID != 0 {
		value := remeasure.Moisture
		measuredAt := remeasure.MeasuredAt
		remeasureID := remeasure.ID
		plan.EffectiveMoisture = &value
		plan.EffectiveMoistureFrom = MoistureSourceRemeasure
		plan.RemeasuredAt = &measuredAt
		plan.RemeasureID = &remeasureID
	}
}

func applyToExecution(execution *model.ValveExecution, remeasure model.PlanRemeasure) {
	approved := execution.MetricValue
	execution.EffectiveMoisture = &approved
	execution.EffectiveMoistureFrom = MoistureSourceApproved
	if remeasure.ID != 0 {
		value := remeasure.Moisture
		measuredAt := remeasure.MeasuredAt
		remeasureID := remeasure.ID
		execution.EffectiveMoisture = &value
		execution.EffectiveMoistureFrom = MoistureSourceRemeasure
		execution.RemeasuredAt = &measuredAt
		execution.RemeasureID = &remeasureID
	}
}
