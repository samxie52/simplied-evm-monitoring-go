#!/bin/bash

echo "🚀 启动以太坊监控系统 - 简单前端"
echo "=================================="

# 检查 Python 是否可用
if command -v python3 &> /dev/null; then
    PYTHON_CMD="python3"
elif command -v python &> /dev/null; then
    PYTHON_CMD="python"
else
    echo "❌ 错误: 未找到 Python 解释器"
    echo "请安装 Python 3.x"
    exit 1
fi

echo "✅ 使用 Python: $PYTHON_CMD"
echo "📍 前端地址: http://localhost:3000"
echo "🔗 后端 API: http://localhost:8080"
echo ""
echo "请确保后端 API 服务器已启动 (端口 8080)"
echo "按 Ctrl+C 停止前端服务器"
echo ""

# 启动服务器
$PYTHON_CMD server.py
