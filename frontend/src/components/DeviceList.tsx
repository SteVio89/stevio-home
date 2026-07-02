import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { renameDevice, type Activation } from '../api/client';

interface Props {
  activations: Activation[];
  maxDevices: number;
  onUpdate: () => void;
}

export default function DeviceList({ activations, maxDevices, onUpdate }: Props) {
  const { t } = useTranslation();
  return (
    <div className="device-list">
      <h4>
        {t('devices.title', { count: activations.length, max: maxDevices })}
      </h4>

      {activations.length === 0 ? (
        <p className="no-devices">{t('devices.none')}</p>
      ) : (
        <ul>
          {activations.map((act) => (
            <DeviceItem key={act.id} activation={act} onUpdate={onUpdate} />
          ))}
        </ul>
      )}
    </div>
  );
}

function DeviceItem({ activation, onUpdate }: { activation: Activation; onUpdate: () => void }) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  const [label, setLabel] = useState(activation.device_label || '');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  async function handleRename() {
    if (!label.trim()) return;
    setLoading(true);
    setError('');
    try {
      await renameDevice(activation.id, label.trim());
      setEditing(false);
      onUpdate();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('devices.rename_error'));
    } finally {
      setLoading(false);
    }
  }

  return (
    <li className="device-item">
      {error && <p className="error">{error}</p>}
      <div className="device-info">
        {editing ? (
          <div className="rename-form">
            <input
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder={t('devices.name_placeholder')}
              autoFocus
            />
            <button className="btn btn-small" onClick={handleRename} disabled={loading}>
              {t('devices.save')}
            </button>
            <button className="btn btn-small btn-secondary" onClick={() => setEditing(false)}>
              {t('devices.cancel')}
            </button>
          </div>
        ) : (
          <>
            <span className="device-label">
              {activation.device_label || t('devices.unnamed')}
            </span>
            <span className="device-hash" title={activation.machine_hash}>
              {activation.machine_hash.slice(0, 8)}...
            </span>
          </>
        )}
      </div>

      <div className="device-actions">
        {!editing && (
          <button className="btn btn-small" onClick={() => setEditing(true)} disabled={loading}>
            {t('devices.rename')}
          </button>
        )}
      </div>
    </li>
  );
}
