const NATIVE_HOST = "com.jw.monitor";

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
        console.error("jw native messaging error:", chrome.runtime.lastError.message);
        return;
      }
      if (response && response.success) {
        console.log("jw:", response.message);
      } else if (response) {
        console.error("jw error:", response.error);
      }
    }
  );
});
