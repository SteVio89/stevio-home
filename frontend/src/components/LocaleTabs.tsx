interface LocaleInfo {
  code: string;
  name: string;
  is_default: boolean;
}

interface Props {
  locales: LocaleInfo[];
  activeLocale: string;
  onChange: (locale: string) => void;
}

export default function LocaleTabs({ locales, activeLocale, onChange }: Props) {
  return (
    <div className="admin-locale-tabs">
      {locales.map((loc) => (
        <button
          key={loc.code}
          type="button"
          className={`btn btn-small${activeLocale === loc.code ? ' btn-primary' : ' btn-secondary'}`}
          onClick={() => onChange(loc.code)}
        >
          {loc.name}
          {loc.is_default && <span className="locale-default-badge">*</span>}
        </button>
      ))}
    </div>
  );
}
