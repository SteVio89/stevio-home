import { useEffect, useState } from 'react';
import { Link, NavLink, Outlet, useLocation } from 'react-router-dom';
import { adminChatUnreadCount } from '../api/client';

interface NavGroup {
  label: string;
  links: { to: string; label: string; isActive?: (path: string) => boolean }[];
}

const NAV_GROUPS: NavGroup[] = [
  {
    label: 'Overview',
    links: [
      { to: '/admin', label: 'Dashboard', isActive: (p) => p === '/admin' },
    ],
  },
  {
    label: 'Commerce',
    links: [
      { to: '/admin/orders', label: 'Orders' },
      { to: '/admin/licenses', label: 'Licenses' },
      { to: '/admin/discount-codes', label: 'Discounts' },
      { to: '/admin/sales', label: 'Sales Report' },
    ],
  },
  {
    label: 'Users',
    links: [
      { to: '/admin/users', label: 'User Lookup' },
      { to: '/admin/chats', label: 'Support', isActive: (p) => p === '/admin/chats' || p.startsWith('/admin/chats/') },
    ],
  },
  {
    label: 'Content',
    links: [
      { to: '/admin/legal-pages', label: 'Legal Pages' },
      { to: '/admin/languages', label: 'Languages' },
      { to: '/admin/hero', label: 'Hero' },
      { to: '/admin/projects', label: 'Projects', isActive: (p) => p === '/admin/projects' || p.startsWith('/admin/projects/') },
      { to: '/admin/social-links', label: 'Social Links', isActive: (p) => p === '/admin/social-links' || p.startsWith('/admin/social-links/') },
    ],
  },
  {
    label: 'Customization',
    links: [
      { to: '/admin/ui-translations', label: 'Translations' },
      { to: '/admin/mail-templates', label: 'Mail Templates' },
    ],
  },
  {
    label: 'System',
    links: [
      { to: '/admin/signing-keys', label: 'Signing Key' },
      { to: '/admin/payment', label: 'Payment' },
      { to: '/admin/settings', label: 'Settings' },
    ],
  },
];

const UNREAD_POLL_INTERVAL = 30_000;

export default function AdminLayout() {
  const { pathname } = useLocation();
  const [unreadCount, setUnreadCount] = useState(0);

  useEffect(() => {
    function fetchUnread() {
      adminChatUnreadCount()
        .then((r) => setUnreadCount(r.count))
        .catch(() => {});
    }
    fetchUnread();
    const interval = setInterval(fetchUnread, UNREAD_POLL_INTERVAL);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="admin-shell">
      <aside className="admin-sidebar">
        <div className="admin-sidebar-brand">
          <span className="admin-sidebar-prompt">&gt;_ admin</span>
        </div>
        <nav className="admin-nav">
          {NAV_GROUPS.map((group) => (
            <div key={group.label} className="admin-nav-group">
              <div className="admin-nav-group-label">{group.label}</div>
              {group.links.map((link) => {
                const active = link.isActive
                  ? link.isActive(pathname)
                  : pathname === link.to;
                return (
                  <NavLink
                    key={link.to}
                    to={link.to}
                    className={() => `admin-nav-link${active ? ' admin-nav-link-active' : ''}`}
                  >
                    {link.label}
                    {link.to === '/admin/chats' && unreadCount > 0 && (
                      <span className="admin-nav-badge">{unreadCount}</span>
                    )}
                  </NavLink>
                );
              })}
            </div>
          ))}
        </nav>
        <Link to="/" className="admin-nav-store">&larr; Site</Link>
      </aside>
      <div className="admin-content">
        <Outlet />
      </div>
    </div>
  );
}
