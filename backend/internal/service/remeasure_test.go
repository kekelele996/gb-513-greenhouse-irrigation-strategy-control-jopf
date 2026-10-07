package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/config"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/dto"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type noopSecurity struct{}

func (noopSecurity) Login(context.Context, dto.LoginRequest) (dto.LoginResponse, error) {
	return dto.LoginResponse{}, nil
}
func (noopSecurity) Audit(context.Context, string, string, string, string, uint, string, string, string) error {
	return nil
}
func (noopSecurity) ListAudits(context.Context, int, int, string) ([]model.AuditLog, int64, error) {
	return nil, 0, nil
}
func (noopSecurity) AuditSummary(context.Context, time.Duration) (model.AuditSummary, error) {
	return model.AuditSummary{}, nil
}
func (noopSecurity) EntityHistory(context.Context, string, uint, int) ([]model.AuditLog, error) {
	return nil, nil
}
func (noopSecurity) RuntimeConfig() config.PublicConfig { return config.PublicConfig{} }

func newRemeasureFixture(t *testing.T) (*gorm.DB, IrrigationPlanService, ValveExecutionService) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.IrrigationPlan{}, &model.PlanRemeasure{}, &model.ValveExecution{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	planRepo := repository.NewIrrigationPlanRepository(db)
	executionRepo := repository.NewValveExecutionRepository(db)
	remeasureRepo := repository.NewPlanRemeasureRepository(db)
	security := noopSecurity{}
	planSvc := NewIrrigationPlanService(planRepo, remeasureRepo, security)
	executionSvc := NewValveExecutionService(executionRepo, remeasureRepo, security)
	return db, planSvc, executionSvc
}

func planCreate(code, zone string, metric float64, effective time.Time) dto.CreateIrrigationPlan {
	return dto.CreateIrrigationPlan{
		Code: code, Name: "灌溉计划-" + code, Facility: "分区-" + zone, Owner: "运行一组",
		Category: "常规", RiskLevel: "medium", MetricValue: metric, MetricUnit: "%",
		EffectiveAt: effective, Evidence: "证据", RelatedCode: zone,
	}
}

func approve(t *testing.T, ctx context.Context, svc IrrigationPlanService, id, version uint) {
	t.Helper()
	if _, err := svc.Transition(ctx, id, dto.TransitionRequest{Status: "approved", ExpectedVersion: version, Reason: "审批通过"}, "reviewer", "req"); err != nil {
		t.Fatalf("approve plan %d: %v", id, err)
	}
}

