import { useRef, useState, useEffect } from 'react';
import type { LocaleInfo } from '../i18n/useT';

interface Props {
  locales: LocaleInfo[];
  currentCode: string;
  onSwitch: (code: string) => void;
}

export default function LanguageSwitcher({ locales, currentCode, onSwitch }: Props) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const current = locales.find(l => l.code === currentCode);

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    if (open) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [open]);

  if (locales.length <= 1) return null;

  return (
    <div className="site-nav-lang-switcher" ref={ref}>
      <button
        className="site-nav-lang-switcher-toggle"
        onClick={() => setOpen(!open)}
        aria-expanded={open}
        aria-haspopup="listbox"
      >
        {current?.name ?? currentCode.toUpperCase()}
      </button>
      {open && (
        <ul className="site-nav-lang-switcher-menu" role="listbox">
          {locales.map(l => (
            <li key={l.code} role="option" aria-selected={l.code === currentCode}>
              <button
                className={`site-nav-lang-switcher-option${l.code === currentCode ? ' active' : ''}`}
                onClick={() => { onSwitch(l.code); setOpen(false); }}
              >
                {l.name}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
