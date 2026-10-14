//! Local IdP configuration only. No authorization request or login session is created here.

use serde::{Deserialize, Serialize};
use std::fs::{self, OpenOptions};
use std::io::Write;
use std::path::Path;
use std::sync::Mutex;
use std::time::{SystemTime, UNIX_EPOCH};

#[derive(Clone, Copy, Deserialize, Serialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum Environment {
    Development,
    Test,
    Production,
}

#[derive(Clone, Copy, Deserialize, Serialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ClientAuthMethod {
    ClientSecretBasic,
    ClientSecretPost,
    None,
}

#[derive(Clone, Deserialize, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct OAuthSettings {
    environment: Environment,
    authorization_endpoint: String,
    token_endpoint: String,
    user_info_endpoint: String,
    client_id: String,
    redirect_uri: String,
    scope: String,
    client_auth_method: ClientAuthMethod,
}

impl Default for OAuthSettings {
    fn default() -> Self {
        Self {
            environment: Environment::Development,
            authorization_endpoint: "https://auth.chaitin.net/oauth2/auth".into(),
            token_endpoint: "https://auth.chaitin.net/oauth2/token".into(),
            user_info_endpoint: "https://auth.chaitin.net/userinfo".into(),
            client_id: String::new(),
            // A callback must be registered with the IdP; do not imply it is implemented.
            redirect_uri: String::new(),
            scope: "openid profile email".into(),
            client_auth_method: ClientAuthMethod::ClientSecretBasic,
        }
    }
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct OAuthConfigInput {
    settings: OAuthSettings,
    client_secret: Option<String>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub struct OAuthConfigView {
    settings: OAuthSettings,
    configured: bool,
    has_client_secret: bool,
}

#[derive(Deserialize, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct StoredConfig {
    settings: OAuthSettings,
    client_secret: Option<String>,
}

impl StoredConfig {
    fn view(&self) -> OAuthConfigView {
        OAuthConfigView {
            settings: self.settings.clone(),
            configured: true,
            has_client_secret: self.client_secret.is_some(),
        }
    }
}

#[derive(Default)]
pub struct OAuthConfigStore {
    lock: Mutex<()>,
}

impl OAuthConfigStore {
    pub fn get(&self) -> Result<OAuthConfigView, String> {
        let _lock = self.lock.lock().map_err(|_| "OAuth 配置暂不可用。")?;
        Ok(read_config(&crate::backend::data_dir()?)?
            .map(|config| config.view())
            .unwrap_or(OAuthConfigView {
                settings: OAuthSettings::default(),
                configured: false,
                has_client_secret: false,
            }))
    }

