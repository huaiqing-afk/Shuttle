// Geometry acceptance: measures the real built frontend at the three required
// resolutions under 100% / 125% / 150% scaling. Screenshots are written next to
// this script; a non-zero exit means a clipped control or horizontal scroll.
const { createServer } = require('node:http')
const { readFile, mkdir, writeFile } = require('node:fs/promises')
const { spawn } = require('node:child_process')
const path = require('node:path')

const fs = require('node:fs')
const EDGE = ['C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe', 'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe'].find(p => fs.existsSync(p)) || 'msedge.exe'
const DIST = path.resolve(__dirname, '../../frontend/dist')
const OUT = __dirname
const PORT = 9333
const MIME = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css' }

const STUB = `
try { localStorage.setItem('shuttle-language', 'zh-CN'); } catch {}
window.go = { main: { App: {
  StartTerminal: async () => 't1', ResumeSession: async () => 't1', ListSessions: async () => [],
  WriteTerminal: async () => {}, ResizeTerminal: async () => {}, CloseTerminal: async () => {},
  DetectEnvironment: async () => [], ExitApp: async () => {},
  OpenInWindowsTerminal: async () => {},
  ConfigurationPaths: async () => ({ WT: 'C:\\\\isolated\\\\settings.json', PowerShell: 'C:\\\\isolated\\\\profile.ps1', Operations: 'C:\\\\isolated\\\\operations' }),
  PlanConfiguration: async () => ({ PlanID: 'plan-1', Diffs: [
    { Target: 'wt:scalar:theme', Action: 'modify', Before: '"system"', After: '"dark"', WasAbsent: false, Conflict: '' },
    { Target: 'ps:managed-block', Action: 'add', Before: '', After: '# >>> shuttle managed (do not edit) >>>', WasAbsent: true, Conflict: '' }], Blocked: false }),
  ExecuteConfiguration: async () => ({ OpID: 'op-1', Steps: [], Succeeded: true }),
  ListConfigurationOperations: async () => [{ ID: 'op-1', State: 'succeeded', StartedAt: '2026-10-25T04:00:00Z', Steps: [
    { Target: 'wt:scalar:theme', Action: 'modify', Changed: true, Skipped: false, Err: '' },
    { Target: 'ps:managed-block', Action: 'add', Changed: true, Skipped: false, Err: '' }] }],
  PlanConfigurationRestore: async () => ({ PlanID: 'plan-2', Diffs: [], Blocked: false }),
  InventoryPaths: async () => ({ Ledger: 'C:\\\\isolated\\\\install-ledger.json' }),
  DetectInventory: async () => ({
    Items: [
      { Name: '@anthropic-ai/claude-code', Version: '2.1.310', Source: 'npm', Root: 'C:\\\\npm', Ownership: 'external', Reason: '安装台账无此记录，视为用户自行安装' },
      { Name: '@openai/codex', Version: '0.164.0', Source: 'npm', Root: 'C:\\\\npm', Ownership: 'external', Reason: '安装台账无此记录，视为用户自行安装' },
      { Name: 'Microsoft.VisualStudioCode', Version: '1.100.0', Source: 'winget', Root: 'winget', Ownership: 'unknown', Reason: '台账记录来源为 npm，实际检测为 winget，需人工核对' },
      { Name: 'Anthropic.Claude', Version: '3.6556.2.0', Source: 'winget', Root: 'winget', Ownership: 'managed', Reason: '安装台账记录与当前检测一致' },
    ],
    Sources: [
      { Name: 'npm', Count: 11, Available: true, Error: '' },
      { Name: 'winget', Count: 65, Available: true, Error: '' },
    ],
    Note: 'winget 导出只包含已关联 winget 源的包；ARP/MSIX 来源的已安装程序不会出现在结果中',
  }),
  PlanInventoryRemoval: async () => ({ Token: 'preview-rm-1', Plan: { Steps: [], Blocked: false, Conflicts: [] } }),
  ExecuteInventoryRemoval: async () => ({ Token: 'preview-rm-1', OperationID: 'op-rm-1', State: 'succeeded', Records: [] }),
  PlanInstall: async () => ({ Token: 'preview-1', Plan: { Steps: [
    { Item: { Name: '@anthropic-ai/claude-code', Version: '', Source: 'npm', Root: '', Ownership: 'external', Reason: '' },
      Package: { Name: '@anthropic-ai/claude-code', Version: '', Source: 'npm', Root: '' },
      Action: 'install', Command: 'npm install -g @anthropic-ai/claude-code',
      Instruction: '官方说明：https://www.npmjs.com/package/@anthropic-ai/claude-code' },
    { Item: { Name: 'Git.Git', Version: '', Source: 'winget', Root: '', Ownership: 'external', Reason: '' },
      Package: { Name: 'Git.Git', Version: '', Source: 'winget', Root: '' },
      Action: 'install', Command: 'winget install --id Git.Git --exact --accept-package-agreements --accept-source-agreements --disable-interactivity',
      Instruction: '官方说明：https://learn.microsoft.com/windows/package-manager/winget/install' }],
    Blocked: false, Conflicts: [] } }),
  ExecuteInstall: async () => ({ Token: 'install-1', OperationID: 'install-1', State: 'succeeded', Records: [] }),
  ReconcileInventory: async () => [],
} } };
window.runtime = { EventsOn: () => () => {} };
try { localStorage.setItem('shuttle-sidebar', 'open') } catch {}
`

