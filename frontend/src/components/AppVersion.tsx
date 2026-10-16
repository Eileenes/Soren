import { version } from '../../package.json';

export function AppVersion() {
  return <span className="app-version" aria-label={`Soren 版本 ${version}`}>v{version}</span>;
}
