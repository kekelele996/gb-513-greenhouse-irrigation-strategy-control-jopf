package service

import (
	"context"
	"testing"
	"time"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/dto"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/repository"
	"gorm.io/gorm"
)

type fakeValveExecutionRepository struct {
	items map[uint]model.ValveExecution
}

func newFakeValveExecutionRepository() *fakeValveExecutionRepository {
	return &fakeValveExecutionRepository{items: make(map[uint]model.ValveExecution)}
}

func (r *fakeValveExecutionRepository) List(_ context.Context, _ dto.PageQuery) (repository.Page[model.ValveExecution], error) {
	items := make([]model.ValveExecution, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}
	return repository.Page[model.ValveExecution]{Items: items, Total: int64(len(items)), Page: 1, PageSize: 20}, nil
}
func (r *fakeValveExecutionRepository) Get(_ context.Context, id uint) (model.ValveExecution, error) {
	item, ok := r.items[id]
	if !ok {
		return model.ValveExecution{}, gorm.ErrRecordNotFound
	}
	return item, nil
}
func (r *fakeValveExecutionRepository) Create(_ context.Context, item *model.ValveExecution) error {
	item.ID = uint(len(r.items) + 1)
	r.items[item.ID] = *item
	return nil
}
func (r *fakeValveExecutionRepository) Update(_ context.Context, id, _ uint, item *model.ValveExecution) error {
	r.items[id] = *item
	return nil
}
func (r *fakeValveExecutionRepository) Delete(_ context.Context, id uint) error {
	delete(r.items, id)
	return nil
}
func (r *fakeValveExecutionRepository) CountByStatus(context.Context) (map[string]int64, error) {
	return map[string]int64{}, nil
}

func TestValveExecutionReadsZoneNewestRemeasure(t *testing.T) {
	remeasures := newFakeRemeasureRepository()
	planSvc := NewIrrigationPlanService(newFakePlanRepository(), remeasures, fakeSecurity{})
	executionSvc := NewValveExecutionService(newFakeValveExecutionRepository(), remeasures, fakeSecurity{})
	now := time.Now().UTC()

	plan := seedPlanForRemeasure(t, planSvc, "IP-T07", "分区F", 25.0, now)
	if _, err := planSvc.Remeasure(context.Background(), plan.ID, remeasureInput(now.Add(time.Hour), 33.0), "operator", "req-test"); err != nil {
		t.Fatalf("remeasure plan: %v", err)
	}

	execution, err := executionSvc.Create(context.Background(), dto.CreateValveExecution{
		Code: "VE-T01", Name: "阀门执行T01", Facility: "分区F", Owner: "运行一组",
		Category: "常规", RiskLevel: "low", MetricValue: 25.0, MetricUnit: "%",
		EffectiveAt: now,
	}, "operator", "req-test")
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}
	if execution.EffectiveMetricValue != 33.0 || execution.EffectiveMetricSource != "remeasure" {
		t.Fatalf("valve page should read the zone remeasure, got %v (%s)", execution.EffectiveMetricValue, execution.EffectiveMetricSource)
	}
	if execution.MetricValue != 25.0 {
		t.Fatalf("execution's own metric must stay untouched, got %v", execution.MetricValue)
	}

	page, err := executionSvc.List(context.Background(), dto.PageQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].EffectiveMetricValue != 33.0 {
		t.Fatalf("list should carry the derived effective metric, got %+v", page.Items)
	}
}

func TestValveExecutionFallsBackToOwnMetric(t *testing.T) {
	executionSvc := NewValveExecutionService(newFakeValveExecutionRepository(), newFakeRemeasureRepository(), fakeSecurity{})
	now := time.Now().UTC()

	execution, err := executionSvc.Create(context.Background(), dto.CreateValveExecution{
		Code: "VE-T02", Name: "阀门执行T02", Facility: "分区G", Owner: "运行一组",
		Category: "常规", RiskLevel: "low", MetricValue: 25.0, MetricUnit: "%",
		EffectiveAt: now,
	}, "operator", "req-test")
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}
	if execution.EffectiveMetricValue != 25.0 || execution.EffectiveMetricSource != "approved" {
		t.Fatalf("without remeasure the execution metric stays effective, got %v (%s)", execution.EffectiveMetricValue, execution.EffectiveMetricSource)
	}
}
