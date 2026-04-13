import { describe, expect, test } from "bun:test";

import {
  base64ToPublicKey,
  CHUNK_HEADER_BYTES,
  computeChunkCount,
  DEFAULT_CHUNK_SIZE,
  decryptChunk,
  deriveTransferKey,
  encryptChunk,
  generateKeyPair,
  publicKeyToBase64,
  sessionCodeToSalt,
  sha256Hex,
  sha256HexIncremental,
  sliceChunk,
} from "../src/index";

describe("generateKeyPair", () => {
  test("produces a key pair and raw public key", async () => {
    const result = await generateKeyPair();

    expect(result.keyPair.privateKey).toBeDefined();
    expect(result.keyPair.publicKey).toBeDefined();
    expect(result.publicKeyRaw.byteLength).toBeGreaterThan(0);
  });

  test("two key pairs produce different public keys", async () => {
    const a = await generateKeyPair();
    const b = await generateKeyPair();

    expect(publicKeyToBase64(a.publicKeyRaw)).not.toBe(
      publicKeyToBase64(b.publicKeyRaw),
    );
  });
});

describe("public key encoding", () => {
  test("round-trips through base64", async () => {
    const { publicKeyRaw } = await generateKeyPair();
    const encoded = publicKeyToBase64(publicKeyRaw);
    const decoded = base64ToPublicKey(encoded);

    expect(new Uint8Array(decoded)).toEqual(new Uint8Array(publicKeyRaw));
  });
});

describe("deriveTransferKey", () => {
  test("both peers derive the same key", async () => {
    const alice = await generateKeyPair();
    const bob = await generateKeyPair();
    const salt = sessionCodeToSalt("amber-anchor-apex");

    const keyA = await deriveTransferKey(
      alice.keyPair.privateKey,
      bob.publicKeyRaw,
      salt,
    );
    const keyB = await deriveTransferKey(
      bob.keyPair.privateKey,
      alice.publicKeyRaw,
      salt,
    );

    const plaintext = new TextEncoder().encode("test message");
    const nonce = crypto.getRandomValues(new Uint8Array(12));

    const ciphertextA = await crypto.subtle.encrypt(
      { name: "AES-GCM", iv: nonce },
      keyA,
      plaintext,
    );

    const decryptedB = await crypto.subtle.decrypt(
      { name: "AES-GCM", iv: nonce },
      keyB,
      ciphertextA,
    );

    expect(new Uint8Array(decryptedB)).toEqual(plaintext);
  });
});

describe("chunk encryption", () => {
  test("encrypt then decrypt round-trips", async () => {
    const alice = await generateKeyPair();
    const bob = await generateKeyPair();
    const salt = sessionCodeToSalt("delta-drift-ember");
    const key = await deriveTransferKey(
      alice.keyPair.privateKey,
      bob.publicKeyRaw,
      salt,
    );

    const plaintext = new TextEncoder().encode("hello bore v2");
    const frame = await encryptChunk(key, 0, plaintext.buffer);

    expect(frame.byteLength).toBeGreaterThan(
      CHUNK_HEADER_BYTES + plaintext.byteLength,
    );

    const decrypted = await decryptChunk(key, frame);
    expect(decrypted.chunkIndex).toBe(0);
    expect(new Uint8Array(decrypted.plaintext)).toEqual(plaintext);
  });

  test("chunk index is preserved", async () => {
    const alice = await generateKeyPair();
    const bob = await generateKeyPair();
    const salt = sessionCodeToSalt("flux-forest-frost");
    const key = await deriveTransferKey(
      alice.keyPair.privateKey,
      bob.publicKeyRaw,
      salt,
    );

    const plaintext = new Uint8Array([1, 2, 3, 4]);
    const frame = await encryptChunk(key, 42, plaintext.buffer);
    const decrypted = await decryptChunk(key, frame);

    expect(decrypted.chunkIndex).toBe(42);
  });

  test("wrong key fails decryption", async () => {
    const alice = await generateKeyPair();
    const bob = await generateKeyPair();
    const carol = await generateKeyPair();
    const salt = sessionCodeToSalt("glow-harbor-helix");

    const keyAB = await deriveTransferKey(
      alice.keyPair.privateKey,
      bob.publicKeyRaw,
      salt,
    );
    const keyAC = await deriveTransferKey(
      alice.keyPair.privateKey,
      carol.publicKeyRaw,
      salt,
    );

    const plaintext = new TextEncoder().encode("secret data");
    const frame = await encryptChunk(keyAB, 0, plaintext.buffer);

    await expect(decryptChunk(keyAC, frame)).rejects.toThrow();
  });

  test("rejects frame that is too small", async () => {
    const alice = await generateKeyPair();
    const bob = await generateKeyPair();
    const salt = sessionCodeToSalt("iris-jade-kepler");
    const key = await deriveTransferKey(
      alice.keyPair.privateKey,
      bob.publicKeyRaw,
      salt,
    );

    const tooSmall = new ArrayBuffer(4);
    await expect(decryptChunk(key, tooSmall)).rejects.toThrow(
      "frame too small",
    );
  });
});

describe("sha256Hex", () => {
  test("produces correct digest for empty input", async () => {
    const digest = await sha256Hex(new ArrayBuffer(0));
    expect(digest).toBe(
      "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    );
  });

  test("produces correct digest for known input", async () => {
    const data = new TextEncoder().encode("bore");
    const digest = await sha256Hex(data.buffer);
    expect(digest).toHaveLength(64);
    expect(digest).toMatch(/^[a-f0-9]{64}$/);
  });
});

describe("sha256HexIncremental", () => {
  test("matches single-pass digest", async () => {
    const data = new TextEncoder().encode("bore v2 transfer");
    const singlePass = await sha256Hex(data.buffer);

    const hasher = await sha256HexIncremental();
    hasher.update(new TextEncoder().encode("bore ").buffer);
    hasher.update(new TextEncoder().encode("v2 ").buffer);
    hasher.update(new TextEncoder().encode("transfer").buffer);
    const incremental = await hasher.finalize();

    expect(incremental).toBe(singlePass);
  });
});

describe("chunking utilities", () => {
  test("computeChunkCount for empty file", () => {
    expect(computeChunkCount(0)).toBe(1);
  });

  test("computeChunkCount for exact multiple", () => {
    expect(computeChunkCount(DEFAULT_CHUNK_SIZE * 3)).toBe(3);
  });

  test("computeChunkCount for partial last chunk", () => {
    expect(computeChunkCount(DEFAULT_CHUNK_SIZE * 2 + 1)).toBe(3);
  });

  test("sliceChunk returns correct ranges", () => {
    const data = new Uint8Array(600);
    for (let i = 0; i < 600; i++) data[i] = i % 256;

    const chunk0 = sliceChunk(data.buffer, 0, 256);
    const chunk1 = sliceChunk(data.buffer, 1, 256);
    const chunk2 = sliceChunk(data.buffer, 2, 256);

    expect(chunk0.byteLength).toBe(256);
    expect(chunk1.byteLength).toBe(256);
    expect(chunk2.byteLength).toBe(88);

    expect(new Uint8Array(chunk0)[0]).toBe(0);
    expect(new Uint8Array(chunk1)[0]).toBe(0);
    expect(new Uint8Array(chunk2)[0]).toBe(0);
  });
});
