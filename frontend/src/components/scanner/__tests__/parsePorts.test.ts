import { describe, it, expect } from "vitest";
import { parsePorts } from "@/components/scanner/ports";

describe("parsePorts", () => {
  it('parses a single port "22"', () => {
    expect(parsePorts("22")).toEqual([22]);
  });

  it('parses a comma-separated list "22,2222"', () => {
    expect(parsePorts("22,2222")).toEqual([22, 2222]);
  });

  it('expands a range "22241-22250"', () => {
    expect(parsePorts("22241-22250")).toEqual([
      22241, 22242, 22243, 22244, 22245, 22246, 22247, 22248, 22249, 22250,
    ]);
  });

  it('parses mixed ports and ranges "22,22241-22242"', () => {
    expect(parsePorts("22,22241-22242")).toEqual([22, 22241, 22242]);
  });

  it('handles whitespace around separators "22, 2222"', () => {
    expect(parsePorts("22, 2222")).toEqual([22, 2222]);
  });

  it('expands a spaced range "22241 - 22250"', () => {
    expect(parsePorts("22241 - 22250")).toEqual([
      22241, 22242, 22243, 22244, 22245, 22246, 22247, 22248, 22249, 22250,
    ]);
    // Mutation check: without dash-space normalization the spaced range splits
    // into ["22241", "-", "22250"] — the lone "-" token is garbage and the
    // output would be [] — this fails.
  });

  it("returns [] for a blank input (backend default [22])", () => {
    expect(parsePorts("")).toEqual([]);
    expect(parsePorts("   ")).toEqual([]);
  });

  it("skips invalid tokens (non-numeric, out of range, reversed ranges)", () => {
    expect(parsePorts("abc")).toEqual([]);
    expect(parsePorts("abc,22,x")).toEqual([22]);
    // out of 1..65535
    expect(parsePorts("0")).toEqual([]);
    expect(parsePorts("70000")).toEqual([]);
    expect(parsePorts("-5")).toEqual([]);
    // reversed / malformed ranges
    expect(parsePorts("22-10")).toEqual([]);
    expect(parsePorts("22-")).toEqual([]);
    expect(parsePorts("1-2-3")).toEqual([]);
  });

  it("drops invalid tokens but keeps the valid ones around them", () => {
    expect(parsePorts("22, abc, 22241-22242, 99999")).toEqual([22, 22241, 22242]);
  });

  // Mutation check: if parsePorts stopped warning on an empty string and
  // returned something non-empty, the "blank = default" behaviour (the backend
  // treats an empty Ports as [22]) would be silently broken — a blank input
  // must produce an empty array, never a surprise default port.
});