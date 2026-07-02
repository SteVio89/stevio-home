import { useContext, useEffect } from 'react';
import { SiteConfigContext } from '../context/SiteConfigContext';
import { DEFAULT_LOCALE } from '../i18n';

export interface DocumentHeadOptions {
  title: string;
  description?: string;
  canonical?: string;
  ogImage?: string;
  noindex?: boolean;
}

function setMeta(name: string, content: string, attr: 'name' | 'property' = 'name') {
  let el = document.querySelector<HTMLMetaElement>(`meta[${attr}="${name}"]`);
  if (!el) {
    el = document.createElement('meta');
    el.setAttribute(attr, name);
    document.head.appendChild(el);
  }
  el.setAttribute('content', content);
}

function setCanonical(href: string) {
  let el = document.querySelector<HTMLLinkElement>('link[rel="canonical"]');
  if (!el) {
    el = document.createElement('link');
    el.setAttribute('rel', 'canonical');
    document.head.appendChild(el);
  }
  el.setAttribute('href', href);
}

function setAlternate(hreflang: string, href: string) {
  let el = document.querySelector<HTMLLinkElement>(`link[rel="alternate"][hreflang="${hreflang}"]`);
  if (!el) {
    el = document.createElement('link');
    el.setAttribute('rel', 'alternate');
    el.setAttribute('hreflang', hreflang);
    document.head.appendChild(el);
  }
  el.setAttribute('href', href);
}

export function useDocumentHead({
  title,
  description,
  canonical,
  ogImage,
  noindex = false,
}: DocumentHeadOptions) {
  const { site_name, base_url, locales } = useContext(SiteConfigContext);
  useEffect(() => {
    const fullTitle = `${title} — ${site_name}`;
    const desc = description ?? '';
    const url = canonical ?? (base_url ? base_url + '/' : '/');

    document.title = fullTitle;
    setMeta('description', desc);
    setMeta('robots', noindex ? 'noindex, nofollow' : 'index, follow');
    setMeta('og:title', fullTitle, 'property');
    setMeta('og:description', desc, 'property');
    setMeta('og:url', url, 'property');
    setCanonical(url);
    if (ogImage) {
      setMeta('og:image', ogImage, 'property');
      setMeta('twitter:image', ogImage);
    }
    setMeta('twitter:title', fullTitle);
    setMeta('twitter:description', desc);

    // Set hreflang alternate links for locale variants
    if (canonical && base_url) {
      const defaultLocale = locales.find(l => l.is_default)?.code ?? DEFAULT_LOCALE;
      // Remove all existing hreflang links first
      document.querySelectorAll('link[rel="alternate"][hreflang]').forEach((el) => el.remove());
      for (const loc of locales) {
        const locUrl = canonical.replace(/\/([a-z]{2,5})\//, `/${loc.code}/`);
        setAlternate(loc.code, locUrl);
      }
      const defaultUrl = canonical.replace(/\/([a-z]{2,5})\//, `/${defaultLocale}/`);
      setAlternate('x-default', defaultUrl);
    } else {
      // Remove stale hreflang links from previous page
      document.querySelectorAll('link[rel="alternate"][hreflang]').forEach((el) => el.remove());
    }
  }, [title, description, canonical, ogImage, noindex, site_name, base_url, locales]);
}
