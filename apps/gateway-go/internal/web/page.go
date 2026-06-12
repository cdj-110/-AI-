package web

const indexHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>微控网关状态</title>
  <style>
    :root { color: #111827; background: #f6f8fb; font-family: Inter, "Microsoft YaHei", sans-serif; }
    * { box-sizing: border-box; }
    body { margin: 0; padding: 20px 24px; }
    .page { width: min(100%, 1600px); margin: 0 auto; }
    .header { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 20px; }
    h1 { margin: 0; font-size: 26px; letter-spacing: -0.03em; }
    .desc { margin: 8px 0 0; color: #64748b; font-size: 14px; }
    .tabs { display: flex; gap: 8px; margin-bottom: 16px; }
    button { height: 36px; padding: 0 14px; border: 1px solid #dbe3ee; border-radius: 10px; background: #fff; color: #334155; cursor: pointer; transition: all 0.18s ease; }
    button:hover { border-color: #93c5fd; color: #1d4ed8; background: #f8fbff; box-shadow: 0 6px 16px rgba(37, 99, 235, 0.10); transform: translateY(-1px); }
    button:active { transform: translateY(0); box-shadow: none; }
    button.primary { border-color: #2563eb; color: #fff; background: #2563eb; }
    button.primary:hover { border-color: #1d4ed8; color: #fff; background: #1d4ed8; }
    button.active { border-color: #2563eb; color: #2563eb; background: #eff6ff; }
    button.danger { border-color: #fecaca; color: #dc2626; background: #fff; }
    button.danger:hover { border-color: #fca5a5; color: #b91c1c; background: #fff7f7; box-shadow: 0 6px 16px rgba(220, 38, 38, 0.10); }
    .pill { display: inline-flex; align-items: center; gap: 8px; padding: 8px 12px; border: 1px solid #e5eaf2; border-radius: 999px; background: #fff; color: #475569; font-size: 13px; }
    .dot { width: 8px; height: 8px; border-radius: 50%; background: #94a3b8; }
    .dot.ok { background: #22c55e; box-shadow: 0 0 0 6px rgba(34, 197, 94, 0.12); }
    .dot.bad { background: #ef4444; box-shadow: 0 0 0 6px rgba(239, 68, 68, 0.12); }
    .hero { display: grid; grid-template-columns: 1.2fr 0.8fr; gap: 16px; margin-bottom: 16px; }
    .card { padding: 20px; border: 1px solid #e9eef5; border-radius: 18px; background: #fff; box-shadow: 0 12px 34px rgba(15, 23, 42, 0.04); }
    .hero-main { background: radial-gradient(circle at top right, rgba(37, 99, 235, 0.14), transparent 32%), linear-gradient(135deg, #f8fbff, #fff); }
    .label { color: #64748b; font-size: 13px; font-weight: 600; }
    .big { display: block; margin-top: 10px; font-size: 34px; font-weight: 800; letter-spacing: -0.04em; }
    .meta { margin-top: 10px; color: #64748b; font-size: 13px; }
    .stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: 14px; margin-bottom: 16px; }
    .stats strong { display: block; margin-top: 8px; font-size: 24px; }
    .section-title { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin: 0 0 14px; }
    .section-title h2 { margin: 0; font-size: 17px; }
    table { width: 100%; border-collapse: collapse; font-size: 13px; }
    th { color: #64748b; background: #f8fafc; font-weight: 600; text-align: left; }
    th, td { padding: 12px; border-bottom: 1px solid #edf1f5; }
    code { padding: 2px 6px; border-radius: 6px; background: #f1f5f9; color: #334155; }
    textarea { width: 100%; min-height: 560px; padding: 16px; border: 1px solid #dbe3ee; border-radius: 14px; outline: none; resize: vertical; color: #1e293b; background: #fbfdff; font-family: Consolas, "Microsoft YaHei", monospace; font-size: 13px; line-height: 1.6; }
    textarea:focus { border-color: #2563eb; box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.10); }
    input, select { width: 100%; height: 36px; padding: 0 10px; border: 1px solid #dbe3ee; border-radius: 10px; color: #1e293b; background: #fff; outline: none; }
    input:focus, select:focus { border-color: #2563eb; box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.10); }
    .form-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin-bottom: 16px; }
    .form-grid label, .mqtt-grid label, .device-head label, .point-form label { display: grid; gap: 5px; min-width: 0; color: #64748b; font-size: 12px; font-weight: 600; }
    .mqtt-panel { margin-bottom: 18px; padding: 14px; border: 1px solid #e2e8f0; border-radius: 16px; background: #fbfdff; }
    .mqtt-head { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
    .mqtt-head h3 { margin: 0; font-size: 15px; }
    .mqtt-head .meta { margin: 4px 0 0; }
    .mqtt-switch { width: 180px; }
    .mqtt-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin-top: 14px; padding-top: 14px; border-top: 1px solid #e9eef5; }
    .mode-switch { display: flex; gap: 8px; margin-bottom: 14px; }
    .device-config-layout { display: grid; grid-template-columns: 260px minmax(0, 1fr); gap: 14px; align-items: start; }
    .interface-layout { display: none; grid-template-columns: 1fr 1fr; gap: 14px; margin-bottom: 18px; }
    .interface-panel { padding: 14px; border: 1px solid #e9eef5; border-radius: 14px; background: #ffffff; }
    .interface-panel h3 { margin: 0; font-size: 15px; }
    .interface-list { display: grid; gap: 10px; margin-top: 12px; }
    .interface-row { display: grid; gap: 8px; align-items: end; padding: 10px; border: 1px solid #edf1f5; border-radius: 12px; background: #fbfdff; }
    .serial-row { grid-template-columns: 1fr 1fr 110px 86px 86px 100px 92px auto; }
    .network-row { grid-template-columns: 1fr 1.4fr 130px 92px auto; }
    .interface-row label { display: grid; gap: 5px; min-width: 0; color: #64748b; font-size: 12px; font-weight: 600; }
    .device-sidebar { position: sticky; top: 16px; display: grid; gap: 10px; padding: 12px; border: 1px solid #e9eef5; border-radius: 16px; background: #fbfdff; }
    .device-sidebar-title { display: flex; align-items: center; justify-content: space-between; color: #64748b; font-size: 12px; font-weight: 700; }
    .device-nav { display: grid; gap: 8px; max-height: 520px; overflow: auto; }
    .device-nav-item { height: auto; min-height: 44px; padding: 9px 10px; text-align: left; border-radius: 12px; }
    .device-nav-item.active { border-color: #2563eb; color: #1d4ed8; background: #eff6ff; }
    .device-nav-name { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 700; }
    .device-nav-meta { display: block; margin-top: 3px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: #94a3b8; font-size: 12px; }
    .device-detail { min-width: 0; }
    .device-list, .point-list { display: grid; gap: 12px; }
    .device-form { overflow-x: auto; padding: 16px; border: 1px solid #dbeafe; border-radius: 16px; background: linear-gradient(180deg, #f8fbff, #fff); }
    .device-head { display: grid; grid-template-columns: minmax(130px, 1fr) minmax(140px, 1fr) minmax(120px, 0.8fr) minmax(180px, 1.2fr) 90px auto; gap: 10px; align-items: end; margin-bottom: 14px; }
    .point-form { display: grid; grid-template-columns: minmax(120px, 1fr) minmax(150px, 1.1fr) minmax(110px, 0.8fr) 86px 82px 70px 70px 84px 92px 76px 120px 118px 82px 86px 86px auto; gap: 8px; align-items: end; min-width: 1680px; padding: 12px; border: 1px solid #e9eef5; border-radius: 14px; background: #fbfdff; }
    .point-actions { display: flex; align-items: flex-end; gap: 6px; white-space: nowrap; }
    .point-actions button { padding: 0 10px; }
    .pdf-import { margin: 12px 0; padding: 12px; border: 1px dashed #bfdbfe; border-radius: 14px; background: #f8fbff; }
    .pdf-import-head { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; justify-content: space-between; }
    .pdf-import-head strong { color: #0f172a; }
    .pdf-import-actions { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
    .pdf-import-actions input { max-width: 260px; padding: 7px; }
    .pdf-preview { margin-top: 10px; }
    .pdf-preview table { margin-top: 8px; }
    .pdf-preview input, .pdf-preview select { height: 30px; min-width: 72px; padding: 0 7px; }
    .pdf-preview input[type="checkbox"] { width: auto; min-width: auto; height: auto; }
    .pdf-preview .pdf-name-input { min-width: 210px; }
    .pdf-preview .pdf-metric-input { min-width: 180px; }
    .pdf-preview .pdf-source { max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: #64748b; }
    .pdf-batch { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; margin-top: 10px; padding: 10px; border: 1px solid #e9eef5; border-radius: 10px; background: #fff; }
    .pdf-batch label { display: inline-grid; gap: 4px; color: #64748b; font-size: 12px; }
    .pdf-batch label input, .pdf-batch label select { width: 120px; }
    .pdf-low-confidence td { background: #fffbeb; }
    .pdf-preview .warning { margin: 6px 0; color: #b45309; font-size: 12px; }
    .point-live-value { display: grid; gap: 5px; min-width: 0; color: #64748b; font-size: 12px; font-weight: 600; }
    .point-live-value span { display: flex; align-items: center; min-height: 36px; padding: 0 10px; border: 1px solid #e5eaf2; border-radius: 8px; background: #f8fafc; color: #0f172a; font-size: 13px; font-weight: 700; }
    .ok-text { color: #16a34a; }
    .bad-text { color: #dc2626; }
    .events { display: grid; gap: 10px; }
    .event { padding: 12px; border-radius: 12px; background: #f8fafc; color: #475569; font-size: 13px; }
    .muted { color: #94a3b8; }
    .hidden { display: none; }
    .toast { min-height: 20px; color: #64748b; font-size: 13px; }
    .toast.ok { color: #16a34a; }
    .toast.bad { color: #dc2626; }
    body { padding: 0; color: #111827; background: #f6f8fb; }
    .app-shell { display: grid; grid-template-columns: 200px minmax(0, 1fr); min-height: 100vh; background: #f6f8fb; }
    .main-nav { border-right: 1px solid #e5eaf2; background: #ffffff; }
    .brand { display: flex; align-items: center; gap: 10px; height: 56px; padding: 0 18px; border-bottom: 1px solid #e5eaf2; color: #1677ff; font-size: 22px; font-weight: 800; letter-spacing: 0.10em; }
    .brand-mark { width: 18px; height: 18px; border-radius: 6px; background: linear-gradient(135deg, #1677ff, #52c41a); box-shadow: 0 8px 18px rgba(22, 119, 255, 0.18); transform: rotate(45deg); }
    .main-menu { display: grid; gap: 6px; padding: 8px; }
    .main-menu button { justify-content: flex-start; width: 100%; height: 42px; border: 0; border-radius: 8px; background: transparent; color: #334155; text-align: left; box-shadow: none; transform: none; }
    .main-menu button:hover, .main-menu button.active { color: #1677ff; background: #eaf4ff; box-shadow: none; transform: none; }
    .gateway-nav-tree { display: grid; gap: 2px; margin: -2px 0 8px; padding: 0 0 4px 10px; border-bottom: 1px solid #eef2f7; }
    .gateway-nav-tree .gateway-tree-title { padding: 6px 8px 4px; color: #64748b; font-size: 12px; font-weight: 800; }
    .gateway-nav-tree .gateway-tree-group { margin-top: 4px; padding: 5px 8px; color: #94a3b8; font-size: 12px; font-weight: 800; }
    .gateway-nav-tree .device-nav-item { width: 100%; height: auto; min-height: 34px; padding: 7px 8px; border: 0; border-radius: 8px; background: transparent; text-align: left; box-shadow: none; transform: none; }
    .gateway-nav-tree .device-nav-item:hover, .gateway-nav-tree .device-nav-item.active { color: #1677ff; background: #eaf4ff; box-shadow: none; transform: none; }
    .gateway-nav-tree .device-nav-item.interface-node { color: #0f172a; font-weight: 700; }
    .gateway-nav-tree .device-nav-item.device-child { margin-left: 14px; width: calc(100% - 14px); min-height: 30px; padding-left: 10px; border-left: 2px solid #e5eaf2; }
    .gateway-nav-tree .device-nav-name { font-size: 13px; }
    .gateway-nav-tree .device-nav-meta { margin-top: 1px; font-size: 11px; }
    .device-status-dot { display: inline-block; width: 8px; height: 8px; margin-right: 7px; border-radius: 50%; vertical-align: 1px; background: #f59e0b; box-shadow: 0 0 0 4px rgba(245, 158, 11, 0.12); }
    .device-status-dot.online { background: #22c55e; box-shadow: 0 0 0 4px rgba(34, 197, 94, 0.12); }
    .device-status-dot.offline { background: #ef4444; box-shadow: 0 0 0 4px rgba(239, 68, 68, 0.12); }
    .device-status-dot.connecting { background: #f59e0b; box-shadow: 0 0 0 4px rgba(245, 158, 11, 0.12); }
    .page { width: 100%; max-width: none; min-width: 0; }
    .header { height: 56px; margin: 0; padding: 0 16px; border-bottom: 1px solid #e5eaf2; background: #ffffff; }
    .header h1 { font-size: 14px; color: #111827; letter-spacing: 0; }
    .desc { display: none; }
    .top-metrics { display: flex; align-items: center; gap: 22px; color: #64748b; font-size: 12px; }
    .metric-bar { display: inline-flex; align-items: center; gap: 8px; }
    .metric-track { display: inline-block; width: 76px; height: 10px; overflow: hidden; border-radius: 999px; background: #e5eaf2; }
    .metric-fill { display: block; height: 100%; border-radius: inherit; background: #72c7ec; }
    .pill { border-color: #e5eaf2; background: #ffffff; color: #475569; }
    .tabs { margin: 16px; }
    .tabs { display: none; }
    .tabs button, .mode-switch button { border-color: #dbe3ee; background: #ffffff; color: #334155; border-radius: 8px; }
    .tabs button.active, .mode-switch button.active { border-color: #1677ff; color: #1677ff; background: #eff6ff; }
    button { border-color: #dbe3ee; border-radius: 8px; background: #ffffff; color: #334155; }
    button:hover { border-color: #1677ff; color: #1677ff; background: #f8fbff; box-shadow: 0 6px 16px rgba(22, 119, 255, 0.10); transform: translateY(-1px); }
    button.primary { border-color: #1677ff; color: #ffffff; background: #1677ff; }
    button.primary:hover { border-color: #0958d9; color: #ffffff; background: #0958d9; }
    button.danger { border-color: #fecaca; color: #dc2626; background: #ffffff; }
    button.danger:hover { border-color: #fca5a5; color: #b91c1c; background: #fff7f7; box-shadow: 0 6px 16px rgba(220, 38, 38, 0.10); }
    #panel-status, #panel-config, #panel-cloud { padding: 0 16px 18px; }
    .card { border-color: #e9eef5; border-radius: 14px; background: #ffffff; box-shadow: 0 12px 34px rgba(15, 23, 42, 0.04); }
    .hero-main { background: radial-gradient(circle at top right, rgba(22, 119, 255, 0.12), transparent 32%), #ffffff; }
    .label, .meta, .muted, .device-nav-meta { color: #64748b; }
    .big { color: #111827; }
    th { color: #64748b; background: #f8fafc; }
    th, td { border-bottom-color: #edf1f5; }
    code { background: #f1f5f9; color: #334155; }
    input, select, textarea { border-color: #dbe3ee; border-radius: 8px; color: #1e293b; background: #ffffff; }
    input:focus, select:focus, textarea:focus { border-color: #1677ff; box-shadow: 0 0 0 3px rgba(22, 119, 255, 0.10); }
    .form-grid label, .mqtt-grid label, .device-head label, .point-form label { color: #64748b; }
    .mqtt-panel, .interface-panel, .device-sidebar, .device-form, .point-form, .event { border-color: #e9eef5; border-radius: 14px; background: #ffffff; }
    .mqtt-grid { border-top-color: #e9eef5; }
    .device-config-layout { grid-template-columns: 300px minmax(0, 1fr); }
    .device-sidebar { top: 72px; min-height: 560px; background: #fbfdff; }
    .device-sidebar-title { color: #334155; }
    .device-nav { gap: 2px; }
    .device-nav-item { min-height: 38px; border: 0; border-radius: 8px; background: transparent; color: #334155; }
    .device-nav-item::before { content: "▸"; margin-right: 6px; color: #94a3b8; }
    .device-nav-item.active { border-color: transparent; color: #1677ff; background: #eaf4ff; }
    .gateway-tree-title { padding: 7px 8px; color: #0f172a; font-size: 13px; font-weight: 800; }
    .gateway-tree-group { margin-top: 6px; padding: 6px 8px; color: #64748b; font-size: 12px; font-weight: 800; }
    .device-nav-item.interface-node { color: #0f172a; font-weight: 700; }
    .device-nav-item.interface-node::before { content: "▾"; }
    .device-nav-item.device-child { margin-left: 18px; min-height: 34px; padding-left: 12px; border-left: 2px solid #e5eaf2; }
    .device-nav-item.device-child::before { content: ""; margin: 0; }
    .interface-summary { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 14px; padding-bottom: 12px; border-bottom: 1px solid #eef2f7; }
    .interface-summary h3 { margin: 0 0 4px; font-size: 17px; }
    .legacy-interface-title { display: none; }
    #panel-config .mode-switch, .device-list-title { display: none; }
    .device-head { padding-bottom: 14px; border-bottom: 1px solid #e9eef5; }
    .point-form { background: #fbfdff; }
    .ok-text { color: #16a34a; }
    .bad-text { color: #dc2626; }
    .toast { color: #64748b; }
    .toast.ok { color: #16a34a; }
    .toast.bad { color: #dc2626; }
    @media (max-width: 820px) {
      body { padding: 16px; }
      .app-shell { grid-template-columns: 1fr; }
      .main-nav { display: none; }
      .header, .hero { grid-template-columns: 1fr; flex-direction: column; align-items: flex-start; }
      .stats { grid-template-columns: 1fr; }
      .table-wrap { overflow-x: auto; }
      .section-title { align-items: flex-start; flex-direction: column; }
      .form-grid, .mqtt-grid, .device-head, .device-config-layout, .interface-layout, .serial-row, .network-row { grid-template-columns: 1fr; }
      .device-sidebar { position: static; }
      .mqtt-head { align-items: flex-start; flex-direction: column; }
      .mqtt-switch { width: 100%; }
    }
  </style>
</head>
<body>
  <div class="app-shell">
    <aside class="main-nav">
      <div class="brand"><span class="brand-mark"></span><span>WK-IOT</span></div>
      <nav class="main-menu">
        <button id="nav-status" class="active" onclick="showTab('status')">运行状态</button>
        <button id="nav-gateway" onclick="showTab('config')">智能网关</button>
        <div id="gateway-nav-tree" class="gateway-nav-tree"></div>
        <button id="nav-cloud" onclick="showTab('cloud')">云平台连接</button>
        <button onclick="showTab('status')">系统维护</button>
      </nav>
    </aside>
  <main class="page">
    <div class="header">
      <div>
        <h1>微控网关状态</h1>
        <p class="desc">本地采集、MQTT 上报、点位运行状态和本地配置。</p>
      </div>
      <div class="top-metrics">
        <span id="top-time">2026-06-11 10:40:15</span>
        <span class="metric-bar">采集 <i class="metric-track"><i class="metric-fill" style="width:72%"></i></i><b id="top-collect">-</b></span>
        <span class="metric-bar">点位 <i class="metric-track"><i class="metric-fill" style="width:48%"></i></i><b id="top-points">-</b></span>
        <span class="metric-bar">异常 <i class="metric-track"><i class="metric-fill" style="width:12%"></i></i><b id="top-errors">-</b></span>
      </div>
      <span class="pill"><i id="mqtt-dot" class="dot"></i><span id="mqtt-text">加载中</span></span>
    </div>

    <div class="tabs">
      <button id="tab-status" class="active" onclick="showTab('status')">运行状态</button>
      <button id="tab-config" onclick="showTab('config')">配置文件</button>
    </div>

    <section id="panel-status">
      <section class="hero">
        <article class="card hero-main">
          <span class="label">网关编号</span>
          <strong id="gateway-key" class="big">-</strong>
          <p id="runtime" class="meta">-</p>
          <p id="hardware-id" class="meta">硬件唯一 ID：-</p>
        </article>
        <article class="card">
          <span class="label">最近采集 / 上报</span>
          <strong id="last-collect" class="big">-</strong>
          <p id="last-publish" class="meta">-</p>
        </article>
      </section>

      <section class="stats">
        <article class="card"><span class="label">点位总数</span><strong id="point-count">0</strong></article>
        <article class="card"><span class="label">正常点位</span><strong id="healthy-count">0</strong></article>
        <article class="card"><span class="label">异常点位</span><strong id="error-count">0</strong></article>
      </section>

      <section class="card" style="margin-bottom:16px">
        <div class="section-title">
          <div>
            <h2>网关基础配置</h2>
            <p class="meta">维护网关本机参数，保存后采集配置会热加载。</p>
          </div>
          <div>
            <button onclick="loadConfig()">重新加载</button>
            <button class="primary" onclick="saveConfig()">保存配置</button>
          </div>
        </div>
        <div class="form-grid">
          <label>网关编号<input id="cfg-gateway-key" /></label>
          <label>采集周期(秒)<input id="cfg-collect-seconds" type="number" min="1" /></label>
          <label>缓存文件<input id="cfg-cache-file" /></label>
          <label>Web 监听<input id="cfg-web-listen" /></label>
        </div>
      </section>

      <section class="card">
        <div class="section-title"><h2>点位状态</h2><span id="refresh-time" class="muted">-</span></div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>子设备</th><th>点位</th><th>协议</th><th>地址</th><th>当前值</th><th>更新时间</th><th>状态</th></tr></thead>
            <tbody id="points"></tbody>
          </table>
        </div>
      </section>

      <section class="card" style="margin-top:16px">
        <div class="section-title"><h2>最近错误</h2><span class="muted">最多显示 50 条</span></div>
        <div id="events" class="events"></div>
      </section>
    </section>

    <section id="panel-config" class="hidden">
      <section class="card">
        <div class="section-title">
          <div>
            <h2>智能网关</h2>
            <p class="meta">从左侧选择串口、网口或设备，右侧只展示当前对象的相关配置。</p>
          </div>
          <div>
            <button onclick="loadConfig()">重新加载</button>
            <button onclick="collectNow()">立即采集一次</button>
            <button class="primary" onclick="saveConfig()">保存配置</button>
          </div>
        </div>
        <div class="mode-switch">
          <button id="mode-form" class="active" onclick="showConfigMode('form')">表单配置</button>
          <button id="mode-json" onclick="showConfigMode('json')">JSON 高级编辑</button>
        </div>
        <div id="form-editor">
          <div class="section-title legacy-interface-title">
            <div>
              <h2>接口配置</h2>
              <p class="meta">先维护网关上的串口和网口，后续设备可按现场接线关系挂到对应接口下。</p>
            </div>
          </div>
          <div class="interface-layout">
            <section class="interface-panel">
              <div class="section-title">
                <div>
                  <h3>串口配置</h3>
                  <p class="meta">适用于 RS485 / Modbus RTU。</p>
                </div>
                <button onclick="addSerialPort()">新增串口</button>
              </div>
              <div id="serial-port-list" class="interface-list"></div>
            </section>
            <section class="interface-panel">
              <div class="section-title">
                <div>
                  <h3>网口配置</h3>
                  <p class="meta">适用于以太网 / Modbus TCP。</p>
                </div>
                <button onclick="addNetworkPort()">新增网口</button>
              </div>
              <div id="network-port-list" class="interface-list"></div>
            </section>
          </div>
          <div class="section-title device-list-title">
            <div>
              <h2>设备与点位配置</h2>
              <p class="meta">先配置设备连接，再在设备下维护点位，结构更贴近现场采集关系。</p>
            </div>
            <button onclick="addDevice()">新增设备</button>
          </div>
          <div id="device-list" class="device-list"></div>
        </div>
        <textarea id="config-editor" class="hidden" spellcheck="false"></textarea>
      </section>
    </section>

    <section id="panel-cloud" class="hidden">
      <section class="card">
        <div class="section-title">
          <div>
            <h2>云平台连接</h2>
            <p class="meta">配置 MQTT 云平台连接。停用后网关只做本地采集和展示，不会上报数据。</p>
          </div>
          <div>
            <button onclick="loadConfig()">重新加载</button>
            <button class="primary" onclick="saveConfig()">保存配置</button>
          </div>
        </div>
        <div class="mqtt-panel">
          <div class="mqtt-head">
            <div>
              <h3>MQTT 云平台连接</h3>
              <p class="meta">启用后网关会连接云平台，并按配置上报采集数据。</p>
            </div>
            <label class="mqtt-switch">MQTT 状态<select id="cfg-mqtt-enabled" onchange="toggleMqttConfig()"><option value="true">启用</option><option value="false">停用</option></select></label>
          </div>
          <div id="mqtt-config-fields" class="mqtt-grid">
            <label>MQTT Broker<input id="cfg-mqtt-broker" /></label>
            <label>MQTT ClientId<input id="cfg-mqtt-client-id" /></label>
            <label>MQTT Username<input id="cfg-mqtt-username" /></label>
            <label>MQTT Password<input id="cfg-mqtt-password" type="password" /></label>
          </div>
        </div>
      </section>
    </section>
    <p id="config-message" class="toast" style="padding:0 16px 18px"></p>
  </main>
  </div>

  <script>
    let currentConfig = null;
    let configMode = 'form';
    let statusTimer = null;
    let activeInterfaceType = 'network';
    let activeInterfaceIndex = 0;
    let activeDeviceIndex = -1;
    let latestPointValues = new Map();
    let latestDeviceStatuses = new Map();
    let pdfPointPreview = [];
    let s7PointPreview = [];

    const formatTime = (value) => value ? new Date(value).toLocaleString() : '-';
    const escapeHtml = (value) => String(value ?? '').replace(/[&<>"']/g, (char) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
    const formatDuration = (seconds) => {
      const h = Math.floor(seconds / 3600);
      const m = Math.floor((seconds % 3600) / 60);
      const s = seconds % 60;
      return h ? h + '小时 ' + m + '分钟' : m + '分钟 ' + s + '秒';
    };

    function showTab(name) {
      document.getElementById('panel-status').classList.toggle('hidden', name !== 'status');
      document.getElementById('panel-config').classList.toggle('hidden', name !== 'config');
      document.getElementById('panel-cloud').classList.toggle('hidden', name !== 'cloud');
      document.getElementById('tab-status').classList.toggle('active', name === 'status');
      document.getElementById('tab-config').classList.toggle('active', name === 'config');
      document.getElementById('nav-status')?.classList.toggle('active', name === 'status');
      document.getElementById('nav-gateway')?.classList.toggle('active', name === 'config');
      document.getElementById('nav-cloud')?.classList.toggle('active', name === 'cloud');
      if (name === 'config' || name === 'cloud' || name === 'status') loadConfig();
    }

    function showConfigMode(mode) {
      syncConfigFromActiveEditor();
      configMode = mode;
      document.getElementById('form-editor').classList.toggle('hidden', mode !== 'form');
      document.getElementById('config-editor').classList.toggle('hidden', mode !== 'json');
      document.getElementById('mode-form').classList.toggle('active', mode === 'form');
      document.getElementById('mode-json').classList.toggle('active', mode === 'json');
      if (mode === 'form') renderConfigForm();
      if (mode === 'json') document.getElementById('config-editor').value = JSON.stringify(currentConfig, null, 2);
    }

    async function loadStatus() {
      const response = await fetch('/api/status');
      const data = await response.json();
      document.getElementById('gateway-key').textContent = data.gatewayKey;
      document.getElementById('runtime').textContent = '运行时长 ' + formatDuration(data.uptimeSeconds) + ' · 采集周期 ' + data.collectSeconds + 's';
      const hardware = data.hardwareIdentity || {};
      document.getElementById('hardware-id').textContent = hardware.available
        ? '硬件唯一 ID：' + hardware.id + ' · 来源：' + hardware.source
        : '硬件唯一 ID：未读取到 · ' + (hardware.message || '当前系统未开放硬件标识');
      document.getElementById('last-collect').textContent = formatTime(data.lastCollectAt);
      document.getElementById('last-publish').textContent = '最近上报：' + formatTime(data.lastPublishAt);
      document.getElementById('point-count').textContent = data.pointCount;
      document.getElementById('healthy-count').textContent = data.healthyCount;
      document.getElementById('error-count').textContent = data.errorCount;
      document.getElementById('refresh-time').textContent = '刷新于 ' + new Date().toLocaleTimeString();
      document.getElementById('top-time').textContent = new Date().toLocaleString();
      document.getElementById('top-collect').textContent = data.collectSeconds + 's';
      document.getElementById('top-points').textContent = data.pointCount;
      document.getElementById('top-errors').textContent = data.errorCount;
      const mqttEnabled = data.mqttEnabled !== false;
      document.getElementById('mqtt-dot').className = 'dot ' + (!mqttEnabled ? '' : (data.mqttConnected ? 'ok' : 'bad'));
      document.getElementById('mqtt-text').textContent = !mqttEnabled ? 'MQTT 未启用' : (data.mqttConnected ? 'MQTT 已连接' : 'MQTT 未连接');
      scheduleStatusRefresh(data.collectSeconds);

      const points = data.points || [];
      const errors = data.errors || [];
      latestPointValues = new Map(points.map((point) => [
        pointValueKey(point.deviceKey, point.metric),
        { value: point.value, updatedAt: point.updatedAt, error: point.error }
      ]));
      latestDeviceStatuses = buildDeviceStatusMap(points);
      updateLivePointValues();
      updateDeviceStatusDots();
      document.getElementById('points').innerHTML = points.map((point) =>
        '<tr>' +
          '<td><code>' + escapeHtml(point.deviceKey) + '</code></td>' +
          '<td>' + escapeHtml(point.name || point.metric) + '<div class="muted">' + escapeHtml(point.metric) + '</div></td>' +
          '<td>' + escapeHtml(point.protocol) + '</td>' +
          '<td>' + escapeHtml(point.address) + '</td>' +
          '<td>' + escapeHtml(point.value === undefined || point.value === null ? '-' : point.value) + '</td>' +
          '<td>' + escapeHtml(formatTime(point.updatedAt)) + '</td>' +
          '<td class="' + (point.error ? 'bad-text' : 'ok-text') + '">' + escapeHtml(point.error || '正常') + '</td>' +
        '</tr>'
      ).join('');

      document.getElementById('events').innerHTML = errors.length
        ? errors.map((event) => '<div class="event"><strong>' + escapeHtml(formatTime(event.time)) + '</strong><div>' + escapeHtml(event.message) + '</div></div>').join('')
        : '<div class="muted">暂无错误</div>';
    }

    function pointValueKey(deviceKey, metric) {
      return String(deviceKey || '') + '::' + String(metric || '');
    }

    function buildDeviceStatusMap(points) {
      const grouped = new Map();
      (points || []).forEach((point) => {
        const key = String(point.deviceKey || '');
        if (!grouped.has(key)) grouped.set(key, []);
        grouped.get(key).push(point);
      });
      const statuses = new Map();
      grouped.forEach((items, deviceKey) => {
        const hasHealthyValue = items.some((item) => !item.error && item.updatedAt);
        const hasError = items.some((item) => item.error);
        if (hasHealthyValue) statuses.set(deviceKey, 'online');
        else if (hasError) statuses.set(deviceKey, 'offline');
        else statuses.set(deviceKey, 'connecting');
      });
      return statuses;
    }

    function deviceStatus(device) {
      return latestDeviceStatuses.get(String(device?.deviceKey || '')) || 'connecting';
    }

    function deviceStatusText(status) {
      if (status === 'online') return '已连接';
      if (status === 'offline') return '离线';
      return '连接中';
    }

    function renderLivePointValue(device, point) {
      const live = latestPointValues.get(pointValueKey(device?.deviceKey, point?.metric));
      const value = live && live.value !== undefined && live.value !== null ? live.value : '-';
      const title = live?.updatedAt ? ' title="更新时间：' + escapeAttr(formatTime(live.updatedAt)) + '"' : '';
      const cls = live?.error ? ' bad-text' : '';
      return '<div class="point-live-value">实时值<span data-live-key="' + escapeAttr(pointValueKey(device?.deviceKey, point?.metric)) + '" class="' + cls + '"' + title + '>' + escapeHtml(value) + '</span></div>';
    }

    function updateLivePointValues() {
      document.querySelectorAll('[data-live-key]').forEach((node) => {
        const live = latestPointValues.get(node.dataset.liveKey);
        node.textContent = live && live.value !== undefined && live.value !== null ? live.value : '-';
        node.classList.toggle('bad-text', Boolean(live?.error));
        if (live?.updatedAt) node.title = '更新时间：' + formatTime(live.updatedAt);
        else node.removeAttribute('title');
      });
    }

    async function loadConfig() {
      const message = document.getElementById('config-message');
      message.className = 'toast';
      message.textContent = '正在读取配置...';
      try {
        const response = await fetch('/api/config');
        if (!response.ok) throw new Error(await response.text());
        const data = await response.json();
        currentConfig = JSON.parse(data.content);
        document.getElementById('config-editor').value = JSON.stringify(currentConfig, null, 2);
        renderConfigForm();
        message.className = 'toast ok';
        message.textContent = '配置已加载';
      } catch (error) {
        message.className = 'toast bad';
        message.textContent = '读取失败：' + error.message;
      }
    }

    async function saveConfig() {
      const message = document.getElementById('config-message');
      message.className = 'toast';
      message.textContent = '正在保存配置...';
      try {
        syncConfigFromActiveEditor();
        const content = JSON.stringify(currentConfig, null, 2);
        const response = await fetch('/api/config', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ content })
        });
        if (!response.ok) throw new Error(await response.text());
        const data = await response.json();
        message.className = 'toast ok';
        message.textContent = data.restartRequired ? '保存成功，重启网关服务后生效。' : '保存成功，采集配置已热加载。';
        await loadStatus();
      } catch (error) {
        message.className = 'toast bad';
        message.textContent = '保存失败：' + error.message;
      }
    }

    async function collectNow() {
      const message = document.getElementById('config-message');
      message.className = 'toast';
      message.textContent = '正在采集...';
      try {
        const response = await fetch('/api/collect-now', { method: 'POST' });
        if (!response.ok) throw new Error(await response.text());
        message.className = 'toast ok';
        message.textContent = '采集完成，实时值已刷新。';
        await loadStatus();
      } catch (error) {
        message.className = 'toast bad';
        message.textContent = '采集失败：' + error.message;
      }
    }

    function syncConfigFromActiveEditor() {
      if (configMode === 'json') {
        currentConfig = JSON.parse(document.getElementById('config-editor').value);
      } else {
        syncActiveInterfaceForm();
        syncActiveDeviceForm();
        currentConfig = readConfigForm();
        document.getElementById('config-editor').value = JSON.stringify(currentConfig, null, 2);
      }
    }

    function renderConfigForm() {
      if (!currentConfig) return;
      document.getElementById('cfg-gateway-key').value = currentConfig.gatewayKey || '';
      document.getElementById('cfg-collect-seconds').value = currentConfig.collectIntervalSeconds || 5;
      document.getElementById('cfg-cache-file').value = currentConfig.cacheFile || '';
      document.getElementById('cfg-web-listen').value = currentConfig.web?.listen || '0.0.0.0:8088';
      document.getElementById('cfg-mqtt-enabled').value = currentConfig.mqtt?.enabled === false ? 'false' : 'true';
      document.getElementById('cfg-mqtt-broker').value = currentConfig.mqtt?.broker || '';
      document.getElementById('cfg-mqtt-client-id').value = currentConfig.mqtt?.clientId || '';
      document.getElementById('cfg-mqtt-username').value = currentConfig.mqtt?.username || '';
      document.getElementById('cfg-mqtt-password').value = currentConfig.mqtt?.password || '';
      toggleMqttConfig();
      ensureDefaultInterfaces();
      renderInterfaces();
      currentConfig.devices = normalizeDevices(currentConfig);
      if (!getInterfaceList(activeInterfaceType).length) {
        activeInterfaceType = 'network';
        activeInterfaceIndex = 0;
      }
      activeInterfaceIndex = Math.min(activeInterfaceIndex, Math.max(getInterfaceList(activeInterfaceType).length - 1, 0));
      if (activeDeviceIndex >= currentConfig.devices.length) activeDeviceIndex = -1;
      renderDevices(currentConfig.devices);
    }

    function toggleMqttConfig() {
      const enabled = document.getElementById('cfg-mqtt-enabled')?.value !== 'false';
      document.getElementById('mqtt-config-fields')?.classList.toggle('hidden', !enabled);
    }

    function renderInterfaces() {
      document.getElementById('serial-port-list').innerHTML = (currentConfig.serialPorts || []).length
        ? currentConfig.serialPorts.map((item, index) =>
          '<div class="interface-row serial-row" data-serial-index="' + index + '">' +
            field('名称', 'name', item.name || '') +
            field('端口', 'port', item.port || '') +
            numberField('波特率', 'baudRate', item.baudRate || 9600) +
            numberField('数据位', 'dataBits', item.dataBits || 8) +
            numberField('停止位', 'stopBits', item.stopBits || 1) +
            selectField('校验', 'parity', item.parity || 'none', ['none', 'odd', 'even']) +
            selectField('状态', 'enabled', item.enabled === false ? 'false' : 'true', ['true', 'false']) +
            '<div class="point-actions"><button class="danger" onclick="removeSerialPort(' + index + ')">删除</button></div>' +
          '</div>'
        ).join('')
        : '<div class="muted">暂无串口配置</div>';

      document.getElementById('network-port-list').innerHTML = (currentConfig.networkPorts || []).length
        ? currentConfig.networkPorts.map((item, index) =>
          '<div class="interface-row network-row" data-network-index="' + index + '">' +
            field('名称', 'name', item.name || '') +
            field('地址', 'address', item.address || '') +
            selectField('模式', 'mode', item.mode || 'tcp-client', ['tcp-client', 'tcp-server']) +
            selectField('状态', 'enabled', item.enabled === false ? 'false' : 'true', ['true', 'false']) +
            '<div class="point-actions"><button class="danger" onclick="removeNetworkPort(' + index + ')">删除</button></div>' +
          '</div>'
        ).join('')
        : '<div class="muted">暂无网口配置</div>';
    }

    function ensureDefaultInterfaces() {
      currentConfig.serialPorts = currentConfig.serialPorts?.length ? currentConfig.serialPorts : [
        { name: 'Serial1', port: 'COM1', baudRate: 9600, dataBits: 8, stopBits: 1, parity: 'none', enabled: true },
        { name: 'Serial2', port: 'COM2', baudRate: 9600, dataBits: 8, stopBits: 1, parity: 'none', enabled: true }
      ];
      currentConfig.networkPorts = currentConfig.networkPorts?.length ? currentConfig.networkPorts : [
        { name: 'net1', address: '192.168.1.50:502', mode: 'tcp-client', enabled: true },
        { name: 'net2', address: '192.168.1.51:502', mode: 'tcp-client', enabled: true }
      ];
    }

    function getInterfaceList(type) {
      return type === 'serial' ? (currentConfig.serialPorts || []) : (currentConfig.networkPorts || []);
    }

    function getInterfaceName(type, index) {
      const item = getInterfaceList(type)[index] || {};
      return item.name || item.port || item.address || (type === 'serial' ? 'Serial' + (index + 1) : 'net' + (index + 1));
    }

    function getInterfaceLabel(type, index) {
      const item = getInterfaceList(type)[index] || {};
      const title = getInterfaceName(type, index);
      const sub = type === 'serial' ? item.port : item.address;
      return sub && sub !== title ? title + ' (' + sub + ')' : title;
    }

    function deviceMatchesInterface(device, type, index) {
      const name = getInterfaceName(type, index);
      if (device.interfaceType || device.interfaceName) {
        return device.interfaceType === type && device.interfaceName === name;
      }
      if (type === 'serial') return device.protocol === 'modbus-rtu' && index === 0;
      return device.protocol !== 'modbus-rtu' && index === 0;
    }

    function devicesForInterface(type, index) {
      return (currentConfig.devices || [])
        .map((device, deviceIndex) => ({ device, deviceIndex }))
        .filter((item) => deviceMatchesInterface(item.device, type, index));
    }

    function renderDevices(devices) {
      const serialTree = (currentConfig.serialPorts || []).map((item, index) => renderInterfaceNode('serial', index)).join('');
      const networkTree = (currentConfig.networkPorts || []).map((item, index) => renderInterfaceNode('network', index)).join('');
      const detail = activeDeviceIndex >= 0 && devices[activeDeviceIndex]
        ? renderDeviceDetail(devices[activeDeviceIndex])
        : renderInterfaceDetail(activeInterfaceType, activeInterfaceIndex);
      const tree =
        '<div class="gateway-tree-title">接口与设备</div>' +
        '<div class="gateway-tree-group">串口配置</div>' + serialTree +
        '<div class="gateway-tree-group">网口配置</div>' + networkTree;
      const treeTarget = document.getElementById('gateway-nav-tree');
      if (treeTarget) treeTarget.innerHTML = tree;
      document.getElementById('device-list').innerHTML = '<section class="device-detail">' + detail + '</section>';
    }

    function renderInterfaceNode(type, index) {
      const active = activeDeviceIndex < 0 && activeInterfaceType === type && activeInterfaceIndex === index;
      const devices = devicesForInterface(type, index);
      return '<button class="device-nav-item interface-node ' + (active ? 'active' : '') + '" onclick="selectInterface(&quot;' + type + '&quot;,' + index + ')">' +
          '<span class="device-nav-name">' + escapeHtml(getInterfaceLabel(type, index)) + '</span>' +
          '<span class="device-nav-meta">' + (type === 'serial' ? '串口 · ' : '网口 · ') + devices.length + ' 台设备</span>' +
        '</button>' +
        devices.map((item) =>
          '<button class="device-nav-item device-child ' + (item.deviceIndex === activeDeviceIndex ? 'active' : '') + '" onclick="selectDevice(' + item.deviceIndex + ',&quot;' + type + '&quot;,' + index + ')">' +
            '<span class="device-nav-name">' + escapeHtml(item.device.name || item.device.deviceKey || ('设备 ' + (item.deviceIndex + 1))) + '</span>' +
            '<span class="device-nav-meta">' + escapeHtml((item.device.deviceKey || '-') + ' · ' + (item.device.address || '-')) + '</span>' +
          '</button>'
        ).join('');
    }

    function renderInterfaceDetail(type, index) {
      const item = getInterfaceList(type)[index] || {};
      const title = getInterfaceLabel(type, index);
      const count = devicesForInterface(type, index).length;
      const fields = type === 'serial'
        ? field('名称', 'name', item.name || '') +
          field('端口', 'port', item.port || '') +
          numberField('波特率', 'baudRate', item.baudRate || 9600) +
          numberField('数据位', 'dataBits', item.dataBits || 8) +
          numberField('停止位', 'stopBits', item.stopBits || 1) +
          selectField('校验', 'parity', item.parity || 'none', ['none', 'odd', 'even']) +
          selectField('状态', 'enabled', item.enabled === false ? 'false' : 'true', ['true', 'false'])
        : field('名称', 'name', item.name || '') +
          field('地址', 'address', item.address || '') +
          selectField('模式', 'mode', item.mode || 'tcp-client', ['tcp-client', 'tcp-server']) +
          selectField('状态', 'enabled', item.enabled === false ? 'false' : 'true', ['true', 'false']);
      return '<div class="device-form interface-detail-form" data-interface-type="' + type + '" data-interface-index="' + index + '">' +
          '<div class="interface-summary">' +
            '<div><h3>' + escapeHtml(title) + '</h3><div class="muted">' + (type === 'serial' ? '串口' : '网口') + '下已配置 ' + count + ' 台设备</div></div>' +
            '<button onclick="addDevice()">新增设备</button>' +
          '</div>' +
          '<div class="device-head">' + fields + '</div>' +
        '</div>';
    }

    function renderDeviceDetail(device) {
      return '<div class="device-form device-detail-form" data-device-index="' + activeDeviceIndex + '">' +
        '<div class="interface-summary">' +
          '<div><h3>' + escapeHtml(device.name || device.deviceKey || '设备配置') + '</h3><div class="muted">所属接口：' + escapeHtml(getInterfaceLabel(activeInterfaceType, activeInterfaceIndex)) + '</div></div>' +
          '<button onclick="selectInterface(&quot;' + activeInterfaceType + '&quot;,' + activeInterfaceIndex + ')">返回接口</button>' +
        '</div>' +
        '<div class="device-head">' +
          field('设备编号', 'deviceKey', device.deviceKey || '') +
          field('设备名称', 'name', device.name || '') +
          selectField('协议', 'protocol', device.protocol || (activeInterfaceType === 'serial' ? 'modbus-rtu' : 'modbus-tcp'), ['modbus-tcp', 'modbus-rtu', 'siemens-s7', 'iec104']) +
          field('地址', 'address', device.address || '') +
          numberField((device.protocol === 'iec104' ? '公共地址' : '从站ID'), 'slaveId', device.slaveId || 1) +
          '<div class="point-actions"><button onclick="testDeviceConnection()">测试连接</button><button onclick="addPoint(' + activeDeviceIndex + ')">新增点位</button><button class="danger" onclick="removeDevice(' + activeDeviceIndex + ')">删除设备</button></div>' +
        '</div>' +
        renderS7ImportPanel(device) +
        '<div class="point-list">' + renderPoints(device.points || [], activeDeviceIndex, device) + '</div>' +
      '</div>';
    }

    function renderS7ImportPanel(device) {
      if (device.protocol !== 'siemens-s7') return '';
      return '<div class="pdf-import">' +
        '<div class="pdf-import-head">' +
          '<div><strong>S7-200 SMART 一键扫描点位</strong><div class="muted">自动探测 V/M/I/Q 常用区域中可读取的地址，并生成候选点位；普通 S7 通信无法读取 PLC 程序里的变量名。</div></div>' +
          '<div class="pdf-import-actions">' +
            '<button onclick="testDeviceConnection()">测试连接</button>' +
            '<button class="primary" onclick="scanS7Points()">一键扫描 PLC 点位</button>' +
            '<button class="primary" onclick="importS7Points()">导入所选点位</button>' +
          '</div>' +
        '</div>' +
        '<div id="s7-point-preview" class="pdf-preview muted">尚未扫描 PLC 点位。点击“一键扫描 PLC 点位”后，会自动探测 SMART 常用地址。</div>' +
      '</div>';
    }

    function renderPdfImportPanel() {
      return '<div class="pdf-import">' +
        '<div class="pdf-import-head">' +
          '<div><strong>PDF 自动识别点位</strong><div class="muted">上传设备 Modbus 说明书，先预览识别结果，再追加到当前设备。</div></div>' +
          '<div class="pdf-import-actions">' +
            '<input id="pdf-point-file" type="file" accept="application/pdf,.pdf" />' +
            '<button onclick="previewPdfPoints()">解析预览</button>' +
          '<button class="primary" onclick="applyPdfPoints()">导入所选点位</button>' +
          '</div>' +
        '</div>' +
        '<div id="pdf-point-preview" class="pdf-preview muted">尚未解析 PDF</div>' +
      '</div>';
    }

    function selectInterface(type, index) {
      syncActiveInterfaceForm();
      syncActiveDeviceForm();
      activeInterfaceType = type;
      activeInterfaceIndex = index;
      activeDeviceIndex = -1;
      currentConfig.points = flattenDevices(currentConfig.devices);
      if (document.getElementById('panel-config')?.classList.contains('hidden')) showTab('config');
      renderDevices(currentConfig.devices || []);
    }

    function selectDevice(index, type, interfaceIndex) {
      syncActiveInterfaceForm();
      syncActiveDeviceForm();
      activeInterfaceType = type;
      activeInterfaceIndex = interfaceIndex;
      activeDeviceIndex = index;
      currentConfig.points = flattenDevices(currentConfig.devices);
      if (document.getElementById('panel-config')?.classList.contains('hidden')) showTab('config');
      renderDevices(currentConfig.devices || []);
    }

    function renderPoints(points, deviceIndex, device) {
      return points.map((point, pointIndex) =>
        '<div class="point-form" data-point-index="' + pointIndex + '">' +
          field('名称', 'name', point.name || '') +
          field('标识符', 'metric', point.metric || '') +
          renderLivePointValue(device, point) +
          selectField('存储区', 'area', point.area || (device.protocol === 'siemens-s7' ? 'DB' : ''), ['', 'DB', 'M', 'I', 'Q']) +
          numberField('DB块', 'dbNumber', point.dbNumber || 1) +
          numberField('Rack', 'rack', point.rack || 0) +
          numberField('Slot', 'slot', point.slot || 0) +
          selectField('功能码', 'function', point.function || 3, ['1', '2', '3', '4']) +
          numberField('寄存器', 'register', point.register || 0) +
          numberField('数量', 'quantity', point.quantity || 1) +
          selectField('数据类型', 'dataType', point.dataType || 'uint16', ['bool', 'uint16', 'int16', 'uint32', 'int32', 'float32']) +
          selectField('字节排列', 'byteOrderMode', byteOrderMode(point), ['ABCD', 'BADC', 'CDAB', 'DCBA']) +
          numberField('位索引', 'bitIndex', point.bitIndex ?? '', true) +
          numberField('倍率', 'scale', point.scale || 1, true) +
          numberField('偏移', 'offset', point.offset || 0, true) +
          '<div class="point-actions"><button onclick="copyPoint(' + deviceIndex + ',' + pointIndex + ')">复制</button><button class="danger" onclick="removePoint(' + deviceIndex + ',' + pointIndex + ')">删除</button></div>' +
        '</div>'
      ).join('');
    }

    function field(label, key, value) {
      return '<label>' + label + '<input data-key="' + key + '" value="' + escapeAttr(value) + '" /></label>';
    }

    function renderInterfaceNode(type, index) {
      const active = activeDeviceIndex < 0 && activeInterfaceType === type && activeInterfaceIndex === index;
      const devices = devicesForInterface(type, index);
      return '<button class="device-nav-item interface-node ' + (active ? 'active' : '') + '" onclick="selectInterface(&quot;' + type + '&quot;,' + index + ')">' +
          '<span class="device-nav-name">' + escapeHtml(getInterfaceLabel(type, index)) + '</span>' +
          '<span class="device-nav-meta">' + (type === 'serial' ? '串口 · ' : '网口 · ') + devices.length + ' 台设备</span>' +
        '</button>' +
        devices.map((item) => {
          const status = deviceStatus(item.device);
          return '<button class="device-nav-item device-child ' + (item.deviceIndex === activeDeviceIndex ? 'active' : '') + '" onclick="selectDevice(' + item.deviceIndex + ',&quot;' + type + '&quot;,' + index + ')">' +
              '<span class="device-nav-name"><span class="device-status-dot ' + status + '" data-device-status-key="' + escapeAttr(item.device.deviceKey || '') + '" title="' + deviceStatusText(status) + '"></span>' + escapeHtml(item.device.name || item.device.deviceKey || ('设备 ' + (item.deviceIndex + 1))) + '</span>' +
              '<span class="device-nav-meta">' + escapeHtml((item.device.deviceKey || '-') + ' · ' + (item.device.address || '-') + ' · ' + deviceStatusText(status)) + '</span>' +
            '</button>';
        }).join('');
    }

    function updateDeviceStatusDots() {
      document.querySelectorAll('[data-device-status-key]').forEach((node) => {
        const status = latestDeviceStatuses.get(String(node.dataset.deviceStatusKey || '')) || 'connecting';
        node.classList.toggle('online', status === 'online');
        node.classList.toggle('offline', status === 'offline');
        node.classList.toggle('connecting', status === 'connecting');
        node.title = deviceStatusText(status);
        const meta = node.closest('.device-nav-item')?.querySelector('.device-nav-meta');
        if (meta) {
          const raw = meta.textContent.split(' · ');
          if (raw.length >= 2) meta.textContent = raw.slice(0, 2).join(' · ') + ' · ' + deviceStatusText(status);
        }
      });
    }

    function renderDeviceDetail(device) {
      const protocol = device.protocol || (activeInterfaceType === 'serial' ? 'modbus-rtu' : 'modbus-tcp');
      const connectionFields = protocol === 'siemens-s7'
        ? numberField('Rack', 'rack', device.rack || 0) +
          numberField('Slot', 'slot', device.slot || 0) +
          field('LocalTSAP', 'localTsap', device.localTsap || '0200') +
          field('RemoteTSAP', 'remoteTsap', device.remoteTsap || '0200')
        : numberField(protocol === 'iec104' ? '公共地址' : '从站ID', 'slaveId', device.slaveId || 1);
      return '<div class="device-form device-detail-form" data-device-index="' + activeDeviceIndex + '">' +
        '<div class="interface-summary">' +
          '<div><h3>' + escapeHtml(device.name || device.deviceKey || '设备配置') + '</h3><div class="muted">所属接口：' + escapeHtml(getInterfaceLabel(activeInterfaceType, activeInterfaceIndex)) + '</div></div>' +
          '<button onclick="selectInterface(&quot;' + activeInterfaceType + '&quot;,' + activeInterfaceIndex + ')">返回接口</button>' +
        '</div>' +
        '<div class="device-head">' +
          field('设备编号', 'deviceKey', device.deviceKey || '') +
          field('设备名称', 'name', device.name || '') +
          selectField('协议', 'protocol', protocol, ['modbus-tcp', 'modbus-rtu', 'siemens-s7', 'iec104']) +
          field(protocol === 'siemens-s7' ? 'PLC地址' : (protocol === 'iec104' ? '104地址' : '地址'), 'address', device.address || '') +
          connectionFields +
          '<div class="point-actions"><button onclick="testDeviceConnection()">测试连接</button><button onclick="addPoint(' + activeDeviceIndex + ')">新增点位</button><button class="danger" onclick="removeDevice(' + activeDeviceIndex + ')">删除设备</button></div>' +
        '</div>' +
        renderS7ImportPanel({ ...device, protocol }) +
        '<div class="point-list">' + renderPoints(device.points || [], activeDeviceIndex, { ...device, protocol }) + '</div>' +
      '</div>';
    }

    function renderPoints(points, deviceIndex, device) {
      return points.map((point, pointIndex) =>
        device.protocol === 'siemens-s7'
          ? renderS7Point(point, pointIndex, deviceIndex, device)
          : device.protocol === 'iec104'
          ? renderIEC104Point(point, pointIndex, deviceIndex, device)
          : renderModbusPoint(point, pointIndex, deviceIndex, device)
      ).join('');
    }

    function renderS7Point(point, pointIndex, deviceIndex, device) {
      return '<div class="point-form" data-point-index="' + pointIndex + '">' +
        field('名称', 'name', point.name || '') +
        field('标识符', 'metric', point.metric || '') +
        renderLivePointValue(device, point) +
        selectField('存储区', 'area', point.area || 'V', ['V', 'DB', 'M', 'I', 'Q']) +
        numberField('DB块', 'dbNumber', point.dbNumber || 1) +
        numberField('偏移', 'register', point.register || 0) +
        numberField('数量', 'quantity', point.quantity || 1) +
        selectField('数据类型', 'dataType', point.dataType || 'uint16', ['bool', 'uint16', 'int16', 'uint32', 'int32', 'float32']) +
        numberField('位索引', 'bitIndex', point.bitIndex ?? '', true) +
        numberField('倍率', 'scale', point.scale || 1, true) +
        numberField('偏移量', 'offset', point.offset || 0, true) +
        '<div class="point-actions"><button onclick="copyPoint(' + deviceIndex + ',' + pointIndex + ')">复制</button><button class="danger" onclick="removePoint(' + deviceIndex + ',' + pointIndex + ')">删除</button></div>' +
      '</div>';
    }

    function renderModbusPoint(point, pointIndex, deviceIndex, device) {
      return '<div class="point-form" data-point-index="' + pointIndex + '">' +
        field('名称', 'name', point.name || '') +
        field('标识符', 'metric', point.metric || '') +
        renderLivePointValue(device, point) +
        selectField('功能码', 'function', point.function || 3, ['1', '2', '3', '4']) +
        numberField('寄存器', 'register', point.register || 0) +
        numberField('数量', 'quantity', point.quantity || 1) +
        selectField('数据类型', 'dataType', point.dataType || 'uint16', ['bool', 'uint16', 'int16', 'uint32', 'int32', 'float32']) +
        selectField('字节排列', 'byteOrderMode', byteOrderMode(point), ['ABCD', 'BADC', 'CDAB', 'DCBA']) +
        numberField('位索引', 'bitIndex', point.bitIndex ?? '', true) +
        numberField('倍率', 'scale', point.scale || 1, true) +
        numberField('偏移', 'offset', point.offset || 0, true) +
        '<div class="point-actions"><button onclick="copyPoint(' + deviceIndex + ',' + pointIndex + ')">复制</button><button class="danger" onclick="removePoint(' + deviceIndex + ',' + pointIndex + ')">删除</button></div>' +
      '</div>';
    }

    function renderIEC104Point(point, pointIndex, deviceIndex, device) {
      return '<div class="point-form" data-point-index="' + pointIndex + '">' +
        field('名称', 'name', point.name || '') +
        field('标识符', 'metric', point.metric || '') +
        renderLivePointValue(device, point) +
        numberField('IOA地址', 'register', point.register || 1) +
        selectField('类型', 'dataType', point.dataType || 'float32', ['single', 'double', 'normalized', 'scaled', 'float32']) +
        numberField('倍率', 'scale', point.scale || 1, true) +
        numberField('偏移', 'offset', point.offset || 0, true) +
        '<div class="point-actions"><button onclick="copyPoint(' + deviceIndex + ',' + pointIndex + ')">复制</button><button class="danger" onclick="removePoint(' + deviceIndex + ',' + pointIndex + ')">删除</button></div>' +
      '</div>';
    }

    function numberField(label, key, value, decimal) {
      return '<label>' + label + '<input data-key="' + key + '" type="number" ' + (decimal ? 'step="any"' : 'step="1"') + ' value="' + escapeAttr(value) + '" /></label>';
    }

    function selectField(label, key, value, options) {
      return '<label>' + label + '<select data-key="' + key + '">' + options.map((option) => '<option value="' + option + '"' + (String(value) === String(option) ? ' selected' : '') + '>' + option + '</option>').join('') + '</select></label>';
    }

    function byteOrderMode(point) {
      const byteOrder = point.byteOrder || 'big';
      const wordOrder = point.wordOrder || 'normal';
      if (byteOrder === 'little' && wordOrder === 'normal') return 'BADC';
      if (byteOrder === 'big' && wordOrder === 'swap') return 'CDAB';
      if (byteOrder === 'little' && wordOrder === 'swap') return 'DCBA';
      return 'ABCD';
    }

    function applyByteOrderMode(point, mode) {
      const map = {
        ABCD: { byteOrder: 'big', wordOrder: 'normal' },
        BADC: { byteOrder: 'little', wordOrder: 'normal' },
        CDAB: { byteOrder: 'big', wordOrder: 'swap' },
        DCBA: { byteOrder: 'little', wordOrder: 'swap' }
      };
      Object.assign(point, map[mode] || map.ABCD);
    }

    function escapeAttr(value) {
      return escapeHtml(value).replace(/"/g, '&quot;');
    }

    function readConfigForm() {
      const config = currentConfig || {};
      config.gatewayKey = document.getElementById('cfg-gateway-key').value.trim();
      config.collectIntervalSeconds = Number(document.getElementById('cfg-collect-seconds').value || 5);
      config.cacheFile = document.getElementById('cfg-cache-file').value.trim();
      config.mqtt = {
        enabled: document.getElementById('cfg-mqtt-enabled').value === 'true',
        broker: document.getElementById('cfg-mqtt-broker').value.trim(),
        clientId: document.getElementById('cfg-mqtt-client-id').value.trim(),
        username: document.getElementById('cfg-mqtt-username').value.trim(),
        password: document.getElementById('cfg-mqtt-password').value
      };
      config.web = { ...(config.web || {}), enabled: true, listen: document.getElementById('cfg-web-listen').value.trim() };
      config.serialPorts = config.serialPorts || [];
      config.networkPorts = config.networkPorts || [];
      config.devices = config.devices || [];
      config.points = flattenDevices(config.devices);
      return config;
    }

    function readSerialPorts() {
      return Array.from(document.querySelectorAll('[data-serial-index]')).map((row) => {
        const item = {};
        row.querySelectorAll('[data-key]').forEach((input) => {
          const key = input.dataset.key;
          const value = input.value;
          if (['baudRate', 'dataBits', 'stopBits'].includes(key)) item[key] = Number(value || 0);
          else if (key === 'enabled') item[key] = value === 'true';
          else item[key] = value.trim();
        });
        return item;
      });
    }

    function readNetworkPorts() {
      return Array.from(document.querySelectorAll('[data-network-index]')).map((row) => {
        const item = {};
        row.querySelectorAll('[data-key]').forEach((input) => {
          const key = input.dataset.key;
          const value = input.value;
          if (key === 'enabled') item[key] = value === 'true';
          else item[key] = value.trim();
        });
        return item;
      });
    }

    function syncActiveInterfaceForm() {
      const row = document.querySelector('.interface-detail-form');
      if (!row || !currentConfig) return;
      const type = row.dataset.interfaceType;
      const index = Number(row.dataset.interfaceIndex || 0);
      const list = getInterfaceList(type);
      const item = list[index] || {};
      const oldName = getInterfaceName(type, index);
      row.querySelectorAll('[data-key]').forEach((input) => {
        const key = input.dataset.key;
        const value = input.value;
        if (['baudRate', 'dataBits', 'stopBits'].includes(key)) item[key] = Number(value || 0);
        else if (key === 'enabled') item[key] = value === 'true';
        else item[key] = value.trim();
      });
      if (type === 'serial') currentConfig.serialPorts[index] = item;
      else currentConfig.networkPorts[index] = item;
      const newName = getInterfaceName(type, index);
      (currentConfig.devices || []).forEach((device) => {
        if (device.interfaceType === type && device.interfaceName === oldName) device.interfaceName = newName;
      });
    }

    function syncActiveDeviceForm() {
      if (!currentConfig?.devices?.length) return;
      const deviceRow = document.querySelector('.device-detail-form');
      if (!deviceRow) return;
      const device = currentConfig.devices[activeDeviceIndex] || {};
      device.interfaceType = activeInterfaceType;
      device.interfaceName = getInterfaceName(activeInterfaceType, activeInterfaceIndex);
      deviceRow.querySelector('.device-head').querySelectorAll('[data-key]').forEach((input) => {
        const key = input.dataset.key;
        const value = input.value;
        if (['slaveId', 'rack', 'slot'].includes(key)) device[key] = Number(value || 0);
        else device[key] = value.trim();
      });
      device.points = Array.from(deviceRow.querySelectorAll('.point-form')).map((row) => {
        const point = { ...((device.points || [])[Number(row.dataset.pointIndex || 0)] || {}) };
        row.querySelectorAll('[data-key]').forEach((input) => {
          const key = input.dataset.key;
          const value = input.value;
          if (['function', 'register', 'quantity', 'dbNumber', 'rack', 'slot', 'slaveId'].includes(key)) point[key] = Number(value || 0);
          else if (['bitIndex'].includes(key)) {
            if (value !== '') point[key] = Number(value);
          } else if (['scale', 'offset'].includes(key)) point[key] = Number(value || 0);
          else if (key === 'byteOrderMode') applyByteOrderMode(point, value);
          else point[key] = value.trim();
        });
        if (device.protocol === 'siemens-s7') {
          point.protocol = 'siemens-s7';
          point.address = device.address || point.address || '';
          point.rack = Number(device.rack || 0);
          point.slot = Number(device.slot || 0);
          point.localTsap = device.localTsap || '';
          point.remoteTsap = device.remoteTsap || '';
          delete point.function;
          delete point.byteOrderMode;
        } else if (device.protocol === 'iec104') {
          point.protocol = 'iec104';
          point.address = device.address || point.address || '';
          point.slaveId = Number(device.slaveId || 1);
          point.quantity = 1;
          delete point.function;
          delete point.area;
          delete point.dbNumber;
          delete point.rack;
          delete point.slot;
          delete point.byteOrder;
          delete point.wordOrder;
          delete point.byteOrderMode;
          delete point.bitIndex;
        }
        return point;
      });
      currentConfig.devices[activeDeviceIndex] = device;
    }

    function normalizeDevices(config) {
      if (config.devices && config.devices.length) return config.devices;
      const groups = new Map();
      (config.points || []).forEach((point) => {
        const key = [point.deviceKey || '', point.protocol || 'modbus-tcp', point.address || '', point.slaveId || 1].join('|');
        if (!groups.has(key)) {
          groups.set(key, {
            deviceKey: point.deviceKey || '',
            name: '',
            protocol: point.protocol || 'modbus-tcp',
            address: point.address || '',
            slaveId: point.slaveId || 1,
            points: []
          });
        }
        const childPoint = { ...point };
        delete childPoint.deviceKey;
        delete childPoint.protocol;
        delete childPoint.address;
        delete childPoint.slaveId;
        groups.get(key).points.push(childPoint);
      });
      return Array.from(groups.values());
    }

    function flattenDevices(devices) {
      return (devices || []).flatMap((device) => (device.points || []).map((point) => ({
        ...point,
        deviceKey: device.deviceKey,
        protocol: device.protocol,
        address: device.address,
        slaveId: point.slaveId || device.slaveId
      })));
    }

    function addDevice() {
      syncConfigFromActiveEditor();
      currentConfig.devices = currentConfig.devices || [];
      const interfaceItem = getInterfaceList(activeInterfaceType)[activeInterfaceIndex] || {};
      const protocol = activeInterfaceType === 'serial' ? 'modbus-rtu' : 'modbus-tcp';
      currentConfig.devices.push({
        deviceKey: 'device-' + String(currentConfig.devices.length + 1).padStart(3, '0'),
        name: '',
        interfaceType: activeInterfaceType,
        interfaceName: getInterfaceName(activeInterfaceType, activeInterfaceIndex),
        protocol,
        address: activeInterfaceType === 'serial' ? (interfaceItem.port || 'COM1') : (interfaceItem.address || '192.168.1.50:502'),
        slaveId: 1,
        points: []
      });
      activeDeviceIndex = currentConfig.devices.length - 1;
      currentConfig.points = flattenDevices(currentConfig.devices);
      renderConfigForm();
    }

    function addSerialPort() {
      syncConfigFromActiveEditor();
      currentConfig.serialPorts = currentConfig.serialPorts || [];
      currentConfig.serialPorts.push({
        name: 'Serial' + (currentConfig.serialPorts.length + 1),
        port: 'COM' + (currentConfig.serialPorts.length + 1),
        baudRate: 9600,
        dataBits: 8,
        stopBits: 1,
        parity: 'none',
        enabled: true
      });
      renderConfigForm();
    }

    function removeSerialPort(index) {
      syncConfigFromActiveEditor();
      currentConfig.serialPorts.splice(index, 1);
      renderConfigForm();
    }

    function addNetworkPort() {
      syncConfigFromActiveEditor();
      currentConfig.networkPorts = currentConfig.networkPorts || [];
      currentConfig.networkPorts.push({
        name: 'net' + (currentConfig.networkPorts.length + 1),
        address: '192.168.1.50:502',
        mode: 'tcp-client',
        enabled: true
      });
      renderConfigForm();
    }

    function removeNetworkPort(index) {
      syncConfigFromActiveEditor();
      currentConfig.networkPorts.splice(index, 1);
      renderConfigForm();
    }

    function removeDevice(index) {
      syncConfigFromActiveEditor();
      currentConfig.devices.splice(index, 1);
      activeDeviceIndex = Math.min(index, Math.max(currentConfig.devices.length - 1, 0));
      currentConfig.points = flattenDevices(currentConfig.devices);
      renderConfigForm();
    }

    function addPoint(deviceIndex) {
      syncConfigFromActiveEditor();
      const device = currentConfig.devices[deviceIndex];
      device.points = device.points || [];
      if (device.protocol === 'iec104') {
        device.points.push({
          name: '',
          metric: '',
          register: 1,
          quantity: 1,
          dataType: 'float32',
          slaveId: device.slaveId || 1,
          scale: 1,
          offset: 0
        });
        currentConfig.points = flattenDevices(currentConfig.devices);
        renderConfigForm();
        return;
      }
      device.points.push({
        name: '',
        metric: '',
        function: 3,
        register: 0,
        quantity: 1,
        dataType: 'uint16',
        area: 'DB',
        dbNumber: 1,
        rack: 0,
        slot: 0,
        byteOrder: 'big',
        wordOrder: 'normal',
        scale: 1,
        offset: 0
      });
      currentConfig.points = flattenDevices(currentConfig.devices);
      renderConfigForm();
    }

    async function previewPdfPoints() {
      syncConfigFromActiveEditor();
      const target = document.getElementById('pdf-point-preview');
      const input = document.getElementById('pdf-point-file');
      const device = currentConfig?.devices?.[activeDeviceIndex];
      if (!device) {
        target.textContent = '请先选择一个设备。';
        return;
      }
      if (!input?.files?.length) {
        target.textContent = '请先选择 PDF 文件。';
        return;
      }
      target.textContent = '正在解析 PDF...';
      const form = new FormData();
      form.append('file', input.files[0]);
      form.append('deviceKey', device.deviceKey || '');
      form.append('protocol', device.protocol || 'modbus-tcp');
      form.append('address', device.address || '');
      form.append('slaveId', String(device.slaveId || 1));
      try {
        const response = await fetch('/api/pdf-points/preview', { method: 'POST', body: form });
        if (!response.ok) throw new Error(await response.text());
        const data = await response.json();
        pdfPointPreview = data.points || [];
        renderPdfPointPreview(data);
      } catch (error) {
        pdfPointPreview = [];
        target.innerHTML = '<span class="bad-text">解析失败：' + escapeHtml(error.message) + '</span>';
      }
    }

    function renderPdfPointPreview(data) {
      const target = document.getElementById('pdf-point-preview');
      const warnings = (data.warnings || []).map((item) => '<div class="warning">' + escapeHtml(item) + '</div>').join('');
      if (!pdfPointPreview.length) {
        target.innerHTML = warnings + '<div class="muted">未识别到可导入点位。可尝试使用包含可复制文字表格的 PDF。</div>';
        return;
      }
      target.innerHTML = warnings +
        '<div class="muted">已识别 ' + pdfPointPreview.length + ' 个点位，确认无误后点击“应用到当前设备”。</div>' +
        '<table><thead><tr><th>名称</th><th>标识符</th><th>功能码</th><th>寄存器</th><th>数量</th><th>类型</th><th>倍率</th><th>偏移</th></tr></thead><tbody>' +
        pdfPointPreview.slice(0, 50).map((point) =>
          '<tr>' +
            '<td>' + escapeHtml(point.name || '-') + '</td>' +
            '<td><code>' + escapeHtml(point.metric || '-') + '</code></td>' +
            '<td>' + escapeHtml(point.function || '-') + '</td>' +
            '<td>' + escapeHtml(point.register ?? '-') + '</td>' +
            '<td>' + escapeHtml(point.quantity || '-') + '</td>' +
            '<td>' + escapeHtml(point.dataType || '-') + '</td>' +
            '<td>' + escapeHtml(point.scale ?? 1) + '</td>' +
            '<td>' + escapeHtml(point.offset ?? 0) + '</td>' +
          '</tr>'
        ).join('') +
        '</tbody></table>' +
        (pdfPointPreview.length > 50 ? '<div class="muted">仅预览前 50 个点位。</div>' : '');
    }

    function applyPdfPoints() {
      syncConfigFromActiveEditor();
      const target = document.getElementById('pdf-point-preview');
      const device = currentConfig?.devices?.[activeDeviceIndex];
      if (!device) {
        target.textContent = '请先选择一个设备。';
        return;
      }
      if (!pdfPointPreview.length) {
        target.textContent = '没有可应用的点位，请先解析预览。';
        return;
      }
      const existing = new Set((device.points || []).map((point) => String(point.metric || '')));
      const startIndex = (device.points || []).length;
      const imported = pdfPointPreview
        .filter((point) => point.metric && !existing.has(String(point.metric)))
        .map((point, index) => {
          const next = { ...point };
          next.name = pdfPointName(device, point, startIndex + index + 1);
          delete next.deviceKey;
          delete next.protocol;
          delete next.address;
          delete next.slaveId;
          return next;
        });
      device.points = [...(device.points || []), ...imported];
      currentConfig.points = flattenDevices(currentConfig.devices);
      const count = imported.length;
      pdfPointPreview = [];
      renderConfigForm();
      const message = document.getElementById('config-message');
      message.className = 'toast ok';
      message.textContent = '已追加 ' + count + ' 个 PDF 点位，请检查后点击“保存配置”。';
    }

    function pdfPointName(device, point, sequence) {
      const interfaceName = device.interfaceName || getInterfaceName(activeInterfaceType, activeInterfaceIndex) || (activeInterfaceType === 'serial' ? 'Serial1' : 'net1');
      const deviceName = device.name || device.deviceKey || '设备';
      const functionCode = point.function || 3;
      return interfaceName + '_' + deviceName + '@F' + functionCode + '_P' + sequence;
    }

    async function previewPdfPoints() {
      syncConfigFromActiveEditor();
      const target = document.getElementById('pdf-point-preview');
      const input = document.getElementById('pdf-point-file');
      const device = currentConfig?.devices?.[activeDeviceIndex];
      if (!device) {
        target.textContent = '请先选择一个设备。';
        return;
      }
      if (!input?.files?.length) {
        target.textContent = '请先选择 PDF 文件。';
        return;
      }
      target.textContent = '正在解析 PDF...';
      const form = new FormData();
      form.append('file', input.files[0]);
      form.append('deviceKey', device.deviceKey || '');
      form.append('protocol', device.protocol || 'modbus-tcp');
      form.append('address', device.address || '');
      form.append('slaveId', String(device.slaveId || 1));
      try {
        const response = await fetch('/api/pdf-points/preview', { method: 'POST', body: form });
        if (!response.ok) throw new Error(await response.text());
        const data = await response.json();
        pdfPointPreview = data.points || [];
        normalizePdfPreviewNames(device);
        renderPdfPointPreview(data);
      } catch (error) {
        pdfPointPreview = [];
        target.innerHTML = '<span class="bad-text">解析失败：' + escapeHtml(error.message) + '</span>';
      }
    }

    function renderPdfPointPreview(data) {
      const target = document.getElementById('pdf-point-preview');
      const warnings = (data.warnings || []).map((item) => '<div class="warning">' + escapeHtml(item) + '</div>').join('');
      if (!pdfPointPreview.length) {
        target.innerHTML = warnings + '<div class="muted">未识别到可导入点位。可尝试使用包含可复制文字表格的 PDF。</div>';
        return;
      }
      const selectedCount = pdfPointPreview.filter((point) => point.selected !== false).length;
      target.innerHTML = warnings +
        '<div class="muted">识别表格：共 ' + pdfPointPreview.length + ' 个点位，已勾选 ' + selectedCount + ' 个。低置信度行会标黄，导入前可以逐行修正。</div>' +
        '<div class="pdf-batch">' +
          '<strong>批量设置默认值</strong>' +
          '<label>功能码<select id="pdf-batch-function"><option value="">不修改</option><option value="1">1</option><option value="2">2</option><option value="3">3</option><option value="4">4</option></select></label>' +
          '<label>数据类型<select id="pdf-batch-data-type"><option value="">不修改</option>' + dataTypeOptions().map((item) => '<option value="' + item + '">' + item + '</option>').join('') + '</select></label>' +
          '<label>倍率<input id="pdf-batch-scale" type="number" step="any" placeholder="不修改" /></label>' +
          '<label>偏移<input id="pdf-batch-offset" type="number" step="any" placeholder="不修改" /></label>' +
          '<label style="align-self:end;"><span>地址减 1</span><input id="pdf-batch-address-minus-one" type="checkbox" /></label>' +
          '<button onclick="applyPdfBatchDefaults()">应用到已勾选</button>' +
        '</div>' +
        '<table><thead><tr><th><input type="checkbox" checked onchange="toggleAllPdfPoints(this.checked)" /></th><th>名称</th><th>标识符</th><th>功能码</th><th>寄存器</th><th>数据类型</th><th>倍率</th><th>偏移</th><th>置信度</th><th>来源行</th></tr></thead><tbody>' +
        pdfPointPreview.map((point, index) =>
          '<tr class="' + ((point.confidence || 0) < 0.65 ? 'pdf-low-confidence' : '') + '">' +
            '<td><input type="checkbox" ' + (point.selected === false ? '' : 'checked') + ' onchange="togglePdfPoint(' + index + ', this.checked)" /></td>' +
            '<td>' + pdfInput(index, 'name', point.name || '', 'text', 'pdf-name-input') + '</td>' +
            '<td>' + pdfInput(index, 'metric', point.metric || '', 'text', 'pdf-metric-input') + '</td>' +
            '<td>' + pdfSelect(index, 'function', point.function || 3, ['1', '2', '3', '4']) + '</td>' +
            '<td>' + pdfInput(index, 'register', point.register ?? 0, 'number') + '</td>' +
            '<td>' + pdfSelect(index, 'dataType', point.dataType || 'uint16', dataTypeOptions()) + '</td>' +
            '<td>' + pdfInput(index, 'scale', point.scale ?? 1, 'number') + '</td>' +
            '<td>' + pdfInput(index, 'offset', point.offset ?? 0, 'number') + '</td>' +
            '<td>' + pdfConfidenceLabel(point) + '</td>' +
            '<td class="pdf-source" title="' + escapeAttr(point.sourceLine || '') + '">' + escapeHtml(point.sourceLine || '-') + '</td>' +
          '</tr>'
        ).join('') +
        '</tbody></table>';
    }

    function dataTypeOptions() {
      return ['bool', 'uint16', 'int16', 'uint32', 'int32', 'float32'];
    }

    function pdfInput(index, key, value, type, className) {
      const step = type === 'number' ? ' step="any"' : '';
      return '<input class="' + (className || '') + '" type="' + type + '"' + step + ' value="' + escapeAttr(value) + '" oninput="updatePdfPoint(' + index + ', \'' + key + '\', this.value, \'' + type + '\')" />';
    }

    function pdfSelect(index, key, value, options) {
      return '<select onchange="updatePdfPoint(' + index + ', \'' + key + '\', this.value, \'numberOrText\')">' +
        options.map((option) => '<option value="' + option + '"' + (String(value) === String(option) ? ' selected' : '') + '>' + option + '</option>').join('') +
        '</select>';
    }

    function updatePdfPoint(index, key, value, type) {
      const point = pdfPointPreview[index];
      if (!point) return;
      const numericKeys = ['function', 'register', 'scale', 'offset'];
      if (type === 'number' || numericKeys.includes(key)) point[key] = Number(value || 0);
      else point[key] = value.trim();
    }

    function togglePdfPoint(index, checked) {
      if (pdfPointPreview[index]) pdfPointPreview[index].selected = checked;
      renderPdfPointPreview({ warnings: [] });
    }

    function toggleAllPdfPoints(checked) {
      pdfPointPreview.forEach((point) => { point.selected = checked; });
      renderPdfPointPreview({ warnings: [] });
    }

    function applyPdfBatchDefaults() {
      const functionValue = document.getElementById('pdf-batch-function')?.value || '';
      const dataTypeValue = document.getElementById('pdf-batch-data-type')?.value || '';
      const scaleValue = document.getElementById('pdf-batch-scale')?.value || '';
      const offsetValue = document.getElementById('pdf-batch-offset')?.value || '';
      const minusOne = document.getElementById('pdf-batch-address-minus-one')?.checked;
      pdfPointPreview.forEach((point) => {
        if (point.selected === false) return;
        if (functionValue) point.function = Number(functionValue);
        if (dataTypeValue) point.dataType = dataTypeValue;
        if (scaleValue !== '') point.scale = Number(scaleValue);
        if (offsetValue !== '') point.offset = Number(offsetValue);
        if (minusOne) point.register = Math.max(0, Number(point.register || 0) - 1);
      });
      renderPdfPointPreview({ warnings: [] });
    }

    function pdfConfidenceLabel(point) {
      const confidence = Math.round(Number(point.confidence || 0) * 100);
      const className = confidence < 65 ? 'bad-text' : 'ok-text';
      return '<span class="' + className + '">' + confidence + '%</span>';
    }

    function normalizePdfPreviewNames(device) {
      const startIndex = (device.points || []).length;
      pdfPointPreview.forEach((point, index) => {
        point.selected = point.selected !== false;
        if (!point.name || /^鐐逛綅|^点位/.test(point.name)) {
          point.name = pdfPointName(device, point, startIndex + index + 1);
        }
      });
    }

    function applyPdfPoints() {
      syncConfigFromActiveEditor();
      const target = document.getElementById('pdf-point-preview');
      const device = currentConfig?.devices?.[activeDeviceIndex];
      if (!device) {
        target.textContent = '请先选择一个设备。';
        return;
      }
      if (!pdfPointPreview.length) {
        target.textContent = '没有可导入的点位，请先解析预览。';
        return;
      }
      const selected = pdfPointPreview.filter((point) => point.selected !== false);
      const existing = new Set((device.points || []).map((point) => String(point.metric || '')));
      const startIndex = (device.points || []).length;
      const imported = selected
        .filter((point) => point.metric && !existing.has(String(point.metric)))
        .map((point, index) => {
          const next = { ...point };
          if (!next.name) next.name = pdfPointName(device, point, startIndex + index + 1);
          next.quantity = dataTypeQuantity(next.dataType);
          delete next.deviceKey;
          delete next.protocol;
          delete next.address;
          delete next.slaveId;
          delete next.selected;
          delete next.confidence;
          delete next.sourceLine;
          return next;
        });
      device.points = [...(device.points || []), ...imported];
      currentConfig.points = flattenDevices(currentConfig.devices);
      pdfPointPreview = [];
      renderConfigForm();
      const message = document.getElementById('config-message');
      message.className = 'toast ok';
      message.textContent = '已导入 ' + imported.length + ' 个 PDF 点位，跳过 ' + (selected.length - imported.length) + ' 个未勾选或重复标识符点位，请检查后点击“保存配置”。';
    }

    function dataTypeQuantity(dataType) {
      if (['uint32', 'int32', 'float32'].includes(dataType)) return 2;
      return 1;
    }

    function s7RequestBody(device) {
      return {
        deviceKey: device.deviceKey || '',
        deviceName: device.name || device.deviceKey || 'PLC',
        address: device.address || '',
        area: document.getElementById('s7-scan-area')?.value || 'AUTO',
        dbNumber: Number(document.getElementById('s7-scan-db')?.value || 1),
        rack: Number(document.getElementById('s7-scan-rack')?.value || device.rack || 0),
        slot: Number(document.getElementById('s7-scan-slot')?.value || device.slot || 0),
        localTsap: document.getElementById('s7-scan-local-tsap')?.value || device.localTsap || '',
        remoteTsap: document.getElementById('s7-scan-remote-tsap')?.value || device.remoteTsap || '',
        start: Number(document.getElementById('s7-scan-start')?.value || 0),
        end: Number(document.getElementById('s7-scan-end')?.value || 200),
        dataType: document.getElementById('s7-scan-type')?.value || 'auto'
      };
    }

    async function testDeviceConnection() {
      syncConfigFromActiveEditor();
      const target = document.getElementById('s7-point-preview') || document.getElementById('config-message');
      const device = currentConfig?.devices?.[activeDeviceIndex];
      if (!device) {
        target.textContent = '请先选择一个设备。';
        return;
      }
      target.textContent = '正在测试连接...';
      try {
        const response = await fetch('/api/connection/test', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            protocol: device.protocol || 'modbus-tcp',
            address: device.address || '',
            rack: Number(device.rack || 0),
            slot: Number(device.slot || 0),
            localTsap: device.localTsap || '',
            remoteTsap: device.remoteTsap || '',
            slaveId: Number(device.slaveId || 1)
          })
        });
        const data = await response.json();
        if (!response.ok || !data.ok) throw new Error(data.message || '连接失败');
        target.innerHTML = '<div class="toast ok">' + escapeHtml(data.message) + '</div>' +
          '<div class="muted">地址：' + escapeHtml(data.address || '-') + '</div>';
      } catch (error) {
        target.innerHTML = '<span class="bad-text">连接失败：' + escapeHtml(error.message) + '</span>';
      }
    }

    async function scanS7Points() {
      syncConfigFromActiveEditor();
      const target = document.getElementById('s7-point-preview');
      const device = currentConfig?.devices?.[activeDeviceIndex];
      if (!device || device.protocol !== 'siemens-s7') {
        target.textContent = '请先选择 Siemens S7 协议设备。';
        return;
      }
      const body = s7RequestBody(device);
      target.textContent = '正在连接 PLC 并扫描点位...';
      try {
        const response = await fetch('/api/s7-points/scan', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body)
        });
        if (!response.ok) throw new Error(await response.text());
        const data = await response.json();
        s7PointPreview = data.points || [];
        renderS7PointPreview(data);
      } catch (error) {
        s7PointPreview = [];
        target.innerHTML = '<span class="bad-text">扫描失败：' + escapeHtml(error.message) + '</span>';
      }
    }

    function renderS7PointPreview(data) {
      const target = document.getElementById('s7-point-preview');
      const warnings = (data.warnings || []).map((item) => '<div class="warning">' + escapeHtml(item) + '</div>').join('');
      if (!s7PointPreview.length) {
        target.innerHTML = warnings + '<div class="muted">没有扫描到可导入点位。可以缩小范围，或检查 PLC 的 PUT/GET 访问权限。</div>';
        return;
      }
      const selectedCount = s7PointPreview.filter((point) => point.selected !== false).length;
      target.innerHTML = warnings +
        '<div class="muted">扫描到 ' + s7PointPreview.length + ' 个候选点位，已勾选 ' + selectedCount + ' 个。导入前可以修改名称、标识符和类型。</div>' +
        '<table><thead><tr><th><input type="checkbox" checked onchange="toggleAllS7Points(this.checked)" /></th><th>名称</th><th>标识符</th><th>区域</th><th>DB</th><th>Rack</th><th>Slot</th><th>偏移</th><th>类型</th><th>当前值</th></tr></thead><tbody>' +
        s7PointPreview.map((point, index) =>
          '<tr>' +
            '<td><input type="checkbox" ' + (point.selected === false ? '' : 'checked') + ' onchange="toggleS7Point(' + index + ', this.checked)" /></td>' +
            '<td>' + s7Input(index, 'name', point.name || '', 'text', 'pdf-name-input') + '</td>' +
            '<td>' + s7Input(index, 'metric', point.metric || '', 'text', 'pdf-metric-input') + '</td>' +
            '<td>' + s7Select(index, 'area', point.area || 'V', ['V', 'DB', 'M', 'I', 'Q']) + '</td>' +
            '<td>' + s7Input(index, 'dbNumber', point.dbNumber || 1, 'number') + '</td>' +
            '<td>' + s7Input(index, 'rack', point.rack || 0, 'number') + '</td>' +
            '<td>' + s7Input(index, 'slot', point.slot || 0, 'number') + '</td>' +
            '<td>' + s7Input(index, 'register', point.register || 0, 'number') + '</td>' +
            '<td>' + s7Select(index, 'dataType', point.dataType || 'uint16', dataTypeOptions()) + '</td>' +
            '<td><code>' + escapeHtml(point.value ?? '-') + '</code></td>' +
          '</tr>'
        ).join('') +
        '</tbody></table>';
    }

    function s7Input(index, key, value, type, className) {
      const step = type === 'number' ? ' step="any"' : '';
      return '<input class="' + (className || '') + '" type="' + type + '"' + step + ' value="' + escapeAttr(value) + '" oninput="updateS7Point(' + index + ', \'' + key + '\', this.value)" />';
    }

    function s7Select(index, key, value, options) {
      return '<select onchange="updateS7Point(' + index + ', \'' + key + '\', this.value)">' +
        options.map((option) => '<option value="' + option + '"' + (String(value) === String(option) ? ' selected' : '') + '>' + option + '</option>').join('') +
        '</select>';
    }

    function updateS7Point(index, key, value) {
      const point = s7PointPreview[index];
      if (!point) return;
      if (['dbNumber', 'rack', 'slot', 'register'].includes(key)) point[key] = Number(value || 0);
      else point[key] = value.trim();
      if (key === 'dataType') point.quantity = dataTypeQuantity(point.dataType);
    }

    function toggleS7Point(index, checked) {
      if (s7PointPreview[index]) s7PointPreview[index].selected = checked;
      renderS7PointPreview({ warnings: [] });
    }

    function toggleAllS7Points(checked) {
      s7PointPreview.forEach((point) => { point.selected = checked; });
      renderS7PointPreview({ warnings: [] });
    }

    function importS7Points() {
      syncConfigFromActiveEditor();
      const target = document.getElementById('s7-point-preview');
      const device = currentConfig?.devices?.[activeDeviceIndex];
      if (!device) {
        target.textContent = '请先选择一个设备。';
        return;
      }
      const selected = s7PointPreview.filter((point) => point.selected !== false);
      if (!selected.length) {
        target.textContent = '请先扫描并勾选需要导入的点位。';
        return;
      }
      const existing = new Set((device.points || []).map((point) => String(point.metric || '')));
      const imported = selected
        .filter((point) => point.metric && !existing.has(String(point.metric)))
        .map((point) => {
          const next = { ...point };
          next.protocol = 'siemens-s7';
          next.address = device.address || next.address || '';
          next.quantity = dataTypeQuantity(next.dataType);
          next.byteOrder = 'big';
          next.wordOrder = 'normal';
          next.scale = next.scale || 1;
          next.offset = next.offset || 0;
          delete next.deviceKey;
          delete next.selected;
          delete next.value;
          return next;
        });
      device.points = [...(device.points || []), ...imported];
      currentConfig.points = flattenDevices(currentConfig.devices);
      s7PointPreview = [];
      renderConfigForm();
      const message = document.getElementById('config-message');
      message.className = 'toast ok';
      message.textContent = '已导入 ' + imported.length + ' 个 S7 点位，跳过 ' + (selected.length - imported.length) + ' 个重复标识符点位，请检查后点击“保存配置”。';
    }

    function copyPoint(deviceIndex, pointIndex) {
      syncConfigFromActiveEditor();
      const points = currentConfig.devices[deviceIndex].points;
      const source = points[pointIndex];
      points.splice(pointIndex + 1, 0, JSON.parse(JSON.stringify(source)));
      currentConfig.points = flattenDevices(currentConfig.devices);
      renderConfigForm();
    }

    function removePoint(deviceIndex, pointIndex) {
      syncConfigFromActiveEditor();
      currentConfig.devices[deviceIndex].points.splice(pointIndex, 1);
      currentConfig.points = flattenDevices(currentConfig.devices);
      renderConfigForm();
    }

    loadStatus();
    loadConfig();

    function scheduleStatusRefresh(collectSeconds) {
      const interval = Math.min(Math.max(Number(collectSeconds || 3) * 1000, 1000), 3000);
      if (statusTimer && statusTimer.interval === interval) return;
      if (statusTimer) clearInterval(statusTimer.id);
      statusTimer = { interval, id: setInterval(loadStatus, interval) };
    }
  </script>
</body>
</html>`
