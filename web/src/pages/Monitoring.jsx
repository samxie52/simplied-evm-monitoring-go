import React from 'react';
import { Row, Col, Card, Statistic, Progress, Table, Tag, Space, Spin } from 'antd';
import {
  DesktopOutlined,
  DatabaseOutlined,
  CloudOutlined,
  AlertOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { usePolling } from '../hooks/useApi';
import apiService from '../services/api';
import {
  formatTime,
  formatNumber,
  formatBytes,
  formatPercent,
  getStatusColor,
  getStatusLabel,
} from '../utils/helpers';

const Monitoring = () => {
  const { data: systemStatus, loading: statusLoading, refetch: refetchStatus } = usePolling(
    apiService.getSystemStatus,
    60000 // 60秒刷新一次
  );

  const { data: systemMetrics, loading: metricsLoading, refetch: refetchMetrics } = usePolling(
    apiService.getSystemMetrics,
    60000 // 60秒刷新一次
  );

  // 模拟性能历史数据
  const performanceData = [
    { time: '10:00', cpu: 15, memory: 45, alerts: 12 },
    { time: '10:05', cpu: 23, memory: 48, alerts: 8 },
    { time: '10:10', cpu: 18, memory: 52, alerts: 15 },
    { time: '10:15', cpu: 32, memory: 47, alerts: 22 },
    { time: '10:20', cpu: 28, memory: 55, alerts: 18 },
    { time: '10:25', cpu: 21, memory: 49, alerts: 14 },
  ];

  const serviceColumns = [
    {
      title: '服务名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status) => (
        <Tag color={getStatusColor(status)}>
          {getStatusLabel(status)}
        </Tag>
      ),
    },
    {
      title: '消息',
      dataIndex: 'message',
      key: 'message',
      ellipsis: true,
    },
    {
      title: '最后检查',
      dataIndex: 'last_check',
      key: 'last_check',
      render: (time) => formatTime(time),
    },
  ];

  const serviceData = systemStatus?.services ? Object.entries(systemStatus.services).map(([name, service]) => ({
    key: name,
    name: name.replace('_', ' ').toUpperCase(),
    status: service.status,
    message: service.message,
    last_check: systemStatus.timestamp,
  })) : [];

  const handleRefresh = () => {
    refetchStatus();
    refetchMetrics();
  };

  return (
    <div style={{ padding: '0 8px' }}>
      <Row gutter={[16, 16]}>
        {/* 系统概览 */}
        <Col xs={24}>
          <Card
            title="系统概览"
            extra={
              <Space>
                <span style={{ color: '#666', fontSize: '12px' }}>
                  最后更新: {formatTime(systemStatus?.timestamp)}
                </span>
                <ReloadOutlined
                  style={{ cursor: 'pointer' }}
                  onClick={handleRefresh}
                  spin={statusLoading || metricsLoading}
                />
              </Space>
            }
          >
            <Row gutter={16}>
              <Col xs={24} sm={12} lg={6}>
                <Card size="small">
                  <Statistic
                    title="系统状态"
                    value={systemStatus?.status || '未知'}
                    prefix={
                      systemStatus?.status === 'healthy' ? 
                        <CheckCircleOutlined style={{ color: '#52c41a' }} /> :
                        <ExclamationCircleOutlined style={{ color: '#ff4d4f' }} />
                    }
                    valueStyle={{ 
                      color: getStatusColor(systemStatus?.status),
                      fontSize: '16px'
                    }}
                  />
                </Card>
              </Col>
              
              <Col xs={24} sm={12} lg={6}>
                <Card size="small">
                  <Statistic
                    title="运行时间"
                    value={systemStatus?.uptime ? Math.floor(systemStatus.uptime / 1000000000 / 3600) : 0}
                    suffix="小时"
                    prefix={<DesktopOutlined />}
                    valueStyle={{ color: '#1890ff', fontSize: '16px' }}
                  />
                </Card>
              </Col>
              
              <Col xs={24} sm={12} lg={6}>
                <Card size="small">
                  <Statistic
                    title="版本"
                    value={systemStatus?.version || '1.0.0'}
                    prefix={<CloudOutlined />}
                    valueStyle={{ color: '#722ed1', fontSize: '16px' }}
                  />
                </Card>
              </Col>
              
              <Col xs={24} sm={12} lg={6}>
                <Card size="small">
                  <Statistic
                    title="活跃连接"
                    value={systemMetrics?.active_connections || 0}
                    prefix={<DatabaseOutlined />}
                    valueStyle={{ color: '#13c2c2', fontSize: '16px' }}
                  />
                </Card>
              </Col>
            </Row>
          </Card>
        </Col>

        {/* 性能指标 */}
        <Col xs={24} lg={12}>
          <Card title="实时性能指标" loading={metricsLoading}>
            {systemMetrics && (
              <div>
                <Row gutter={16}>
                  <Col span={12}>
                    <div style={{ marginBottom: 24 }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
                        <span>CPU 使用率</span>
                        <span>{formatPercent(systemMetrics.cpu_usage || 0)}</span>
                      </div>
                      <Progress
                        percent={systemMetrics.cpu_usage || 0}
                        strokeColor={
                          (systemMetrics.cpu_usage || 0) > 80 ? '#ff4d4f' :
                          (systemMetrics.cpu_usage || 0) > 60 ? '#faad14' : '#52c41a'
                        }
                        showInfo={false}
                      />
                    </div>
                    
                    <div style={{ marginBottom: 24 }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
                        <span>内存使用率</span>
                        <span>{formatPercent(systemMetrics.memory_usage || 0)}</span>
                      </div>
                      <Progress
                        percent={systemMetrics.memory_usage || 0}
                        strokeColor={
                          (systemMetrics.memory_usage || 0) > 80 ? '#ff4d4f' :
                          (systemMetrics.memory_usage || 0) > 60 ? '#faad14' : '#52c41a'
                        }
                        showInfo={false}
                      />
                    </div>
                  </Col>
                  
                  <Col span={12}>
                    <Statistic
                      title="内存使用量"
                      value={formatBytes((systemMetrics.memory_used || 0) * 1024 * 1024)}
                      valueStyle={{ fontSize: '14px' }}
                      style={{ marginBottom: 16 }}
                    />
                    
                    <Statistic
                      title="Goroutines"
                      value={systemMetrics.goroutines || 0}
                      valueStyle={{ fontSize: '14px' }}
                      style={{ marginBottom: 16 }}
                    />
                    
                    <Statistic
                      title="平均响应时间"
                      value={systemMetrics.avg_response_time || 0}
                      suffix="ms"
                      valueStyle={{ fontSize: '14px' }}
                    />
                  </Col>
                </Row>
              </div>
            )}
          </Card>
        </Col>

        {/* 性能趋势图 */}
        <Col xs={24} lg={12}>
          <Card title="性能趋势">
            <ResponsiveContainer width="100%" height={250}>
              <LineChart data={performanceData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="time" />
                <YAxis />
                <Tooltip />
                <Line
                  type="monotone"
                  dataKey="cpu"
                  stroke="#1890ff"
                  strokeWidth={2}
                  name="CPU (%)"
                />
                <Line
                  type="monotone"
                  dataKey="memory"
                  stroke="#52c41a"
                  strokeWidth={2}
                  name="内存 (%)"
                />
                <Line
                  type="monotone"
                  dataKey="alerts"
                  stroke="#faad14"
                  strokeWidth={2}
                  name="告警数"
                />
              </LineChart>
            </ResponsiveContainer>
          </Card>
        </Col>

        {/* 服务状态 */}
        <Col xs={24}>
          <Card title="服务状态" loading={statusLoading}>
            <Table
              columns={serviceColumns}
              dataSource={serviceData}
              pagination={false}
              size="small"
            />
          </Card>
        </Col>

        {/* 系统指标详情 */}
        <Col xs={24}>
          <Card title="系统指标详情" loading={metricsLoading}>
            {systemMetrics && (
              <Row gutter={16}>
                <Col xs={24} sm={12} lg={8}>
                  <Statistic
                    title="活跃规则数"
                    value={systemMetrics.active_rules || 0}
                    prefix={<AlertOutlined />}
                  />
                </Col>
                <Col xs={24} sm={12} lg={8}>
                  <Statistic
                    title="今日处理告警"
                    value={systemMetrics.alerts_processed_today || 0}
                    prefix={<CheckCircleOutlined />}
                  />
                </Col>
                <Col xs={24} sm={12} lg={8}>
                  <Statistic
                    title="错误率"
                    value={formatPercent(systemMetrics.error_rate || 0)}
                    prefix={<ExclamationCircleOutlined />}
                    valueStyle={{ 
                      color: (systemMetrics.error_rate || 0) > 5 ? '#ff4d4f' : '#52c41a' 
                    }}
                  />
                </Col>
                <Col xs={24} sm={12} lg={8}>
                  <Statistic
                    title="请求总数"
                    value={formatNumber(systemMetrics.total_requests || 0)}
                  />
                </Col>
                <Col xs={24} sm={12} lg={8}>
                  <Statistic
                    title="成功请求"
                    value={formatNumber(systemMetrics.successful_requests || 0)}
                  />
                </Col>
                <Col xs={24} sm={12} lg={8}>
                  <Statistic
                    title="失败请求"
                    value={formatNumber(systemMetrics.failed_requests || 0)}
                    valueStyle={{ color: '#ff4d4f' }}
                  />
                </Col>
              </Row>
            )}
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Monitoring;
