// Registry manifests use sha256:<hex>; Ollama /api/tags returns plain hex.
// Normalize only this known prefix and reject malformed digests.
export function sameDigest(actual, expected) {
  const normalize = value => {
    if (typeof value !== 'string') return null;
    const hex = value.replace(/^sha256:/, '');
    return /^[a-f0-9]{64}$/.test(hex) ? hex : null;
  };
  const digest = normalize(actual);
  return digest !== null && digest === normalize(expected);
}
