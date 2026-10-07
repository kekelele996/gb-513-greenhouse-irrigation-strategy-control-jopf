import { Alert, Button, Checkbox, DatePicker, Descriptions, Drawer, InputNumber, Space, Tag } from 'antd';
import { ExperimentOutlined, SafetyCertificateOutlined, SendOutlined } from '@ant-design/icons';
import { useEffect, useState } from 'react';
import type { Dayjs } from 'dayjs';
import type { DomainRecord } from '../types/domain';
import { formatDate } from '../utils/format';
import { StatusBadge } from './common/StatusBadge';

interface PlanDrawerProps {
  open: boolean;
  record: DomainRecord | null;
  kind: 'plan' | 'execution';
  username: string;
  role: string;
  busy?: boolean;
  error?: string;
  onClose: () => void;
  onRequest?: (record: DomainRecord) => Promise<void>;
  onConfirm?: (record: DomainRecord) => Promise<void>;
  onRemeasure?: (record: DomainRecord, input: { measuredAt: string; moisture: number }) => Promise<void>;
}

export function PlanDrawer({ open, record, kind, username, role, busy = false, error = '', onClose, onRequest, onConfirm, onRemeasure }: PlanDrawerProps) {
  const [acknowledged, setAcknowledged] = useState(false);
  const [remeasureAt, setRemeasureAt] = useState<Dayjs | null>(null);
  const [remeasureMoisture, setRemeasureMoisture] = useState<number | null>(null);
  useEffect(() => { setAcknowledged(false); setRemeasureAt(null); setRemeasureMoisture(null); }, [open, record?.id, record?.version, kind]);
  const canRequest = kind === 'execution' && record?.status === 'planned' && !record.controlRequestedBy && ['operator', 'admin'].includes(role);
  const canConfirm = kind === 'execution' && record?.status === 'planned' && Boolean(record.controlRequestedBy) && record?.controlRequestedBy !== username && ['reviewer', 'admin'].includes(role);
  const canRemeasure = kind === 'plan' && Boolean(onRemeasure) && !record?.remeasuredAt && ['operator', 'admin'].includes(role);
  const effectiveValue = record?.effectiveMetricValue ?? record?.metricValue;
  const effectiveUnit = record?.effectiveMetricUnit || record?.metricUnit;

  const submitRemeasure = () => {
    if (record && onRemeasure && remeasureAt && remeasureMoisture !== null) {
      void onRemeasure(record, { measuredAt: remeasureAt.toISOString(), moisture: remeasureMoisture });
    }
  };

  return <Drawer title={kind === 'plan' ? '灌溉策略详情' : '计划与阀门控制'} placement="right" width="min(520px, 100vw)" open={open} onClose={onClose}>
    {record && <div className="plan-drawer">
      <div className="drawer-heading"><div><span>{record.code}</span><h2>{record.name}</h2></div><StatusBadge status={record.status} /></div>
      <Descriptions bordered size="small" column={1}>
        <Descriptions.Item label="策略版本">v{record.version}</Descriptions.Item>
        <Descriptions.Item label="目标分区">{record.facility}</Descriptions.Item>
        <Descriptions.Item label="责任人">{record.owner}</Descriptions.Item>
        <Descriptions.Item label="灌溉指标">{record.metricValue} {record.metricUnit}</Descriptions.Item>
        <Descriptions.Item label="生效指标">
          <Space size={6}>
            <strong>{effectiveValue} {effectiveUnit}</strong>
            {record.effectiveMetricSource === 'remeasure'
              ? <Tag color="cyan">复测{record.effectiveMeasuredAt ? ` · ${formatDate(record.effectiveMeasuredAt)}` : ''}</Tag>
              : <Tag>审批值</Tag>}
          </Space>
        </Descriptions.Item>
        <Descriptions.Item label="生效时间">{formatDate(record.effectiveAt)}</Descriptions.Item>
        <Descriptions.Item label="证据">{record.evidence || '未附加'}</Descriptions.Item>
      </Descriptions>

      {kind === 'plan' && <section className="control-approval" aria-label="现场复测补登">
        <h3>现场复测</h3>
        {record.remeasuredAt
          ? <div className="approval-stage"><span>✓</span><div><strong>已补登复测</strong><p>{formatDate(record.remeasuredAt)} · 含水率 {record.remeasureMoisture} {record.metricUnit}，每条计划仅可补登一次</p></div><Tag color="cyan">复测</Tag></div>
          : canRemeasure
            ? <div className="approval-action">
                <DatePicker showTime placeholder="复测时间" value={remeasureAt} onChange={(value) => setRemeasureAt(value)} style={{ width: '100%' }} />
                <InputNumber min={0} max={100} step={0.1} placeholder="复测含水率" value={remeasureMoisture} onChange={(value) => setRemeasureMoisture(value)} style={{ width: '100%' }} addonAfter="%" />
                <Button type="primary" icon={<ExperimentOutlined />} disabled={!remeasureAt || remeasureMoisture === null} loading={busy} onClick={submitRemeasure}>补登复测</Button>
                <p className="remeasure-hint">复测时间早于计划生效时间将被拒绝；同一分区重复复测按时间取最新一条。</p>
              </div>
            : <p className="remeasure-hint">尚未补登复测，列表与阀门页暂按审批值执行。</p>}
        {error && <Alert type="error" showIcon message={error} />}
      </section>}

      {kind === 'execution' && <section className="control-approval" aria-label="远程控制双确认">
        <h3>远程控制双确认</h3>
        <div className="approval-stage"><span>1</span><div><strong>操作员请求</strong><p>{record.controlRequestedBy ? `${record.controlRequestedBy} · ${formatDate(record.controlRequestedAt || '')}` : '等待操作员核对策略和阀门连通性'}</p></div>{record.controlRequestedBy && <Tag color="gold">已请求</Tag>}</div>
        <div className="approval-stage"><span>2</span><div><strong>独立复核</strong><p>{record.controlConfirmedBy ? `${record.controlConfirmedBy} · ${formatDate(record.controlConfirmedAt || '')}` : '请求人之外的复核员确认后才会启动'}</p></div>{record.controlConfirmedBy && <Tag color="green">已确认</Tag>}</div>
        {error && <Alert type="error" showIcon message={error} />}
        {canRequest && <div className="approval-action"><Checkbox checked={acknowledged} onChange={(event) => setAcknowledged(event.target.checked)}>我已核对分区、计划版本与阀门连通性</Checkbox><Button type="primary" icon={<SendOutlined />} disabled={!acknowledged} loading={busy} onClick={() => record && onRequest && void onRequest(record)}>提交远程启动请求</Button></div>}
        {canConfirm && <div className="approval-action"><Checkbox checked={acknowledged} onChange={(event) => setAcknowledged(event.target.checked)}>我确认复核人与请求人不同，并批准远程启动</Checkbox><Button type="primary" icon={<SafetyCertificateOutlined />} disabled={!acknowledged} loading={busy} onClick={() => record && onConfirm && void onConfirm(record)}>复核并启动阀门</Button></div>}
        {record.controlRequestedBy === username && !record.controlConfirmedBy && <Alert type="warning" showIcon message="请求人不能复核自己的远程启动，请切换独立复核账号。" />}
        {role === 'viewer' && !record.controlConfirmedBy && <Alert type="info" showIcon message="当前账号仅可查看远程控制确认链。" />}
        {record.status !== 'planned' && <Space><Tag color="green">控制链已闭环</Tag><span>当前执行状态为 {record.status}</span></Space>}
      </section>}
    </div>}
  </Drawer>;
}