const PROBE = `(() => {
  const doc = document.documentElement;
  const rect = el => { const r = el.getBoundingClientRect(); return { x: +r.x.toFixed(2), y: +r.y.toFixed(2), w: +r.width.toFixed(2), h: +r.height.toFixed(2), right: +r.right.toFixed(2), bottom: +r.bottom.toFixed(2) }; };
  const visible = el => { const r = el.getBoundingClientRect(); return r.width > 0 && r.height > 0; };
  const out = { viewport: [innerWidth, innerHeight], dpr: devicePixelRatio,
    docOverflowX: doc.scrollWidth > doc.clientWidth + 1, docScroll: [doc.scrollWidth, doc.clientWidth],
    aside: !!document.querySelector('aside') };
  const check = (name, boxes) => {
    const result = { overlaps: [], minGap: null, minGapPair: null, clipped: [] };
    const contains = (a, b) => a.el !== b.el && (a.el.contains(b.el) || b.el.contains(a.el));
    for (let i = 0; i < boxes.length; i++) for (let j = i + 1; j < boxes.length; j++) {
      const a = boxes[i], b = boxes[j];
      if (contains(a, b)) continue; // nesting is not overlap
      const overlap = Math.min(a.right, b.right) - Math.max(a.x, b.x) > 0.5 && Math.min(a.bottom, b.bottom) - Math.max(a.y, b.y) > 0.5;
      if (overlap) { result.overlaps.push([a.label, b.label]); continue; }
      if (Math.min(a.bottom, b.bottom) - Math.max(a.y, b.y) > 0.5) {
        const gap = Math.max(a.x, b.x) - Math.min(a.right, b.right);
        if (result.minGap === null || gap < result.minGap) { result.minGap = +gap.toFixed(2); result.minGapPair = [a.label, b.label]; }
      }
    }
    return result;
  };
  const pick = (root, selectors) => selectors.map(([label, sel]) => {
    const el = root.querySelector(sel);
    return el && visible(el) ? { label, el, ...rect(el) } : null;
  }).filter(Boolean);

  // only interactive controls share the 12px line; containers such as the
  // sidebar and main pane meet at a border, which is not a control gap
  const shell = pick(document, [['side', 'header > button'], ['brand', 'header > span'], ['theme', 'header [class*="absolute"]'],
    ['config', 'header div[class*="ml-auto"] button:last-of-type'], ['search', 'aside input'], ['filter', 'aside button']]);
  out.shell = { ...check('shell', shell), boxes: shell.map(({ el, ...rest }) => rest) };

  const lang = document.querySelector('#app-language');
  out.lang = lang ? { fits: rect(lang).right <= innerWidth && rect(lang).x >= 0 } : null;
  const picker = document.querySelector('#terminal-palette');
  const pickerLabel = document.querySelector('label[for="terminal-palette"]');
  out.palette = picker && pickerLabel ? {
    fits: rect(picker).right <= innerWidth && rect(picker).x >= 0,
    gap: +(rect(picker).x - rect(pickerLabel).right).toFixed(2),
    options: picker.options.length,
  } : null;
  const dialog = document.querySelector('[role="dialog"]');
  out.dialog = null;
  if (dialog) {
    const dr = dialog.getBoundingClientRect();
    const controls = [...dialog.querySelectorAll('input, select, textarea, button')].filter(visible).map(el => ({
      label: el.getAttribute('aria-label') || el.textContent.trim().slice(0, 16) || el.tagName.toLowerCase(), el, ...rect(el) }));
    // sibling gaps inside every flex container: the 12px line is about
    // adjacent controls, so measure element children of flex rows only
    const siblingGaps = [];
    for (const holder of dialog.querySelectorAll('*')) {
      if (getComputedStyle(holder).display.indexOf('flex') < 0) continue;
      const kids = [...holder.children].filter(visible);
      for (let i = 1; i < kids.length; i++) {
        const a = kids[i - 1].getBoundingClientRect(), b = kids[i].getBoundingClientRect();
        const sameRow = Math.min(a.bottom, b.bottom) - Math.max(a.y, b.y) > 0.5;
        const sameCol = Math.min(a.right, b.right) - Math.max(a.x, b.x) > 0.5;
        if (sameRow) siblingGaps.push({ gap: +(b.x - a.right).toFixed(2), pair: [kids[i - 1].tagName.toLowerCase(), kids[i].tagName.toLowerCase()] });
        else if (sameCol) siblingGaps.push({ gap: +(b.y - a.bottom).toFixed(2), pair: [kids[i - 1].tagName.toLowerCase(), kids[i].tagName.toLowerCase()] });
      }
    }
    siblingGaps.sort((a, b) => a.gap - b.gap);
    const inner = dialog.querySelector('.overflow-y-auto') || dialog;
    out.dialog = { box: rect(dialog),
      fitsViewport: dr.x >= -0.5 && dr.right <= innerWidth + 0.5 && dr.y >= -0.5 && dr.bottom <= innerHeight + 0.5,
      horizontalScroll: inner.scrollWidth > inner.clientWidth + 1,
      clipped: controls.filter(c => c.x < dr.x - 0.5 || c.right > dr.right + 0.5).map(c => c.label),
      ...check('dialog', controls), controls: controls.length, siblingGaps: siblingGaps.slice(0, 5) };
  }
  return out;
})()`

