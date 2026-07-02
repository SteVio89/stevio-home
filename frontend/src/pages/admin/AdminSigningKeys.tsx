import { useEffect, useState } from 'react';
import {
  adminGetSigningKey,
  adminGenerateSigningKey,
  type SigningKey,
} from '../../api/client';
import PageHeader from '../../components/PageHeader';
import { SkeletonTable } from '../../components/Skeleton';
import { useToast } from '../../context/ToastContext';

export default function AdminSigningKeys() {
  const [key, setKey] = useState<SigningKey | null>(null);
  const [loading, setLoading] = useState(true);
  const [generating, setGenerating] = useState(false);
  const { addToast } = useToast();

  function loadKey() {
    adminGetSigningKey()
      .then((res) => setKey(res.key))
      .catch(() => addToast('Failed to load signing key', 'error'))
      .finally(() => setLoading(false));
  }

  useEffect(() => { loadKey(); }, []);

  async function handleGenerate() {
    if (key) {
      const confirmed = window.confirm(
        'This will replace the current signing key. Existing app binaries using the old pinned public key will need to be updated. Continue?'
      );
      if (!confirmed) return;
    }
    setGenerating(true);
    try {
      await adminGenerateSigningKey();
      addToast('New signing key generated and activated', 'success');
      loadKey();
    } catch {
      addToast('Failed to generate signing key', 'error');
    } finally {
      setGenerating(false);
    }
  }

  function handleCopy(text: string) {
    navigator.clipboard.writeText(text)
      .then(() => addToast('Copied to clipboard', 'success'))
      .catch(() => addToast('Copy failed', 'error'));
  }

  if (loading) {
    return (
      <div className="page">
        <PageHeader title="Signing Key" />
        <SkeletonTable rows={1} cols={3} />
      </div>
    );
  }

  return (
    <div className="page">
      <PageHeader
        title="Signing Key"
        actions={
          <button className="btn btn-primary" onClick={handleGenerate} disabled={generating}>
            {generating ? 'Generating...' : key ? 'Rotate Key' : 'Generate Key'}
          </button>
        }
      />

      {!key ? (
        <div className="admin-empty">
          <p>No signing key yet. Generate one to enable license signing and purchases.</p>
        </div>
      ) : (
        <div className="admin-table-wrapper">
          <table className="admin-table">
            <thead>
              <tr>
                <th>Key ID</th>
                <th>Public Key</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>
                  <code className="signing-key-id">{key.key_id}</code>
                </td>
                <td>
                  <span className="signing-key-pubkey">
                    {key.public_key_b64.slice(0, 24)}...
                  </span>
                  <button
                    className="signing-key-copy-btn"
                    onClick={() => handleCopy(key.public_key_b64)}
                    title="Copy full public key"
                  >
                    Copy
                  </button>
                </td>
                <td>{new Date(key.created_at).toLocaleDateString()}</td>
              </tr>
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
