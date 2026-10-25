// Fork: blurhash helpers on top of fast-blurhash, which replaced the blurhash package.

const BASE83 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz#$%*+,-.:;=?@[]^_{|}~";

// isBlurhashValid checks a hash's length against the component counts in its first character,
// which is what the blurhash package checked. Some MediUX hashes are truncated.
export function isBlurhashValid(hash: string): { result: boolean; errorReason?: string } {
  if (!hash || hash.length < 6) return { result: false, errorReason: "blurhash is too short" };
  const sizeFlag = BASE83.indexOf(hash[0]);
  if (sizeFlag < 0) return { result: false, errorReason: "blurhash has an invalid size flag" };
  const numX = (sizeFlag % 9) + 1;
  const numY = Math.floor(sizeFlag / 9) + 1;
  const expected = 4 + 2 * numX * numY;
  if (hash.length !== expected) {
    return { result: false, errorReason: `blurhash length is ${hash.length}, expected ${expected}` };
  }
  return { result: true };
}
