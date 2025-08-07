import React from 'react';
import { Layout, Space, Badge, Button, Dropdown, Avatar } from 'antd';
import {
  BellOutlined,
  UserOutlined,
  SettingOutlined,
  LogoutOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { useApi } from '../hooks/useApi';
import apiService from '../services/api';
import { getStatusColor, formatTime } from '../utils/helpers';

const { Header: AntHeader } = Layout;

const Header = () => {
  const { data: healthData, loading: healthLoading, refetch: refetchHealth } = useApi(
    apiService.getHealth,
    [],
    { immediate: true }
  );

  const userMenuItems = [
    {
      key: 'settings',
      icon: <SettingOutlined />,
      label: '设置',
    },
    {
      type: 'divider',
    },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
      danger: true,
    },
  ];

  const handleUserMenuClick = ({ key }) => {
    switch (key) {
      case 'settings':
        // TODO: 打开设置页面
        console.log('打开设置');
        break;
      case 'logout':
        // TODO: 退出登录
        console.log('退出登录');
        break;
      default:
        break;
    }
  };

  const handleRefresh = () => {
    refetchHealth();
    window.location.reload();
  };

  return (
    <AntHeader
      style={{
        padding: '0 24px',
        background: '#fff',
        borderBottom: '1px solid #f0f0f0',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center' }}>
        <h2 style={{ margin: 0, color: '#1890ff' }}>
          以太坊监控系统
        </h2>
        {healthData && (
          <Space style={{ marginLeft: 24 }}>
            <Badge
              color={getStatusColor(healthData.status)}
              text={`系统状态: ${healthData.status || '未知'}`}
            />
            <span style={{ color: '#666', fontSize: '12px' }}>
              更新时间: {formatTime(healthData.timestamp)}
            </span>
          </Space>
        )}
      </div>

      <Space size="middle">
        <Button
          type="text"
          icon={<ReloadOutlined />}
          loading={healthLoading}
          onClick={handleRefresh}
          title="刷新页面"
        />
        
        <Badge count={0} showZero={false}>
          <Button
            type="text"
            icon={<BellOutlined />}
            title="通知"
          />
        </Badge>

        <Dropdown
          menu={{
            items: userMenuItems,
            onClick: handleUserMenuClick,
          }}
          placement="bottomRight"
        >
          <Button type="text" style={{ padding: '4px 8px' }}>
            <Space>
              <Avatar size="small" icon={<UserOutlined />} />
              <span>管理员</span>
            </Space>
          </Button>
        </Dropdown>
      </Space>
    </AntHeader>
  );
};

export default Header;
