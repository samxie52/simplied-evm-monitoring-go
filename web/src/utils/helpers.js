import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';
import 'dayjs/locale/zh-cn';

// 配置 dayjs
dayjs.extend(relativeTime);
dayjs.locale('zh-cn');

// 格式化时间
export const formatTime = (timestamp, format = 'YYYY-MM-DD HH:mm:ss') => {
  if (!timestamp) return '-';
  return dayjs(timestamp).format(format);
};

// 相对时间
export const formatRelativeTime = (timestamp) => {
  if (!timestamp) return '-';
  return dayjs(timestamp).fromNow();
};

// 格式化数字
export const formatNumber = (num, decimals = 2) => {
  if (num === null || num === undefined) return '-';
  return Number(num).toLocaleString('zh-CN', {
    minimumFractionDigits: 0,
    maximumFractionDigits: decimals,
  });
};

// 格式化文件大小
export const formatBytes = (bytes, decimals = 2) => {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
};

// 格式化百分比
export const formatPercent = (value, decimals = 1) => {
  if (value === null || value === undefined) return '-';
  return `${Number(value).toFixed(decimals)}%`;
};

// 获取状态颜色
export const getStatusColor = (status) => {
  const statusColors = {
    healthy: '#52c41a',
    unhealthy: '#ff4d4f',
    degraded: '#faad14',
    unavailable: '#d9d9d9',
    active: '#52c41a',
    inactive: '#d9d9d9',
    sent: '#52c41a',
    pending: '#faad14',
    failed: '#ff4d4f',
  };
  return statusColors[status?.toLowerCase()] || '#d9d9d9';
};

// 获取严重程度颜色
export const getSeverityColor = (severity) => {
  const severityColors = {
    critical: '#ff4d4f',
    high: '#fa8c16',
    medium: '#faad14',
    low: '#52c41a',
  };
  return severityColors[severity?.toLowerCase()] || '#d9d9d9';
};

// 获取严重程度标签
export const getSeverityLabel = (severity) => {
  const severityLabels = {
    critical: '严重',
    high: '高',
    medium: '中',
    low: '低',
  };
  return severityLabels[severity?.toLowerCase()] || severity;
};

// 获取告警类型标签
export const getAlertTypeLabel = (type) => {
  const typeLabels = {
    large_transfer: '大额转账',
    gas_price: 'Gas价格',
    network_congestion: '网络拥堵',
    contract_event: '合约事件',
    token_transfer: '代币转账',
    system_health: '系统健康',
    custom: '自定义',
  };
  return typeLabels[type?.toLowerCase()] || type;
};

// 获取状态标签
export const getStatusLabel = (status) => {
  const statusLabels = {
    healthy: '健康',
    unhealthy: '异常',
    degraded: '降级',
    unavailable: '不可用',
    active: '活跃',
    inactive: '非活跃',
    sent: '已发送',
    pending: '待发送',
    failed: '发送失败',
  };
  return statusLabels[status?.toLowerCase()] || status;
};

// 复制到剪贴板
export const copyToClipboard = async (text) => {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch (err) {
    console.error('复制失败:', err);
    return false;
  }
};

// 防抖函数
export const debounce = (func, wait) => {
  let timeout;
  return function executedFunction(...args) {
    const later = () => {
      clearTimeout(timeout);
      func(...args);
    };
    clearTimeout(timeout);
    timeout = setTimeout(later, wait);
  };
};

// 节流函数
export const throttle = (func, limit) => {
  let inThrottle;
  return function executedFunction(...args) {
    if (!inThrottle) {
      func.apply(this, args);
      inThrottle = true;
      setTimeout(() => inThrottle = false, limit);
    }
  };
};
