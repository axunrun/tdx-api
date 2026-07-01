const fmtMoney = new Intl.NumberFormat("zh-CN", {
  style: "currency",
  currency: "CNY",
  maximumFractionDigits: 2
});
const fmtNumber = new Intl.NumberFormat("zh-CN");

const state = {
  dashboard: null,
  activity: [],
  closed: []
};

document.addEventListener("DOMContentLoaded", init);
window.addEventListener("resize", () => drawEquityCurve(state.dashboard?.equityCurve || []));

async function init() {
  try {
    const [dashboard, activity] = await Promise.all([
      fetchData("/api/paper/dashboard?range=20d"),
      fetchData("/api/paper/activity?limit=80")
    ]);

    state.dashboard = dashboard;
    state.activity = activity.items || [];

    if (dashboard.selectedAccount?.id) {
      state.closed = (await fetchData(
        `/api/paper/closed-positions?accountId=${encodeURIComponent(
          dashboard.selectedAccount.id
        )}&range=60d`
      )).items || [];
    }

    render();
  } catch (error) {
    renderError(error);
  }
}

async function fetchData(url) {
  const response = await fetch(url);
  const body = await response.json();
  if (!response.ok || body.code !== 0) {
    throw new Error(body.message || `请求失败: ${url}`);
  }
  return body.data || {};
}

function render() {
  const dashboard = state.dashboard || {};
  const selectedAccount = dashboard.selectedAccount;
  const accounts = dashboard.accounts || [];

  text("marketStatus", dashboard.marketStatus?.status || "unknown");
  text("updatedAt", formatTime(dashboard.updatedAt));
  document.getElementById("emptyState").hidden = accounts.length > 0;

  renderAccounts(accounts, selectedAccount);
  renderMarketOverview(dashboard);
  renderPositions(dashboard.positions || []);
  renderOrdersTrades(dashboard.orders || [], dashboard.trades || []);
  renderClosedPositions(state.closed);
  renderActivity(state.activity);
  drawEquityCurve(dashboard.equityCurve || []);
}

function renderAccounts(accounts, selectedAccount) {
  const root = document.getElementById("accountCards");
  root.innerHTML = accounts.map((account) => {
    const isSelected = account.id === selectedAccount?.id;
    const totalCash = account.availableCash + account.frozenCash;
    return `
      <article class="account-card">
        <div class="name">
          <span>${escapeHtml(account.name)}</span>
          <span class="pill">${isSelected ? "当前" : account.status}</span>
        </div>
        <div class="cash">${fmtMoney.format(totalCash)}</div>
        <div class="subline">
          可用 ${fmtMoney.format(account.availableCash)}
          · 冻结 ${fmtMoney.format(account.frozenCash)}
        </div>
      </article>
    `;
  }).join("");
}

function renderMarketOverview(dashboard) {
  const account = dashboard.selectedAccount;
  const positions = dashboard.positions || [];
  const orders = dashboard.orders || [];
  const trades = dashboard.trades || [];
  const status = dashboard.marketStatus || {};
  const values = [
    ["账户", account ? account.name : "未选择", account ? "good" : "warn"],
    ["市场状态", status.note || status.status || "未知", "warn"],
    ["持仓标的", `${positions.length} 个`, ""],
    ["待处理委托", `${orders.filter((item) => item.status === "pending").length} 笔`, ""],
    ["成交记录", `${trades.length} 笔`, ""]
  ];

  document.getElementById("marketOverview").innerHTML = values.map(([label, value, tone]) => `
    <div class="metric">
      <span class="muted">${label}</span>
      <strong class="${tone}">${escapeHtml(value)}</strong>
    </div>
  `).join("");
}

function renderPositions(items) {
  text("positionCount", items.length);
  const body = document.getElementById("positionsBody");
  if (!items.length) {
    body.innerHTML = `<tr><td colspan="4" class="empty-line">暂无持仓</td></tr>`;
    return;
  }

  body.innerHTML = items.map((item) => `
    <tr>
      <td>${escapeHtml(item.name || item.code)}<div class="muted">${item.code}</div></td>
      <td>${fmtNumber.format(item.quantity)}</td>
      <td>${fmtNumber.format(item.sellableQuantity)}</td>
      <td>${fmtMoney.format(item.avgCost)}</td>
    </tr>
  `).join("");
}

