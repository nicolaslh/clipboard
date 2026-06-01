import { Events } from "@wailsio/runtime";
import { ClipboardService } from "../bindings/github.com/nic/clipboard";

let items = [];
let searchTimeout = null;
let currentFilter = "all";

// DOM elements
const listEl = document.getElementById("clipboard-list");
const emptyEl = document.getElementById("empty-state");
const searchInput = document.getElementById("search-input");
const datePicker = document.getElementById("date-picker");
const statsEl = document.getElementById("stats");
const clearBtn = document.getElementById("btn-clear");
const filterBar = document.getElementById("filter-bar");
const settingsBtn = document.getElementById("btn-settings");
const settingsOverlay = document.getElementById("settings-overlay");
const settingsClose = document.getElementById("settings-close");
const settingsSave = document.getElementById("settings-save");
const retentionInput = document.getElementById("retention-input");

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
    if (currentFilter === "all") {
      items = await ClipboardService.GetItems(50, 0);
    } else if (currentFilter === "snippets") {
      items = await loadSnippets();
    } else {
      items = await ClipboardService.FilterByCategory(currentFilter);
    }
    renderItems(items);
  } catch (err) {
    console.error("Failed to load items:", err);
  }
}

// Load snippets (items with group names)
async function loadSnippets() {
  try {
    const groups = await ClipboardService.GetGroups();
    let allItems = [];
    for (const group of groups) {
      const groupItems = await ClipboardService.GetGroupItems(group);
      allItems = allItems.concat(groupItems);
    }
    return allItems;
  } catch (err) {
    console.error("Failed to load snippets:", err);
    return [];
  }
}

// Load stats
async function loadStats() {
  try {
    const stats = await ClipboardService.GetStats();
    statsEl.textContent = `共 ${stats.total} 条 · ${stats.pinned} 固定 · 保留 ${stats.retentionDays} 天`;
  } catch (err) {
    console.error("Failed to load stats:", err);
  }
}

// Category label map
const categoryLabels = {
  text: "文本",
  url: "链接",
  code: "代码",
  path: "路径",
  email: "邮箱",
  image: "图片",
};

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
    .map((item) => {
      const isImage = item.category === "image";
      const isLong = !isImage && item.content.length > 200;
      const catLabel = categoryLabels[item.category] || item.category;
      const groupTag = item.groupName
        ? `<span class="tag tag-group">${escapeHtml(item.groupName)}</span>`
        : "";

      let contentHtml;
      if (isImage) {
        contentHtml = `<div class="item-image"><img src="/api/clipboard?id=${item.id}" alt="图片" loading="lazy"></div>`;
      } else {
        contentHtml = `<div class="item-text ${isLong ? "truncated" : ""}">${escapeHtml(isLong ? item.content.slice(0, 200) : item.content)}</div>`;
        if (isLong) {
          contentHtml += `<button class="preview-btn" onclick="event.stopPropagation(); window.__preview(${item.id})">展开预览</button>`;
        }
      }

      const clickAction = isImage
        ? `window.__copyImage(${item.id})`
        : `window.__copyItem(${item.id}, ${JSON.stringify(item.content).replace(/"/g, "&quot;")})`;

      return `
    <div class="clipboard-item ${item.pinned ? "pinned" : ""}" data-id="${item.id}">
      <div class="item-content" onclick="${clickAction}">
        <div class="item-tags">
          <span class="tag tag-${item.category || "text"}">${catLabel}</span>
          ${groupTag}
        </div>
        ${contentHtml}
        <div class="item-meta">
          <span>${formatTime(item.createdAt)}</span>
          ${!isImage ? `<span>·</span><span>${item.content.length} 字符</span>` : ""}
        </div>
      </div>
      <div class="item-actions">
        <button class="group-btn" onclick="event.stopPropagation(); window.__setGroup(${item.id})" title="加入片段库">📁</button>
        <button class="pin-btn ${item.pinned ? "active" : ""}" onclick="window.__togglePin(${item.id})" title="${item.pinned ? "取消固定" : "固定"}">📌</button>
        <button class="delete-btn" onclick="window.__deleteItem(${item.id})" title="删除">🗑️</button>
      </div>
    </div>
  `;
    })
    .join("");
}

// Setup event listeners
function setupEventListeners() {
  // Search
  searchInput.addEventListener("input", (e) => {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      datePicker.value = ""; // clear date when typing
      searchItems(e.target.value);
    }, 300);
  });

  // Date picker
  datePicker.addEventListener("change", async (e) => {
    const date = e.target.value;
    if (!date) {
      await loadItems();
      return;
    }
    searchInput.value = ""; // clear text search when picking date
    try {
      const results = await ClipboardService.FilterByDate(date);
      renderItems(results);
    } catch (err) {
      console.error("Failed to filter by date:", err);
    }
  });

  // Filter buttons
  filterBar.addEventListener("click", (e) => {
    const btn = e.target.closest(".filter-btn");
    if (!btn) return;
    filterBar.querySelectorAll(".filter-btn").forEach((b) => b.classList.remove("active"));
    btn.classList.add("active");
    currentFilter = btn.dataset.filter;
    loadItems();
  });

  // Clear all
  clearBtn.addEventListener("click", () => {
    if (clearBtn.dataset.confirming === "true") return;
    clearBtn.dataset.confirming = "true";
    clearBtn.textContent = "确认？";
    clearBtn.classList.add("btn-confirm");

    const timer = setTimeout(() => resetClearBtn(), 3000);

    clearBtn._confirmHandler = async () => {
      clearTimeout(timer);
      try {
        await ClipboardService.ClearAll();
        await loadItems();
        await loadStats();
        showToast("已清除");
      } catch (err) {
        console.error("Failed to clear:", err);
      }
      resetClearBtn();
    };
    clearBtn.addEventListener("click", clearBtn._confirmHandler, { once: true });
  });

  function resetClearBtn() {
    clearBtn.dataset.confirming = "false";
    clearBtn.textContent = "清除";
    clearBtn.classList.remove("btn-confirm");
    if (clearBtn._confirmHandler) {
      clearBtn.removeEventListener("click", clearBtn._confirmHandler);
      clearBtn._confirmHandler = null;
    }
  }

  // Settings
  settingsBtn.addEventListener("click", openSettings);
  settingsClose.addEventListener("click", () => (settingsOverlay.style.display = "none"));
  settingsOverlay.addEventListener("click", (e) => {
    if (e.target === settingsOverlay) settingsOverlay.style.display = "none";
  });
  settingsSave.addEventListener("click", saveSettings);

  // Keyboard shortcuts
  document.addEventListener("keydown", (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === "f") {
      e.preventDefault();
      searchInput.focus();
    }
    if (e.key === "Escape") {
      if (settingsOverlay.style.display !== "none") {
        settingsOverlay.style.display = "none";
      } else {
        searchInput.value = "";
        datePicker.value = "";
        searchInput.blur();
        currentFilter = "all";
        filterBar.querySelectorAll(".filter-btn").forEach((b) => b.classList.remove("active"));
        filterBar.querySelector('[data-filter="all"]').classList.add("active");
        loadItems();
      }
    }
  });
}

