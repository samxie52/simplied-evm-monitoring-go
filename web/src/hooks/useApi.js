import { useState, useEffect, useCallback } from 'react';
import { message } from 'antd';

// 通用 API Hook
export const useApi = (apiCall, dependencies = [], options = {}) => {
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  const { 
    immediate = true, 
    onSuccess, 
    onError,
    showErrorMessage = true 
  } = options;

  const execute = useCallback(async (...args) => {
    try {
      setLoading(true);
      setError(null);
      const result = await apiCall(...args);
      setData(result);
      onSuccess?.(result);
      return result;
    } catch (err) {
      const errorMsg = err.response?.data?.message || err.message || '请求失败';
      setError(errorMsg);
      if (showErrorMessage) {
        message.error(errorMsg);
      }
      onError?.(err);
      throw err;
    } finally {
      setLoading(false);
    }
  }, [apiCall, onSuccess, onError, showErrorMessage]);

  useEffect(() => {
    if (immediate) {
      execute();
    }
  }, dependencies);

  return { data, loading, error, execute, refetch: execute };
};

// 轮询 Hook
export const usePolling = (apiCall, interval = 30000, dependencies = []) => {
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [isPolling, setIsPolling] = useState(false);

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const result = await apiCall();
      setData(result);
      return result;
    } catch (err) {
      const errorMsg = err.response?.data?.message || err.message || '请求失败';
      setError(errorMsg);
      console.error('轮询请求失败:', errorMsg);
    } finally {
      setLoading(false);
    }
  }, [apiCall]);

  const startPolling = useCallback(() => {
    setIsPolling(true);
  }, []);

  const stopPolling = useCallback(() => {
    setIsPolling(false);
  }, []);

  useEffect(() => {
    let intervalId;
    
    if (isPolling) {
      // 立即执行一次
      fetchData();
      
      // 设置定时器
      intervalId = setInterval(fetchData, interval);
    }

    return () => {
      if (intervalId) {
        clearInterval(intervalId);
      }
    };
  }, [isPolling, fetchData, interval, ...dependencies]);

  // 组件挂载时开始轮询
  useEffect(() => {
    startPolling();
    return stopPolling;
  }, []);

  return { 
    data, 
    loading, 
    error, 
    isPolling, 
    startPolling, 
    stopPolling, 
    refetch: fetchData 
  };
};

// 分页 Hook
export const usePagination = (apiCall, pageSize = 20) => {
  const [data, setData] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize,
    total: 0,
  });
  const [filters, setFilters] = useState({});

  const fetchData = useCallback(async (page = 1, size = pageSize, filterParams = {}) => {
    try {
      setLoading(true);
      setError(null);
      
      const params = {
        page,
        page_size: size,
        ...filterParams,
      };
      
      const result = await apiCall(params);
      
      setData(result.data || []);
      setPagination({
        current: page,
        pageSize: size,
        total: result.total || 0,
      });
      
      return result;
    } catch (err) {
      const errorMsg = err.response?.data?.message || err.message || '请求失败';
      setError(errorMsg);
      message.error(errorMsg);
    } finally {
      setLoading(false);
    }
  }, [apiCall, pageSize]);

  const handlePageChange = useCallback((page, size) => {
    fetchData(page, size, filters);
  }, [fetchData, filters]);

  const handleFilterChange = useCallback((newFilters) => {
    setFilters(newFilters);
    fetchData(1, pagination.pageSize, newFilters);
  }, [fetchData, pagination.pageSize]);

  const refresh = useCallback(() => {
    fetchData(pagination.current, pagination.pageSize, filters);
  }, [fetchData, pagination.current, pagination.pageSize, filters]);

  // 初始加载
  useEffect(() => {
    fetchData();
  }, []);

  return {
    data,
    loading,
    error,
    pagination,
    filters,
    handlePageChange,
    handleFilterChange,
    refresh,
  };
};
