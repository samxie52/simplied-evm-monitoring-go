import React, { useState } from 'react';
import {
  Card,
  Table,
  Tag,
  Space,
  Button,
  Input,
  Select,
  DatePicker,
  Modal,
  Descriptions,
  Typography,
  Row,
  Col,
  Statistic,
} from 'antd';
import {
  SearchOutlined,
  EyeOutlined,
  ReloadOutlined,
  FilterOutlined,
} from '@ant-design/icons';
import { usePagination } from '../hooks/useApi';
import apiService from '../services/api';
import {
  formatTime,
  formatNumber,
  getSeverityColor,
  getSeverityLabel,
  getAlertTypeLabel,
  getStatusLabel,
  getStatusColor,
  copyToClipboard,
} from '../utils/helpers';

const { RangePicker } = DatePicker;
const { Option } = Select;
const { Text } = Typography;

const Alerts = () => {
  const [selectedAlert, setSelectedAlert] = useState(null);
  const [detailModalVisible, setDetailModalVisible] = useState(false);
  const [filters, setFilters] = useState({});

  const {
    data: alerts,
    loading,
    pagination,
    handlePageChange,
    handleFilterChange,
    refresh,
  } = usePagination(apiService.getAlerts);

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 80,
      render: (id) => (
        <Text
          copyable={{
            text: id,
            onCopy: () => copyToClipboard(id.toString()),
          }}
          style={{ fontSize: '12px' }}
        >
          {id}
        </Text>
      ),
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
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 120,
      render: (type) => getAlertTypeLabel(type),
    },
    {
      title: '标题',
      dataIndex: 'title',
      key: 'title',
      ellipsis: true,
      render: (title) => title || '未知告警',
    },
    {
      title: '金额',
      dataIndex: 'amount',
      key: 'amount',
      width: 120,
      render: (amount) => `${formatNumber(amount)} ETH`,
      sorter: true,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status) => (
        <Tag color={getStatusColor(status)}>
          {getStatusLabel(status)}
        </Tag>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
      render: (time) => formatTime(time),
      sorter: true,
    },
    {
      title: '操作',
      key: 'actions',
      width: 100,
      render: (_, record) => (
        <Space>
          <Button
            type="text"
            icon={<EyeOutlined />}
            size="small"
            onClick={() => handleViewDetail(record)}
          />
        </Space>
      ),
    },
  ];

  const handleViewDetail = async (alert) => {
    try {
      const detailData = await apiService.getAlert(alert.id);
      setSelectedAlert(detailData);
      setDetailModalVisible(true);
    } catch (error) {
      console.error('获取告警详情失败:', error);
    }
  };

  const handleSearch = (value) => {
    const newFilters = { ...filters, search: value };
    setFilters(newFilters);
    handleFilterChange(newFilters);
  };

  const handleFilterReset = () => {
    setFilters({});
    handleFilterChange({});
  };

  const handleDateRangeChange = (dates) => {
    const newFilters = {
      ...filters,
      start_time: dates?.[0]?.toISOString(),
      end_time: dates?.[1]?.toISOString(),
    };
    setFilters(newFilters);
    handleFilterChange(newFilters);
  };

  const handleSeverityChange = (severity) => {
    const newFilters = { ...filters, severity };
    setFilters(newFilters);
    handleFilterChange(newFilters);
  };

  const handleTypeChange = (type) => {
    const newFilters = { ...filters, type };
    setFilters(newFilters);
    handleFilterChange(newFilters);
  };

  return (
    <div style={{ padding: '0 8px' }}>
      <Card>
        <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
          <Col xs={24} sm={12} md={8} lg={6}>
            <Input.Search
              placeholder="搜索告警..."
              allowClear
              onSearch={handleSearch}
              style={{ width: '100%' }}
            />
          </Col>
          
          <Col xs={24} sm={12} md={8} lg={6}>
            <Select
              placeholder="选择严重程度"
              allowClear
              style={{ width: '100%' }}
              onChange={handleSeverityChange}
              value={filters.severity}
            >
              <Option value="critical">严重</Option>
              <Option value="high">高</Option>
              <Option value="medium">中</Option>
              <Option value="low">低</Option>
            </Select>
          </Col>
          
          <Col xs={24} sm={12} md={8} lg={6}>
            <Select
              placeholder="选择告警类型"
              allowClear
              style={{ width: '100%' }}
              onChange={handleTypeChange}
              value={filters.type}
            >
              <Option value="large_transfer">大额转账</Option>
              <Option value="gas_price">Gas价格</Option>
              <Option value="network_congestion">网络拥堵</Option>
              <Option value="contract_event">合约事件</Option>
            </Select>
          </Col>
          
          <Col xs={24} sm={12} md={8} lg={6}>
            <RangePicker
              style={{ width: '100%' }}
              onChange={handleDateRangeChange}
              placeholder={['开始时间', '结束时间']}
            />
          </Col>
          
          <Col xs={24} sm={24} md={24} lg={24}>
            <Space>
              <Button
                icon={<ReloadOutlined />}
                onClick={refresh}
                loading={loading}
              >
                刷新
              </Button>
              <Button
                icon={<FilterOutlined />}
                onClick={handleFilterReset}
              >
                重置筛选
              </Button>
            </Space>
          </Col>
        </Row>

        <Table
          columns={columns}
          dataSource={alerts}
          loading={loading}
          rowKey="id"
          pagination={{
            ...pagination,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total, range) =>
              `第 ${range[0]}-${range[1]} 条，共 ${total} 条`,
            onChange: handlePageChange,
            onShowSizeChange: handlePageChange,
          }}
          scroll={{ x: 800 }}
        />
      </Card>

      {/* 告警详情模态框 */}
      <Modal
        title="告警详情"
        open={detailModalVisible}
        onCancel={() => setDetailModalVisible(false)}
        footer={null}
        width={800}
      >
        {selectedAlert && (
          <div>
            <Row gutter={16} style={{ marginBottom: 24 }}>
              <Col span={8}>
                <Statistic
                  title="告警ID"
                  value={selectedAlert.id}
                  valueStyle={{ fontSize: '16px' }}
                />
              </Col>
              <Col span={8}>
                <Statistic
                  title="严重程度"
                  value={getSeverityLabel(selectedAlert.severity)}
                  valueStyle={{ 
                    color: getSeverityColor(selectedAlert.severity),
                    fontSize: '16px' 
                  }}
                />
              </Col>
              <Col span={8}>
                <Statistic
                  title="状态"
                  value={getStatusLabel(selectedAlert.status)}
                  valueStyle={{ 
                    color: getStatusColor(selectedAlert.status),
                    fontSize: '16px' 
                  }}
                />
              </Col>
            </Row>

            <Descriptions bordered column={2}>
              <Descriptions.Item label="告警类型">
                {getAlertTypeLabel(selectedAlert.type)}
              </Descriptions.Item>
              <Descriptions.Item label="创建时间">
                {formatTime(selectedAlert.created_at)}
              </Descriptions.Item>
              <Descriptions.Item label="标题" span={2}>
                {selectedAlert.title || '未知告警'}
              </Descriptions.Item>
              <Descriptions.Item label="描述" span={2}>
                {selectedAlert.description || '无描述'}
              </Descriptions.Item>
              <Descriptions.Item label="交易哈希">
                <Text
                  copyable={{
                    text: selectedAlert.transaction_hash,
                    onCopy: () => copyToClipboard(selectedAlert.transaction_hash),
                  }}
                  ellipsis={{ tooltip: selectedAlert.transaction_hash }}
                  style={{ width: 200 }}
                >
                  {selectedAlert.transaction_hash}
                </Text>
              </Descriptions.Item>
              <Descriptions.Item label="金额">
                {formatNumber(selectedAlert.amount)} ETH
              </Descriptions.Item>
              <Descriptions.Item label="发送方">
                <Text
                  copyable={{
                    text: selectedAlert.from_address,
                    onCopy: () => copyToClipboard(selectedAlert.from_address),
                  }}
                  ellipsis={{ tooltip: selectedAlert.from_address }}
                  style={{ width: 200 }}
                >
                  {selectedAlert.from_address}
                </Text>
              </Descriptions.Item>
              <Descriptions.Item label="接收方">
                <Text
                  copyable={{
                    text: selectedAlert.to_address,
                    onCopy: () => copyToClipboard(selectedAlert.to_address),
                  }}
                  ellipsis={{ tooltip: selectedAlert.to_address }}
                  style={{ width: 200 }}
                >
                  {selectedAlert.to_address}
                </Text>
              </Descriptions.Item>
            </Descriptions>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default Alerts;
