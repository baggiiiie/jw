const NATIVE_HOST = "com.jw.monitor";

function showToast(tabId, message, isError) {
  chrome.scripting.executeScript({
    target: { tabId },
    args: [message, isError],
    func: (msg, err) => {
      const existing = document.getElementById("jw-toast");
      if (existing) existing.remove();

      const toast = document.createElement("div");
      toast.id = "jw-toast";
      toast.textContent = msg;
      Object.assign(toast.style, {
        position: "fixed",
        top: "16px",
        right: "16px",
        zIndex: "2147483647",
        padding: "12px 20px",
        borderRadius: "8px",
        fontSize: "14px",
        fontFamily: "system-ui, sans-serif",
        color: "#fff",
        background: err ? "#d32f2f" : "#2e7d32",
        boxShadow: "0 4px 12px rgba(0,0,0,0.3)",
        opacity: "0",
        transition: "opacity 0.3s",
      });
      document.body.appendChild(toast);
      requestAnimationFrame(() => (toast.style.opacity = "1"));
      setTimeout(() => {
        toast.style.opacity = "0";
        setTimeout(() => toast.remove(), 300);
      }, 3000);
    },
  });
}

chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({
    id: "watch-with-jw",
    title: "Watch with jw",
    contexts: ["page"],
    documentUrlPatterns: ["http://*/*", "https://*/*"],
  });
});

chrome.contextMenus.onClicked.addListener((info, tab) => {
  if (info.menuItemId !== "watch-with-jw") return;

  const url = tab.url;
  if (!url) return;

  chrome.runtime.sendNativeMessage(
    NATIVE_HOST,
    { url: url },
    (response) => {
      if (chrome.runtime.lastError) {
        showToast(tab.id, "jw: failed to connect to native host", true);
        return;
      }
      if (response && response.success) {
        showToast(tab.id, "✓ " + response.message, false);
      } else if (response) {
        showToast(tab.id, "jw: " + response.error, true);
      }
    }
  );
});