function renderOrdersTrades(orders, trades) {
  const latest = [
    ...orders.map((item) => ({ type: "委托", at: item.updatedAt, item })),
    ...trades.map((item) => ({ type: "成交", at: item.tradedAt, item }))
  ].sort((a, b) => String(b.at).localeCompare(String(a.at))).slice(0, 8);

  document.getElementById("ordersTrades").innerHTML = latest.length
    ? latest.map(renderOrderTradeRow).join("")
    : `<div class="empty-line">暂无委托或成交</div>`;
}

function renderOrderTradeRow(entry) {
  const item = entry.item;
  const side = item.side === "buy" ? "买入" : "卖出";
  const tone = item.side === "buy" ? "good" : "bad";
  const amount = item.amount || item.price * item.quantity;
  return `
    <div class="row">
      <div>
        <strong>${entry.type} · ${escapeHtml(item.name || item.code)}</strong>
        <div class="meta">${formatTime(entry.at)} · ${item.code} · ${item.status || "filled"}</div>
      </div>
      <div class="${tone}">${side} ${fmtMoney.format(amount || 0)}</div>
    </div>
  `;
}

function renderClosedPositions(items) {
  document.getElementById("closedPositions").innerHTML = items.length
    ? items.slice(0, 8).map((item) => {
      const tone = item.realizedPnl >= 0 ? "good" : "bad";
      return `
        <div class="row">
          <div>
            <strong>${escapeHtml(item.name || item.code)}</strong>
            <div class="meta">${item.openedAt || "--"} → ${item.closedAt}</div>
          </div>
          <div class="${tone}">${fmtMoney.format(item.realizedPnl)}</div>
        </div>
      `;
    }).join("")
    : `<div class="empty-line">暂无清仓记录</div>`;
}

function renderActivity(items) {
  text("activityCount", items.length);
  document.getElementById("activityTimeline").innerHTML = items.length
    ? items.slice(0, 12).map((item) => `
      <li>
        <strong>${escapeHtml(item.actionType)}</strong>
        <span class="muted">${formatTime(item.createdAt)} · ${escapeHtml(item.accountId || "--")}</span>
      </li>
    `).join("")
    : `<li class="empty-line">暂无 Agent 行为</li>`;
}

function drawEquityCurve(points) {
  const canvas = document.getElementById("equityCanvas");
  const rect = canvas.getBoundingClientRect();
  const scale = window.devicePixelRatio || 1;
  canvas.width = Math.max(1, Math.floor(rect.width * scale));
  canvas.height = Math.floor(300 * scale);

  const ctx = canvas.getContext("2d");
  ctx.scale(scale, scale);
  ctx.clearRect(0, 0, rect.width, 300);
  ctx.strokeStyle = "#243140";
  ctx.lineWidth = 1;

  for (let i = 0; i < 5; i++) {
    const y = 24 + i * 56;
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(rect.width, y);
    ctx.stroke();
  }

  if (!points.length) {
    ctx.fillStyle = "#8795a5";
    ctx.font = "13px Microsoft YaHei, sans-serif";
    ctx.textAlign = "center";
    ctx.fillText("暂无资产曲线", rect.width / 2, 150);
    return;
  }

  const values = points.map((item) => item.totalAssets);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const width = rect.width - 28;
  const height = 230;

  ctx.strokeStyle = "#4fb3ff";
  ctx.lineWidth = 2;
  ctx.beginPath();
  points.forEach((point, index) => {
    const x = 14 + (width * index) / Math.max(1, points.length - 1);
    const y = 252 - ((point.totalAssets - min) / span) * height;
    if (index === 0) {
      ctx.moveTo(x, y);
    } else {
      ctx.lineTo(x, y);
    }
  });
  ctx.stroke();

  ctx.fillStyle = "#e7edf4";
  ctx.font = "12px Microsoft YaHei, sans-serif";
  ctx.textAlign = "left";
  ctx.fillText(`最高 ${fmtMoney.format(max)}`, 14, 20);
  ctx.fillText(`最低 ${fmtMoney.format(min)}`, 14, 292);
}

function renderError(error) {
  document.getElementById("emptyState").hidden = false;
  document.getElementById("emptyState").innerHTML = `
    <p class="eyebrow">LOAD FAILED</p>
    <h2>看板数据加载失败</h2>
    <p>${escapeHtml(error.message)}</p>
  `;
  drawEquityCurve([]);
}

function text(id, value) {
  document.getElementById(id).textContent = value;
}

function formatTime(value) {
  if (!value) {
    return "--";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

function escapeHtml(value) {
  return String(value ?? "").replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;"
  }[char]));
}
