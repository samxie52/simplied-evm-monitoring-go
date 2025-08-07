import React, { useState } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Tag,
  Modal,
  Form,
  Input,
  Select,
  InputNumber,
  Switch,
  message,
  Popconfirm,
  Tooltip,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { useApi } from '../hooks/useApi';
import apiService from '../services/api';
import {
  formatTime,
  getSeverityColor,
  getSeverityLabel,
  getAlertTypeLabel,
  getStatusColor,
  getStatusLabel,
} from '../utils/helpers';

const { Option } = Select;
const { TextArea } = Input;

const Rules = () => {
  const [form] = Form.useForm();
  const [modalVisible, setModalVisible] = useState(false);
  const [editingRule, setEditingRule] = useState(null);
  const [loading, setLoading] = useState(false);

  const {
    data: rules,
    loading: rulesLoading,
    refetch: refetchRules,
  } = useApi(apiService.getRules, [], { immediate: true });

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 80,
    },
    {
      title: '规则名称',
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 120,
      render: (type) => getAlertTypeLabel(type),
    },
    {
      title: '严重程度',
      dataIndex: 'severity',
      key: 'severity',
      width: 100,
      render: (severity) => (
        <Tag color={getSeverityColor(severity)}>
          {getSeverityLabel(severity)}
        </Tag>
      ),
    },
    {
      title: '阈值',
      dataIndex: 'threshold',
      key: 'threshold',
      width: 100,
      render: (threshold) => threshold || '-',
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      render: (enabled) => (
        <Tag color={enabled ? 'green' : 'red'}>
          {enabled ? '启用' : '禁用'}
        </Tag>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
      render: (time) => formatTime(time),
    },
    {
      title: '操作',
      key: 'actions',
      width: 200,
      render: (_, record) => (
        <Space>
          <Tooltip title={record.enabled ? '禁用规则' : '启用规则'}>
            <Button
              type="text"
              icon={record.enabled ? <PauseCircleOutlined /> : <PlayCircleOutlined />}
              size="small"
              onClick={() => handleToggleRule(record)}
            />
          </Tooltip>
          <Tooltip title="编辑规则">
            <Button
              type="text"
              icon={<EditOutlined />}
              size="small"
              onClick={() => handleEditRule(record)}
            />
          </Tooltip>
          <Popconfirm
            title="确定要删除这个规则吗？"
            onConfirm={() => handleDeleteRule(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Tooltip title="删除规则">
              <Button
                type="text"
                icon={<DeleteOutlined />}
                size="small"
                danger
              />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const handleAddRule = () => {
    setEditingRule(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEditRule = (rule) => {
    setEditingRule(rule);
    form.setFieldsValue({
      ...rule,
      conditions: rule.conditions ? JSON.parse(rule.conditions) : [],
    });
    setModalVisible(true);
  };

  const handleToggleRule = async (rule) => {
    try {
      setLoading(true);
      await apiService.updateRule(rule.id, {
        ...rule,
        enabled: !rule.enabled,
      });
      message.success(`规则已${rule.enabled ? '禁用' : '启用'}`);
      refetchRules();
    } catch (error) {
      message.error('操作失败');
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteRule = async (id) => {
    try {
      setLoading(true);
      await apiService.deleteRule(id);
      message.success('规则删除成功');
      refetchRules();
    } catch (error) {
      message.error('删除失败');
    } finally {
      setLoading(false);
    }
  };

  const handleModalOk = async () => {
    try {
      const values = await form.validateFields();
      setLoading(true);

      const ruleData = {
        ...values,
        conditions: values.conditions || [],
      };

      if (editingRule) {
        await apiService.updateRule(editingRule.id, ruleData);
        message.success('规则更新成功');
      } else {
        await apiService.createRule(ruleData);
        message.success('规则创建成功');
      }

      setModalVisible(false);
      refetchRules();
    } catch (error) {
      if (error.errorFields) {
        // 表单验证错误
        return;
      }
      message.error(editingRule ? '更新失败' : '创建失败');
    } finally {
      setLoading(false);
    }
  };

  const handleModalCancel = () => {
    setModalVisible(false);
    form.resetFields();
  };

  return (
    <div style={{ padding: '0 8px' }}>
      <Card>
        <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between' }}>
          <h3>告警规则管理</h3>
          <Space>
            <Button
              icon={<ReloadOutlined />}
              onClick={refetchRules}
              loading={rulesLoading}
            >
              刷新
            </Button>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={handleAddRule}
            >
              新建规则
            </Button>
          </Space>
        </div>

        <Table
          columns={columns}
          dataSource={rules || []}
          loading={rulesLoading || loading}
          rowKey="id"
          pagination={{
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total, range) =>
              `第 ${range[0]}-${range[1]} 条，共 ${total} 条`,
          }}
        />
      </Card>

      {/* 新建/编辑规则模态框 */}
      <Modal
        title={editingRule ? '编辑规则' : '新建规则'}
        open={modalVisible}
        onOk={handleModalOk}
        onCancel={handleModalCancel}
        confirmLoading={loading}
        width={600}
        destroyOnClose
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={{
            enabled: true,
            severity: 'medium',
            type: 'large_transfer',
            time_window: 300,
            cooldown: 600,
          }}
        >
          <Form.Item
            name="name"
            label="规则名称"
            rules={[{ required: true, message: '请输入规则名称' }]}
          >
            <Input placeholder="请输入规则名称" />
          </Form.Item>

          <Form.Item
            name="description"
            label="规则描述"
          >
            <TextArea
              rows={3}
              placeholder="请输入规则描述"
            />
          </Form.Item>

          <Form.Item
            name="type"
            label="告警类型"
            rules={[{ required: true, message: '请选择告警类型' }]}
          >
            <Select placeholder="请选择告警类型">
              <Option value="large_transfer">大额转账</Option>
              <Option value="gas_price">Gas价格</Option>
              <Option value="network_congestion">网络拥堵</Option>
              <Option value="contract_event">合约事件</Option>
              <Option value="token_transfer">代币转账</Option>
              <Option value="custom">自定义</Option>
            </Select>
          </Form.Item>

          <Form.Item
            name="severity"
            label="严重程度"
            rules={[{ required: true, message: '请选择严重程度' }]}
          >
            <Select placeholder="请选择严重程度">
              <Option value="low">低</Option>
              <Option value="medium">中</Option>
              <Option value="high">高</Option>
              <Option value="critical">严重</Option>
            </Select>
          </Form.Item>

          <Form.Item
            name="threshold"
            label="阈值"
            rules={[{ required: true, message: '请输入阈值' }]}
          >
            <InputNumber
              style={{ width: '100%' }}
              placeholder="请输入阈值"
              min={0}
              precision={2}
            />
          </Form.Item>

          <Form.Item
            name="time_window"
            label="时间窗口 (秒)"
            rules={[{ required: true, message: '请输入时间窗口' }]}
          >
            <InputNumber
              style={{ width: '100%' }}
              placeholder="请输入时间窗口"
              min={1}
            />
          </Form.Item>

          <Form.Item
            name="cooldown"
            label="冷却时间 (秒)"
            rules={[{ required: true, message: '请输入冷却时间' }]}
          >
            <InputNumber
              style={{ width: '100%' }}
              placeholder="请输入冷却时间"
              min={0}
            />
          </Form.Item>

          <Form.Item
            name="enabled"
            label="启用状态"
            valuePropName="checked"
          >
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Rules;
