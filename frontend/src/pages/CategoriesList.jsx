import React, { useState, useEffect } from 'react';
import {
  Box,
  Button,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  IconButton,
  Typography,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Alert,
  Snackbar,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import EditIcon from '@mui/icons-material/Edit';
import DeleteIcon from '@mui/icons-material/Delete';
import { categoriesAPI } from '../services/api';

function CategoriesList() {
  const [categories, setCategories] = useState([]);
  const [loading, setLoading] = useState(true);
  const [dialog, setDialog] = useState({ open: false, mode: 'create', category: null });
  const [deleteDialog, setDeleteDialog] = useState({ open: false, category: null });
  const [formData, setFormData] = useState({ name: '' });
  const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'success' });

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      setLoading(true);
      const res = await categoriesAPI.getAll();
      setCategories(res.data.data || []);
    } catch (error) {
      showSnackbar('加载数据失败: ' + error.message, 'error');
    } finally {
      setLoading(false);
    }
  };

  const handleOpenDialog = (mode, category = null) => {
    setDialog({ open: true, mode, category });
    setFormData({ name: category ? category.name : '' });
  };

  const handleCloseDialog = () => {
    setDialog({ open: false, mode: 'create', category: null });
    setFormData({ name: '' });
  };

  const handleSubmit = async () => {
    if (!formData.name.trim()) {
      showSnackbar('请输入分类名称', 'warning');
      return;
    }

    try {
      if (dialog.mode === 'create') {
        await categoriesAPI.create({ name: formData.name });
        showSnackbar('创建成功', 'success');
      } else {
        await categoriesAPI.update(dialog.category.id, { name: formData.name });
        showSnackbar('更新成功', 'success');
      }
      handleCloseDialog();
      loadData();
    } catch (error) {
      showSnackbar(
        (dialog.mode === 'create' ? '创建' : '更新') + '失败: ' +
        (error.response?.data?.error || error.message),
        'error'
      );
    }
  };

  const handleDelete = async () => {
    if (!deleteDialog.category) return;

    try {
      await categoriesAPI.delete(deleteDialog.category.id);
      showSnackbar('删除成功', 'success');
      setDeleteDialog({ open: false, category: null });
      loadData();
    } catch (error) {
      showSnackbar(
        '删除失败: ' + (error.response?.data?.error || error.message),
        'error'
      );
    }
  };

  const showSnackbar = (message, severity = 'success') => {
    setSnackbar({ open: true, message, severity });
  };

  return (
    <Box>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 3 }}>
        <Typography variant="h4">分类管理</Typography>
        <Button
          variant="contained"
          startIcon={<AddIcon />}
          onClick={() => handleOpenDialog('create')}
        >
          添加分类
        </Button>
      </Box>

      <TableContainer component={Paper}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>名称</TableCell>
              <TableCell>应用数量</TableCell>
              <TableCell>排序</TableCell>
              <TableCell>创建时间</TableCell>
              <TableCell align="right">操作</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={5} align="center">
                  加载中...
                </TableCell>
              </TableRow>
            ) : categories.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} align="center">
                  暂无数据
                </TableCell>
              </TableRow>
            ) : (
              categories.map((category) => (
                <TableRow key={category.id}>
                  <TableCell>{category.name}</TableCell>
                  <TableCell>{category.appCount || 0}</TableCell>
                  <TableCell>{category.order}</TableCell>
                  <TableCell>
                    {new Date(category.createdAt).toLocaleString('zh-CN')}
                  </TableCell>
                  <TableCell align="right">
                    <IconButton
                      color="primary"
                      onClick={() => handleOpenDialog('edit', category)}
                    >
                      <EditIcon />
                    </IconButton>
                    <IconButton
                      color="error"
                      onClick={() => setDeleteDialog({ open: true, category })}
                    >
                      <DeleteIcon />
                    </IconButton>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>

      {/* 创建/编辑对话框 */}
      <Dialog open={dialog.open} onClose={handleCloseDialog} maxWidth="sm" fullWidth>
        <DialogTitle>
          {dialog.mode === 'create' ? '添加分类' : '编辑分类'}
        </DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            fullWidth
            label="分类名称"
            value={formData.name}
            onChange={(e) => setFormData({ name: e.target.value })}
            sx={{ mt: 2 }}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={handleCloseDialog}>取消</Button>
          <Button onClick={handleSubmit} variant="contained">
            保存
          </Button>
        </DialogActions>
      </Dialog>

      {/* 删除确认对话框 */}
      <Dialog
        open={deleteDialog.open}
        onClose={() => setDeleteDialog({ open: false, category: null })}
      >
        <DialogTitle>确认删除</DialogTitle>
        <DialogContent>
          确定要删除分类 "{deleteDialog.category?.name}" 吗？
          如果该分类下还有应用，将无法删除。
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteDialog({ open: false, category: null })}>
            取消
          </Button>
          <Button onClick={handleDelete} color="error" variant="contained">
            删除
          </Button>
        </DialogActions>
      </Dialog>

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

export default CategoriesList;