async function serve() {
  const server = createServer(async (req, res) => {
    const rel = decodeURIComponent(req.url.split('?')[0]) === '/' ? '/index.html' : decodeURIComponent(req.url.split('?')[0])
    try {
      const body = await readFile(path.join(DIST, rel))
      res.writeHead(200, { 'content-type': MIME[path.extname(rel)] || 'application/octet-stream' })
      res.end(body)
    } catch { res.writeHead(404).end() }
  })
  await new Promise(done => server.listen(PORT, '127.0.0.1', done))
  return server
}

async function target(port) {
  for (let i = 0; i < 60; i++) {
    try {
      const pages = await (await fetch(`http://127.0.0.1:${port}/json`)).json()
      const page = pages.find(p => p.type === 'page')
      if (page) return page
    } catch {}
    await new Promise(done => setTimeout(done, 250))
  }
  throw new Error(`CDP ${port} unavailable`)
}

function session(url) {
  const socket = new WebSocket(url)
  let id = 0
  const waiting = new Map()
  const listeners = []
  socket.addEventListener('message', event => {
    const msg = JSON.parse(event.data)
    if (msg.id && waiting.has(msg.id)) { waiting.get(msg.id)(msg); waiting.delete(msg.id) } else listeners.forEach(fn => fn(msg))
  })
  const ready = new Promise((done, fail) => { socket.addEventListener('open', done); socket.addEventListener('error', fail) })
  return {
    ready,
    send(method, params = {}) {
      const at = ++id
      return new Promise((done, fail) => {
        waiting.set(at, msg => msg.error ? fail(new Error(`${method}: ${msg.error.message}`)) : done(msg.result))
        socket.send(JSON.stringify({ id: at, method, params }))
      })
    },
    waitFor(method) {
      return new Promise(done => listeners.push(msg => { if (msg.method === method) done(msg.params) }))
    },
    close() { socket.close() },
  }
}

