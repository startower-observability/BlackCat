import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import AuthGuard from './components/AuthGuard';
import DashboardLayout from './layouts/DashboardLayout';
import HomePage from './pages/HomePage';
import AgentsPage from './pages/AgentsPage';
import TasksPage from './pages/TasksPage';
import SchedulePage from './pages/SchedulePage';
import QRPage from './pages/QRPage';
import LoginPage from './pages/LoginPage';
import './styles/rpg-theme.css';

export default function App() {
  return (
    <BrowserRouter basename="/dashboard">
      <Routes>
        <Route path="login" element={<LoginPage />} />
        <Route element={<AuthGuard />}>
          <Route element={<DashboardLayout />}>
            <Route index element={<HomePage />} />
            <Route path="agents" element={<AgentsPage />} />
            <Route path="tasks" element={<TasksPage />} />
            <Route path="schedule" element={<SchedulePage />} />
            <Route path="qr" element={<QRPage />} />
          </Route>
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
