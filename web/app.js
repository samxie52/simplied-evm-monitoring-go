// API 基础配置
const API_BASE_URL = 'http://localhost:8080';

// API 调用封装
class ApiService {
    async request(endpoint) {
        try {
            const response = await fetch(`${API_BASE_URL}${endpoint}`);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            return await response.json();
        } catch (error) {
            console.error(`API 请求失败 [${endpoint}]:`, error);
            throw error;
        }
    }

    // 获取系统健康状态
    async getHealth() {
        return this.request('/health');
    }

    // 获取系统状态
    async getSystemStatus() {
        return this.request('/api/v1/monitoring/status');
    }

    // 获取系统指标
    async getSystemMetrics() {
        return this.request('/api/v1/monitoring/metrics');
    }

    // 获取告警统计
    async getAlertStats() {
        return this.request('/api/v1/alerts/stats');
    }

    // 获取最近告警
    async getRecentAlerts() {
        return this.request('/api/v1/alerts?page=1&page_size=10');
    }
}

const apiService = new ApiService();

// DOM 元素引用
const elements = {
    systemStatus: document.getElementById('system-status'),
    systemMetrics: document.getElementById('system-metrics'),
    alertStats: document.getElementById('alert-stats'),
    ethereumStatus: document.getElementById('ethereum-status'),
    recentAlerts: document.getElementById('recent-alerts'),
    lastUpdateTime: document.getElementById('last-update-time')
};

// 工具函数
function formatTime(timestamp) {
    if (!timestamp) return '--';
    return new Date(timestamp).toLocaleString('zh-CN');
}

function formatNumber(num) {
    if (typeof num !== 'number') return '--';
    return num.toLocaleString();
}

function getSeverityClass(severity) {
    const severityMap = {
        'high': 'alert-high',
        'medium': 'alert-medium',
        'low': 'alert-low'
    };
    return severityMap[severity] || 'alert-low';
}

function getStatusIndicator(status) {
    const statusMap = {
        'healthy': 'status-healthy',
        'warning': 'status-warning',
        'error': 'status-error'
    };
    return statusMap[status] || 'status-error';
}

// 渲染函数
function renderSystemMetrics(data) {
    if (!data) {
        elements.systemMetrics.innerHTML = '<div class="error">无法获取系统指标</div>';
        return;
    }

    const metrics = [
        { label: 'CPU 使用率', value: `${data.cpu_usage || 0}%` },
        { label: '内存使用率', value: `${data.memory_usage || 0}%` },
        { label: '磁盘使用率', value: `${data.disk_usage || 0}%` },
        { label: '网络延迟', value: `${data.network_latency || 0}ms` },
        { label: '活跃连接', value: formatNumber(data.active_connections || 0) },
        { label: '运行时间', value: data.uptime || '--' }
    ];

    elements.systemMetrics.innerHTML = metrics.map(metric => `
        <div class="metric">
            <span class="metric-label">${metric.label}</span>
            <span class="metric-value">${metric.value}</span>
        </div>
    `).join('');
}

function renderAlertStats(data) {
    if (!data) {
        elements.alertStats.innerHTML = '<div class="error">无法获取告警统计</div>';
        return;
    }

    const stats = [
        { label: '总告警数', value: formatNumber(data.total_alerts || 0) },
        { label: '高级告警', value: formatNumber(data.high_severity || 0) },
        { label: '中级告警', value: formatNumber(data.medium_severity || 0) },
        { label: '低级告警', value: formatNumber(data.low_severity || 0) },
        { label: '今日新增', value: formatNumber(data.today_alerts || 0) },
        { label: '活跃规则', value: formatNumber(data.active_rules || 0) }
    ];

    elements.alertStats.innerHTML = stats.map(stat => `
        <div class="metric">
            <span class="metric-label">${stat.label}</span>
            <span class="metric-value">${stat.value}</span>
        </div>
    `).join('');
}

function renderEthereumStatus(healthData, metricsData) {
    const ethereumHealthy = healthData?.ethereum_manager === 'healthy';
    const status = [
        { label: '连接状态', value: ethereumHealthy ? '✅ 已连接' : '❌ 断开' },
        { label: '网络', value: 'Mainnet' },
        { label: '最新区块', value: formatNumber(metricsData?.latest_block || 0) },
        { label: '同步状态', value: ethereumHealthy ? '✅ 已同步' : '❌ 未同步' },
        { label: '节点延迟', value: `${metricsData?.node_latency || 0}ms` },
        { label: 'Gas 价格', value: `${metricsData?.gas_price || 0} Gwei` }
    ];

    elements.ethereumStatus.innerHTML = status.map(item => `
        <div class="metric">
            <span class="metric-label">${item.label}</span>
            <span class="metric-value">${item.value}</span>
        </div>
    `).join('');
}

