//! The renderer may only open the login URL issued by the local company bridge.
//! Company bearer tokens stay in the Go backend's system vault.
use std::time::Duration;

#[tauri::command]
pub async fn open_company_login() -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(|| {
        let client = reqwest::blocking::Client::builder()
            .timeout(Duration::from_secs(5))
            .redirect(reqwest::redirect::Policy::none())
            .build()
            .map_err(|_| "无法打开公司登录")?;
        let response = client
            .get("http://127.0.0.1:8080/api/enterprise/login-url")
            .send()
            .map_err(|_| "本地服务不可用，请重试")?;
        if !response.status().is_success() {
            return Err("登录已过期，请重新登录".to_string());
        }
        let value: serde_json::Value = response.json().map_err(|_| "登录地址无效")?;
        let raw = value["url"].as_str().ok_or("登录地址无效")?;
        let url = tauri::Url::parse(raw).map_err(|_| "登录地址无效")?;
        let dev_local = cfg!(debug_assertions)
            && url.scheme() == "http"
            && matches!(url.host_str(), Some("127.0.0.1" | "localhost" | "[::1]"));
        if (url.scheme() != "https" && !dev_local)
            || url.host_str().is_none()
            || !url.username().is_empty()
            || url.password().is_some()
            || url.fragment().is_some()
            || url.path() != "/auth/browser"
            || url.query_pairs().count() != 1
            || !url
                .query_pairs()
                .any(|(key, value)| key == "ticket" && value.len() == 43)
        {
            return Err("登录地址无效".into());
        }
        #[cfg(target_os = "macos")]
        let result = std::process::Command::new("open").arg(raw).spawn();
        #[cfg(target_os = "linux")]
        let result = std::process::Command::new("xdg-open").arg(raw).spawn();
        #[cfg(target_os = "windows")]
        let result = std::process::Command::new("rundll32")
            .arg("url.dll,FileProtocolHandler")
            .arg(raw)
            .spawn();
        result
            .map(|_| ())
            .map_err(|_| "无法打开系统浏览器".to_string())
    })
    .await
    .map_err(|_| "公司登录任务失败".to_string())?
}
