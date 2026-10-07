import { Alert, Button, Checkbox, DatePicker, Descriptions, Drawer, Form, Input, InputNumber, Space, Tag } from 'antd';
import { SafetyCertificateOutlined, SendOutlined } from '@ant-design/icons';
import { useEffect, useState } from 'react';
import dayjs, { type Dayjs } from 'dayjs';
import type { DomainRecord } from '../types/domain';
import { formatDate } from '../utils/format';
import { registerPlanRemeasure } from '../api/irrigation-plan';
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
  onRemeasured?: (record: DomainRecord) => void;
}

interface RemeasureFormValues {
  measuredAt: Dayjs;
  moisture: number;
  evidence?: string;
}

export function PlanDrawer({ open, record, kind, username, role, busy = false, error = '', onClose, onRequest, onConfirm, onRemeasured }: PlanDrawerProps) {
  const [acknowledged, setAcknowledged] = useState(false);
  const [form] = Form.useForm<RemeasureFormValues>();
  const [remeasureMode, setRemeasureMode] = useState(false);
  const [remeasureBusy, setRemeasureBusy] = useState(false);
  const [remeasureError, setRemeasureError] = useState('');
  useEffect(() => {
    setAcknowledged(false);
    setRemeasureMode(false);
    setRemeasureError('');
    form.resetFields();
  }, [open, record?.id, record?.version, kind, form]);
  const canRequest = kind === 'execution' && record?.status === 'planned' && !record.controlRequestedBy && ['operator', 'admin'].includes(role);
  const canConfirm = kind === 'execution' && record?.status === 'planned' && Boolean(record.controlRequestedBy) && record?.controlRequestedBy !== username && ['reviewer', 'admin'].includes(role);

  const effectiveValue = typeof record?.effectiveMoisture === 'number' ? record.effectiveMoisture : (record?.metricValue ?? 0);
  const fromRemeasure = record?.effectiveMoistureFrom === 'remeasure';
  const approvedBaseline = typeof record?.approvedMetricValue === 'number' ? record.approvedMetricValue : (record?.metricValue ?? 0);
  const canRegisterRemeasure = kind === 'plan' && !!record && record.status !== 'draft' && !record.remeasureRegistered && ['operator', 'admin'].includes(role);
  const effectiveMin = record ? dayjs(record.effectiveAt) : null;

  const submitRemeasure = async () => {
    if (!record) return;
    const values = await form.validateFields();
    if (effectiveMin && values.measuredAt.isBefore(effectiveMin)) {
      setRemeasureError('复测时间不得早于计划生效时间。');
      return;
    }
    setRemeasureBusy(true);
    setRemeasureError('');
    try {
      const result = await registerPlanRemeasure(record.id, {
        measuredAt: values.measuredAt.toISOString(),
        moisture: values.moisture,
        evidence: values.evidence?.trim() || '',
      });
      setRemeasureMode(false);
      onRemeasured?.(result.data);
    } catch (reason) {
      if (reason instanceof Error) setRemeasureError(reason.message);
      else setRemeasureError(String(reason));
    } finally {
      setRemeasureBusy(false);
    }
  };

  return <Drawer title={kind === 'plan' ? '灌溉策略详情' : '计划与阀门控制'} placement="right" width="min(520px, 100vw)" open={open} onClose={onClose}>
    {record && <div className="plan-drawer">
      <div className="drawer-heading"><div><span>{record.code}</span><h2>{record.name}</h2></div><StatusBadge status={record.status} /></div>
      <Descriptions bordered size="small" column={1}>
        <Descriptions.Item label="策略版本">v{record.version}</Descriptions.Item>
        <Descriptions.Item label="目标分区">{record.facility}</Descriptions.Item>
        <Descriptions.Item label="责任人">{record.owner}</Descriptions.Item>
        <Descriptions.Item label="生效含水率（现场执行）">
          <Space size={8}><strong>{effectiveValue.toFixed(1)}%</strong>{fromRemeasure ? <Tag color="processing">复测生效</Tag> : <Tag>审批值</Tag>}</Space>
        </Descriptions.Item>
        <Descriptions.Item label="审批时指标（留底）">{kind === 'plan' ? `${approvedBaseline} ${record.metricUnit}` : `${record.metricValue} ${record.metricUnit}`}</Descriptions.Item>
        <Descriptions.Item label="复测时间">{record.remeasuredAt ? formatDate(record.remeasuredAt) : '未复测'}</Descriptions.Item>
        <Descriptions.Item label="生效时间">{formatDate(record.effectiveAt)}</Descriptions.Item>
        <Descriptions.Item label="证据">{record.evidence || '未附加'}</Descriptions.Item>
      </Descriptions>

      {kind === 'plan' && <section className="remeasure-entry" aria-label="现场复测补登">
        <h3>现场复测补登</h3>
        {record.remeasureRegistered
          ? <Alert type="success" showIcon message="该计划已补登复测，复测含水率已作为生效指标展示，且不可重复补登。" />
          : remeasureMode
            ? <Form form={form} layout="vertical" initialValues={{ measuredAt: dayjs(), moisture: effectiveValue }}>
                <Form.Item name="measuredAt" label="复测时间" rules={[{ required: true, message: '请选择复测时间' }]}
                  extra={effectiveMin ? `复测时间不得早于生效时间 ${formatDate(record.effectiveAt)}` : undefined}>
                  <DatePicker showTime={{ format: 'HH:mm' }} format="YYYY-MM-DD HH:mm" style={{ width: '100%' }}
                    disabledDate={(current) => Boolean(effectiveMin && current.isBefore(effectiveMin, 'day'))} />
                </Form.Item>
                <Form.Item name="moisture" label="复测含水率（%）" rules={[{ required: true, message: '请输入复测含水率' }]}>
                  <InputNumber min={0} max={100} step={0.1} precision={1} style={{ width: '100%' }} addonAfter="%" />
                </Form.Item>
                <Form.Item name="evidence" label="复测说明">
                  <Input.TextArea rows={2} maxLength={2000} showCount placeholder="复测方式、读数来源等，可选" />
                </Form.Item>
              </Form>
            : canRegisterRemeasure
              ? <Button onClick={() => setRemeasureMode(true)}>补登一次复测</Button>
              : <Alert type="info" showIcon message={record.status === 'draft' ? '草稿计划审批后才能补登复测。' : '当前账号仅可查看复测信息。'} />}
        {remeasureError && <Alert type="error" showIcon message={remeasureError} />}
        {remeasureMode && <Space><Button type="primary" loading={remeasureBusy} onClick={() => void submitRemeasure()}>提交复测</Button><Button onClick={() => { setRemeasureMode(false); setRemeasureError(''); }}>取消</Button></Space>}
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