    pub fn save(&self, input: OAuthConfigInput) -> Result<OAuthConfigView, String> {
        let _lock = self.lock.lock().map_err(|_| "OAuth 配置暂不可用。")?;
        save_config(&crate::backend::data_dir()?, input)
    }
}

fn validate_url(
    value: &str,
    label: &str,
    environment: Environment,
    callback: bool,
) -> Result<String, String> {
    let value = value.trim();
    if value.is_empty()
        || value.len() > 2048
        || value.chars().any(|c| c.is_whitespace() || c.is_control())
        || value.contains('\\')
        || !(value.starts_with("https://") || value.starts_with("http://"))
    {
        return Err(format!("请输入有效的{label}。"));
    }
    let url = tauri::Url::parse(value).map_err(|_| format!("请输入有效的{label}。"))?;
    let local = matches!(url.host_str(), Some("localhost" | "127.0.0.1" | "[::1]"));
    if url.host_str().is_none()
        || !url.username().is_empty()
        || url.password().is_some()
        || url.fragment().is_some()
        || (!callback && url.query().is_some())
    {
        return Err(format!(
            "{label}不能包含账号、密码或片段；接口地址也不能包含查询参数。"
        ));
    }
    if url.scheme() != "https"
        && !(url.scheme() == "http" && local && environment == Environment::Development)
    {
        return Err(format!(
            "{label}须使用 HTTPS；仅本地开发环境的 localhost、127.0.0.1 或 [::1] 可使用 HTTP。"
        ));
    }
    // Keep the callback spelling and query intact: IdP registration matches redirect_uri exactly.
    Ok(value.to_string())
}

fn same_client(a: &OAuthSettings, b: &OAuthSettings) -> bool {
    a.environment == b.environment
        && a.client_id == b.client_id
        && a.authorization_endpoint == b.authorization_endpoint
        && a.token_endpoint == b.token_endpoint
        && a.user_info_endpoint == b.user_info_endpoint
        && a.client_auth_method == b.client_auth_method
}

fn validate_config(
    input: OAuthConfigInput,
    previous: Option<StoredConfig>,
) -> Result<StoredConfig, String> {
    let mut settings = input.settings;
    settings.authorization_endpoint = validate_url(
        &settings.authorization_endpoint,
        "授权地址",
        settings.environment,
        false,
    )?;
    settings.token_endpoint = validate_url(
        &settings.token_endpoint,
        "令牌地址",
        settings.environment,
        false,
    )?;
    settings.user_info_endpoint = validate_url(
        &settings.user_info_endpoint,
        "用户信息地址",
        settings.environment,
        false,
    )?;
    settings.redirect_uri = validate_url(
        &settings.redirect_uri,
        "回调地址",
        settings.environment,
        true,
    )?;
    settings.client_id = settings.client_id.trim().to_string();
    if settings.client_id.is_empty()
        || settings.client_id.len() > 512
        || settings.client_id.chars().any(char::is_control)
    {
        return Err("请填写有效的 Client ID（不超过 512 字符）。".into());
    }
    if settings.scope.len() > 1024 {
        return Err("Scope 过长。".into());
    }
    let mut scopes = Vec::new();
    for scope in settings.scope.split_whitespace() {
        if !scope
            .bytes()
            .all(|b| b == 0x21 || (0x23..=0x5b).contains(&b) || (0x5d..=0x7e).contains(&b))
        {
            return Err("Scope 请使用以空格分隔的有效权限名称。".into());
        }
        if !scopes.contains(&scope) {
            scopes.push(scope);
        }
    }
    if !scopes.contains(&"openid") {
        return Err("Scope 必须包含 openid；多个权限请用空格分隔。".into());
    }
    settings.scope = scopes.join(" ");
    let client_secret = if settings.client_auth_method == ClientAuthMethod::None {
        // Switching to a public client explicitly removes the old secret from storage.
        None
    } else {
        let supplied = input.client_secret.filter(|secret| !secret.is_empty());
        let secret = supplied.or_else(|| {
            previous
                .filter(|old| same_client(&old.settings, &settings))
                .and_then(|old| old.client_secret)
        });
        let secret = secret.ok_or(
            "请填写 Client Secret；更换环境、Client ID、接口地址或认证方式后需要重新填写。",
        )?;
        if secret.trim().is_empty() || secret.len() > 8192 || secret.chars().any(char::is_control) {
            return Err("Client Secret 无效，不能包含换行符或控制字符。".into());
        }
        Some(secret)
    };
    Ok(StoredConfig {
        settings,
        client_secret,
    })
}

fn read_config(dir: &Path) -> Result<Option<StoredConfig>, String> {
    match fs::read(dir.join("oauth.json")) {
        Ok(bytes) => serde_json::from_slice(&bytes)
            .map(Some)
            .map_err(|_| "本地 OAuth 配置无法读取，请检查数据目录中的 oauth.json。".into()),
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(None),
        Err(_) => Err("无法读取本地 OAuth 配置，请检查数据目录权限。".into()),
    }
}

fn save_config(dir: &Path, input: OAuthConfigInput) -> Result<OAuthConfigView, String> {
    let config = validate_config(input, read_config(dir)?)?;
    crate::backend::ensure_data_directory(dir)?;
    let bytes = serde_json::to_vec_pretty(&config).map_err(|_| "OAuth 配置编码失败。")?;
    let stamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_err(|_| "系统时间无效。")?
        .as_nanos();
    let temporary = dir.join(format!("oauth.json.{}.{stamp}.tmp", std::process::id()));
    let mut options = OpenOptions::new();
    options.create_new(true).write(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    let mut file = options
        .open(&temporary)
        .map_err(|_| "无法创建 OAuth 配置，请检查数据目录权限。")?;
    let result = (|| {
        file.write_all(&bytes)?;
        file.sync_all()?;
        drop(file);
        fs::rename(&temporary, dir.join("oauth.json"))
    })();
    if result.is_err() {
        let _ = fs::remove_file(&temporary);
        return Err("无法保存 OAuth 配置，原配置已保留。".into());
    }
    Ok(config.view())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::PathBuf;

    fn input() -> OAuthConfigInput {
        OAuthConfigInput {
            settings: OAuthSettings {
                client_id: "test-client".into(),
                redirect_uri: "http://localhost:8080/oauth/callback?env=dev".into(),
                ..OAuthSettings::default()
            },
            client_secret: Some("test-only-secret".into()),
        }
    }

    struct TestDirectory(PathBuf);
    impl TestDirectory {
        fn new() -> Self {
            Self(std::env::temp_dir().join(format!(
                    "soren-oauth-test-{}-{}",
                    std::process::id(),
                    SystemTime::now()
                        .duration_since(UNIX_EPOCH)
                        .unwrap()
                        .as_nanos()
                )))
        }
    }
    impl Drop for TestDirectory {
        fn drop(&mut self) {
            let _ = fs::remove_dir_all(&self.0);
        }
    }

    #[test]
    fn secrets_are_retained_only_for_the_same_client_and_environment() {
        let previous = || validate_config(input(), None).unwrap();
        let mut updated = input();
        updated.client_secret = None;
        updated.settings.scope = "openid email".into();
        assert_eq!(
            validate_config(updated, Some(previous()))
                .unwrap()
                .client_secret
                .as_deref(),
            Some("test-only-secret")
        );
        for field in [
            "client",
            "auth",
            "token",
            "userinfo",
            "method",
            "environment",
        ] {
            let mut updated = input();
            updated.client_secret = None;
            updated.settings.redirect_uri = "https://app.example/callback".into();
            match field {
                "client" => updated.settings.client_id = "another-client".into(),
                "auth" => {
                    updated.settings.authorization_endpoint = "https://other.example/auth".into()
                }
                "token" => updated.settings.token_endpoint = "https://other.example/token".into(),
                "userinfo" => {
                    updated.settings.user_info_endpoint = "https://other.example/userinfo".into()
                }
                "method" => {
                    updated.settings.client_auth_method = ClientAuthMethod::ClientSecretPost
                }
                _ => updated.settings.environment = Environment::Production,
            }
            assert!(
                validate_config(updated, Some(previous())).is_err(),
                "retained secret after changing {field}"
            );
        }
    }

    #[test]
    fn validates_https_and_loopback_exceptions_and_preserves_callback_exactly() {
        for endpoint in [
            "https:idp.example/auth",
            "https://idp.example/au\0th",
            "http://idp.example/auth",
            "javascript:alert(1)",
            "https://name:secret@idp.example/auth",
            "https://idp.example/auth#fragment",
            "https://idp.example/auth?secret=x",
            "http://localhost.evil.test/auth",
        ] {
            let mut value = input();
            value.settings.authorization_endpoint = endpoint.into();
            assert!(validate_config(value, None).is_err(), "accepted {endpoint}");
        }
        for host in ["localhost", "127.0.0.1", "[::1]"] {
            let mut value = input();
            value.settings.redirect_uri = format!("http://{host}:8080/callback?env=dev");
            assert!(validate_config(value, None).is_ok());
        }
        for environment in [Environment::Test, Environment::Production] {
            let mut value = input();
            value.settings.environment = environment;
            assert!(validate_config(value, None).is_err());
        }
        let mut value = input();
        let callback = "https://app.example/callback/?a=%2F&b=two+words";
        value.settings.redirect_uri = callback.into();
        assert_eq!(
            validate_config(value, None).unwrap().settings.redirect_uri,
            callback
        );
    }

    #[test]
    fn enforces_openid_and_valid_scope_tokens() {
        for scope in [
            "profile email",
            "",
            "openid profile\"",
            "openid 用户",
            "openid profile\\email",
        ] {
            let mut value = input();
            value.settings.scope = scope.into();
            assert!(validate_config(value, None).is_err());
        }
        let mut value = input();
        value.settings.scope = "  openid   profile openid\temail  ".into();
        assert_eq!(
            validate_config(value, None).unwrap().settings.scope,
            "openid profile email"
        );
    }

    #[test]
    fn public_clients_remove_secrets_and_confidential_clients_require_them() {
        let mut value = input();
        value.settings.client_auth_method = ClientAuthMethod::None;
        let saved = validate_config(value, Some(validate_config(input(), None).unwrap())).unwrap();
        assert!(saved.client_secret.is_none());
        assert!(!saved.view().has_client_secret);
        for secret in [None, Some(""), Some("  "), Some("line\nbreak")] {
            let mut value = input();
            value.client_secret = secret.map(str::to_string);
            assert!(validate_config(value, None).is_err());
        }
    }

    #[test]
    fn persists_privately_returns_no_secret_and_preserves_old_config_on_error() {
        let dir = TestDirectory::new();
        assert!(read_config(&dir.0).unwrap().is_none());
        let view = save_config(&dir.0, input()).unwrap();
        let json = serde_json::to_value(view).unwrap();
        assert_eq!(json["configured"], true);
        assert_eq!(json["hasClientSecret"], true);
        assert!(json.get("clientSecret").is_none());
        assert!(!json.to_string().contains("test-only-secret"));
        let path = dir.0.join("oauth.json");
        let original = fs::read(&path).unwrap();
        assert_eq!(
            read_config(&dir.0)
                .unwrap()
                .unwrap()
                .client_secret
                .as_deref(),
            Some("test-only-secret")
        );
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            assert_eq!(
                fs::metadata(&path).unwrap().permissions().mode() & 0o777,
                0o600
            );
            assert_eq!(
                fs::metadata(&dir.0).unwrap().permissions().mode() & 0o777,
                0o700
            );
        }
        let mut invalid = input();
        invalid.settings.scope.clear();
        assert!(save_config(&dir.0, invalid).is_err());
        assert_eq!(fs::read(&path).unwrap(), original);
        let mut replacement = input();
        replacement.client_secret = Some("replacement-secret".into());
        save_config(&dir.0, replacement).unwrap();
        assert_eq!(
            read_config(&dir.0)
                .unwrap()
                .unwrap()
                .client_secret
                .as_deref(),
            Some("replacement-secret")
        );
        assert_eq!(fs::read_dir(&dir.0).unwrap().count(), 1);
        fs::write(&path, b"invalid json containing a secret").unwrap();
        assert!(save_config(&dir.0, input()).is_err());
        assert_eq!(
            fs::read(&path).unwrap(),
            b"invalid json containing a secret"
        );
    }
}
