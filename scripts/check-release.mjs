import { readFileSync, existsSync } from 'node:fs';
import assert from 'node:assert/strict';
const read = (path) => readFileSync(new URL(`../${path}`, import.meta.url), 'utf8');
const version = JSON.parse(read('package.json')).version;
assert.match(version, /^\d+\.\d+\.\d+$/);
const tag = process.argv[2] || `v${version}`;
assert.equal(tag, `v${version}`, 'Release tag must match package version');
for (const file of ['frontend/package.json', 'frontend/package-lock.json', 'mobile/package.json', 'frontend/src-tauri/tauri.conf.json', 'mobile/src-tauri/tauri.conf.json']) {
  assert.equal(JSON.parse(read(file)).version, version, `Version mismatch in ${file}`);
}
assert.equal(JSON.parse(read('frontend/package-lock.json')).packages[''].version, version);
for (const prefix of ['frontend', 'mobile']) {
  assert.equal(read(`${prefix}/src-tauri/Cargo.toml`).match(/^version = "([^"]+)"/m)?.[1], version);
  const name = prefix === 'frontend' ? 'soren' : 'soren-mobile';
  assert.ok(read(`${prefix}/src-tauri/Cargo.lock`).includes(`name = "${name}"\nversion = "${version}"`), 'Cargo.lock package version must match');
}
assert.ok(existsSync(new URL(`../docs/releases/v${version}.md`, import.meta.url)), 'Release notes are required');
// These are public build settings. Never place credentials in VITE_* variables.
const ios = process.env.VITE_SOREN_IOS_DOWNLOAD_URL?.trim();
if (ios) {
  const url = new URL(ios);
  assert.ok(url.protocol === 'https:' && !url.username && !url.password && !url.port && !url.search && !url.hash);
  assert.ok((url.hostname === 'testflight.apple.com' && /^\/join\/[a-zA-Z0-9]{8}\/?$/.test(url.pathname)) ||
    (url.hostname === 'apps.apple.com' && /^\/(?:[a-z]{2}\/)?app\/(?:[^/]+\/)?id\d+\/?$/.test(url.pathname)), 'Use an official TestFlight or App Store link');
  assert.ok(process.env.SOREN_RELEASE_RELAY_URL && process.env.SOREN_RELEASE_MOBILE_URL, 'iOS distribution requires public pairing services');
}
if (process.env.SOREN_RELEASE_RELAY_URL || process.env.SOREN_RELEASE_MOBILE_URL || process.env.VITE_SOREN_RELAY_URL) {
  for (const name of ['SOREN_RELEASE_RELAY_URL', 'SOREN_RELEASE_MOBILE_URL', 'VITE_SOREN_RELAY_URL']) {
    const url = new URL(process.env[name]);
    assert.ok(url.protocol === 'https:' && !url.username && !url.password && !url.port && !url.search && !url.hash && ['','/'].includes(url.pathname), `${name} must be a plain HTTPS origin`);
    assert.ok(!['localhost', 'example.com'].includes(url.hostname) && !url.hostname.endsWith('.example.com'), `${name} must be a real public service`);
  }
  assert.equal(new URL(process.env.SOREN_RELEASE_RELAY_URL).origin, new URL(process.env.VITE_SOREN_RELAY_URL).origin, 'Desktop and mobile relay origins must match');
}
console.log(`Release configuration verified: ${tag}`);
