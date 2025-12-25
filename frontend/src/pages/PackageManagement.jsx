import React, { useState, useEffect } from 'react';
import {
  Box,
  Paper,
  Typography,
  Button,
  Card,
  CardContent,
  Grid,
  Chip,
  Alert,
  Snackbar,
  CircularProgress,
  Divider,
} from '@mui/material';
import BuildIcon from '@mui/icons-material/Build';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import ErrorIcon from '@mui/icons-material/Error';
import { packageAPI, versionAPI } from '../services/api';

function PackageManagement() {
  const [packageInfo, setPackageInfo] = useState(null);
  const [serverVersion, setServerVersion] = useState(null);
  const [loading, setLoading] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'success' });

  useEffect(() => {
    loadData();
    // 自动刷新（每5秒）
    const interval = setInterval(loadData, 5000);
    return () => clearInterval(interval);
  }, []);

  const loadData = async () => {
    try {
      const [packageRes, versionRes] = await Promise.all([
        packageAPI.getStatus(),
        versionAPI.get(),
      ]);
      
      setPackageInfo(packageRes.data.data);
      setServerVersion(versionRes.data.data);
    } catch (error) {
      console.error('加载数据失败:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleGenerate = async () => {
    try {
      setGenerating(true);
      await packageAPI.generate();
      showSnackbar('客户端包生成任务已启动，请稍候...', 'info');
      
      // 延迟后刷新状态
      setTimeout(loadData, 2000);
    } catch (error) {
      showSnackbar(
        '生成失败: ' + (error.response?.data?.error || error.message),
        'error'
      );
    } finally {
      setGenerating(false);
    }
  };

  const showSnackbar = (message, severity = 'success') => {
    setSnackbar({ open: true, message, severity });
  };

  const formatFileSize = (bytes) => {
    if (!bytes) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
  };

  const getStatusInfo = (status) => {
    switch (status) {
      case 'ready':
        return { color: 'success', icon: <CheckCircleIcon />, text: '就绪' };
      case 'generating':
        return { color: 'info', icon: <CircularProgress size={20} />, text: '生成中...' };
      case 'error':
        return { color: 'error', icon: <ErrorIcon />, text: '错误' };
      default:
        return { color: 'default', icon: null, text: '未生成' };
    }
  };

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '50vh' }}>
        <CircularProgress />
      </Box>
    );
  }

  const statusInfo = getStatusInfo(packageInfo?.status);
  const needsUpdate = serverVersion && packageInfo && 
    serverVersion.version > packageInfo.dbVersion;

  return (
    <Box>
      <Typography variant="h4" sx={{ mb: 3 }}>
        客户端包管理
      </Typography>

      {needsUpdate && (
        <Alert severity="warning" sx={{ mb: 3 }}>
          服务器数据已更新（版本 {serverVersion.version}），建议重新生成客户端包
        </Alert>
      )}

      <Grid container spacing={3}>
        {/* 状态卡片 */}
        <Grid item xs={12} md={6}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                客户端包状态
              </Typography>
              <Divider sx={{ my: 2 }} />
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                  <Typography color="text.secondary">状态：</Typography>
                  <Chip
                    label={statusInfo.text}
                    color={statusInfo.color}
                    icon={statusInfo.icon}
                    size="small"
                  />
                </Box>
                <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                  <Typography color="text.secondary">客户端包版本：</Typography>
                  <Typography>{packageInfo?.packageVersion || 0}</Typography>
                </Box>
                <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                  <Typography color="text.secondary">包含数据版本：</Typography>
                  <Typography>{packageInfo?.dbVersion || 0}</Typography>
                </Box>
                <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                  <Typography color="text.secondary">生成时间：</Typography>
                  <Typography>
                    {packageInfo?.generatedAt
                      ? new Date(packageInfo.generatedAt).toLocaleString('zh-CN')
                      : '未生成'}
                  </Typography>
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>

        {/* 文件信息卡片 */}
        <Grid item xs={12} md={6}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                文件信息
              </Typography>
              <Divider sx={{ my: 2 }} />
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                  <Typography color="text.secondary">数据库文件：</Typography>
                  <Typography>{formatFileSize(packageInfo?.dbSize)}</Typography>
                </Box>
                <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                  <Typography color="text.secondary">资源包文件：</Typography>
                  <Typography>{formatFileSize(packageInfo?.resourcesSize)}</Typography>
                </Box>
                <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                  <Typography color="text.secondary">总大小：</Typography>
                  <Typography fontWeight="bold">
                    {formatFileSize((packageInfo?.dbSize || 0) + (packageInfo?.resourcesSize || 0))}
                  </Typography>
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>

        {/* 服务器数据版本 */}
        <Grid item xs={12}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                服务器数据版本
              </Typography>
              <Divider sx={{ my: 2 }} />
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <Box>
                  <Typography color="text.secondary">当前数据版本：</Typography>
                  <Typography variant="h5" sx={{ mt: 1 }}>
                    {serverVersion?.version || 0}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    最后更新：{serverVersion?.updatedAt
                      ? new Date(serverVersion.updatedAt).toLocaleString('zh-CN')
                      : '未知'}
                  </Typography>
                </Box>
                <Button
                  variant="contained"
                  size="large"
                  startIcon={generating ? <CircularProgress size={20} color="inherit" /> : <BuildIcon />}
                  onClick={handleGenerate}
                  disabled={generating || packageInfo?.status === 'generating'}
                >
                  {generating || packageInfo?.status === 'generating' ? '生成中...' : '生成客户端包'}
                </Button>
              </Box>
            </CardContent>
          </Card>
        </Grid>

        {/* 使用说明 */}
        <Grid item xs={12}>
          <Paper sx={{ p: 3 }}>
            <Typography variant="h6" gutterBottom>
              使用说明
            </Typography>
            <Typography variant="body2" color="text.secondary" paragraph>
              1. 当管理员修改应用或分类数据后，服务器数据版本会自动增加
            </Typography>
            <Typography variant="body2" color="text.secondary" paragraph>
              2. 点击"生成客户端包"按钮，系统会从 MySQL 导出数据到 SQLite 数据库，并打包所有图片资源
            </Typography>
            <Typography variant="body2" color="text.secondary" paragraph>
              3. 生成完成后，客户端可以通过 API 下载最新的数据库和资源包
            </Typography>
            <Typography variant="body2" color="text.secondary">
              4. 建议在完成一批数据修改后统一生成客户端包，避免频繁生成
            </Typography>
          </Paper>
        </Grid>
      </Grid>

      {/* 提示消息 */}
      <Snackbar
        open={snackbar.open}
        autoHideDuration={3000}
        onClose={() => setSnackbar({ ...snackbar, open: false })}
      >
        <Alert severity={snackbar.severity} sx={{ width: '100%' }}>
          {snackbar.message}
        </Alert>
      </Snackbar>
    </Box>
  );
}

export default PackageManagement;


