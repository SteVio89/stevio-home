// Flash-of-wrong-theme prevention: apply the saved (or system) theme to the
// document before React renders. Kept as an external, same-origin file rather
// than an inline <script> so it runs under a strict CSP (script-src 'self')
// without needing 'unsafe-inline' — which would defeat the app's main XSS
// defense. Must be loaded synchronously in <head> so it executes before paint.
(function () {
  var t = localStorage.getItem("theme");
  if (!t) t = window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  document.documentElement.setAttribute("data-theme", t);
})();
