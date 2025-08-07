import React from 'react';
import { Row, Col, Card, Statistic, Progress, List, Tag, Space, Spin } from 'antd';
import {
  AlertOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  ExclamationCircleOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
} from '@ant-design/icons';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts';
import { usePolling } from '../hooks/useApi';
import apiService from '../services/api';
import { 
  formatNumber, 
  formatTime, 
  getSeverityColor, 
  getSeverityLabel,
  getAlertTypeLabel,
  formatRelativeTime 
} from '../utils/helpers';

const Dashboard = () => {
  const { data: alertStats, loading: statsLoading } = usePolling(
    apiService.getAlertStats,
    10000 // 10秒刷新一次
  );

  const { data: systemMetrics, loading: metricsLoading } = usePolling(
    apiService.getSystemMetrics,
    60000 // 60秒刷新一次
  );

  const { data: recentAlerts, loading: alertsLoading } = usePolling(
    () => apiService.getAlerts({ page: 1, page_size: 10 }),
    60000 // 60秒刷新一次
  );

  // 模拟图表数据
  const alertTrendData = [
    { time: '00:00', alerts: 12 },
    { time: '04:00', alerts: 8 },
    { time: '08:00', alerts: 25 },
    { time: '12:00', alerts: 18 },
    { time: '16:00', alerts: 32 },
    { time: '20:00', alerts: 15 },
  ];

  const severityData = alertStats ? [
    { name: '严重', value: alertStats.by_severity?.critical || 0, color: '#ff4d4f' },
    { name: '高', value: alertStats.by_severity?.high || 0, color: '#fa8c16' },
    { name: '中', value: alertStats.by_severity?.medium || 0, color: '#faad14' },
    { name: '低', value: alertStats.by_severity?.low || 0, color: '#52c41a' },
  ] : [];

  return (
    <div style={{ padding: '0 8px' }}>
      <Row gutter={[16, 16]}>
        {/* 统计卡片 */}
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="总告警数"
              value={alertStats?.total_alerts || 0}
              prefix={<AlertOutlined />}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="已处理"
              value={alertStats?.by_status?.sent || 0}
              prefix={<CheckCircleOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
        
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="待处理"
              value={alertStats?.by_status?.pending || 0}
              prefix={<ClockCircleOutlined />}
              valueStyle={{ color: '#faad14' }}
            />
          </Card>
        </Col>
        
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="处理失败"
              value={alertStats?.by_status?.failed || 0}
              prefix={<ExclamationCircleOutlined />}
              valueStyle={{ color: '#ff4d4f' }}
            />
          </Card>
        </Col>

        {/* 系统性能指标 */}
        <Col xs={24} lg={12}>
          <Card title="系统性能" loading={metricsLoading}>
            {systemMetrics && (
              <Row gutter={16}>
                <Col span={12}>
                  <div style={{ marginBottom: 16 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
                      <span>CPU 使用率</span>
                      <span>{formatNumber(systemMetrics.cpu_usage || 0, 1)}%</span>
                    </div>
                    <Progress 
                      percent={systemMetrics.cpu_usage || 0} 
                      strokeColor={systemMetrics.cpu_usage > 80 ? '#ff4d4f' : '#52c41a'}
                      showInfo={false}
                    />
                  </div>
                  
                  <div style={{ marginBottom: 16 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
                      <span>内存使用率</span>
                      <span>{formatNumber(systemMetrics.memory_usage || 0, 1)}%</span>
                    </div>
                    <Progress 
                      percent={systemMetrics.memory_usage || 0} 
                      strokeColor={systemMetrics.memory_usage > 80 ? '#ff4d4f' : '#52c41a'}
                      showInfo={false}
                    />
                  </div>
                </Col>
                
                <Col span={12}>
                  <Statistic
                    title="活跃规则数"
                    value={systemMetrics.active_rules || 0}
                    prefix={<ArrowUpOutlined />}
                    valueStyle={{ color: '#52c41a' }}
                  />
                  
                  <Statistic
                    title="平均响应时间"
                    value={systemMetrics.avg_response_time || 0}
                    suffix="ms"
                    prefix={<ArrowDownOutlined />}
                    valueStyle={{ color: '#1890ff' }}
                    style={{ marginTop: 16 }}
                  />
                </Col>
              </Row>
            )}
          </Card>
        </Col>

        {/* 告警趋势图 */}
        <Col xs={24} lg={12}>
          <Card title="24小时告警趋势">
            <ResponsiveContainer width="100%" height={200}>
              <LineChart data={alertTrendData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="time" />
                <YAxis />
                <Tooltip />
                <Line 
                  type="monotone" 
                  dataKey="alerts" 
                  stroke="#1890ff" 
                  strokeWidth={2}
                  dot={{ fill: '#1890ff' }}
                />
              </LineChart>
            </ResponsiveContainer>
          </Card>
        </Col>

        {/* 告警严重程度分布 */}
        <Col xs={24} lg={8}>
          <Card title="告警严重程度分布" loading={statsLoading}>
            {severityData.length > 0 && (
              <ResponsiveContainer width="100%" height={200}>
                <PieChart>
                  <Pie
                    data={severityData}
                    cx="50%"
                    cy="50%"
                    innerRadius={40}
                    outerRadius={80}
                    paddingAngle={5}
                    dataKey="value"
                  >
                    {severityData.map((entry, index) => (
                      <Cell key={`cell-${index}`} fill={entry.color} />
                    ))}
                  </Pie>
                  <Tooltip />
                </PieChart>
              </ResponsiveContainer>
            )}
          </Card>
        </Col>

        {/* 最近告警 */}
        <Col xs={24} lg={16}>
          <Card title="最近告警" loading={alertsLoading}>
            <List
              dataSource={recentAlerts?.data || []}
              renderItem={(alert) => (
                <List.Item>
                  <List.Item.Meta
                    title={
                      <Space>
                        <Tag color={getSeverityColor(alert.severity)}>
                          {getSeverityLabel(alert.severity)}
                        </Tag>
                        <span>{alert.title || '未知告警'}</span>
                      </Space>
                    }
                    description={
                      <Space split={<span style={{ color: '#d9d9d9' }}>|</span>}>
                        <span>{getAlertTypeLabel(alert.type)}</span>
                        <span>{formatRelativeTime(alert.created_at)}</span>
                        <span>金额: {formatNumber(alert.amount)} ETH</span>
                      </Space>
                    }
                  />
                </List.Item>
              )}
              locale={{ emptyText: '暂无告警数据' }}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;
