import assert from 'node:assert/strict';
import { test } from 'node:test';
import { sameDigest } from './model-digest.mjs';

test('compares registry and Ollama digests without weakening validation', () => {
  const hex = '7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab';
  assert.ok(sameDigest(hex, `sha256:${hex}`));
  assert.ok(sameDigest(`sha256:${hex}`, `sha256:${hex}`));
  assert.equal(sameDigest('0'.repeat(64), `sha256:${hex}`), false);
  for (const invalid of [null, undefined, '', 'sha256:', 'sha512:' + hex, hex.slice(0, 12)]) {
    assert.equal(sameDigest(invalid, invalid), false);
    assert.equal(sameDigest(invalid, hex), false);
  }
});
