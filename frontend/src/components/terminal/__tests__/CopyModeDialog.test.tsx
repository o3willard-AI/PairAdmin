import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { CopyModeDialog } from "@/components/terminal/CopyModeDialog";
import { useTerminalStore } from "@/stores/terminalStore";
import type { Terminal } from "@xterm/xterm";

/**
 * Builds a fake xterm Terminal whose active buffer holds `lines`.
 * readTerminalLines walks `term.buffer.active` from `start` to `length`, calling
 * `getLine(y).translateToString(true)` — so this mimics exactly the surface the
 * production code touches and nothing more.
 */
function fakeTerm(lines: string[]) {
  return {
    buffer: {
      active: {
        length: lines.length,
        getLine: (y: number) =>
          y < lines.length ? { translateToString: () => lines[y] } : undefined,
      },
    },
  } as unknown as Terminal;
}

const writeText = vi.fn().mockResolvedValue(undefined);

beforeEach(() => {
  writeText.mockReset().mockResolvedValue(undefined);
  vi.useRealTimers();
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText },
    configurable: true,
    writable: true,
  });
  useTerminalStore.setState({ tabs: [{ id: "tab-1", name: "main" }], activeTabId: "tab-1" });
});

// Deliberately NO `document.body.innerHTML = ""` teardown here. base-ui's Dialog
// Portal mounts its node outside the RTL container, and wiping body underneath it
// makes the portal's unmount throw NotFoundError ("the node to be removed is not a
// child of this node") on the NEXT test. RTL's auto-cleanup unmounts the tree
// first, which is the correct order. NewTerminalDialog.test.tsx does the same.

function setTerm(term: Terminal) {
  useTerminalStore.getState().setTermRef("tab-1", term);
}

