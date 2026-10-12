// Dedicated mobile runtime: no desktop sidecar, tray, model keys or filesystem API.
#[cfg(not(any(target_os = "ios", target_os = "macos")))]
compile_error!("Android secure credential storage must be implemented before releasing this mobile shell.");
fn entry() -> Result<keyring::Entry, String> {
    keyring::Entry::new("com.soren.mobile", "paired-device").map_err(|_| "系统安全存储不可用".into())
}
#[tauri::command]
fn load_mobile_credential() -> Result<Option<String>, String> {
    match entry()?.get_password() {
        Ok(value) => Ok(Some(value)),
        Err(keyring::Error::NoEntry) => Ok(None),
        Err(_) => Err("无法读取设备身份".into()),
    }
}
#[tauri::command]
fn save_mobile_credential(value: String) -> Result<(), String> {
    if value.len() > 8192 || serde_json::from_str::<serde_json::Value>(&value).is_err() { return Err("设备身份无效".into()); }
    entry()?.set_password(&value).map_err(|_| "无法保存设备身份".into())
}
#[tauri::command]
fn delete_mobile_credential() -> Result<(), String> {
    match entry()?.delete_credential() { Ok(()) | Err(keyring::Error::NoEntry) => Ok(()), Err(_) => Err("无法清除设备身份".into()) }
}
#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_barcode_scanner::init())
        .invoke_handler(tauri::generate_handler![load_mobile_credential, save_mobile_credential, delete_mobile_credential])
        .run(tauri::generate_context!())
        .expect("Cannot launch Soren mobile");
}
