package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/constants"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/dto"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/repository"
	"gorm.io/gorm"
)

type IrrigationPlanService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.IrrigationPlan], error)
	Get(context.Context, uint) (model.IrrigationPlan, error)
	Create(context.Context, dto.CreateIrrigationPlan, string, string) (model.IrrigationPlan, error)
	Update(context.Context, uint, dto.UpdateIrrigationPlan, string, string) (model.IrrigationPlan, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.IrrigationPlan, error)
	Remeasure(context.Context, uint, dto.RemeasureIrrigationPlan, string, string) (model.IrrigationPlan, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type irrigationPlanService struct {
	repository repository.IrrigationPlanRepository
	remeasures repository.PlanRemeasureRepository
	security   SecurityService
}

func NewIrrigationPlanService(repo repository.IrrigationPlanRepository, remeasures repository.PlanRemeasureRepository, security SecurityService) IrrigationPlanService {
	return &irrigationPlanService{repository: repo, remeasures: remeasures, security: security}
}

func (s *irrigationPlanService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.IrrigationPlan], error) {
	page, err := s.repository.List(ctx, query)
	if err != nil {
		return page, err
	}
	return page, s.enrich(ctx, page.Items)
}

func (s *irrigationPlanService) Get(ctx context.Context, id uint) (model.IrrigationPlan, error) {
	item, err := s.repository.Get(ctx, id)
	if err != nil {
		return item, err
	}
	items := []model.IrrigationPlan{item}
	if err := s.enrich(ctx, items); err != nil {
		return model.IrrigationPlan{}, err
	}
	return items[0], nil
}

func (s *irrigationPlanService) Create(ctx context.Context, input dto.CreateIrrigationPlan, actor, requestID string) (model.IrrigationPlan, error) {
	if err := validateIrrigationPlanBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.IrrigationPlan{}, err
	}
	item := model.IrrigationPlan{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.IrrigationPlanInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.IrrigationPlan{}, fmt.Errorf("create 灌溉计划: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "IrrigationPlan", item.ID, "", item.Status, "created 灌溉计划")
	return s.Get(ctx, item.ID)
}

func (s *irrigationPlanService) Update(ctx context.Context, id uint, input dto.UpdateIrrigationPlan, actor, requestID string) (model.IrrigationPlan, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.IrrigationPlan{}, err
	}
	if err := validateIrrigationPlanBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.IrrigationPlan{}, err
	}
	current.Name = strings.TrimSpace(input.Name)
	current.Description = strings.TrimSpace(input.Description)
	current.Facility = strings.TrimSpace(input.Facility)
	current.Owner = strings.TrimSpace(input.Owner)
	current.Category = strings.TrimSpace(input.Category)
	current.RiskLevel = input.RiskLevel
	current.MetricValue = input.MetricValue
	current.MetricUnit = strings.TrimSpace(input.MetricUnit)
	current.EffectiveAt = input.EffectiveAt.UTC()
	current.Evidence = strings.TrimSpace(input.Evidence)
	current.RelatedCode = strings.ToUpper(strings.TrimSpace(input.RelatedCode))
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.IrrigationPlan{}, fmt.Errorf("update 灌溉计划: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "IrrigationPlan", id, current.Status, current.Status, "updated business fields")
	return s.Get(ctx, id)
}