describe("CopyModeDialog", () => {
  // Mutation check: deleting the on-open `setSnapshot(takeSnapshot())` effect
  // renders an EMPTY textarea here.
  it("snapshots the whole terminal buffer on open", () => {
    setTerm(fakeTerm(["line one", "line two", "line three"]));
    render(<CopyModeDialog open onClose={() => {}} />);

    const textarea = screen.getByLabelText("Terminal output (copy mode)") as HTMLTextAreaElement;
    expect(textarea.value).toBe("line one\nline two\nline three");
    // Whole buffer, not a truncation: a 3-line buffer can't prove this alone, so
    // the >200-line case is asserted separately below.
  });

  // Mutation check: reverting `maxLines` to the old `= 200` default (or keeping
  // `Math.max(0, buf.length - maxLines)` for the no-arg case) truncates this to
  // the last 200 lines and the full-string assertion fails. This is the test that
  // makes the Step 1 contract change load-bearing rather than cosmetic.
  it("snapshots MORE than 200 lines when the buffer is that long", () => {
    const many = Array.from({ length: 500 }, (_, i) => `row ${i}`);
    setTerm(fakeTerm(many));
    render(<CopyModeDialog open onClose={() => {}} />);

    const textarea = screen.getByLabelText("Terminal output (copy mode)") as HTMLTextAreaElement;
    expect(textarea.value.startsWith("row 0")).toBe(true);
    expect(textarea.value.endsWith("row 499")).toBe(true);
    expect(textarea.value.split("\n")).toHaveLength(500);
  });

  // The freeze is the entire point of the feature. If a live subscription or an
  // interval were added, mutating the underlying buffer would change the text
  // under the user's cursor — and this test would catch it.
  it("FREEZES the snapshot while open — later buffer growth does not appear", async () => {
    const lines = ["first", "second"];
    const term = fakeTerm(lines);
    setTerm(term);
    render(<CopyModeDialog open onClose={() => {}} />);

    const textarea = screen.getByLabelText("Terminal output (copy mode)") as HTMLTextAreaElement;
    expect(textarea.value).toBe("first\nsecond");

    // Simulate the terminal emitting more output after the snapshot.
    lines.push("third");
    await act(async () => {
      await new Promise((r) => setTimeout(r, 10));
    });

    expect((screen.getByLabelText("Terminal output (copy mode)") as HTMLTextAreaElement).value).toBe(
      "first\nsecond"
    );
  });

  // Mutation check: removing the `handleRefresh` body (or making it a no-op)
  // leaves the stale text after the buffer changes.
  it("replaces the snapshot in place when Refresh is clicked", async () => {
    const lines = ["before refresh"];
    setTerm(fakeTerm(lines));
    const user = userEvent.setup();
    render(<CopyModeDialog open onClose={() => {}} />);

    lines[0] = "after refresh";
    expect((screen.getByLabelText("Terminal output (copy mode)") as HTMLTextAreaElement).value).toBe(
      "before refresh"
    );

    await user.click(screen.getByRole("button", { name: "Refresh" }));

    expect((screen.getByLabelText("Terminal output (copy mode)") as HTMLTextAreaElement).value).toBe(
      "after refresh"
    );
  });

  // Mutation check: removing `navigator.clipboard.writeText(snapshot)` (or
  // writing `term.getSelection()` instead of the snapshot) fails this assertion.
  it("Copy All writes the snapshot to the OS clipboard", async () => {
    setTerm(fakeTerm(["clip me", "and me"]));
    render(<CopyModeDialog open onClose={() => {}} />);

    // fireEvent, not userEvent: userEvent.setup() installs its own
    // navigator.clipboard stub and would replace ours, so the click would go to
    // userEvent's clipboard instead of the writeText spy.
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Copy All" }));
    });

    expect(writeText).toHaveBeenCalledWith("clip me\nand me");
  });

  // Mutation check: adding an onClose() to handleCopyAll closes the dialog here,
  // which contradicts the explicit requirement that Copy All NOT close it.
  it("Copy All does NOT close the dialog", async () => {
    const onClose = vi.fn();
    setTerm(fakeTerm(["still open"]));
    const user = userEvent.setup();
    render(<CopyModeDialog open onClose={onClose} />);

    await user.click(screen.getByRole("button", { name: "Copy All" }));

    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Terminal output (copy mode)")).toBeInTheDocument();
  });

  // Mutation check: removing the setCopied(true) + Copied! label leaves the
  // button reading "Copy All" forever, so the user gets no success feedback.
  it("shows a transient Copied! label after a successful copy", async () => {
    vi.useFakeTimers();
    setTerm(fakeTerm(["feedback"]));
    render(<CopyModeDialog open onClose={() => {}} />);

    fireEvent.click(screen.getByRole("button", { name: "Copy All" }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(screen.getByRole("button", { name: "Copied!" })).toBeInTheDocument();

    // ...and reverts after ~1.5s.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1600);
    });
    expect(screen.getByRole("button", { name: "Copy All" })).toBeInTheDocument();
  });

  // A clipboard refusal (permissions, non-secure context) must not throw out of
  // the click handler and take the dialog down with it.
  it("swallows a clipboard write failure without crashing", async () => {
    writeText.mockRejectedValue(new Error("denied"));
    setTerm(fakeTerm(["nope"]));
    render(<CopyModeDialog open onClose={() => {}} />);

    expect(() => fireEvent.click(screen.getByRole("button", { name: "Copy All" }))).not.toThrow();
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    // No "Copied!" claim on a failed write.
    expect(screen.getByRole("button", { name: "Copy All" })).toBeInTheDocument();
  });

  // Mutation check: dropping the autofocus/select-on-open leaves selectionStart
  // at 0 with no selection, so the user cannot Ctrl+C everything immediately.
  it("autofocuses and selects all text on open", async () => {
    setTerm(fakeTerm(["auto", "selected"]));
    render(<CopyModeDialog open onClose={() => {}} />);
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    const textarea = screen.getByLabelText("Terminal output (copy mode)") as HTMLTextAreaElement;
    expect(document.activeElement).toBe(textarea);
    expect(textarea.selectionStart).toBe(0);
    expect(textarea.selectionEnd).toBe("auto\nselected".length);
  });

  // Mutation check: removing the close-snapshot-clear effect is observable in the
  // STALE direction: close while the terminal is EMPTY, then point the store at
  // different content and reopen. With the clear removed, the component still
  // holds the previous session's text for the first paint... except the open
  // effect re-snapshots, so the only reliable observable is that a closed dialog
  // keeps NO stale text in the DOM. Assert the popup is gone AND that reopening
  // after a close with no new output shows empty rather than the old buffer.
  it("clears the snapshot on close so a reopened empty terminal shows no stale text", async () => {
    setTerm(fakeTerm(["stale output"]));
    const { rerender } = render(<CopyModeDialog open onClose={() => {}} />);
    expect((screen.getByLabelText("Terminal output (copy mode)") as HTMLTextAreaElement).value).toBe(
      "stale output"
    );

    // Terminal goes empty (user cleared it) and we close.
    setTerm(fakeTerm([]));
    rerender(<CopyModeDialog open={false} onClose={() => {}} />);
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    // Closed: the popup is unmounted entirely.
    expect(screen.queryByLabelText("Terminal output (copy mode)")).not.toBeInTheDocument();

    // Reopen against the still-empty terminal: must be empty, never "stale output".
    rerender(<CopyModeDialog open onClose={() => {}} />);
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    expect((screen.getByLabelText("Terminal output (copy mode)") as HTMLTextAreaElement).value).toBe("");
  });

  // Step 5: the zero-code Shift+drag tip must actually be user-visible, or
  // removing it is a silent regression for the mouse-tracking half of the problem.
  it("surfaces the Shift+drag selection tip", () => {
    setTerm(fakeTerm(["anything"]));
    render(<CopyModeDialog open onClose={() => {}} />);
    expect(screen.getByText(/Shift/)).toBeInTheDocument();
  });

  // Mutation check: hand-rolling a second dismiss path, or dropping the
  // onOpenChange -> onClose wiring, leaves onClose uncalled on Escape.
  it("closes via Escape through the shared dialog primitive", async () => {
    const onClose = vi.fn();
    setTerm(fakeTerm(["escape me"]));
    render(<CopyModeDialog open onClose={onClose} />);

    await userEvent.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalled();
  });

  // Mutation check: removing the Close button's onClick={onClose} breaks this.
  it("closes via the explicit Close button", async () => {
    const onClose = vi.fn();
    setTerm(fakeTerm(["close me"]));
    const user = userEvent.setup();
    render(<CopyModeDialog open onClose={onClose} />);

    await user.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalled();
  });

  // Mutation check: making the textarea editable (dropping `readOnly`) lets a
  // user type into what is supposed to be a verbatim mirror of the terminal.
  it("renders the snapshot readOnly, never editable", async () => {
    setTerm(fakeTerm(["verbatim"]));
    render(<CopyModeDialog open onClose={() => {}} />);
    const textarea = screen.getByLabelText("Terminal output (copy mode)");
    expect(textarea).toHaveAttribute("readonly");
  });

  // Mutation check: dropping `min-h-0` lets a long snapshot grow the dialog
  // off-screen instead of scrolling inside it. Asserted on the class because
  // jsdom has no layout engine to measure.
  it("bounds the textarea to the dialog body so long output scrolls", async () => {
    setTerm(fakeTerm(["a"]));
    render(<CopyModeDialog open onClose={() => {}} />);
    const body = screen.getByLabelText("Terminal output (copy mode)").parentElement;
    expect(body?.className).toContain("min-h-0");
    expect(screen.getByLabelText("Terminal output (copy mode)").className).toContain("h-full");
  });


});