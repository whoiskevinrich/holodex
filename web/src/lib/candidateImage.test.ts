import { describe, expect, it } from "vitest";
import { showThumb } from "./candidateImage";

// F63 slot rule (design handoff "Content spec"): image when the server let an
// http(s) image_url through and the <img> has not errored; monogram otherwise.
describe("showThumb", () => {
  it("shows the image for an http(s) image_url that has not failed", () => {
    expect(
      showThumb({ image_url: "https://cdn.acme.example/t/w185/1.jpg" }, false),
    ).toBe(true);
    expect(
      showThumb(
        { image_url: "http://stub:9100/p/twins/thumb/portrait-1.png" },
        false,
      ),
    ).toBe(true);
  });
  it("falls back to the monogram when the key is absent (pre-F63 provider or stripped by core)", () => {
    expect(showThumb({}, false)).toBe(false);
    expect(showThumb({ image_url: undefined }, false)).toBe(false);
  });
  it("falls back to the monogram once the image errored", () => {
    expect(
      showThumb({ image_url: "https://cdn.acme.example/t/w185/404.jpg" }, true),
    ).toBe(false);
  });
  it("never renders a non-http(s) value even if one slipped past the server", () => {
    expect(showThumb({ image_url: "javascript:alert(1)" }, false)).toBe(false);
    expect(showThumb({ image_url: "data:image/png;base64,AAAA" }, false)).toBe(
      false,
    );
    expect(showThumb({ image_url: "" }, false)).toBe(false);
  });
});
