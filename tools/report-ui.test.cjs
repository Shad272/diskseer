// Logic tests for the embedded scripts, without a browser or npm dependencies.
const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const template = fs.readFileSync(path.join(__dirname, '../internal/report/report.html.tmpl'), 'utf8');
const scripts = [...template.matchAll(/<script>([\s\S]*?)<\/script>/g)]
  .map(match => match[1].replaceAll('{{.RicaricaOgni}}', '3'));

function storage() {
  const values = new Map();
  return { getItem: key => values.get(key) ?? null, setItem: (key, value) => values.set(key, value) };
}

function page({ denied = false, session = storage(), local = storage(), pathname = '/report.html' } = {}) {
  const makeButton = value => ({
    dataset: { filter: value }, handlers: {}, attributes: {},
    classList: { toggle() {} },
    addEventListener(type, handler) { this.handlers[type] = handler; },
    setAttribute(name, value) { this.attributes[name] = value; },
  });
  const filters = ['all', 'critical', 'warn', 'info'].map(makeButton);
  const findings = ['critical', 'warn', 'info'].map(severity => ({ dataset: { severity }, hidden: false }));
  const theme = makeButton('theme');
  const root = { dataset: { theme: 'dark' } };
  const events = {};
  const timers = new Map();
  let nextTimer = 0;
  const window = { scrollTo() {} };
  for (const [name, value] of [['localStorage', local], ['sessionStorage', session]]) {
    Object.defineProperty(window, name, { get() {
      if (denied) throw new Error('Storage denied');
      return value;
    } });
  }
  const context = vm.createContext({
    window,
    document: {
      documentElement: root,
      getElementById: () => theme,
      querySelectorAll: selector => selector === '.filter' ? filters : findings,
    },
    location: { pathname, reload() {} },
    scrollY: 500,
    addEventListener: (type, handler) => { events[type] = handler; },
    setTimeout: (handler, delay) => { const id = ++nextTimer; timers.set(id, { handler, delay }); return id; },
    clearTimeout: id => timers.delete(id),
  });
  // Browser storage globals use the same accessors as window properties.
  for (const name of ['localStorage', 'sessionStorage']) {
    Object.defineProperty(context, name, { get: () => window[name] });
  }
  scripts.forEach(script => vm.runInContext(script, context));
  return { filters, findings, theme, root, events, timers };
}

test('theme, filters and refresh work when storage access throws', () => {
  const p = page({ denied: true });
  p.theme.handlers.click();
  assert.equal(p.root.dataset.theme, 'light');
  p.filters[1].handlers.click();
  assert.deepEqual(p.findings.map(f => f.hidden), [false, true, true]);
  assert.equal(p.filters[1].attributes['aria-pressed'], 'true');
  assert.equal(p.timers.size, 1);
  assert.doesNotThrow(() => p.events.beforeunload());
});

test('live reload restores the selected filter for the same report only', () => {
  const session = storage();
  const first = page({ session });
  first.filters[2].handlers.click();
  const reloaded = page({ session });
  assert.deepEqual(reloaded.findings.map(f => f.hidden), [true, false, true]);
  const different = page({ session, pathname: '/another-report.html' });
  assert.ok(different.findings.every(f => !f.hidden));
});

test('refresh pauses for printing and resumes once afterwards', () => {
  const p = page();
  p.events.beforeprint();
  assert.equal(p.timers.size, 0);
  p.events.afterprint();
  p.events.afterprint();
  assert.equal(p.timers.size, 1);
  assert.equal([...p.timers.values()][0].delay, 3000);
});

test('unexpected stored values do not hide all findings or apply an invalid theme', () => {
  const session = storage(), local = storage();
  session.setItem('diskseer-filter:/report.html', 'invalid');
  local.setItem('diskseer-theme', 'invalid');
  const p = page({ session, local });
  assert.ok(p.findings.every(f => !f.hidden));
  assert.equal(p.root.dataset.theme, 'dark');
});
