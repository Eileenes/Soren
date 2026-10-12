use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use std::io::Read;
use std::time::{Duration, Instant};

#[derive(Serialize)]
pub struct ProviderModel {
    pub id: String,
    pub name: String,
}
#[derive(Deserialize)]
struct ModelItem {
    id: String,
    display_name: Option<String>,
}
#[derive(Deserialize)]
struct ModelPage {
    data: Vec<ModelItem>,
    #[serde(default)]
    has_more: bool,
    last_id: Option<String>,
}

pub fn fetch(provider: &str, base: &str, key: &str) -> Result<Vec<ProviderModel>, String> {
    let client = reqwest::blocking::Client::builder()
        .redirect(reqwest::redirect::Policy::none())
        .connect_timeout(Duration::from_secs(8))
        .build()
        .map_err(|_| "无法初始化模型连接")?;
    let path = if provider == "anthropic" && !base.ends_with("/v1") {
        format!("{base}/v1/models")
    } else {
        format!("{base}/models")
    };
    let mut url = reqwest::Url::parse(&path).map_err(|_| "模型接口地址无效")?;
    let deadline = Instant::now() + Duration::from_secs(30);
    let mut items = BTreeMap::new();
    let mut cursor = String::new();
    for _ in 0..10 {
        let remaining = deadline
            .checked_duration_since(Instant::now())
            .ok_or("获取模型超时，请重试")?;
        let mut request = client.get(url.clone()).timeout(remaining);
        if provider == "anthropic" {
            request = request
                .header("x-api-key", key)
                .header("anthropic-version", "2023-06-01");
        } else {
            request = request.bearer_auth(key);
        }
        let response = request
            .send()
            .map_err(|_| "无法连接模型列表接口，请检查地址、网络或证书")?;
        if !response.status().is_success() {
            return Err(match response.status().as_u16() {
                401 | 403 => "模型列表授权失败，请检查 API Key 和权限".into(),
                404 | 405 => "该接口不提供模型列表，请手动填写模型名称".into(),
                429 => "请求过于频繁，请稍后重试".into(),
                code => format!("模型列表请求失败（HTTP {code}），可手动填写模型名称"),
            });
        }
        let mut body = Vec::new();
        response
            .take(2_000_001)
            .read_to_end(&mut body)
            .map_err(|_| "读取模型列表失败")?;
        if body.len() > 2_000_000 {
            return Err("模型列表响应过大，请手动填写模型名称".into());
        }
        let page: ModelPage = serde_json::from_slice(&body)
            .map_err(|_| "服务商返回的模型列表格式不兼容，请手动填写模型名称")?;
        for item in page.data {
            if item.id.is_empty() || item.id.len() > 200 || item.id.chars().any(char::is_control) {
                continue;
            }
            let name = item
                .display_name
                .filter(|s| !s.trim().is_empty())
                .unwrap_or_else(|| item.id.clone());
            items.insert(item.id, name.chars().take(200).collect::<String>());
        }
        if items.len() > 5000 {
            return Err("模型数量过多，请手动填写模型名称".into());
        }
        if !page.has_more {
            if items.is_empty() {
                return Err("服务商没有返回可选模型，请手动填写模型名称".into());
            }
            return Ok(items
                .into_iter()
                .map(|(id, name)| ProviderModel { id, name })
                .collect());
        }
        let next = page
            .last_id
            .filter(|s| !s.is_empty() && s != &cursor)
            .ok_or("模型列表分页无效，请手动填写模型名称")?;
        cursor = next;
        url.set_query(None);
        url.query_pairs_mut().append_pair("after_id", &cursor);
    }
    Err("模型列表分页过多，请手动填写模型名称".into())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{Read, Write};
    use std::net::TcpListener;
    fn server(
        replies: Vec<&'static str>,
        provider: &'static str,
    ) -> (String, std::thread::JoinHandle<()>) {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = format!("http://{}", listener.local_addr().unwrap());
        let handle = std::thread::spawn(move || {
            for (i, reply) in replies.into_iter().enumerate() {
                let (mut socket, _) = listener.accept().unwrap();
                socket
                    .set_read_timeout(Some(Duration::from_secs(3)))
                    .unwrap();
                let mut request = Vec::new();
                let mut byte = [0];
                while !request.ends_with(b"\r\n\r\n") {
                    socket.read_exact(&mut byte).unwrap();
                    request.push(byte[0]);
                }
                let request = String::from_utf8(request).unwrap().to_lowercase();
                assert!(request.starts_with("get /v1/models"));
                assert!(request.contains(if provider == "anthropic" {
                    "x-api-key: test-key"
                } else {
                    "authorization: bearer test-key"
                }));
                if i > 0 {
                    assert!(request.contains("after_id="));
                }
                write!(socket,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",reply.len(),reply).unwrap();
            }
        });
        (address, handle)
    }
    #[test]
    fn lists_openai_compatible_models_and_deduplicates() {
        let (base, handle) = server(
            vec![r#"{"data":[{"id":"b"},{"id":"a"},{"id":"b"}]}"#],
            "openai",
        );
        let result = fetch("openai", &(base + "/v1"), "test-key").unwrap();
        handle.join().unwrap();
        assert_eq!(result.len(), 2);
        assert_eq!(result[0].id, "a");
    }
    #[test]
    fn follows_anthropic_pagination_and_preserves_labels() {
        let (base, handle) = server(
            vec![
                r#"{"data":[{"id":"a","display_name":"Model A"}],"has_more":true,"last_id":"a"}"#,
                r#"{"data":[{"id":"b"}],"has_more":false}"#,
            ],
            "anthropic",
        );
        let result = fetch("anthropic", &base, "test-key").unwrap();
        handle.join().unwrap();
        assert_eq!(result.len(), 2);
        assert_eq!(result[0].name, "Model A");
    }
    #[test]
    fn malformed_or_empty_catalog_is_not_success() {
        for response in [r#"{"error":"private provider detail"}"#, r#"{"data":[]}"#] {
            let (base, handle) = server(vec![response], "openai");
            let error = fetch("openai", &(base + "/v1"), "test-key").err().unwrap();
            handle.join().unwrap();
            assert!(!error.contains("private provider detail"));
        }
    }
    #[test]
    fn redirects_do_not_forward_credentials() {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = format!("http://{}", listener.local_addr().unwrap());
        let handle = std::thread::spawn(move || {
            let (mut socket, _) = listener.accept().unwrap();
            let mut data = [0; 4096];
            let _ = socket.read(&mut data).unwrap();
            socket.write_all(b"HTTP/1.1 302 Found\r\nLocation: https://untrusted.example/models\r\nContent-Length: 0\r\nConnection: close\r\n\r\n").unwrap();
        });
        let error = fetch("openai", &address, "test-key").err().unwrap();
        handle.join().unwrap();
        assert!(error.contains("302"));
    }
}