async function measure(browserPort, device, scale, name) {
  const cssW = Math.round(device[0] / scale)
  const cssH = Math.round(device[1] / scale)
  const page = await target(browserPort)
  const cdp = session(page.webSocketDebuggerUrl)
  await cdp.ready
  await cdp.send('Page.enable')
  await cdp.send('Runtime.enable')
  await cdp.send('Emulation.setDeviceMetricsOverride', { width: cssW, height: cssH, deviceScaleFactor: scale, mobile: false })
  await cdp.send('Page.addScriptToEvaluateOnNewDocument', { source: STUB })
  const loaded = cdp.waitFor('Page.loadEventFired')
  await cdp.send('Page.navigate', { url: `http://127.0.0.1:${PORT}/index.html` })
  await loaded
  await new Promise(done => setTimeout(done, 700))

  const shell = await cdp.send('Runtime.evaluate', { expression: PROBE, returnByValue: true, awaitPromise: true })
  const shot = await cdp.send('Page.captureScreenshot', { format: 'png' })
  const shellPng = Buffer.from(shot.data, 'base64')
  await writeFile(path.join(OUT, `geometry-${name}.png`), shellPng)
  // collapsed sidebar: toggle must hide the aside without overflow, then restore
  await cdp.send('Runtime.evaluate', { expression: `document.querySelector('header > button')?.click()`, returnByValue: true })
  await new Promise(done => setTimeout(done, 300))
  const collapsed = await cdp.send('Runtime.evaluate', { expression: PROBE, returnByValue: true, awaitPromise: true })
  const collapsedShot = await cdp.send('Page.captureScreenshot', { format: 'png' })
  const collapsedPng = Buffer.from(collapsedShot.data, 'base64')
  await writeFile(path.join(OUT, `geometry-${name}-collapsed.png`), collapsedPng)
  await cdp.send('Runtime.evaluate', { expression: `document.querySelector('header > button')?.click()`, returnByValue: true })
  await new Promise(done => setTimeout(done, 300))
  // open the config panel (the densest surface) and measure it too
  await cdp.send('Runtime.evaluate', { expression: `[...document.querySelectorAll('header button')].find(b => b.textContent.trim() === '配置与还原')?.click()`, returnByValue: true })
  await new Promise(done => setTimeout(done, 500))
  // add a shortcut row and preview a diff so the widest controls are on screen
  await cdp.send('Runtime.evaluate', { expression: `[...document.querySelectorAll('[role="dialog"] button')].find(b => b.textContent.trim() === '添加快捷键')?.click()`, returnByValue: true })
  await new Promise(done => setTimeout(done, 300))
  await cdp.send('Runtime.evaluate', { expression: `document.querySelector('[role="dialog"] select').value = 'dark'; document.querySelector('[role="dialog"] select').dispatchEvent(new Event('change', { bubbles: true }))`, returnByValue: true })
  await new Promise(done => setTimeout(done, 200))
  await cdp.send('Runtime.evaluate', { expression: `[...document.querySelectorAll('[role="dialog"] button')].find(b => b.textContent.trim() === '预览差异')?.click()`, returnByValue: true })
  await new Promise(done => setTimeout(done, 500))
  const panel = await cdp.send('Runtime.evaluate', { expression: PROBE, returnByValue: true, awaitPromise: true })
  const panelShot = await cdp.send('Page.captureScreenshot', { format: 'png' })
  const panelPng = Buffer.from(panelShot.data, 'base64')
  await writeFile(path.join(OUT, `geometry-${name}-config.png`), panelPng)
  // the inventory panel is the other dense surface; close config first
  await cdp.send('Runtime.evaluate', { expression: `[...document.querySelectorAll('[role="dialog"] button')].find(b => b.textContent.trim() === '关闭')?.click()`, returnByValue: true })
  await new Promise(done => setTimeout(done, 300))
  await cdp.send('Runtime.evaluate', { expression: `[...document.querySelectorAll('header button')].find(b => b.textContent.trim() === '安装归属')?.click()`, returnByValue: true })
  await new Promise(done => setTimeout(done, 700))
  await cdp.send('Runtime.evaluate', { expression: `[...document.querySelectorAll('[role="dialog"] button')].find(b => b.textContent.trim() === '自动安装')?.click()`, returnByValue: true })
  // open the install preview so the widest surface (verbatim commands) is shown
  const picked = await cdp.send('Runtime.evaluate', { expression: `(() => {
    const boxes = [...document.querySelectorAll('[role="dialog"] input[type="checkbox"]')]
      .filter(box => (box.getAttribute('aria-label') || '').startsWith('选择组件'))
    boxes.slice(0, 2).forEach(box => box.click())
    return boxes.length
  })()`, returnByValue: true })
  await new Promise(done => setTimeout(done, 300))
  await cdp.send('Runtime.evaluate', { expression: `[...document.querySelectorAll('[role="dialog"] button')].find(b => b.textContent.trim() === '生成安装预览')?.click()`, returnByValue: true })
  await new Promise(done => setTimeout(done, 700))
  // prove the preview rendered; measuring an unopened section would pass silently
  const preview = await cdp.send('Runtime.evaluate', { expression: `(() => {
    const text = document.querySelector('[role="dialog"]')?.textContent || ''
    return { components: ${picked.result.value}, preview: text.includes('确认并执行安装'),
      commands: (text.match(/npm install -g|winget install --id/g) || []).length }
  })()`, returnByValue: true })
  await cdp.send('Runtime.evaluate', { expression: `[...document.querySelectorAll('[role="dialog"] button')].find(b => b.textContent.trim() === '确认并执行安装')?.scrollIntoView({block:'end'})`, returnByValue: true })
  const inventory = await cdp.send('Runtime.evaluate', { expression: PROBE, returnByValue: true, awaitPromise: true })
  const inventoryShot = await cdp.send('Page.captureScreenshot', { format: 'png' })
  const inventoryPng = Buffer.from(inventoryShot.data, 'base64')
  await writeFile(path.join(OUT, `geometry-${name}-inventory.png`), inventoryPng)
  cdp.close()
  return { name, device, scale, css: [cssW, cssH], shell: shell.result.value, collapsed: collapsed.result.value,
    panel: panel.result.value, inventory: inventory.result.value, preview: preview.result.value,
    shots: { shell: distinctColors(shellPng), collapsed: distinctColors(collapsedPng), panel: distinctColors(panelPng), inventory: distinctColors(inventoryPng) } }
}

