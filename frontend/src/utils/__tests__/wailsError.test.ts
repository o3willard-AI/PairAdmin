import { describe, it, expect } from "vitest";
import { wailsErrorMessage } from "@/utils/wailsError";

describe("wailsErrorMessage", () => {
  it("returns the raw string for a plain-string rejection (the real Wails v2 shape)", () => {
    // Wails v2's dispatcher sets callbackMessage.Err = err.Error() (a Go string)
    // and calls.js does reject(message.error) with that raw string directly —
    // never a JS Error instance. This is the shape every real RPC failure has.
    expect(wailsErrorMessage("ssh: unable to authenticate", "fallback")).toBe(
      "ssh: unable to authenticate"
    );
  });

  it("returns .message for a genuine Error instance", () => {
    expect(wailsErrorMessage(new Error("boom"), "fallback")).toBe("boom");
  });

  it("falls back for an empty string", () => {
    expect(wailsErrorMessage("", "fallback")).toBe("fallback");
  });

  it("falls back for non-string, non-Error values", () => {
    expect(wailsErrorMessage(null, "fallback")).toBe("fallback");
    expect(wailsErrorMessage(undefined, "fallback")).toBe("fallback");
    expect(wailsErrorMessage({ code: 500 }, "fallback")).toBe("fallback");
  });
});