// Settings
async function openSettings() {
  try {
    const stats = await ClipboardService.GetStats();
    retentionInput.value = stats.retentionDays || 30;
  } catch (err) {
    retentionInput.value = 30;
  }
  settingsOverlay.style.display = "flex";
}

async function saveSettings() {
  const days = parseInt(retentionInput.value) || 30;
  try {
    await ClipboardService.SetRetentionDays(days);
    await loadItems();
    await loadStats();
    showToast(`保留天数已设为 ${days} 天`);
  } catch (err) {
    console.error("Failed to save settings:", err);
  }
  settingsOverlay.style.display = "none";
}

// Listen for new clipboard items from backend
function listenForNewItems() {
  Events.On("clipboard:new", () => {
    if (currentFilter === "all") loadItems();
    loadStats();
  });
  Events.On("clipboard:cleaned", () => {
    loadItems();
    loadStats();
  });
}

// Search items
async function searchItems(query) {
  try {
    if (!query) {
      await loadItems();
      return;
    }
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
    showToast("已复制");
  } catch (err) {
    console.error("Failed to copy:", err);
  }
};

// Copy image to clipboard
window.__copyImage = async (id) => {
  try {
    await ClipboardService.CopyImageToClipboard(id);
    showToast("图片已复制");
  } catch (err) {
    console.error("Failed to copy image:", err);
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

// Set group for snippet
window.__setGroup = async (id) => {
  const groups = await ClipboardService.GetGroups();
  const overlay = document.createElement("div");
  overlay.className = "preview-overlay";
  overlay.onclick = (e) => {
    if (e.target === overlay) overlay.remove();
  };

  const groupList = groups.length
    ? groups.map((g) => `<button class="group-option" data-group="${escapeHtml(g)}">${escapeHtml(g)}</button>`).join("")
    : '<p class="setting-hint">暂无分组</p>';

  overlay.innerHTML = `
    <div class="preview-modal" style="max-width:320px;">
      <div class="preview-header">
        <span class="preview-meta">加入片段库</span>
        <button class="preview-close" onclick="this.closest('.preview-overlay').remove()">✕</button>
      </div>
      <div class="settings-content">
        <div class="group-list">${groupList}</div>
        <div class="setting-item" style="margin-top:8px;">
          <input type="text" id="new-group-input" placeholder="新建分组名称..." class="group-input">
        </div>
      </div>
      <div class="preview-actions">
        <button id="group-save-btn">保存</button>
      </div>
    </div>
  `;
  document.body.appendChild(overlay);

  // Select existing group
  overlay.querySelectorAll(".group-option").forEach((btn) => {
    btn.addEventListener("click", () => {
      overlay.querySelector("#new-group-input").value = btn.dataset.group;
      overlay.querySelectorAll(".group-option").forEach((b) => b.classList.remove("active"));
      btn.classList.add("active");
    });
  });

  // Save
  overlay.querySelector("#group-save-btn").addEventListener("click", async () => {
    const name = overlay.querySelector("#new-group-input").value.trim();
    if (!name) {
      showToast("请输入分组名称");
      return;
    }
    try {
      await ClipboardService.SetItemGroup(id, name);
      await loadItems();
      showToast(`已加入「${name}」`);
    } catch (err) {
      console.error("Failed to set group:", err);
    }
    overlay.remove();
  });
};

// Preview item
window.__preview = (id) => {
  const item = items.find((i) => i.id === id);
  if (!item) return;

  const overlay = document.createElement("div");
  overlay.className = "preview-overlay";
  overlay.onclick = (e) => {
    if (e.target === overlay) overlay.remove();
  };

  overlay.innerHTML = `
    <div class="preview-modal">
      <div class="preview-header">
        <span class="preview-meta">${item.content.length} 字符 · ${formatTime(item.createdAt)}</span>
        <button class="preview-close" onclick="this.closest('.preview-overlay').remove()">✕</button>
      </div>
      <pre class="preview-content">${escapeHtml(item.content)}</pre>
      <div class="preview-actions">
        <button onclick="window.__copyItem(${item.id}, ${JSON.stringify(item.content).replace(/"/g, "&quot;")}); this.closest('.preview-overlay').remove();">复制</button>
      </div>
    </div>
  `;
  document.body.appendChild(overlay);
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
