
import { request } from './client';
import type { DomainRecord, RegisterRemeasureInput } from '../types/domain';

export async function listIrrigationPlan(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/plans?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createIrrigationPlan(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/plans', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionIrrigationPlan(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/plans/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
export async function registerPlanRemeasure(id: number, input: RegisterRemeasureInput) {
  return request<DomainRecord>(`/plans/${id}/remeasure`, {
    method: 'POST', body: JSON.stringify(input),
  });
}