// A screenshot that rendered nothing (blank frame) must fail: a silently black
// capture would otherwise look like a passing acceptance record.
function decodePng(buffer) {
  let offset = 8, width = 0, height = 0, bitDepth = 0, colorType = 0
  const idat = []
  while (offset < buffer.length) {
    const length = buffer.readUInt32BE(offset)
    const type = buffer.toString('ascii', offset + 4, offset + 8)
    const data = buffer.subarray(offset + 8, offset + 8 + length)
    if (type === 'IHDR') { width = data.readUInt32BE(0); height = data.readUInt32BE(4); bitDepth = data[8]; colorType = data[9] }
    else if (type === 'IDAT') idat.push(data)
    else if (type === 'IEND') break
    offset += 12 + length
  }
  const channels = { 0: 1, 2: 3, 4: 2, 6: 4 }[colorType]
  if (!width || !height || bitDepth !== 8 || !channels) return { width, height, bytes: null }
  return { width, height, bytes: require('node:zlib').inflateSync(Buffer.concat(idat)), channels }
}

function distinctColors(buffer) {
  const png = decodePng(buffer)
  if (!png.bytes) return { colors: 0, width: png.width, height: png.height }
  const stride = png.width * png.channels + 1 // leading filter byte per row
  const seen = new Set()
  for (let y = 0; y < png.height; y += 7) {
    for (let x = 0; x < png.width; x += 7) {
      const at = y * stride + 1 + x * png.channels
      if (at + 2 < png.bytes.length) seen.add((png.bytes[at] << 16) | (png.bytes[at + 1] << 8) | png.bytes[at + 2])
    }
  }
  return { colors: seen.size, width: png.width, height: png.height }
}