func (s *irrigationPlanService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.IrrigationPlan, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.IrrigationPlan{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.IrrigationPlanTransitions, current.Status, target) {
		return model.IrrigationPlan{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.IrrigationPlan{}, fmt.Errorf("transition 灌溉计划: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "IrrigationPlan", id, before, target, input.Reason); err != nil {
		return model.IrrigationPlan{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.Get(ctx, id)
}

// Remeasure registers 现场复测 for a plan. The approved metric stays pinned; the
// remeasure is stored as its own record and pages read the derived 生效指标.
// Rules: 每条计划只能补登一次; 复测早于生效时间的不认.
func (s *irrigationPlanService) Remeasure(ctx context.Context, id uint, input dto.RemeasureIrrigationPlan, actor, requestID string) (model.IrrigationPlan, error) {
	plan, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.IrrigationPlan{}, err
	}
	measuredAt := input.MeasuredAt.UTC()
	if measuredAt.Before(plan.EffectiveAt) {
		return model.IrrigationPlan{}, ErrRemeasureBeforeEffective
	}
	if _, err := s.remeasures.GetByPlanID(ctx, id); err == nil {
		return model.IrrigationPlan{}, ErrRemeasureExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.IrrigationPlan{}, err
	}
	item := model.PlanRemeasure{
		PlanID: plan.ID, Facility: plan.Facility, MeasuredAt: measuredAt,
		Moisture: *input.Moisture, MetricUnit: plan.MetricUnit, CreatedBy: actor,
	}
	if err := s.remeasures.Create(ctx, &item); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return model.IrrigationPlan{}, ErrRemeasureExists
		}
		return model.IrrigationPlan{}, fmt.Errorf("create 复测记录: %w", err)
	}
	detail := fmt.Sprintf("remeasure moisture=%.2f%s measuredAt=%s", item.Moisture, item.MetricUnit, measuredAt.Format(time.RFC3339))
	if err := s.security.Audit(ctx, actor, requestID, "remeasure", "IrrigationPlan", id, plan.Status, plan.Status, detail); err != nil {
		return model.IrrigationPlan{}, fmt.Errorf("persist remeasure audit: %w", err)
	}
	return s.Get(ctx, id)
}

func (s *irrigationPlanService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "IrrigationPlan", id, current.Status, "deleted", "soft deleted 灌溉计划")
}

func (s *irrigationPlanService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateIrrigationPlanBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}

// enrich fills the display-only fields on 灌溉计划: 生效指标 comes from the newest
// remeasure of the record's 分区 (审批值保持不动), and Remeasure* exposes the
// plan's own remeasure so the detail page can gate 补登 to a single entry.
func (s *irrigationPlanService) enrich(ctx context.Context, items []model.IrrigationPlan) error {
	if len(items) == 0 {
		return nil
	}
	facilities := make([]string, 0, len(items))
	planIDs := make([]uint, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if !seen[item.Facility] {
			seen[item.Facility] = true
			facilities = append(facilities, item.Facility)
		}
		planIDs = append(planIDs, item.ID)
	}
	latest, err := s.remeasures.LatestByFacilities(ctx, facilities)
	if err != nil {
		return err
	}
	own, err := s.remeasures.ByPlanIDs(ctx, planIDs)
	if err != nil {
		return err
	}
	for i := range items {
		value, unit, source, measuredAt := effectiveMetric(items[i].Facility, items[i].MetricValue, items[i].MetricUnit, latest)
		items[i].EffectiveMetricValue = value
		items[i].EffectiveMetricUnit = unit
		items[i].EffectiveMetricSource = source
		items[i].EffectiveMeasuredAt = measuredAt
		if remeasure, ok := own[items[i].ID]; ok {
			moisture := remeasure.Moisture
			recordedAt := remeasure.MeasuredAt
			items[i].RemeasureMoisture = &moisture
			items[i].RemeasuredAt = &recordedAt
		}
	}
	return nil
}

// effectiveMetric derives 生效指标 for one 分区: the newest remeasure by measured
// time wins; without any remeasure the approved metric stays in effect. Shared
// by the plan and valve execution services so every page reads the same number.
func effectiveMetric(facility string, baseValue float64, baseUnit string, latest map[string]model.PlanRemeasure) (float64, string, string, *time.Time) {
	if remeasure, ok := latest[facility]; ok {
		measuredAt := remeasure.MeasuredAt
		unit := remeasure.MetricUnit
		if unit == "" {
			unit = baseUnit
		}
		return remeasure.Moisture, unit, "remeasure", &measuredAt
	}
	return baseValue, baseUnit, "approved", nil
}
