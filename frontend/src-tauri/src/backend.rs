//! Owns the packaged Go sidecar and desktop-only provider configuration.

use serde::{Deserialize, Serialize};
use std::fs::{self, OpenOptions};
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::Mutex;
use std::time::{Duration, Instant};

#[derive(Clone, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
struct StoredConfig {
    provider: String,
    #[serde(rename = "baseURL")]
    base_url: String,
    api_key: String,
    #[serde(default)]
    model: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ConfigInput {
    provider: String,
    #[serde(rename = "baseURL")]
    base_url: String,
    api_key: Option<String>,
    model: Option<String>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DesktopConfig {
    provider: String,
    #[serde(rename = "baseURL")]
    base_url: String,
    configured: bool,
    model: String,
    data_dir: String,
    backend_ready: bool,
    startup_error: Option<String>,
}

pub struct BackendProcess {
    child: Mutex<Option<Child>>,
    startup_error: Mutex<Option<String>>,
    config_lock: Mutex<()>,
}

impl BackendProcess {
    pub fn new() -> Self {
        Self {
            child: Mutex::new(None),
            startup_error: Mutex::new(None),
            config_lock: Mutex::new(()),
        }
    }

    pub fn start(&self) -> Result<(), String> {
        let result = self.start_inner();
        if let Ok(mut error) = self.startup_error.lock() {
            *error = result.as_ref().err().cloned();
        }
        result
    }

    fn start_inner(&self) -> Result<(), String> {
        let mut guard = self.child.lock().map_err(|e| e.to_string())?;
        if let Some(child) = guard.as_mut() {
            if matches!(child.try_wait(), Ok(None)) {
                return Ok(());
            }
            *guard = None;
        }
        // Never attach to or terminate a process we did not spawn.
        let reservation = TcpListener::bind("127.0.0.1:8080")
            .map_err(|_| "本地端口 8080 已被占用，请关闭占用它的程序后重启 Soren。".to_string())?;
        let exe = find_backend_binary()
            .ok_or_else(|| "安装包中缺少 soren-server，请重新构建或安装 Soren。".to_string())?;
        let dir = data_dir()?;
        ensure_data_directory(&dir)?;
        let log_path = dir.join("backend.log");
        if fs::metadata(&log_path)
            .map(|m| m.len() > 5 * 1024 * 1024)
            .unwrap_or(false)
        {
            let _ = fs::rename(&log_path, dir.join("backend.previous.log"));
        }
        let log = private_file(&log_path, false)?;
        let stderr = log.try_clone().map_err(|e| e.to_string())?;
        let mut cmd = Command::new(exe);
        cmd.current_dir(&dir)
            .stdin(Stdio::null())
            .stdout(log)
            .stderr(stderr)
            .env("SERVER_PORT", "8080")
            .env("SERVER_HOST", "127.0.0.1")
            .env("SOREN_DATA_DIR", &dir)
            .env("GIN_MODE", "release");
        if let Some(config) = read_config(&dir)? {
            // Explicitly clear the other provider: the Go server prioritizes Anthropic.
            cmd.env("OPENAI_API_KEY", "")
                .env("ANTHROPIC_API_KEY", "")
                .env("SOREN_MODEL", &config.model);
            if config.provider == "anthropic" {
                cmd.env("ANTHROPIC_API_KEY", &config.api_key)
                    .env("ANTHROPIC_BASE_URL", &config.base_url);
            } else {
                cmd.env("OPENAI_API_KEY", &config.api_key)
                    .env("OPENAI_BASE_URL", &config.base_url);
            }
        }
        drop(reservation);
        let mut child = cmd.spawn().map_err(|e| format!("启动本地服务失败：{e}"))?;
        let deadline = Instant::now() + Duration::from_secs(10);
        while Instant::now() < deadline {
            if let Some(status) = child.try_wait().map_err(|e| e.to_string())? {
                return Err(format!(
                    "本地服务已退出（{status}），详情见 {}。",
                    log_path.display()
                ));
            }
            if health_check() {
                *guard = Some(child);
                return Ok(());
            }
            std::thread::sleep(Duration::from_millis(150));
        }
        let _ = child.kill();
        let _ = child.wait();
        Err(format!("本地服务启动超时，详情见 {}。", log_path.display()))
    }

    pub fn stop(&self) -> Result<(), String> {
        let mut guard = self.child.lock().map_err(|e| e.to_string())?;
        if let Some(mut child) = guard.take() {
            if matches!(child.try_wait(), Ok(Some(_))) {
                return Ok(());
            }
            #[cfg(unix)]
            {
                let _ = Command::new("/bin/kill")
                    .args(["-TERM", &child.id().to_string()])
                    .stdout(Stdio::null())
                    .stderr(Stdio::null())
                    .status();
                let deadline = Instant::now() + Duration::from_secs(2);
                while Instant::now() < deadline {
                    if matches!(child.try_wait(), Ok(Some(_))) {
                        return Ok(());
                    }
                    std::thread::sleep(Duration::from_millis(50));
                }
            }
            let _ = child.kill();
            let _ = child.wait();
        }
        Ok(())
    }

    pub fn config(&self) -> Result<DesktopConfig, String> {
        let dir = data_dir()?;
        let stored = read_config(&dir)?;
        let (provider, base_url, configured, model) = match stored {
            Some(config) => (
                config.provider,
                config.base_url,
                !config.api_key.is_empty(),
                config.model,
            ),
            None => (
                "openai".into(),
                "https://api.openai.com/v1".into(),
                false,
                String::new(),
            ),
        };
        let startup_error = self
            .startup_error
            .lock()
            .map_err(|e| e.to_string())?
            .clone();
        let owns_running_child = self
            .child
            .lock()
            .map_err(|e| e.to_string())?
            .as_mut()
            .map(|child| matches!(child.try_wait(), Ok(None)))
            .unwrap_or(false);
        Ok(DesktopConfig {
            provider,
            base_url,
            configured,
            model,
            data_dir: dir.to_string_lossy().into_owned(),
            backend_ready: owns_running_child && health_check(),
            startup_error,
        })
    }

    pub fn save_and_restart(&self, input: ConfigInput) -> Result<DesktopConfig, String> {
        let _lock = self.config_lock.lock().map_err(|e| e.to_string())?;
        let dir = data_dir()?;
        ensure_data_directory(&dir)?;
        let config = validate_config(input, read_config(&dir)?)?;
        let bytes = serde_json::to_vec_pretty(&config).map_err(|e| e.to_string())?;
        let temp_path = dir.join("provider.json.tmp");
        let mut file = private_file(&temp_path, true)?;
        file.write_all(&bytes).map_err(|e| e.to_string())?;
        file.sync_all().map_err(|e| e.to_string())?;
        fs::rename(temp_path, dir.join("provider.json")).map_err(|e| e.to_string())?;
        self.stop()?;
        self.start()?;
        self.config()
    }
}

fn validate_config(
    input: ConfigInput,
    previous: Option<StoredConfig>,
) -> Result<StoredConfig, String> {
    if input.provider != "openai" && input.provider != "anthropic" {
        return Err("请选择 OpenAI 兼容接口或 Anthropic。".into());
    }
    let base_url = input.base_url.trim().trim_end_matches('/').to_string();
    let url = tauri::Url::parse(&base_url).map_err(|_| "请输入有效的接口地址。".to_string())?;
    if !matches!(url.scheme(), "https" | "http")
        || url.host_str().is_none()
        || !url.username().is_empty()
        || url.password().is_some()
        || url.query().is_some()
        || url.fragment().is_some()
    {
        return Err("接口地址必须是 http(s) URL，且不能包含用户名、密码、查询参数或片段。".into());
    }
    let model = input.model.unwrap_or_else(|| {
        previous
            .as_ref()
            .filter(|old| old.provider == input.provider && old.base_url == base_url)
            .map(|old| old.model.clone())
            .unwrap_or_default()
    });
    let model = model.trim().to_string();
    if model.len() > 200 || model.chars().any(char::is_control) {
        return Err("模型名称无效".into());
    }
    let api_key = input.api_key.unwrap_or_default().trim().to_string();
    let api_key = if api_key.is_empty() {
        previous
            .filter(|old| old.provider == input.provider && old.base_url == base_url)
            .map(|old| old.api_key)
            .unwrap_or_default()
    } else {
        api_key
    };
    if api_key.is_empty() {
        return Err("请输入 API Key；更换服务商或接口地址时需要重新填写。".into());
    }
    if api_key.chars().any(char::is_control) {
        return Err("API Key 不能包含换行符或控制字符。".into());
    }
    Ok(StoredConfig {
        provider: input.provider,
        base_url,
        api_key,
        model,
    })
}

pub fn data_dir() -> Result<PathBuf, String> {
    if let Some(dir) = std::env::var_os("SOREN_DATA_DIR") {
        return Ok(PathBuf::from(dir));
    }
    std::env::var_os(if cfg!(windows) { "USERPROFILE" } else { "HOME" })
        .map(|home| PathBuf::from(home).join(".soren"))
        .ok_or_else(|| "无法确定用户数据目录。".into())
}

fn ensure_data_directory(dir: &Path) -> Result<(), String> {
    fs::create_dir_all(dir).map_err(|e| e.to_string())?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        fs::set_permissions(dir, fs::Permissions::from_mode(0o700)).map_err(|e| e.to_string())?;
    }
    Ok(())
}

fn private_file(path: &Path, truncate: bool) -> Result<fs::File, String> {
    let mut options = OpenOptions::new();
    options
        .create(true)
        .write(true)
        .truncate(truncate)
        .append(!truncate);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    options.open(path).map_err(|e| e.to_string())
}

fn read_config(dir: &Path) -> Result<Option<StoredConfig>, String> {
    match fs::read(dir.join("provider.json")) {
        Ok(bytes) => serde_json::from_slice(&bytes)
            .map(Some)
            .map_err(|_| "本地服务商配置损坏，请检查 ~/.soren/provider.json。".into()),
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(None),
        Err(error) => Err(error.to_string()),
    }
}

pub fn open_data_directory() -> Result<(), String> {
    let dir = data_dir()?;
    ensure_data_directory(&dir)?;
    let opener = if cfg!(target_os = "macos") {
        "open"
    } else if cfg!(windows) {
        "explorer"
    } else {
        "xdg-open"
    };
    Command::new(opener)
        .arg(dir)
        .spawn()
        .map_err(|e| e.to_string())?;
    Ok(())
}

fn find_backend_binary() -> Option<PathBuf> {
    if cfg!(debug_assertions) {
        if let Some(path) = std::env::var_os("SOREN_BACKEND_BIN").map(PathBuf::from) {
            if path.is_file() {
                return Some(path);
            }
        }
    }
    let exe_name = if cfg!(windows) {
        "soren-server.exe"
    } else {
        "soren-server"
    };
    if let Ok(current) = std::env::current_exe() {
        let candidate = current.parent()?.join(exe_name);
        if candidate.is_file() {
            return Some(candidate);
        }
    }
    None
}

fn health_check() -> bool {
    let Ok(mut stream) = TcpStream::connect_timeout(
        &"127.0.0.1:8080".parse().unwrap(),
        Duration::from_millis(250),
    ) else {
        return false;
    };
    let _ = stream.set_read_timeout(Some(Duration::from_millis(400)));
    let _ = stream.set_write_timeout(Some(Duration::from_millis(400)));
    if stream
        .write_all(b"GET /api/health HTTP/1.0\r\nHost: 127.0.0.1\r\nConnection: close\r\n\r\n")
        .is_err()
    {
        return false;
    }
    let mut bytes = Vec::new();
    if stream.take(16384).read_to_end(&mut bytes).is_err() {
        return false;
    }
    let response = String::from_utf8_lossy(&bytes);
    let Some((headers, body)) = response.split_once("\r\n\r\n") else {
        return false;
    };
    if !headers.lines().next().unwrap_or_default().contains(" 200 ") {
        return false;
    }
    serde_json::from_str::<serde_json::Value>(body)
        .map(|body| {
            body["status"] == "ok"
                && body["storageDriver"].is_string()
                && body["memoryEnabled"].is_boolean()
        })
        .unwrap_or(false)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn previous() -> StoredConfig {
        StoredConfig {
            provider: "openai".into(),
            base_url: "https://api.openai.com/v1".into(),
            api_key: "test-secret".into(),
            model: "test-model".into(),
        }
    }

    #[test]
    fn blank_key_preserves_credentials_only_for_same_endpoint() {
        let input = ConfigInput {
            provider: "openai".into(),
            base_url: "https://api.openai.com/v1/".into(),
            api_key: None,
            model: None,
        };
        assert_eq!(
            validate_config(input, Some(previous())).unwrap().api_key,
            "test-secret"
        );
        let input = ConfigInput {
            provider: "openai".into(),
            base_url: "https://another.example/v1".into(),
            api_key: None,
            model: None,
        };
        assert!(validate_config(input, Some(previous())).is_err());
    }

    #[test]
    fn reject_credentials_in_endpoint_and_control_characters() {
        let input = ConfigInput {
            provider: "openai".into(),
            base_url: "https://user:pass@example.com".into(),
            api_key: Some("key".into()),
            model: None,
        };
        assert!(validate_config(input, None).is_err());
        let input = ConfigInput {
            provider: "openai".into(),
            base_url: "https://example.com".into(),
            api_key: Some("key\nvalue".into()),
            model: None,
        };
        assert!(validate_config(input, None).is_err());
    }

    #[test]
    fn selected_model_is_persisted_and_legacy_config_still_loads() {
        let old: StoredConfig = serde_json::from_str(
            r#"{"provider":"openai","baseURL":"https://example.com/v1","apiKey":"test"}"#,
        )
        .unwrap();
        assert!(old.model.is_empty());
        let input = ConfigInput {
            provider: "openai".into(),
            base_url: "https://api.openai.com/v1".into(),
            api_key: None,
            model: Some("chosen-chat".into()),
        };
        let stored = validate_config(input, Some(previous())).unwrap();
        let encoded = serde_json::to_value(stored).unwrap();
        assert_eq!(encoded["model"], "chosen-chat");
    }

    #[test]
    fn frontend_configuration_does_not_serialize_secret() {
        let config = DesktopConfig {
            provider: "openai".into(),
            base_url: "https://example.com".into(),
            configured: true,
            model: "test-model".into(),
            data_dir: "/tmp/soren".into(),
            backend_ready: true,
            startup_error: None,
        };
        let json = serde_json::to_value(config).unwrap();
        assert!(json.get("apiKey").is_none());
        assert_eq!(json["baseURL"], "https://example.com");
    }
}

pub fn fetch_models(input: ConfigInput) -> Result<Vec<crate::models::ProviderModel>, String> {
    let config = validate_config(input, read_config(&data_dir()?)?)?;
    crate::models::fetch(&config.provider, &config.base_url, &config.api_key)
}
