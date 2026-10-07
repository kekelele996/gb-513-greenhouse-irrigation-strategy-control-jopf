
export interface DomainRecord {
  id: number;
  code: string;
  name: string;
  status: string;
  version: number;
  description: string;
  facility: string;
  owner: string;
  category: string;
  riskLevel: 'low' | 'medium' | 'high' | 'critical';
  metricValue: number;
  metricUnit: string;
  approvedMetricValue?: number | null;
  effectiveAt: string;
  evidence: string;
  relatedCode: string;
  controlRequestedBy?: string;
  controlRequestedAt?: string | null;
  controlConfirmedBy?: string;
  controlConfirmedAt?: string | null;
  // 生效含水率：审批指标留底在 metricValue，复测后由后端派生到下列字段。
  effectiveMoisture?: number | null;
  effectiveMoistureFrom?: 'approved' | 'remeasure' | null;
  remeasuredAt?: string | null;
  remeasureId?: number | null;
  remeasureRegistered?: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface RegisterRemeasureInput {
  measuredAt: string;
  moisture: number;
  evidence?: string;
}

export interface PageMeta { page: number; pageSize: number; total: number }
export interface ApiEnvelope<T> { data: T; error?: string; message?: string; meta?: PageMeta }
export interface UserSession { token: string; username: string; displayName: string; role: string; expiresIn: number }
export interface AuditLog {
  id: number; requestId: string; actor: string; action: string; entityType: string;
  entityId: number; beforeState: string; afterState: string; detail: string; createdAt: string;
}
export interface EntityConfig { key: string; path: string; label: string; statuses: readonly string[] }
