import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { adminListChats, APIError } from '../../api/client';
import type { AdminChatListItem } from '../../api/client';
import PageHeader from '../../components/PageHeader';
import Skeleton from '../../components/Skeleton';

const PER_PAGE = 20;

export default function AdminChat() {
  const navigate = useNavigate();
  const [items, setItems] = useState<AdminChatListItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    adminListChats({ page: String(page), per_page: String(PER_PAGE) })
      .then((res) => {
        setItems(res.items);
        setTotal(res.total);
      })
      .catch((err) => setError(err instanceof APIError ? err.message : 'Failed to load chats'))
      .finally(() => setLoading(false));
  }, [page]);

  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));

  return (
    <div className="page">
      <PageHeader title="Support Chats" />

      {error && <p className="error">{error}</p>}

      <div className="admin-section">
        {loading ? (
          <Skeleton variant="card" count={3} />
        ) : items.length === 0 ? (
          <p className="admin-empty">No conversations yet.</p>
        ) : (
          <>
            <table className="admin-table">
              <thead>
                <tr>
                  <th>User</th>
                  <th>Last Message</th>
                  <th>Messages</th>
                  <th>Status</th>
                  <th>Last Activity</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr
                    key={item.id}
                    className="admin-table-clickable"
                    onClick={() => navigate(`/admin/chats/${item.id}`)}
                  >
                    <td>
                      <span className="chat-admin-name">{item.display_name}</span>
                      {item.email && (
                        <span className="chat-admin-email"> ({item.email})</span>
                      )}
                    </td>
                    <td className="chat-admin-preview">{item.last_message_preview || '—'}</td>
                    <td>{item.message_count}</td>
                    <td>
                      {item.has_unread && <span className="badge badge-danger">Unread</span>}
                    </td>
                    <td>{new Date(item.updated_at).toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>

            {total > PER_PAGE && (
              <div className="admin-pagination">
                <span className="admin-pagination-info">
                  {total} conversation{total !== 1 ? 's' : ''}
                </span>
                <div className="admin-pagination-controls">
                  <button
                    className="btn btn-secondary btn-small"
                    disabled={page <= 1}
                    onClick={() => setPage((p) => p - 1)}
                  >
                    Previous
                  </button>
                  <span>Page {page} of {totalPages}</span>
                  <button
                    className="btn btn-secondary btn-small"
                    disabled={page >= totalPages}
                    onClick={() => setPage((p) => p + 1)}
                  >
                    Next
                  </button>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
