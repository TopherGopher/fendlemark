// Theme toggle, mobile navigation, copy buttons, and scroll regions. Everything works without it.
(function () {
  var d = document, root = d.documentElement, dark = window.matchMedia('(prefers-color-scheme: dark)');

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
})();
