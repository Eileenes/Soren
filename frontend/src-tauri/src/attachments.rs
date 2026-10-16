//! Download only scoped attachment IDs through the local backend. The renderer
//! cannot choose a remote URL or a filesystem destination.
use std::{
    io::{Read, Write},
    time::Duration,
};

fn valid_id(id: &str) -> bool {
    id.len() == 36 && id.bytes().all(|b| b.is_ascii_hexdigit() || b == b'-')
}
fn safe_name(name: &str) -> String {
    let clean: String = name
        .chars()
        .filter(|c| c.is_alphanumeric() || matches!(c, '.' | '-' | '_'))
        .take(100)
        .collect();
    if clean.trim_matches('.').is_empty() {
        "attachment.bin".into()
    } else {
        clean.trim_start_matches('.').into()
    }
}
#[tauri::command]
pub async fn export_attachment(
    run_id: String,
    file_id: String,
    bot_id: String,
    name: String,
    workspace: String,
) -> Result<String, String> {
    if !valid_id(&run_id)
        || !valid_id(&file_id)
        || !(bot_id == "soren" || valid_id(&bot_id))
        || workspace.len() > 512
    {
        return Err("成果标识无效".into());
    }
    tauri::async_runtime::spawn_blocking(move || {
        let prefix = if workspace.is_empty() {
            ""
        } else {
            "/enterprise"
        };
        let url = format!("http://127.0.0.1:8080{prefix}/api/runs/{run_id}/files/{file_id}");
        let client = reqwest::blocking::Client::builder()
            .timeout(Duration::from_secs(180))
            .no_proxy()
            .redirect(reqwest::redirect::Policy::none())
            .build()
            .map_err(|_| "无法创建下载请求")?;
        let mut response = client
            .get(url)
            .header("X-Bot-ID", bot_id)
            .header("X-Soren-Workspace", workspace)
            .send()
            .map_err(|_| "下载失败，请检查连接后重试")?;
        if response.status() != reqwest::StatusCode::OK {
            return Err("成果不可用或无权下载".to_string());
        }
        let size = response
            .content_length()
            .filter(|n| *n <= 200 * 1024 * 1024)
            .ok_or("成果过大或长度未知")?;
        let dir = crate::backend::data_dir()?.join("exports");
        std::fs::create_dir_all(&dir).map_err(|_| "无法创建下载目录")?;
        let stamp = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .map_err(|_| "时间无效")?
            .as_nanos();
        let path = dir.join(format!("{}-{}", stamp, safe_name(&name)));
        let mut options = std::fs::OpenOptions::new();
        options.write(true).create_new(true);
        #[cfg(unix)]
        {
            use std::os::unix::fs::OpenOptionsExt;
            options.mode(0o600);
        }
        let mut file = options.open(&path).map_err(|_| "无法保存文件")?;
        let result = std::io::copy(&mut (&mut response).take(size + 1), &mut file);
        if result.ok() != Some(size) || file.flush().is_err() || file.sync_all().is_err() {
            drop(file);
            let _ = std::fs::remove_file(&path);
            return Err("下载未完成，请重试".to_string());
        }
        #[cfg(target_os = "macos")]
        {
            let _ = std::process::Command::new("open")
                .arg("-R")
                .arg(&path)
                .spawn();
        }
        Ok(path.to_string_lossy().into_owned())
    })
    .await
    .map_err(|_| "下载任务失败".to_string())?
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn export_names_cannot_choose_a_path() {
        assert_eq!(safe_name("../../my song.mp3"), "mysong.mp3");
        assert_eq!(safe_name("..."), "attachment.bin");
        assert!(!valid_id("../../etc/passwd"));
    }
}