function renderRecentAlerts(data) {
    if (!data || !data.alerts || data.alerts.length === 0) {
        elements.recentAlerts.innerHTML = '<div class="loading">暂无最近告警</div>';
        return;
    }

    elements.recentAlerts.innerHTML = data.alerts.map(alert => `
        <div class="alert-item ${getSeverityClass(alert.severity)}">
            <div class="alert-title">
                ${alert.title || alert.type || '未知告警'}
                <span style="float: right; font-size: 0.9em; color: #666;">
                    ${alert.severity?.toUpperCase() || 'UNKNOWN'}
                </span>
            </div>
            <div class="alert-time">
                ${formatTime(alert.created_at)} | ${alert.message || '无详细信息'}
            </div>
        </div>
    `).join('');
}

function updateSystemStatus(healthData) {
    const isHealthy = healthData?.status === 'healthy';
    const statusText = isHealthy ? '系统运行正常' : '系统异常';
    const statusClass = isHealthy ? 'status-healthy' : 'status-error';
    
    elements.systemStatus.innerHTML = `
        <span class="status-indicator ${statusClass}"></span>
        ${statusText}
    `;
}

function updateLastUpdateTime() {
    elements.lastUpdateTime.textContent = formatTime(new Date());
}

// 数据加载函数
async function loadSystemData() {
    try {
        const [healthData, systemMetrics, alertStats, recentAlerts] = await Promise.allSettled([
            apiService.getHealth(),
            apiService.getSystemMetrics(),
            apiService.getAlertStats(),
            apiService.getRecentAlerts()
        ]);

        // 更新系统状态
        if (healthData.status === 'fulfilled') {
            updateSystemStatus(healthData.value);
            renderEthereumStatus(healthData.value, systemMetrics.status === 'fulfilled' ? systemMetrics.value : null);
        }

        // 更新系统指标
        if (systemMetrics.status === 'fulfilled') {
            renderSystemMetrics(systemMetrics.value);
        } else {
            elements.systemMetrics.innerHTML = '<div class="error">系统指标加载失败</div>';
        }

        // 更新告警统计
        if (alertStats.status === 'fulfilled') {
            renderAlertStats(alertStats.value);
        } else {
            elements.alertStats.innerHTML = '<div class="error">告警统计加载失败</div>';
        }

        // 更新最近告警
        if (recentAlerts.status === 'fulfilled') {
            renderRecentAlerts(recentAlerts.value);
        } else {
            elements.recentAlerts.innerHTML = '<div class="error">最近告警加载失败</div>';
        }

        updateLastUpdateTime();

    } catch (error) {
        console.error('数据加载失败:', error);
        // 显示全局错误
        document.querySelector('.container').insertAdjacentHTML('afterbegin', `
            <div class="error" style="margin-bottom: 20px;">
                ⚠️ 数据加载失败: ${error.message}
            </div>
        `);
    }
}

// 刷新所有数据
function refreshAllData() {
    // 清除之前的错误信息
    const errorElements = document.querySelectorAll('.error');
    errorElements.forEach(el => {
        if (el.style.marginBottom === '20px') {
            el.remove();
        }
    });

    // 显示加载状态
    elements.systemMetrics.innerHTML = '<div class="loading">正在刷新系统状态...</div>';
    elements.alertStats.innerHTML = '<div class="loading">正在刷新告警统计...</div>';
    elements.ethereumStatus.innerHTML = '<div class="loading">正在刷新以太坊状态...</div>';
    elements.recentAlerts.innerHTML = '<div class="loading">正在刷新最近告警...</div>';

    // 重新加载数据
    loadSystemData();
}

// 初始化应用
document.addEventListener('DOMContentLoaded', function() {
    console.log('🚀 以太坊监控系统启动');
    loadSystemData();
    
    // 设置自动刷新 (每2分钟)
    setInterval(loadSystemData, 120000);
});

// 全局错误处理
window.addEventListener('error', function(event) {
    console.error('全局错误:', event.error);
});

// 导出到全局作用域供 HTML 调用
window.refreshAllData = refreshAllData;
