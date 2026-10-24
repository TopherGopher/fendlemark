// Theme toggle, mobile navigation, copy buttons, scroll regions, and the analytics choice. Everything works without it.
(function () {
  var d = document, root = d.documentElement, me = d.currentScript, dark = window.matchMedia('(prefers-color-scheme: dark)');

  var toggle = d.getElementById('theme-toggle');
  function isDark() { var t = root.getAttribute('data-theme'); return t ? t === 'dark' : dark.matches; }
  function sync() { toggle.setAttribute('aria-pressed', String(isDark())); }
  if (toggle) {
    sync();
    if (dark.addEventListener) dark.addEventListener('change', sync);
    toggle.addEventListener('click', function () {
      var next = isDark() ? 'light' : 'dark';
      root.setAttribute('data-theme', next);
      try { localStorage.setItem('fm-theme', next); } catch (e) {}
      sync();
    });
  }

  var menu = d.getElementById('menu-toggle'), nav = d.getElementById('sidebar');
  function setOpen(open, refocus) {
    if (open) nav.style.setProperty('--panel-top', Math.round(menu.closest('header').getBoundingClientRect().bottom) + 'px');
    nav.classList.toggle('open', open);
    menu.setAttribute('aria-expanded', String(open));
    if (!open && refocus) menu.focus();
  }
  if (menu && nav) {
    menu.addEventListener('click', function () { setOpen(menu.getAttribute('aria-expanded') !== 'true'); });
    d.addEventListener('keydown', function (e) { if (e.key === 'Escape' && nav.classList.contains('open')) setOpen(false, true); });
    nav.addEventListener('focusout', function (e) { var t = e.relatedTarget; if (t && t !== menu && !nav.contains(t)) setOpen(false); });
    d.addEventListener('click', function (e) { if (nav.classList.contains('open') && !nav.contains(e.target) && !menu.contains(e.target)) setOpen(false); });
  }

  // Copy buttons, announced through a polite live region.
  var live = d.createElement('div');
  live.className = 'sr-only';
  live.setAttribute('aria-live', 'polite');
  d.body.appendChild(live);
  if (navigator.clipboard) d.querySelectorAll('.code > pre').forEach(function (pre) {
    var btn = d.createElement('button');
    btn.type = 'button';
    btn.className = 'copy';
    btn.textContent = 'Copy';
    btn.setAttribute('aria-label', 'Copy ' + pre.getAttribute('aria-label'));
    btn.addEventListener('click', function () {
      var lines = pre.innerText.replace(/\n$/, '').split('\n'), cmds = lines.filter(function (l) { return /^\$ /.test(l); });
      navigator.clipboard.writeText((cmds.length ? cmds : lines).join('\n').replace(/^\$ /gm, '')).then(function () {
        btn.textContent = 'Copied';
        btn.setAttribute('data-copied', '');
        live.textContent = 'Copied';
        setTimeout(function () { btn.textContent = 'Copy'; btn.removeAttribute('data-copied'); live.textContent = ''; }, 2000);
      });
    });
    pre.parentNode.insertBefore(btn, pre);
  });

  // Scroll regions are only tab stops while they actually scroll.
  var regions = d.querySelectorAll('[data-scroll]');
  function fit() { regions.forEach(function (el) { if (el.scrollWidth > el.clientWidth + 1) el.setAttribute('tabindex', '0'); else el.removeAttribute('tabindex'); }); }
  if (window.ResizeObserver) { var ro = new ResizeObserver(fit); regions.forEach(function (el) { ro.observe(el); }); }
  if (d.fonts && d.fonts.ready) d.fonts.ready.then(fit);

  // Google Analytics loads only after the visitor allows it. The choice bar is
  // non-modal, never takes focus on its own, and exists only when this runs.
  var ga = me && me.getAttribute('data-ga'), KEY = 'fm-analytics';
  if (!ga) return;
  function stored() { try { return localStorage.getItem(KEY); } catch (e) { return null; } }
  function loadGA() {
    window['ga-disable-' + ga] = false;
    if (window.gtag) return;
    window.dataLayer = window.dataLayer || [];
    window.gtag = function () { window.dataLayer.push(arguments); };
    window.gtag('js', new Date());
    window.gtag('config', ga);
    var s = d.createElement('script');
    s.async = true;
    s.src = 'https://www.googletagmanager.com/gtag/js?id=' + encodeURIComponent(ga);
    d.head.appendChild(s);
  }
  function stopGA() {
    window['ga-disable-' + ga] = true;
    d.cookie.split(';').forEach(function (c) {
      var n = c.split('=')[0].trim();
      if (/^_ga/.test(n)) ['', '; domain=' + location.hostname, '; domain=.' + location.hostname].forEach(function (dom) { d.cookie = n + '=; Max-Age=0; path=/' + dom; });
    });
  }
  var bar, opener;
  function hide() {
    if (bar) bar.hidden = true;
    root.style.removeProperty('--consent-space');
    if (opener) { opener.focus(); opener = null; }
  }
  function decide(v) {
    try { localStorage.setItem(KEY, v); } catch (e) {}
    if (v === 'granted') loadGA(); else stopGA();
    live.textContent = v === 'granted' ? 'Google Analytics allowed' : 'Google Analytics declined';
    hide();
  }
  function show(from) {
    if (!bar) {
      var cf = d.querySelector('script[data-cf-beacon]');
      bar = d.createElement('section');
      bar.className = 'consent';
      bar.setAttribute('aria-label', 'Analytics choice');
      bar.innerHTML = '<p>' + (cf ? 'This site counts visits with Cloudflare Web Analytics, which sets no cookies. ' : '') +
        'May Google Analytics also measure your visit? It sets cookies. <a href="/about/#privacy">Details</a></p>' +
        '<div class="consent-actions"><button type="button" class="btn primary" data-v="granted">Allow</button>' +
        '<button type="button" class="btn" data-v="denied">No thanks</button></div>';
      bar.addEventListener('click', function (e) { var v = e.target.getAttribute('data-v'); if (v) decide(v); });
      bar.addEventListener('keydown', function (e) { if (e.key === 'Escape') hide(); });
      d.body.appendChild(bar);
    }
    bar.hidden = false;
    root.style.setProperty('--consent-space', (bar.offsetHeight + 32) + 'px');
    if (from) { opener = from; bar.querySelector('button').focus(); }
  }
  var links = d.querySelector('.footer-links');
  if (links) {
    var b = d.createElement('button');
    b.type = 'button';
    b.className = 'link-button';
    b.textContent = 'Analytics choices';
    b.addEventListener('click', function () { show(b); });
    links.appendChild(d.createTextNode(' · '));
    links.appendChild(b);
  }
  var c = stored();
  if (c === 'granted') loadGA(); else if (c !== 'denied') show();
})();