// 审批钉值、复测生效、每计划一条、同分区取最新、早于生效不认，全链路验证。
func TestPlanRemeasureRules(t *testing.T) {
	ctx := context.Background()
	_, svc, _ := newRemeasureFixture(t)
	base := time.Date(2026, 10, 7, 8, 0, 0, 0, time.UTC)

	planA, err := svc.Create(ctx, planCreate("IP-A", "Z1", 25.0, base), "operator", "req")
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	approve(t, ctx, svc, planA.ID, planA.Version)

	// 草稿计划不能补登。
	planDraft, err := svc.Create(ctx, planCreate("IP-D0", "Z9", 20.0, base), "operator", "req")
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if _, err := svc.RegisterRemeasure(ctx, planDraft.ID, dto.RegisterRemeasure{MeasuredAt: base.Add(time.Hour), Moisture: 30}, "operator", "req"); !errors.Is(err, ErrRemeasureNotAllowed) {
		t.Fatalf("draft remeasure expected ErrRemeasureNotAllowed, got %v", err)
	}

	// 复测早于生效时间不认。
	if _, err := svc.RegisterRemeasure(ctx, planA.ID, dto.RegisterRemeasure{MeasuredAt: base.Add(-time.Hour), Moisture: 30}, "operator", "req"); !errors.Is(err, ErrRemeasureBeforeEffective) {
		t.Fatalf("early remeasure expected ErrRemeasureBeforeEffective, got %v", err)
	}

	// 合格复测：生效含水率变为复测值，审批值留底不变。
	updated, err := svc.RegisterRemeasure(ctx, planA.ID, dto.RegisterRemeasure{MeasuredAt: base.Add(10 * time.Minute), Moisture: 31.8, Evidence: "手持探针"}, "operator", "req")
	if err != nil {
		t.Fatalf("register A remeasure: %v", err)
	}
	if got := *updated.EffectiveMoisture; got != 31.8 {
		t.Fatalf("effective moisture = %v, want 31.8", got)
	}
	if updated.EffectiveMoistureFrom != MoistureSourceRemeasure || !updated.RemeasureRegistered {
		t.Fatalf("expected remeasure source and registered flag, got %s/%v", updated.EffectiveMoistureFrom, updated.RemeasureRegistered)
	}
	if updated.ApprovedMetricValue == nil || *updated.ApprovedMetricValue != 25.0 {
		t.Fatalf("approved baseline = %v, want pinned 25.0", updated.ApprovedMetricValue)
	}

	// 每条计划只能补登一次。
	if _, err := svc.RegisterRemeasure(ctx, planA.ID, dto.RegisterRemeasure{MeasuredAt: base.Add(20 * time.Minute), Moisture: 33}, "operator", "req"); !errors.Is(err, ErrRemeasureExists) {
		t.Fatalf("duplicate remeasure expected ErrRemeasureExists, got %v", err)
	}

	// 普通编辑改 MetricValue 不应改变页面生效含水率，也不应抹掉审批值。
	current, _ := svc.Get(ctx, planA.ID)
	editInput := dto.UpdateIrrigationPlan{
		ExpectedVersion: current.Version, Name: current.Name, Facility: current.Facility, Owner: current.Owner,
		Category: current.Category, RiskLevel: current.RiskLevel, MetricValue: 40, MetricUnit: "%",
		EffectiveAt: current.EffectiveAt, Evidence: current.Evidence, RelatedCode: current.RelatedCode,
	}
	edited, err := svc.Update(ctx, planA.ID, editInput, "operator", "req")
	if err != nil {
		t.Fatalf("edit A: %v", err)
	}
	if got := *edited.EffectiveMoisture; got != 31.8 {
		t.Fatalf("effective after edit = %v, want 31.8", got)
	}
	if *edited.ApprovedMetricValue != 25.0 {
		t.Fatalf("approved baseline after edit = %v, want 25.0", *edited.ApprovedMetricValue)
	}

	// 同分区另一条计划补登更晚复测：同分区按时间取最新，A 页面也跟着取新值。
	planE, err := svc.Create(ctx, planCreate("IP-E", "Z1", 26.0, base), "operator", "req")
	if err != nil {
		t.Fatalf("create E: %v", err)
	}
	approve(t, ctx, svc, planE.ID, planE.Version)
	if _, err := svc.RegisterRemeasure(ctx, planE.ID, dto.RegisterRemeasure{MeasuredAt: base.Add(20 * time.Minute), Moisture: 45.0}, "operator", "req"); err != nil {
		t.Fatalf("register E remeasure: %v", err)
	}
	aAfter, _ := svc.Get(ctx, planA.ID)
	if got := *aAfter.EffectiveMoisture; got != 45.0 {
		t.Fatalf("zone-latest effective = %v, want 45.0", got)
	}
	if aAfter.RemeasuredAt == nil || !aAfter.RemeasuredAt.Equal(base.Add(20*time.Minute)) {
		t.Fatalf("remeasuredAt = %v, want %v", aAfter.RemeasuredAt, base.Add(20*time.Minute))
	}

	// 同分区但生效时间晚于所有复测：没有合格复测，回落到自身审批值。
	planB, err := svc.Create(ctx, planCreate("IP-B", "Z1", 28.0, base.Add(time.Hour)), "operator", "req")
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	approve(t, ctx, svc, planB.ID, planB.Version)
	bView, _ := svc.Get(ctx, planB.ID)
	if got := *bView.EffectiveMoisture; got != 28.0 || bView.EffectiveMoistureFrom != MoistureSourceApproved {
		t.Fatalf("late-effective plan = %v/%s, want 28.0/approved", got, bView.EffectiveMoistureFrom)
	}

	// 分区互不串值。
	planC, err := svc.Create(ctx, planCreate("IP-C", "Z2", 50.0, base), "operator", "req")
	if err != nil {
		t.Fatalf("create C: %v", err)
	}
	approve(t, ctx, svc, planC.ID, planC.Version)
	cView, _ := svc.Get(ctx, planC.ID)
	if got := *cView.EffectiveMoisture; got != 50.0 {
		t.Fatalf("other zone effective = %v, want own 50.0", got)
	}
}

// 阀门页与计划页共用同一份派生规则。
func TestExecutionMoistureDerivedFromZone(t *testing.T) {
	ctx := context.Background()
	_, planSvc, execSvc := newRemeasureFixture(t)
	base := time.Date(2026, 10, 7, 8, 0, 0, 0, time.UTC)

	plan, err := planSvc.Create(ctx, planCreate("IP-V", "ZV", 25.0, base), "operator", "req")
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	approve(t, ctx, planSvc, plan.ID, plan.Version)
	if _, err := planSvc.RegisterRemeasure(ctx, plan.ID, dto.RegisterRemeasure{MeasuredAt: base.Add(15 * time.Minute), Moisture: 33.3}, "operator", "req"); err != nil {
		t.Fatalf("register remeasure: %v", err)
	}

	executionInput := dto.CreateValveExecution{
		Code: "VE-X", Name: "阀门执行-X", Facility: "分区-ZV", Owner: "运行一组",
		Category: "常规", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
		EffectiveAt: base, Evidence: "证据", RelatedCode: "ZV",
	}
	execution, err := execSvc.Create(ctx, executionInput, "operator", "req")
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}
	view, err := execSvc.Get(ctx, execution.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if got := *view.EffectiveMoisture; got != 33.3 || view.EffectiveMoistureFrom != MoistureSourceRemeasure {
		t.Fatalf("execution effective = %v/%s, want 33.3/remeasure", got, view.EffectiveMoistureFrom)
	}

	// 生效时间晚于复测的执行记录回落到自身指标。
	later := executionInput
	later.Code = "VE-Y"
	later.EffectiveAt = base.Add(time.Hour)
	executionLate, err := execSvc.Create(ctx, later, "operator", "req")
	if err != nil {
		t.Fatalf("create late execution: %v", err)
	}
	lateView, _ := execSvc.Get(ctx, executionLate.ID)
	if got := *lateView.EffectiveMoisture; got != 25.0 || lateView.EffectiveMoistureFrom != MoistureSourceApproved {
		t.Fatalf("late execution = %v/%s, want 25.0/approved", got, lateView.EffectiveMoistureFrom)
	}

	// 列表同样带出派生字段。
	page, err := execSvc.List(ctx, dto.PageQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(page.Items) != 2 || page.Items[0].EffectiveMoisture == nil {
		t.Fatalf("list decoration missing, %d items", len(page.Items))
	}
}
