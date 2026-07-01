const fmtMoney = new Intl.NumberFormat("zh-CN", {
  style: "currency",
  currency: "CNY",
  maximumFractionDigits: 2
});
const fmtNumber = new Intl.NumberFormat("zh-CN");
const curveColors = ["#4fb3ff", "#42d392", "#f4c95d", "#ff626e", "#b68cff", "#35d0ba"];

const state = {
  accountId: "",
  range: "20d",
  dashboard: null,
  activity: [],
  closed: []
};

document.addEventListener("DOMContentLoaded", () => loadAccount(""));
window.addEventListener("resize", () => drawEquityCurve(getEquitySeries()));

async function loadAccount(accountId) {
  try {
    const query = accountId ? `&accountId=${encodeURIComponent(accountId)}` : "";
    const dashboard = await fetchData(`/api/paper/dashboard?range=${state.range}${query}`);
    const selectedID = dashboard.selectedAccount?.id || "";
    const activityQuery = selectedID ? `&accountId=${encodeURIComponent(selectedID)}` : "";
    const [activity, closed] = await Promise.all([
      fetchData(`/api/paper/activity?limit=80${activityQuery}`),
      selectedID
        ? fetchData(`/api/paper/closed-positions?accountId=${encodeURIComponent(selectedID)}&range=60d`)
        : Promise.resolve({ items: [] })
    ]);

    state.accountId = selectedID;
    state.dashboard = dashboard;
    state.activity = activity.items || [];
    state.closed = closed.items || [];
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
  renderActivity(state.activity, accounts);
  renderCurveRange();
  drawEquityCurve(getEquitySeries());
}

function renderCurveRange() {
  const root = document.getElementById("curveRange");
  root.className = "range-switch";
  const ranges = [
    ["20d", "20日"],
    ["60d", "60日"],
    ["120d", "120日"],
    ["all", "全部"]
  ];
  root.innerHTML = ranges.map(([value, label]) => `
    <button class="${state.range === value ? "active" : ""}" data-range="${value}">
      ${label}
    </button>
  `).join("");
  root.querySelectorAll("button").forEach((button) => {
    button.addEventListener("click", () => {
      state.range = button.dataset.range;
      loadAccount(state.accountId);
    });
  });
}

function renderAccounts(accounts, selectedAccount) {
  const root = document.getElementById("accountCards");
  root.innerHTML = accounts.map((account) => {
    const isSelected = account.id === selectedAccount?.id;
    const totalCash = account.availableCash + account.frozenCash;
    return `
      <article class="account-card ${isSelected ? "selected" : ""}">
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
  const accounts = dashboard.accounts || [];
  const positions = dashboard.positions || [];
  const orders = dashboard.orders || [];
  const trades = dashboard.trades || [];
  const status = dashboard.marketStatus || {};
  const selectedID = account?.id || "";
  const accountOptions = accounts.map((item) => `
    <option value="${escapeHtml(item.id)}" ${item.id === selectedID ? "selected" : ""}>
      ${escapeHtml(item.name)}
    </option>
  `).join("");
  const values = [
    ["市场状态", status.note || status.status || "未知", "warn"],
    ["持仓标的", `${positions.length} 个`, ""],
    ["待处理委托", `${orders.filter((item) => item.status === "pending").length} 笔`, ""],
    ["成交记录", `${trades.length} 笔`, ""]
  ];

  document.getElementById("marketOverview").innerHTML = `
    <div class="metric account-filter">
      <span class="muted">账户</span>
      <select id="accountFilter" aria-label="切换账户">
        ${accountOptions || "<option>未选择</option>"}
      </select>
    </div>
    ${values.map(([label, value, tone]) => `
      <div class="metric">
        <span class="muted">${label}</span>
        <strong class="${tone}">${escapeHtml(value)}</strong>
      </div>
    `).join("")}
  `;
  const select = document.getElementById("accountFilter");
  select.disabled = accounts.length === 0;
  select.addEventListener("change", (event) => loadAccount(event.target.value));
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

function renderActivity(items, accounts) {
  const accountNames = Object.fromEntries(accounts.map((item) => [item.id, item.name]));
  text("activityCount", items.length);
  document.getElementById("activityTimeline").innerHTML = items.length
    ? items.slice(0, 12).map((item) => `
      <li>
        <strong>${escapeHtml(describeActivity(item))}</strong>
        <span class="muted">${formatTime(item.createdAt)} · ${escapeHtml(accountNames[item.accountId] || item.accountId || "--")}</span>
      </li>
    `).join("")
    : `<li class="empty-line">暂无 Agent 行为</li>`;
}

function getEquitySeries() {
  const dashboard = state.dashboard || {};
  if (Array.isArray(dashboard.equityCurves) && dashboard.equityCurves.length) {
    return dashboard.equityCurves.filter((item) => item.points?.length);
  }
  if (dashboard.equityCurve?.length) {
    return [{
      accountId: dashboard.selectedAccount?.id || "selected",
      accountName: dashboard.selectedAccount?.name || "当前账户",
      points: dashboard.equityCurve
    }];
  }
  return [];
}

function drawEquityCurve(series) {
  const canvas = document.getElementById("equityCanvas");
  const days = getCurveDays(series);
  const containerWidth = canvas.parentElement.clientWidth || canvas.getBoundingClientRect().width;
  const logicalWidth = days.length > 60
    ? Math.max(containerWidth, days.length * 14 + 48)
    : containerWidth;
  canvas.style.width = `${logicalWidth}px`;
  const scale = window.devicePixelRatio || 1;
  canvas.width = Math.max(1, Math.floor(logicalWidth * scale));
  canvas.height = Math.floor(300 * scale);

  const ctx = canvas.getContext("2d");
  ctx.scale(scale, scale);
  ctx.clearRect(0, 0, logicalWidth, 300);
  ctx.strokeStyle = "#243140";
  ctx.lineWidth = 1;

  const plot = {
    left: 18,
    right: 14,
    top: 28,
    bottom: 238,
    axisY: 258,
    labelY: 284
  };
  for (let i = 0; i < 5; i++) {
    const y = plot.top + i * ((plot.bottom - plot.top) / 4);
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(logicalWidth, y);
    ctx.stroke();
  }

  renderCurveLegend(series);
  if (!series.length) {
    ctx.fillStyle = "#8795a5";
    ctx.font = "13px Microsoft YaHei, sans-serif";
    ctx.textAlign = "center";
    ctx.fillText("暂无资产曲线", logicalWidth / 2, 150);
    return;
  }

  const values = series.flatMap((item) => dailyCurvePoints(item.points).map(
    (point) => point.totalAssets
  ));
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const width = logicalWidth - plot.left - plot.right;
  const height = plot.bottom - plot.top;

  drawCurveXAxis(ctx, days, plot, width);

  series.forEach((item, seriesIndex) => {
    const pointsByDay = Object.fromEntries(dailyCurvePoints(item.points).map((point) => [
      point.tradingDay,
      point
    ]));
    ctx.strokeStyle = curveColors[seriesIndex % curveColors.length];
    ctx.lineWidth = 2;
    ctx.beginPath();
    let started = false;
    days.forEach((day, index) => {
      const point = pointsByDay[day];
      if (!point) {
        return;
      }
      const x = plot.left + (width * index) / Math.max(1, days.length - 1);
      const y = plot.bottom - ((point.totalAssets - min) / span) * height;
      if (!started) {
        ctx.moveTo(x, y);
        started = true;
      } else {
        ctx.lineTo(x, y);
      }
    });
    ctx.stroke();
  });

  ctx.fillStyle = "#e7edf4";
  ctx.font = "12px Microsoft YaHei, sans-serif";
  ctx.textAlign = "left";
  ctx.fillText(`最高 ${fmtMoney.format(max)}`, 14, 20);
  ctx.fillText(`最低 ${fmtMoney.format(min)}`, 14, 250);
}

function getCurveDays(series) {
  return Array.from(new Set(
    series.flatMap((item) => dailyCurvePoints(item.points).map((point) => point.tradingDay))
  )).sort();
}

function dailyCurvePoints(points) {
  const byDay = new Map();
  for (const point of points || []) {
    if (point.tradingDay) {
      byDay.set(point.tradingDay, point);
    }
  }
  return Array.from(byDay.values()).sort((a, b) => a.tradingDay.localeCompare(b.tradingDay));
}

function drawCurveXAxis(ctx, days, plot, width) {
  ctx.strokeStyle = "#2b3a4a";
  ctx.fillStyle = "#8795a5";
  ctx.font = "11px Microsoft YaHei, sans-serif";
  ctx.textAlign = "center";
  const labelStep = Math.max(1, Math.ceil(days.length / Math.max(1, Math.floor(width / 72))));
  days.forEach((day, index) => {
    const x = plot.left + (width * index) / Math.max(1, days.length - 1);
    ctx.beginPath();
    ctx.moveTo(x, plot.axisY - 4);
    ctx.lineTo(x, plot.axisY + 4);
    ctx.stroke();
    if (index % labelStep === 0 || index === days.length - 1) {
      ctx.fillText(formatDayLabel(day), x, plot.labelY);
    }
  });
  ctx.beginPath();
  ctx.moveTo(plot.left, plot.axisY);
  ctx.lineTo(plot.left + width, plot.axisY);
  ctx.stroke();
}

function formatDayLabel(day) {
  const [, month, date] = String(day).split("-");
  return month && date ? `${month}/${date}` : day;
}

function renderCurveLegend(series) {
  const canvas = document.getElementById("equityCanvas");
  let legend = document.getElementById("equityLegend");
  if (!legend) {
    legend = document.createElement("div");
    legend.id = "equityLegend";
    legend.className = "curve-legend";
    canvas.insertAdjacentElement("afterend", legend);
  }
  legend.innerHTML = series.map((item, index) => `
    <span>
      <i style="background:${curveColors[index % curveColors.length]}"></i>
      ${escapeHtml(item.accountName || item.accountId)}
    </span>
  `).join("");
}

function describeActivity(item) {
  const request = parseJSON(item.request);
  const response = parseJSON(item.response);
  if (item.actionType === "create_account") {
    const positions = request.initialPositions || [];
    return `创建账户 ${request.name || ""}，初始资金 ${fmtMoney.format(request.initialCash || 0)}，初始持仓 ${positions.length} 只`;
  }
  if (item.actionType === "place_order") {
    const side = request.side === "sell" ? "卖出" : "买入";
    const name = request.name || request.code || response.name || response.code || "";
    const price = request.price ? `，委托价 ${fmtMoney.format(request.price)}` : "";
    return `提交委托：${side} ${name} ${fmtNumber.format(request.quantity || 0)} 股${price}`;
  }
  if (item.actionType === "fill_order") {
    const side = response.side === "sell" ? "卖出成交" : "买入成交";
    const name = response.name || response.code || request.name || request.code || "";
    return `${side}：${name} ${fmtNumber.format(response.quantity || 0)} 股，成交额 ${fmtMoney.format(response.amount || 0)}`;
  }
  if (item.actionType === "cancel_order") {
    return `撤销委托：${request.orderId || ""}`;
  }
  return item.actionType || "账户行为";
}

function parseJSON(value) {
  if (!value) {
    return {};
  }
  try {
    return JSON.parse(value);
  } catch {
    return {};
  }
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
