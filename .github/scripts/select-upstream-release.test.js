const assert = require('node:assert/strict');
const { test } = require('node:test');
const { selectUpstreamRelease } = require('./select-upstream-release');

const release = (tag_name, attributes = {}) => ({ tag_name, ...attributes });
const upstream = [[release('v0.10.0'), release('v0.9.6'), release('v0.9.7', { draft: true })], [
  release('v0.9.4'), release('v0.9.5'), release('v0.11.0', { prerelease: true }),
]];

test('takes the oldest stable update after the last published MACC release', () => {
  assert.equal(selectUpstreamRelease(upstream, [[release('v0.9.4-macc'), release('v0.9.5-macc', { draft: true })]]), 'v0.9.5');
  assert.equal(selectUpstreamRelease(upstream, [[release('v0.9.6-macc')]]), 'v0.10.0');
});

test('uses the latest stable tag on bootstrap or when there is no update', () => {
  assert.equal(selectUpstreamRelease(upstream, [[]]), 'v0.10.0');
  assert.equal(selectUpstreamRelease(upstream, [[release('v0.10.0-macc')]]), 'v0.10.0');
});
