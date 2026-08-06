import type { Message } from "@bufbuild/protobuf";

/**
 * Convert a proto Message to a plain JS object with:
 * - snake_case field names (useProtoFieldName)
 * - int64 as number (protobuf v1 JSON format uses number for int64)
 * - Nested messages and repeated fields recursively converted
 *
 * This keeps the same shape that components currently expect
 * (snake_case keys, number values) so migration only requires
 * changing the import path and removing `.data` accessor.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function toPlain(msg: Message): any {
  const json = msg.toJson({ useProtoFieldName: true });
  return normalizeProtoJson(json);
}

/** Recursively normalize proto JSON values. */
function normalizeProtoJson(value: unknown): unknown {
  if (value === null || value === undefined) return value;
  if (typeof value === "string") {
    // Proto3 JSON v1 encodes int64 as decimal strings ("12345")
    // Convert back to number for compatibility with existing components.
    if (/^-?\d+$/.test(value) && Number.isSafeInteger(Number(value))) {
      return Number(value);
    }
    return value;
  }
  if (typeof value === "bigint") {
    return Number(value);
  }
  if (Array.isArray(value)) {
    return value.map(normalizeProtoJson);
  }
  if (typeof value === "object") {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      if (v !== undefined && v !== null) {
        out[k] = normalizeProtoJson(v);
      }
    }
    return out;
  }
  return value;
}