async function main() {
  await mkdir(OUT, { recursive: true })
  const server = await serve()
  const results = []
  let failed = 0
  const combos = []
  for (const device of [[1080, 720], [1440, 900], [1920, 1080]]) for (const scale of [1, 1.25, 1.5]) combos.push([device, scale])
  // smallest CSS viewports a 1080x720 screen presents at 125%/150% scaling
  for (const tiny of [[864, 576], [720, 480], [720, 448], [640, 400]]) combos.push([tiny, 1])

  const browser = spawn(EDGE, ['--headless=new', `--remote-debugging-port=${PORT + 1}`, '--no-first-run',
    '--user-data-dir=' + path.join(require('node:os').tmpdir(), `shuttle-geometry-${Date.now()}`), 'about:blank'],
    { stdio: 'ignore' })

  try {
    for (const [device, scale] of combos) {
      const name = `${device[0]}x${device[1]}-${String(scale * 100).replace('.', '_')}`
      const run = await measure(PORT + 1, device, scale, name)
      const problems = []
      if (run.shell.docOverflowX) problems.push('shell horizontal scroll')
      if (!run.shell.lang || !run.shell.lang.fits) problems.push('language switcher missing or clipped')
      if (!run.shell.palette || !run.shell.palette.fits || run.shell.palette.gap < 12 || run.shell.palette.options !== 9) problems.push('palette picker missing, clipped or tight')
      if (!run.shell.aside) problems.push('sidebar missing when expanded')
      if (!run.collapsed) problems.push('collapsed probe missing')
      else {
        // the collapsed state is a first-class layout: it must hide the aside,
        // keep the shell scroll-free and non-overlapping, and not render blank
        if (run.collapsed.aside) problems.push('sidebar did not collapse')
        if (run.collapsed.docOverflowX) problems.push('collapsed horizontal scroll')
        if (run.collapsed.shell.overlaps.length) problems.push(`collapsed overlap ${JSON.stringify(run.collapsed.shell.overlaps)}`)
        if (run.shots.collapsed.colors < 2) problems.push(`collapsed screenshot blank (${run.shots.collapsed.colors} colors)`)
      }
      if (run.shell.shell.overlaps.length) problems.push(`shell overlap ${JSON.stringify(run.shell.shell.overlaps)}`)
      if (run.panel.dialog) {
        if (!run.panel.dialog.fitsViewport) problems.push('dialog outside viewport')
        if (run.panel.dialog.horizontalScroll) problems.push('dialog horizontal scroll')
        if (run.panel.dialog.clipped.length) problems.push(`dialog clipped ${JSON.stringify(run.panel.dialog.clipped)}`)
        if (run.panel.dialog.overlaps.length) problems.push(`dialog overlap ${JSON.stringify(run.panel.dialog.overlaps)}`)
        const tight = run.panel.dialog.siblingGaps.filter(item => item.gap < 12)
        if (tight.length) problems.push(`sibling gap < 12: ${JSON.stringify(tight)}`)
      } else problems.push('config dialog did not open')
      if (run.inventory.dialog) {
        if (!run.inventory.dialog.fitsViewport) problems.push('inventory outside viewport')
        if (run.inventory.dialog.horizontalScroll) problems.push('inventory horizontal scroll')
        if (run.inventory.dialog.clipped.length) problems.push(`inventory clipped ${JSON.stringify(run.inventory.dialog.clipped)}`)
        if (run.inventory.dialog.overlaps.length) problems.push(`inventory overlap ${JSON.stringify(run.inventory.dialog.overlaps)}`)
        const tightInv = run.inventory.dialog.siblingGaps.filter(item => item.gap < 12)
        if (tightInv.length) problems.push(`inventory sibling gap < 12: ${JSON.stringify(tightInv)}`)
      } else problems.push('inventory dialog did not open')
      // the install preview is the widest inventory surface; if it never
      // rendered, the measurement above proved nothing
      if (!run.preview.preview) problems.push('install preview did not open')
      if (run.preview.commands < 2) problems.push(`install preview commands missing (${run.preview.commands})`)
      if (run.preview.components < 2) problems.push(`install components missing (${run.preview.components})`)
      if (run.shots.shell.colors < 2) problems.push(`shell screenshot blank (${run.shots.shell.colors} colors)`)
      if (run.shots.panel.colors < 2) problems.push(`panel screenshot blank (${run.shots.panel.colors} colors)`)
      if (run.shots.inventory.colors < 2) problems.push(`inventory screenshot blank (${run.shots.inventory.colors} colors)`)
      if (problems.length) failed++
      results.push({ name, css: run.css, problems, shellGap: run.shell.shell.minGap,
        minSiblingGap: run.panel.dialog?.siblingGaps[0] ?? null, screenshots: run.shots,
        installPreview: run.preview, inventoryGap: run.inventory.dialog?.siblingGaps[0] ?? null })
      console.log(`${problems.length ? 'FAIL' : 'PASS'} ${name} css=${run.css.join('x')} dpr=${run.panel.dpr} ` +
        `shellGap=${run.shell.shell.minGap} minSiblingGap=${run.panel.dialog?.siblingGaps[0]?.gap} ` +
        `colors=${run.shots.shell.colors}/${run.shots.panel.colors} ${problems.join('; ')}`)
    }
  } finally {
    browser.kill()
    server.close()
  }
  await writeFile(path.join(OUT, 'geometry-report.json'), JSON.stringify(results, null, 2))
  if (failed) { console.error(`${failed} of ${results.length} combinations failed`); process.exit(1) }
  console.log(`${results.length} combinations passed`)
}

main().catch(error => { console.error(error); process.exit(1) })
