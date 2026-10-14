use std::collections::VecDeque;
use std::sync::Mutex;
use std::time::{Duration, Instant};
use tauri::{Emitter, Manager};

#[derive(Default)]
pub struct TemplateLinks {
    pending: Mutex<VecDeque<String>>,
    recent: Mutex<Option<(String, Instant)>>,
}

fn is_import_link(url: &tauri::Url) -> bool {
    url.as_str().len() <= 2048
        && url.scheme() == "soren"
        && url.host_str() == Some("templates")
        && url.path() == "/import"
        && url.username().is_empty()
        && url.password().is_none()
        && url.port().is_none()
        && url.fragment().is_none()
}

pub fn receive(app: &tauri::AppHandle, urls: Vec<tauri::Url>) {
    let state = app.state::<TemplateLinks>();
    for url in urls {
        if !is_import_link(&url) {
            continue;
        }
        let value = url.to_string();
        let Ok(mut recent) = state.recent.lock() else {
            continue;
        };
        if recent.as_ref().is_some_and(|(previous, at)| {
            previous == &value && at.elapsed() < Duration::from_secs(2)
        }) {
            continue;
        }
        *recent = Some((value.clone(), Instant::now()));
        if let Ok(mut pending) = state.pending.lock() {
            if pending.len() < 10 && !pending.contains(&value) {
                pending.push_back(value);
            }
        }
        // Events are only notifications. The command atomically drains the queue
        // so startup and running-instance delivery cannot import a link twice.
        let _ = app.emit("soren-template-link", ());
        if let Some(window) = app.get_webview_window("main") {
            let _ = window.show();
            let _ = window.unminimize();
            let _ = window.set_focus();
        }
    }
}

#[tauri::command]
pub fn take_template_links(state: tauri::State<'_, TemplateLinks>) -> Result<Vec<String>, String> {
    let mut pending = state.pending.lock().map_err(|_| "无法读取模板链接")?;
    Ok(pending.drain(..).collect())
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn only_template_routes_are_queued() {
        assert!(is_import_link(
            &tauri::Url::parse("soren://templates/import?id=example&revision=1").unwrap()
        ));
        for value in [
            "https://templates/import",
            "soren://templates/execute",
            "soren://user:secret@templates/import",
            "soren://templates/import#script",
        ] {
            assert!(!is_import_link(&tauri::Url::parse(value).unwrap()));
        }
    }
}
