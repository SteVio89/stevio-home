import { useEffect, useState, useRef, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useSiteConfig } from '../../context/SiteConfigContext';
import {
  adminGetChat, adminSendChatMessage, adminDeleteChat,
  adminBanChatUser, adminUnbanChatUser,
  APIError,
} from '../../api/client';
import type { AdminChatConversation, ChatMessage } from '../../api/client';
import PageHeader from '../../components/PageHeader';
import ConfirmModal from '../../components/ConfirmModal';
import { useToast } from '../../context/ToastContext';

const POLL_INTERVAL = 10_000;

export default function AdminChatDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { addToast } = useToast();
  const { site_name } = useSiteConfig();

  const [conversation, setConversation] = useState<AdminChatConversation | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [isBanned, setIsBanned] = useState(false);
  const [loading, setLoading] = useState(true);
  const [body, setBody] = useState('');
  const [sending, setSending] = useState(false);
  const [error, setError] = useState('');
  const [deleteTarget, setDeleteTarget] = useState(false);
  const [banTarget, setBanTarget] = useState(false);

  const messagesEndRef = useRef<HTMLDivElement>(null);

  const fetchData = useCallback(async () => {
    if (!id) return;
    try {
      const data = await adminGetChat(id);
      setConversation(data.conversation);
      setMessages(data.messages);
      setIsBanned(data.is_banned);
    } catch (err) {
      if (err instanceof APIError && err.status === 404) {
        navigate('/admin/chats');
      } else {
        setError(err instanceof APIError ? err.message : 'Failed to load chat');
      }
    }
  }, [id, navigate]);

  useEffect(() => {
    fetchData().finally(() => setLoading(false));
  }, [fetchData]);

  useEffect(() => {
    if (!conversation && !loading) return;
    const interval = setInterval(fetchData, POLL_INTERVAL);
    return () => clearInterval(interval);
  }, [fetchData, conversation, loading]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages.length]);

  async function doSend() {
    if (!body.trim() || sending || !id) return;
    setSending(true);
    setError('');
    try {
      const msg = await adminSendChatMessage(id, body.trim());
      setMessages((prev) => [...prev, msg]);
      setBody('');
    } catch (err) {
      setError(err instanceof APIError ? err.message : 'Failed to send');
    } finally {
      setSending(false);
    }
  }

  async function handleDelete() {
    if (!id) return;
    try {
      await adminDeleteChat(id);
      addToast('Chat deleted.', 'success');
      navigate('/admin/chats');
    } catch (err) {
      setError(err instanceof APIError ? err.message : 'Failed to delete');
    }
    setDeleteTarget(false);
  }

  async function handleBan() {
    if (!id) return;
    try {
      await adminBanChatUser(id, '');
      setIsBanned(true);
      addToast('User banned from chat.', 'success');
    } catch (err) {
      setError(err instanceof APIError ? err.message : 'Failed to ban');
    }
    setBanTarget(false);
  }

  async function handleUnban() {
    if (!id) return;
    try {
      await adminUnbanChatUser(id);
      setIsBanned(false);
      addToast('User unbanned.', 'success');
    } catch (err) {
      setError(err instanceof APIError ? err.message : 'Failed to unban');
    }
  }

  if (loading) {
    return (
      <div className="page">
        <PageHeader title="Chat" backTo="/admin/chats" />
        <p>Loading…</p>
      </div>
    );
  }

  if (!conversation) {
    return (
      <div className="page">
        <PageHeader title="Chat" backTo="/admin/chats" />
        <p className="admin-empty">Conversation not found.</p>
      </div>
    );
  }

  return (
    <div className="page">
      <PageHeader title={conversation.display_name} backTo="/admin/chats" />

      {deleteTarget && (
        <ConfirmModal
          title="Delete conversation?"
          message="This permanently deletes the conversation and all messages."
          confirmLabel="Delete"
          onConfirm={handleDelete}
          onCancel={() => setDeleteTarget(false)}
          danger
        />
      )}

      {banTarget && (
        <ConfirmModal
          title="Ban user from chat?"
          message="The user will no longer be able to create conversations or send messages."
          confirmLabel="Ban User"
          onConfirm={handleBan}
          onCancel={() => setBanTarget(false)}
          danger
        />
      )}

      <div className="admin-section">
        <div className="admin-section-header">
          <h2>Conversation Info</h2>
          <div className="chat-admin-actions">
            {isBanned ? (
              <button className="btn btn-secondary btn-small" onClick={handleUnban}>
                Unban User
              </button>
            ) : (
              <button className="btn btn-danger btn-small" onClick={() => setBanTarget(true)}>
                Ban User
              </button>
            )}
            <button className="btn btn-danger btn-small" onClick={() => setDeleteTarget(true)}>
              Delete Chat
            </button>
          </div>
        </div>
        <div className="chat-admin-info">
          <span><strong>Display Name:</strong> {conversation.display_name}</span>
          {conversation.email && <span><strong>Email:</strong> {conversation.email}</span>}
          <span><strong>Created:</strong> {new Date(conversation.created_at).toLocaleString()}</span>
          {isBanned && <span className="badge badge-danger">Banned</span>}
        </div>
      </div>

      {error && <p className="error">{error}</p>}

      <div className="chat-container">
        <div className="chat-messages">
          {messages.length === 0 ? (
            <p className="chat-empty">No messages yet.</p>
          ) : (
            messages.map((msg) => (
              <div
                key={msg.id}
                className={`chat-message chat-message-${msg.sender}`}
              >
                <div className="chat-message-sender">
                  {msg.sender === 'user' ? conversation.display_name : site_name}
                </div>
                <div className="chat-message-body">{msg.body}</div>
                <div className="chat-message-time">
                  {new Date(msg.created_at).toLocaleString()}
                </div>
              </div>
            ))
          )}
          <div ref={messagesEndRef} />
        </div>

        <form className="chat-input" onSubmit={(e) => { e.preventDefault(); doSend(); }}>
          <textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            placeholder="Type your reply…"
            maxLength={2000}
            rows={2}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                doSend();
              }
            }}
          />
          <button type="submit" className="btn btn-primary" disabled={sending || !body.trim()}>
            {sending ? 'Sending…' : 'Send'}
          </button>
        </form>
      </div>
    </div>
  );
}
