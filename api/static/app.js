// Repo page: chart options builder + status polling while the repo is being analyzed.
(function () {
  const el = document.getElementById('app');
  if (!el) return;
  const cfg = JSON.parse(el.dataset.config);

  const defaults = Object.fromEntries(new URLSearchParams(cfg.defaults));
  const current = Object.fromEntries(new URLSearchParams(cfg.query));
  const base = { top: '8', x: 'date', mode: 'stacked', metric: 'code', theme: 'auto', exclude: '' };

  Vue.createApp({
    delimiters: ['${', '}'],
    data() {
      return {
        status: cfg.status,
        hasChart: cfg.hasChart,
        progress: null,
        opts: Object.assign({}, base, defaults, current),
        version: 0, // bumps to bust the image cache when analysis finishes
        copied: false,
      };
    },
    computed: {
      query() {
        const q = new URLSearchParams();
        for (const [k, v] of Object.entries(this.opts)) {
          const def = defaults[k] !== undefined ? defaults[k] : base[k];
          if (String(v).trim() !== '' && String(v) !== String(def)) q.set(k, String(v).trim());
        }
        return q.toString();
      },
      embedURL() {
        return cfg.pageURL + (this.query ? '?' + this.query : '');
      },
      imageSrc() {
        const q = new URLSearchParams(this.query);
        if (this.version) q.set('v', this.version);
        const s = q.toString();
        return '/' + cfg.slug + '.svg' + (s ? '?' + s : '');
      },
      markdown() {
        return '[![Lines of code](' + this.embedURL + ')](' + cfg.pageURL + ')';
      },
      progressText() {
        const p = this.progress;
        if (!p) return this.status === 'pending' ? 'Waiting in the queue…' : 'Working…';
        if (p.phase === 'sync') return 'Cloning repository…';
        if (p.total) return 'Counting lines: commit ' + p.done + ' of ' + p.total;
        return 'Counting lines…';
      },
    },
    watch: {
      query(q) {
        // keep the page URL shareable with the chosen options
        history.replaceState(null, '', '/' + cfg.slug + (q ? '?' + q : ''));
      },
    },
    mounted() {
      if (this.status === 'pending' || this.status === 'processing') this.poll();
    },
    methods: {
      async poll() {
        try {
          const r = await fetch('/' + cfg.slug + '.json?points=0', { cache: 'no-store' });
          const body = await r.json();
          this.progress = body.progress || null;
          const prev = this.status;
          this.status = body.repo ? body.repo.status : 'error';
          if (prev !== this.status && this.status !== 'processing' && this.status !== 'pending') {
            // finished: reload to get the server rendered stats table
            window.location.reload();
            return;
          }
        } catch (e) {
          console.warn('poll failed', e);
        }
        setTimeout(() => this.poll(), 2000);
      },
      async copy() {
        try {
          await navigator.clipboard.writeText(this.markdown);
          this.copied = true;
          setTimeout(() => (this.copied = false), 1500);
        } catch (e) {
          console.warn('clipboard', e);
        }
      },
    },
  }).mount(el);
})();
