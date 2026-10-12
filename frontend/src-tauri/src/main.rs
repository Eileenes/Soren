#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod backend;
mod models;

use backend::{BackendProcess, ConfigInput, DesktopConfig};
use tauri::menu::{Menu, MenuItem};
use tauri::tray::TrayIconBuilder;
use tauri::{Manager, RunEvent, WindowEvent};

#[tauri::command]
fn get_desktop_config(state: tauri::State<'_, BackendProcess>) -> Result<DesktopConfig, String> {
    state.config()
}

#[tauri::command]
async fn save_desktop_config(
    app: tauri::AppHandle,
    config: ConfigInput,
) -> Result<DesktopConfig, String> {
    tauri::async_runtime::spawn_blocking(move || {
        app.state::<BackendProcess>().save_and_restart(config)
    })
    .await
    .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn list_provider_models(config: ConfigInput) -> Result<Vec<models::ProviderModel>, String> {
    tauri::async_runtime::spawn_blocking(move || backend::fetch_models(config))
        .await
        .map_err(|_| "模型请求任务失败".to_string())?
}

#[tauri::command]
fn open_data_directory() -> Result<(), String> {
    backend::open_data_directory()
}

#[tauri::command]
fn open_external(url: String) -> Result<(), String> {
    let parsed = tauri::Url::parse(&url).map_err(|_| "地址无效")?;
    if parsed.scheme() != "https"
        || !matches!(
            parsed.host_str(),
            Some(
                "github.com"
                    | "accounts.google.com"
                    | "developers.notion.com"
                    | "console.cloud.google.com"
            )
        )
        || !parsed.username().is_empty()
        || parsed.password().is_some()
    {
        return Err("不支持的授权地址".into());
    }
    std::process::Command::new("open")
        .arg(url)
        .spawn()
        .map_err(|e| e.to_string())?;
    Ok(())
}

#[tauri::command]
fn export_markdown(title: String, content: String) -> Result<String, String> {
    if content.len() > 1_000_000 {
        return Err("内容过长".into());
    }
    let dir = backend::data_dir()?.join("exports");
    std::fs::create_dir_all(&dir).map_err(|e| e.to_string())?;
    let name: String = title
        .chars()
        .filter(|c| c.is_alphanumeric() || *c == '-' || *c == '_')
        .take(60)
        .collect();
    let stamp = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map_err(|e| e.to_string())?
        .as_nanos();
    let path = dir.join(format!(
        "{}-{}.md",
        if name.is_empty() { "Soren" } else { &name },
        stamp
    ));
    use std::io::Write;
    let mut options = std::fs::OpenOptions::new();
    options.write(true).create_new(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    let mut file = options.open(&path).map_err(|e| e.to_string())?;
    file.write_all(content.as_bytes())
        .map_err(|e| e.to_string())?;
    file.sync_all().map_err(|e| e.to_string())?;
    std::process::Command::new("open")
        .arg("-R")
        .arg(&path)
        .spawn()
        .map_err(|e| e.to_string())?;
    Ok(path.to_string_lossy().into_owned())
}

fn main() {
    tauri::Builder::default()
        .manage(BackendProcess::new())
        .invoke_handler(tauri::generate_handler![
            get_desktop_config,
            list_provider_models,
            save_desktop_config,
            open_data_directory,
            open_external,
            export_markdown
        ])
        .setup(|app| {
            let show = MenuItem::with_id(app, "show", "打开 Soren", true, None::<&str>)?;
            let quit = MenuItem::with_id(app, "quit", "退出 Soren", true, None::<&str>)?;
            let menu = Menu::with_items(app, &[&show, &quit])?;
            let mut tray = TrayIconBuilder::new()
                .tooltip("Soren · 后台运行")
                .menu(&menu)
                .on_menu_event(|app, event| match event.id().as_ref() {
                    "show" => {
                        if let Some(w) = app.get_webview_window("main") {
                            let _ = w.show();
                            let _ = w.set_focus();
                        }
                    }
                    "quit" => app.exit(0),
                    _ => {}
                });
            if let Some(icon) = app.default_window_icon() {
                tray = tray.icon(icon.clone());
            }
            tray.build(app)?;
            // Starting the backend off the main thread keeps the native window responsive.
            let handle = app.handle().clone();
            std::thread::spawn(move || {
                if let Err(error) = handle.state::<BackendProcess>().start() {
                    eprintln!("[soren] {error}");
                }
                if let Some(window) = handle.get_webview_window("main") {
                    let _ = window.show();
                    let _ = window.set_focus();
                }
            });
            Ok(())
        })
        .on_window_event(|window, event| {
            if let WindowEvent::CloseRequested { api, .. } = event {
                api.prevent_close();
                let _ = window.hide();
            }
        })
        .build(tauri::generate_context!())
        .expect("启动 Soren 失败")
        .run(|app, event| {
            #[cfg(target_os = "macos")]
            if matches!(event, RunEvent::Reopen { .. }) {
                if let Some(w) = app.get_webview_window("main") {
                    let _ = w.show();
                    let _ = w.set_focus();
                }
            }
            if let RunEvent::ExitRequested { .. } | RunEvent::Exit = event {
                let _ = app.state::<BackendProcess>().stop();
            }
        });
}
