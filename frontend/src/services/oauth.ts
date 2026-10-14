import { invoke } from '@tauri-apps/api/core';

export type OAuthEnvironment = 'development' | 'test' | 'production';
export type ClientAuthMethod = 'client_secret_basic' | 'client_secret_post' | 'none';

export interface OAuthSettings {
  environment: OAuthEnvironment;
  authorizationEndpoint: string;
  tokenEndpoint: string;
  userInfoEndpoint: string;
  clientId: string;
  redirectUri: string;
  scope: string;
  clientAuthMethod: ClientAuthMethod;
}

export interface OAuthConfig {
  settings: OAuthSettings;
  configured: boolean;
  hasClientSecret: boolean;
}

export const DEFAULT_OAUTH_SETTINGS: OAuthSettings = {
  environment: 'development',
  authorizationEndpoint: 'https://auth.chaitin.net/oauth2/auth',
  tokenEndpoint: 'https://auth.chaitin.net/oauth2/token',
  userInfoEndpoint: 'https://auth.chaitin.net/userinfo',
  clientId: '',
  redirectUri: '',
  scope: 'openid profile email',
  clientAuthMethod: 'client_secret_basic',
};

/** Read commands return only public settings and a secret-presence flag. */
export const oauthConfigAPI = {
  get: () => invoke<OAuthConfig>('get_oauth_config'),
  save: (settings: OAuthSettings, clientSecret?: string) => invoke<OAuthConfig>('save_oauth_config', {
    config: { settings, ...(clientSecret ? { clientSecret } : {}) },
  }),
};

export function canKeepOAuthSecret(saved: OAuthConfig | null, current: OAuthSettings): boolean {
  return Boolean(saved?.hasClientSecret && current.clientAuthMethod !== 'none' &&
    (['environment', 'clientId', 'authorizationEndpoint', 'tokenEndpoint', 'userInfoEndpoint', 'clientAuthMethod'] as const)
      .every(key => saved.settings[key] === current[key].trim()));
}
