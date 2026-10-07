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
	RegisterRemeasure(context.Context, uint, dto.RegisterRemeasure, string, string) (model.IrrigationPlan, error)
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
	if err := decoratePlanMoisture(ctx, s.remeasures, page.Items); err != nil {
		return page, err
	}
	return page, nil
}

func (s *irrigationPlanService) Get(ctx context.Context, id uint) (model.IrrigationPlan, error) {
	item, err := s.repository.Get(ctx, id)
	if err != nil {
		return item, err
	}
	items := []model.IrrigationPlan{item}
	if err := decoratePlanMoisture(ctx, s.remeasures, items); err != nil {
		return item, err
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
	return item, nil
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
	updated, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.IrrigationPlan{}, err
	}
	items := []model.IrrigationPlan{updated}
	if err := decoratePlanMoisture(ctx, s.remeasures, items); err != nil {
		return model.IrrigationPlan{}, err
	}
	return items[0], nil
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
	if before == model.IrrigationPlanInitialStatus && target != model.IrrigationPlanInitialStatus {
		// 离开草稿进入生效流程（approved/scheduled）的瞬间把当时含水率钉为审批
		// 指标；之后普通编辑只改 MetricValue，不再改动这里，保证复测与页面回落值
		// 始终可追溯到审批时刻。
		approved := current.MetricValue
		current.ApprovedMetricValue = &approved
	}
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.IrrigationPlan{}, fmt.Errorf("transition 灌溉计划: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "IrrigationPlan", id, before, target, input.Reason); err != nil {
		return model.IrrigationPlan{}, fmt.Errorf("persist transition audit: %w", err)
	}
	updated, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.IrrigationPlan{}, err
	}
	items := []model.IrrigationPlan{updated}
	if err := decoratePlanMoisture(ctx, s.remeasures, items); err != nil {
		return model.IrrigationPlan{}, err
	}
	return items[0], nil
}

// RegisterRemeasure 为已审批的计划补登一次现场复测。审批指标保留不覆盖；复测
// 时间早于计划生效时间不予采信，每条计划最多一条，重复补登返回业务冲突。
func (s *irrigationPlanService) RegisterRemeasure(ctx context.Context, id uint, input dto.RegisterRemeasure, actor, requestID string) (model.IrrigationPlan, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.IrrigationPlan{}, err
	}
	if current.Status == model.IrrigationPlanInitialStatus {
		return model.IrrigationPlan{}, fmt.Errorf("%w: only approved plans can register a remeasure", ErrRemeasureNotAllowed)
	}
	exists, err := s.remeasures.ExistsByPlanID(ctx, id)
	if err != nil {
		return model.IrrigationPlan{}, fmt.Errorf("check existing remeasure: %w", err)
	}
	if exists {
		return model.IrrigationPlan{}, ErrRemeasureExists
	}
	measuredAt := input.MeasuredAt.UTC()
	if measuredAt.Before(current.EffectiveAt) {
		return model.IrrigationPlan{}, fmt.Errorf("%w: measured at %s is earlier than effective time %s", ErrRemeasureBeforeEffective, measuredAt.Format(time.RFC3339), current.EffectiveAt.Format(time.RFC3339))
	}
	remeasure := model.PlanRemeasure{
		PlanID:       id,
		ZoneCode:     current.RelatedCode,
		MeasuredAt:   measuredAt,
		Moisture:     input.Moisture,
		RegisteredBy: actor,
		Evidence:     strings.TrimSpace(input.Evidence),
	}
	if err := s.remeasures.Create(ctx, &remeasure); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return model.IrrigationPlan{}, ErrRemeasureExists
		}
		return model.IrrigationPlan{}, fmt.Errorf("register plan remeasure: %w", err)
	}
	detail := fmt.Sprintf("remeasure moisture %.1f%% at %s", input.Moisture, measuredAt.Format(time.RFC3339))
	if err := s.security.Audit(ctx, actor, requestID, "remeasure_register", "IrrigationPlan", id, current.Status, current.Status, detail); err != nil {
		return model.IrrigationPlan{}, fmt.Errorf("persist remeasure audit: %w", err)
	}
	updated, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.IrrigationPlan{}, err
	}
	items := []model.IrrigationPlan{updated}
	if err := decoratePlanMoisture(ctx, s.remeasures, items); err != nil {
		return model.IrrigationPlan{}, err
	}
	return items[0], nil
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
