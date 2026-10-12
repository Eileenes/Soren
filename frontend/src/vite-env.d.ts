/// <reference types="vite/client" />

/**
 * Vite 环境变量类型声明
 *
 * 只声明本项目实际使用的变量，避免 import.meta.env 退化为 any。
 */
interface ImportMetaEnv {
  /** 后端 API 基础地址，默认 http://localhost:8080 */
  readonly VITE_API_BASE_URL?: string;
  /** 桌面端单用户标识，默认 default-user */
  readonly VITE_USER_ID?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
