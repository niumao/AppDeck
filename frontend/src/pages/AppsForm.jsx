import React, { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  Box,
  Paper,
  Typography,
  TextField,
  Button,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Chip,
  OutlinedInput,
  Alert,
  Snackbar,
} from '@mui/material';
import { appsAPI, categoriesAPI } from '../services/api';

const ITEM_HEIGHT = 48;
const ITEM_PADDING_TOP = 8;
const MenuProps = {
  PaperProps: {
    style: {
      maxHeight: ITEM_HEIGHT * 4.5 + ITEM_PADDING_TOP,
      width: 250,
    },
  },
};

function AppsForm() {
  const navigate = useNavigate();
  const { id } = useParams();
  const isEdit = Boolean(id);

  const [categories, setCategories] = useState([]);
  const [formData, setFormData] = useState({
    name: '',
    description: '',
    iconUrl: '',
    bigPicUrl: '',
    launchParam: '',
    categoryIds: [],
  });
  const [loading, setLoading] = useState(false);
  const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'success' });

  useEffect(() => {
    loadCategories();
    if (isEdit) {
      loadApp();
    }
  }, [id]);

  const loadCategories = async () => {
    try {
      const res = await categoriesAPI.getAll();
      setCategories(res.data.data || []);
    } catch (error) {
      showSnackbar('加载分类失败: ' + error.message, 'error');
    }
  };

  const loadApp = async () => {
    try {
      const res = await appsAPI.getById(id);
      const app = res.data.data;
      setFormData({
        name: app.name || '',
        description: app.description || '',
        iconUrl: app.iconUrl || '',
        bigPicUrl: app.bigPicUrl || '',
        launchParam: app.launchParam || '',
        categoryIds: app.categoryIds || [],
      });
    } catch (error) {
      showSnackbar('加载应用数据失败: ' + error.message, 'error');
    }
  };

  const handleChange = (field) => (event) => {
    setFormData({
      ...formData,
      [field]: event.target.value,
    });
  };

  const handleSubmit = async (event) => {
    event.preventDefault();
    
    // 验证
    if (!formData.name.trim()) {
      showSnackbar('请输入应用名称', 'warning');
      return;
    }
    if (!formData.launchParam.trim()) {
      showSnackbar('请输入启动参数', 'warning');
      return;
    }
    if (formData.categoryIds.length === 0) {
      showSnackbar('请至少选择一个分类', 'warning');
      return;
    }

    setLoading(true);
    try {
      if (isEdit) {
        await appsAPI.update(id, formData);
        showSnackbar('更新成功', 'success');
      } else {
        await appsAPI.create(formData);
        showSnackbar('创建成功', 'success');
      }
      setTimeout(() => {
        navigate('/apps');
      }, 1000);
    } catch (error) {
      showSnackbar(
        (isEdit ? '更新' : '创建') + '失败: ' + (error.response?.data?.error || error.message),
        'error'
      );
    } finally {
      setLoading(false);
    }
  };

  const showSnackbar = (message, severity = 'success') => {
    setSnackbar({ open: true, message, severity });
  };

  return (
    <Box>
      <Typography variant="h4" sx={{ mb: 3 }}>
        {isEdit ? '编辑应用' : '添加应用'}
      </Typography>

      <Paper sx={{ p: 3 }}>
        <form onSubmit={handleSubmit}>
          <TextField
            fullWidth
            label="应用名称"
            value={formData.name}
            onChange={handleChange('name')}
            required
            sx={{ mb: 2 }}
          />

          <TextField
            fullWidth
            label="应用描述"
            value={formData.description}
            onChange={handleChange('description')}
            multiline
            rows={3}
            sx={{ mb: 2 }}
          />

          <TextField
            fullWidth
            label="图标 URL"
            value={formData.iconUrl}
            onChange={handleChange('iconUrl')}
            placeholder="http://server:8080/api/v1/icons/app.png"
            required
            sx={{ mb: 2 }}
          />

          <TextField
            fullWidth
            label="大图 URL"
            value={formData.bigPicUrl}
            onChange={handleChange('bigPicUrl')}
            placeholder="http://server:8080/api/v1/bigpics/app.jpg"
            required
            sx={{ mb: 2 }}
          />

          <TextField
            fullWidth
            label="启动参数"
            value={formData.launchParam}
            onChange={handleChange('launchParam')}
            placeholder="app-launch-param"
            required
            sx={{ mb: 2 }}
          />

          <FormControl fullWidth sx={{ mb: 3 }}>
            <InputLabel id="categories-label">分类（可多选）*</InputLabel>
            <Select
              labelId="categories-label"
              multiple
              value={formData.categoryIds}
              onChange={handleChange('categoryIds')}
              input={<OutlinedInput label="分类（可多选）*" />}
              renderValue={(selected) => (
                <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                  {selected.map((value) => {
                    const category = categories.find((c) => c.id === value);
                    return (
                      <Chip key={value} label={category?.name || value} size="small" />
                    );
                  })}
                </Box>
              )}
              MenuProps={MenuProps}
            >
              {categories.map((category) => (
                <MenuItem key={category.id} value={category.id}>
                  {category.name}
                </MenuItem>
              ))}
            </Select>
          </FormControl>

          <Box sx={{ display: 'flex', gap: 2 }}>
            <Button
              type="submit"
              variant="contained"
              disabled={loading}
            >
              {loading ? '保存中...' : '保存'}
            </Button>
            <Button
              variant="outlined"
              onClick={() => navigate('/apps')}
              disabled={loading}
            >
              取消
            </Button>
          </Box>
        </form>
      </Paper>

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

export default AppsForm;


