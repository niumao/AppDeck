import React from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import Layout from './components/Layout';
import AppsList from './pages/AppsList';
import AppsForm from './pages/AppsForm';
import CategoriesList from './pages/CategoriesList';
import PackageManagement from './pages/PackageManagement';

function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Navigate to="/apps" replace />} />
        <Route path="/apps" element={<AppsList />} />
        <Route path="/apps/new" element={<AppsForm />} />
        <Route path="/apps/edit/:id" element={<AppsForm />} />
        <Route path="/categories" element={<CategoriesList />} />
        <Route path="/package" element={<PackageManagement />} />
      </Routes>
    </Layout>
  );
}

export default App;


