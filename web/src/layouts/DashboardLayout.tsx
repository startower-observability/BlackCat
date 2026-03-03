import { Outlet, NavLink } from 'react-router-dom';

export default function DashboardLayout() {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', minHeight: '100vh' }}>
      <nav className="rpg-nav">
        <NavLink to="." end className={({ isActive }) => isActive ? 'active' : ''}>OVERVIEW</NavLink>
        <NavLink to="agents" className={({ isActive }) => isActive ? 'active' : ''}>AGENTS</NavLink>
        <NavLink to="tasks" className={({ isActive }) => isActive ? 'active' : ''}>TASKS</NavLink>
        <NavLink to="schedule" className={({ isActive }) => isActive ? 'active' : ''}>SCHEDULE</NavLink>
        <NavLink to="qr" className={({ isActive }) => isActive ? 'active' : ''}>QR</NavLink>
      </nav>
      <div style={{ flex: 1 }}>
        <Outlet />
      </div>
    </div>
  );
}
