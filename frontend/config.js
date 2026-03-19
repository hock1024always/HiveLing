/**
 * 慧备灵师 - 前端全局配置
 *
 * 修改 API_BASE 为你的后端服务地址
 * 如果前端由后端静态文件服务托管，设为空字符串即可自动使用 window.location.origin
 */
const APP_CONFIG = {
    // 后端 API 地址（修改为你的服务器 IP:端口）
    API_BASE: 'http://172.20.19.106:9090',

    // 应用名称
    APP_NAME: '慧备灵师',
};

// 导出供各页面使用
const API_BASE = APP_CONFIG.API_BASE || window.location.origin;
