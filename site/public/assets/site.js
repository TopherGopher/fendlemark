(function () {
  var root = document.documentElement;
  var toggle = document.getElementById('theme-toggle');
  if (toggle) {
    toggle.addEventListener('click', function () {
      var dark = root.getAttribute('data-theme') === 'dark' ||
        (!root.getAttribute('data-theme') && window.matchMedia('(prefers-color-scheme: dark)').matches);
      var next = dark ? 'light' : 'dark';
      root.setAttribute('data-theme', next);
      try { localStorage.setItem('fm-theme', next); } catch (e) {}
    });
  }
  var menu = document.getElementById('menu-toggle');
  var sidebar = document.getElementById('sidebar');
  if (menu && sidebar) {
    menu.addEventListener('click', function () {
      var open = sidebar.classList.toggle('open');
      menu.setAttribute('aria-expanded', String(open));
    });
  }
  // Copy buttons on code blocks.
  if (navigator.clipboard) {
    document.querySelectorAll('pre').forEach(function (pre) {
      var btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'copy';
      btn.textContent = 'Copy';
      btn.addEventListener('click', function () {
        var text = pre.querySelector('code') ? pre.querySelector('code').innerText : pre.innerText;
        navigator.clipboard.writeText(text.replace(/^\$ /gm, '')).then(function () {
          btn.textContent = 'Copied';
          setTimeout(function () { btn.textContent = 'Copy'; }, 1500);
        });
      });
      pre.appendChild(btn);
    });
  }
  // Heading anchors.
  document.querySelectorAll('.main h2[id], .main h3[id]').forEach(function (h) {
    var a = document.createElement('a');
    a.className = 'anchor';
    a.href = '#' + h.id;
    a.setAttribute('aria-label', 'Link to this section');
    a.textContent = '#';
    h.appendChild(a);
  });
})();
