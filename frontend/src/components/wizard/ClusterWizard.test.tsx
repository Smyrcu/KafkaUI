import { describe, it, expect } from "vitest";
import { validateBootstrap, validateName } from "./steps/ConnectionStep";

describe("ConnectionStep validation", () => {
  describe("validateBootstrap", () => {
    it("accepts valid host:port", () => {
      expect(validateBootstrap("localhost:9092")).toBeNull();
    });
    it("accepts multiple brokers", () => {
      expect(validateBootstrap("broker1:9092,broker2:9093")).toBeNull();
    });
    it("rejects missing port", () => {
      expect(validateBootstrap("localhost")).not.toBeNull();
    });
    it("rejects invalid port", () => {
      expect(validateBootstrap("localhost:99999")).not.toBeNull();
    });
    it("rejects random string", () => {
      expect(validateBootstrap("123094112013")).not.toBeNull();
    });
    it("returns null for empty", () => {
      expect(validateBootstrap("")).toBeNull();
    });
  });

  describe("validateName", () => {
    it("accepts alphanumeric with hyphens", () => {
      expect(validateName("my-cluster-1")).toBeNull();
    });
    it("rejects spaces", () => {
      expect(validateName("my cluster")).not.toBeNull();
    });
    it("rejects special chars", () => {
      expect(validateName("cluster@!")).not.toBeNull();
    });
    it("returns null for empty", () => {
      expect(validateName("")).toBeNull();
    });
  });
});
