#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod backend;
mod attachments;
mod company;
mod models;
mod template_links;
use tauri_plugin_deep_link::DeepLinkExt;
use template_links::take_template_links;

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
                    | "testflight.apple.com"
                    | "apps.apple.com"
                    | "ai-sound-lab.app.baizhi.cloud"
                    | "image-generate.app.baizhi.cloud"
                    | "video-api.app.baizhi.cloud"
                    | "capybrowser.app.baizhi.cloud"
                    | "design-prompt.app.baizhi.cloud"
                    | "agent-toolkit.app.baizhi.cloud"
                    | "octobus.chaitin.net"
            )
        )
        || !parsed.username().is_empty()
        || parsed.password().is_some()
        || parsed.port().is_some()
    {
        return Err("不支持的外部地址".into());
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
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            if let Some(window) = app.get_webview_window("main") {
                let _ = window.show();
                let _ = window.set_focus();
            }
        }))
        .plugin(tauri_plugin_deep_link::init())
        .manage(template_links::TemplateLinks::default())
        .manage(BackendProcess::new())
        .invoke_handler(tauri::generate_handler![
            take_template_links,
            get_desktop_config,
            list_provider_models,
            save_desktop_config,
            open_data_directory,
            open_external,
            company::open_company_login,
            attachments::export_attachment,
            export_markdown
        ])
        .setup(|app| {
            #[cfg(any(target_os = "linux", target_os = "windows"))]
            app.deep_link().register_all()?;
            let link_handle = app.handle().clone();
            app.deep_link()
                .on_open_url(move |event| template_links::receive(&link_handle, event.urls()));
            if let Some(urls) = app.deep_link().get_current()? {
                template_links::receive(app.handle(), urls);
            }
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
