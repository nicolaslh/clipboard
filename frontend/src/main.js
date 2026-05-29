import { Events } from "@wailsio/runtime";
import { ClipboardService } from "../bindings/github.com/nic/clipboard";

let items = [];
let searchTimeout = null;

// DOM elements
const listEl = document.getElementById("clipboard-list");
const emptyEl = document.getElementById("empty-state");
const searchInput = document.getElementById("search-input");
const statsEl = document.getElementById("stats");
const clearBtn = document.getElementById("btn-clear");

// Initialize
async function init() {
  await loadItems();
  await loadStats();
  setupEventListeners();
  listenForNewItems();
}

// Load clipboard items
async function loadItems() {
  try {
    items = await ClipboardService.GetItems(50, 0);
    renderItems(items);
  } catch (err) {
    console.error("Failed to load items:", err);
  }
}

// Load stats
async function loadStats() {
  try {
    const stats = await ClipboardService.GetStats();
    statsEl.textContent = `共 ${stats.total} 条记录 · ${stats.pinned} 条已固定`;
  } catch (err) {
    console.error("Failed to load stats:", err);
  }
}

// Render items to DOM
function renderItems(itemList) {
  if (!itemList || itemList.length === 0) {
    listEl.style.display = "none";
    emptyEl.style.display = "flex";
    return;
  }

  listEl.style.display = "block";
  emptyEl.style.display = "none";

  listEl.innerHTML = itemList
    .map(
      (item) => `
    <div class="clipboard-item ${item.pinned ? "pinned" : ""}" data-id="${item.id}">
      <div class="item-content" onclick="window.__copyItem(${item.id}, ${JSON.stringify(item.content).replace(/"/g, "&quot;")})">
        <div class="item-text">${escapeHtml(item.content)}</div>
        <div class="item-meta">
          <span>${formatTime(item.createdAt)}</span>
          <span>·</span>
          <span>${item.content.length} 字符</span>
        </div>
      </div>
      <div class="item-actions">
        <button class="pin-btn ${item.pinned ? "active" : ""}" onclick="window.__togglePin(${item.id})" title="${item.pinned ? "取消固定" : "固定"}">📌</button>
        <button class="delete-btn" onclick="window.__deleteItem(${item.id})" title="删除">🗑️</button>
      </div>
    </div>
  `
    )
    .join("");
}

// Setup event listeners
function setupEventListeners() {
  // Search
  searchInput.addEventListener("input", (e) => {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      searchItems(e.target.value);
    }, 300);
  });

  // Clear all
  clearBtn.addEventListener("click", async () => {
    try {
      await ClipboardService.ClearAll();
      await loadItems();
      await loadStats();
      showToast("已清除所有未固定记录");
    } catch (err) {
      console.error("Failed to clear:", err);
    }
  });

  // Keyboard shortcut
  document.addEventListener("keydown", (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === "f") {
      e.preventDefault();
      searchInput.focus();
    }
    if (e.key === "Escape") {
      searchInput.value = "";
      searchInput.blur();
      loadItems();
    }
  });
}

// Listen for new clipboard items from backend
function listenForNewItems() {
  Events.On("clipboard:new", (event) => {
    // Reload items when new clipboard content is detected
    loadItems();
    loadStats();
  });
}

// Search items
async function searchItems(query) {
  try {
    const results = await ClipboardService.SearchItems(query);
    renderItems(results);
  } catch (err) {
    console.error("Failed to search:", err);
  }
}

// Copy item to clipboard
window.__copyItem = async (id, content) => {
  try {
    await ClipboardService.CopyToClipboard(content);
    showToast("已复制到剪切板");
  } catch (err) {
    console.error("Failed to copy:", err);
  }
};

// Toggle pin
window.__togglePin = async (id) => {
  try {
    await ClipboardService.TogglePin(id);
    await loadItems();
    await loadStats();
  } catch (err) {
    console.error("Failed to toggle pin:", err);
  }
};

// Delete item
window.__deleteItem = async (id) => {
  try {
    await ClipboardService.DeleteItem(id);
    await loadItems();
    await loadStats();
    showToast("已删除");
  } catch (err) {
    console.error("Failed to delete:", err);
  }
};

// Utility: escape HTML
function escapeHtml(text) {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}

// Utility: format time
function formatTime(isoString) {
  const date = new Date(isoString);
  const now = new Date();
  const diff = now - date;

  if (diff < 60000) return "刚刚";
  if (diff < 3600000) return `${Math.floor(diff / 60000)} 分钟前`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)} 小时前`;
  if (diff < 604800000) return `${Math.floor(diff / 86400000)} 天前`;

  return date.toLocaleDateString("zh-CN", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

// Utility: show toast
function showToast(message) {
  const toast = document.createElement("div");
  toast.className = "toast";
  toast.textContent = message;
  document.body.appendChild(toast);
  setTimeout(() => toast.remove(), 2000);
}

// Start the app
init();
