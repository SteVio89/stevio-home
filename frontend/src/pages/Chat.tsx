import { useEffect, useState, useRef, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../context/AuthContext';
import { useSiteConfig } from '../context/SiteConfigContext';
import {
  getChat, createChat, sendChatMessage, shareChatEmail, deleteChat,
  APIError,
} from '../api/client';
import type { ChatConversation, ChatMessage } from '../api/client';
import PageHeader from '../components/PageHeader';
import ConfirmModal from '../components/ConfirmModal';

const POLL_INTERVAL = 10_000;

export default function Chat() {
  const { t } = useTranslation();
  const { email } = useAuth();
  const { site_name } = useSiteConfig();

  const [conversation, setConversation] = useState<ChatConversation | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [loading, setLoading] = useState(true);
  const [noChat, setNoChat] = useState(false);
  const [banned, setBanned] = useState(false);
  const [body, setBody] = useState('');
  const [sending, setSending] = useState(false);
  const [error, setError] = useState('');
  const [showShareConfirm, setShowShareConfirm] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  const messagesEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, []);

  const fetchChat = useCallback(async () => {
    try {
      const data = await getChat();
      setConversation(data.conversation);
      setMessages(data.messages);
      setNoChat(false);
    } catch (err) {
      if (err instanceof APIError && err.status === 404) {
        setNoChat(true);
        setConversation(null);
        setMessages([]);
      } else if (err instanceof APIError && err.status === 403) {
        setBanned(true);
      } else {
        setError(err instanceof APIError ? err.message : t('chat.error'));
      }
    }
  }, [t]);

  useEffect(() => {
    fetchChat().finally(() => setLoading(false));
  }, [fetchChat]);

  // Polling
  useEffect(() => {
    if (noChat || banned) return;
    const interval = setInterval(fetchChat, POLL_INTERVAL);
    return () => clearInterval(interval);
  }, [noChat, banned, fetchChat]);

  // Scroll on new messages
  useEffect(() => {
    scrollToBottom();
  }, [messages.length, scrollToBottom]);

  async function handleStartChat() {
    setError('');
    try {
      const conv = await createChat();
      setConversation(conv);
      setNoChat(false);
    } catch (err) {
      if (err instanceof APIError && err.status === 403) {
        setBanned(true);
      } else {
        setError(err instanceof APIError ? err.message : t('chat.error'));
      }
    }
  }

  async function doSend() {
    if (!body.trim() || sending) return;
    setSending(true);
    setError('');
    try {
      const msg = await sendChatMessage(body.trim());
      setMessages((prev) => [...prev, msg]);
      setBody('');
    } catch (err) {
      if (err instanceof APIError && err.status === 403) {
        setBanned(true);
      } else {
        setError(err instanceof APIError ? err.message : t('chat.error'));
      }
    } finally {
      setSending(false);
    }
  }

  async function handleShareEmail() {
    setError('');
    const userEmail = email || localStorage.getItem('user_email');
    if (!userEmail) {
      setError(t('chat.error'));
      return;
    }
    try {
      await shareChatEmail(userEmail);
      setConversation((prev) => prev ? { ...prev, email_shared: true, display_name: userEmail } : prev);
      setShowShareConfirm(false);
    } catch (err) {
      setError(err instanceof APIError ? err.message : t('chat.error'));
    }
  }

  async function handleDelete() {
    setError('');
    try {
      await deleteChat();
      setConversation(null);
      setMessages([]);
      setNoChat(true);
      setShowDeleteConfirm(false);
    } catch (err) {
      setError(err instanceof APIError ? err.message : t('chat.error'));
    }
  }

  if (loading) {
    return (
      <div className="page">
        <PageHeader title={t('chat.nav')} />
        <p className="loading-text">{t('chat.loading')}</p>
      </div>
    );
  }

  if (banned) {
    return (
      <div className="page">
        <PageHeader title={t('chat.nav')} />
        <p className="error">{t('chat.banned')}</p>
      </div>
    );
  }

  if (noChat) {
    return (
      <div className="page">
        <PageHeader title={t('chat.nav')} />
        <div className="chat-start">
          <p>{t('chat.start_desc')}</p>
          <button className="btn btn-primary" onClick={handleStartChat}>
            {t('chat.start')}
          </button>
          {error && <p className="error">{error}</p>}
        </div>
      </div>
    );
  }

  return (
    <div className="page">
      <PageHeader title={t('chat.nav')} />

      {showShareConfirm && (
        <ConfirmModal
          title={t('chat.share_email_confirm_title')}
          message={t('chat.share_email_confirm')}
          confirmLabel={t('chat.share_email')}
          onConfirm={handleShareEmail}
          onCancel={() => setShowShareConfirm(false)}
        />
      )}

      {showDeleteConfirm && (
        <ConfirmModal
          title={t('chat.delete_confirm_title')}
          message={t('chat.delete_confirm')}
          confirmLabel={t('chat.delete')}
          onConfirm={handleDelete}
          onCancel={() => setShowDeleteConfirm(false)}
          danger
        />
      )}

      <div className="chat-container">
        <div className="chat-header">
          <span className="chat-display-name">{conversation?.display_name}</span>
          <div className="chat-header-actions">
            {conversation && !conversation.email_shared && email && (
              <button className="btn btn-secondary btn-small" onClick={() => setShowShareConfirm(true)}>
                {t('chat.share_email')}
              </button>
            )}
            {conversation?.email_shared && (
              <span className="badge badge-success">{t('chat.email_shared')}</span>
            )}
            <button className="btn btn-danger btn-small" onClick={() => setShowDeleteConfirm(true)}>
              {t('chat.delete')}
            </button>
          </div>
        </div>

        <div className="chat-messages">
          {messages.length === 0 ? (
            <p className="chat-empty">{t('chat.empty')}</p>
          ) : (
            messages.map((msg) => (
              <div
                key={msg.id}
                className={`chat-message chat-message-${msg.sender}`}
              >
                <div className="chat-message-sender">
                  {msg.sender === 'user' ? t('chat.you') : site_name}
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
            placeholder={t('chat.placeholder')}
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
            {sending ? t('chat.sending') : t('chat.send')}
          </button>
        </form>

        {error && <p className="error">{error}</p>}
      </div>
    </div>
  );
}
