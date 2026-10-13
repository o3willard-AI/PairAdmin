import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, act, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { CopyModeDialog } from "@/components/terminal/CopyModeDialog";
import { useTerminalStore } from "@/stores/terminalStore";
import type { Terminal } from "@xterm/xterm";

// The two backend bindings the dialog reaches for. Mocked at module level and
// routed through spies so the tests can assert BOTH that the call happened and
// what it was handed — the filtered view's whole contract is "the backend's own
// pipeline, over the same window the context builder uses".
const filterText = vi.fn();
const getSettings = vi.fn();
vi.mock("../../../../wailsjs/go/services/LLMService", () => ({
  FilterText: (...args: unknown[]) => filterText(...args),
}));
vi.mock("../../../../wailsjs/go/services/SettingsService", () => ({
  GetSettings: (...args: unknown[]) => getSettings(...args),
}));

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
  filterText.mockReset().mockResolvedValue("filtered output");
  getSettings.mockReset().mockResolvedValue({});
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

/** The complete-buffer textarea (the view that is active by default). */
function completeArea() {
  return screen.getByLabelText("Terminal output (copy mode)") as HTMLTextAreaElement;
}

/** Switches to the filtered view and waits for the backend round trip to land. */
async function openFilteredView() {
  const user = userEvent.setup();
  await user.click(screen.getByRole("tab", { name: "Filtered LLM buffer" }));
  await act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });
  return user;
}

describe("CopyModeDialog", () => {
  // Mutation check: deleting the on-open `setSnapshot(takeSnapshot())` effect
  // renders an EMPTY textarea here.
  it("snapshots the whole terminal buffer on open", () => {
    setTerm(fakeTerm(["line one", "line two", "line three"]));
    render(<CopyModeDialog open onClose={() => {}} />);

    expect(completeArea().value).toBe("line one\nline two\nline three");
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

    const value = completeArea().value;
    expect(value.startsWith("row 0")).toBe(true);
    expect(value.endsWith("row 499")).toBe(true);
    expect(value.split("\n")).toHaveLength(500);
  });

  // The freeze is the entire point of the feature. If a live subscription or an
  // interval were added, mutating the underlying buffer would change the text
  // under the user's cursor — and this test would catch it.
  it("FREEZES the snapshot while open — later buffer growth does not appear", async () => {
    const lines = ["first", "second"];
    const term = fakeTerm(lines);
    setTerm(term);
    render(<CopyModeDialog open onClose={() => {}} />);

    expect(completeArea().value).toBe("first\nsecond");

    // Simulate the terminal emitting more output after the snapshot.
    lines.push("third");
    await act(async () => {
      await new Promise((r) => setTimeout(r, 10));
    });

    expect(completeArea().value).toBe("first\nsecond");
  });

  // Mutation check: removing the `handleRefresh` body (or making it a no-op)
  // leaves the stale text after the buffer changes.
  it("replaces the snapshot in place when Refresh is clicked", async () => {
    const lines = ["before refresh"];
    setTerm(fakeTerm(lines));
    const user = userEvent.setup();
    render(<CopyModeDialog open onClose={() => {}} />);

    lines[0] = "after refresh";
    expect(completeArea().value).toBe("before refresh");

    await user.click(screen.getByRole("button", { name: "Refresh" }));

    expect(completeArea().value).toBe("after refresh");
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

  // Mutation check (the one this task named): restoring `el.select()` in the
  // focus effect fails this test. The spy is what catches it — base-ui's focus
  // manager focuses the textarea a tick AFTER the component's own effect, so a
  // re-added select() is collapsed again and leaves no trace in the caret state
  // by the time we look. Focus must survive; the selection must not.
  it("autofocuses on open and selects NOTHING", async () => {
    setTerm(fakeTerm(["auto", "selected"]));
    const selectSpy = vi.spyOn(HTMLTextAreaElement.prototype, "select");
    // Mounted CLOSED and then opened, which is how the app uses it (the dialog
    // lives in the layout and the hotkey flips `open`). Mounting it already-open
    // takes base-ui's mount-time focus path instead of its open transition, and
    // the two disagree about where focus lands.
    const { rerender } = render(<CopyModeDialog open={false} onClose={() => {}} />);
    rerender(<CopyModeDialog open onClose={() => {}} />);
    // waitFor, not a single tick: base-ui's focus manager moves focus
    // asynchronously after the popup mounts, so asserting once would be a race.
    const textarea = completeArea();
    await waitFor(() => expect(document.activeElement).toBe(textarea));
    // NOTHING selected: the caret is collapsed wherever the focus landed. Where it
    // sits is the browser's business (a textarea with content typically takes the
    // caret at the end) — what matters is that no text is selected, so a stray
    // Ctrl+C cannot dump the whole buffer onto the clipboard.
    expect(textarea.selectionEnd - textarea.selectionStart).toBe(0);
    // ...and specifically not the whole buffer, which is what el.select() did.
    expect(textarea.selectionEnd - textarea.selectionStart).not.toBe(textarea.value.length);

    // The same must hold when the VIEW changes: on open it is the dialog's focus
    // manager that moves focus (the panel's textarea is not attached yet when the
    // component's own effect runs), but on a view switch the component's effect is
    // the one that focuses the newly mounted textarea — so the spy below is what
    // catches a re-added el.select(), which the open path alone would not.
    filterText.mockResolvedValue("filtered text");
    await openFilteredView();
    const filtered = screen.getByLabelText(
      "Filtered LLM buffer (copy mode)"
    ) as HTMLTextAreaElement;
    await waitFor(() => expect(document.activeElement).toBe(filtered));
    expect(filtered.selectionEnd - filtered.selectionStart).toBe(0);
    expect(selectSpy).not.toHaveBeenCalled();
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
    expect(completeArea().value).toBe("stale output");

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
    expect(completeArea().value).toBe("");
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
    expect(completeArea()).toHaveAttribute("readonly");
  });

  // Mutation check: dropping `min-h-0` lets a long snapshot grow the dialog
  // off-screen instead of scrolling inside it. Asserted on the class because
  // jsdom has no layout engine to measure.
  it("bounds the textarea to the dialog body so long output scrolls", async () => {
    setTerm(fakeTerm(["a"]));
    render(<CopyModeDialog open onClose={() => {}} />);
    const body = completeArea().parentElement;
    expect(body?.className).toContain("min-h-0");
    expect(completeArea().className).toContain("h-full");
  });

  // ---- Change 1: the two views (UAT) -------------------------------------

  // Mutation check: removing either Tabs.Tab (or renaming a label) fails this —
  // the labels are the ruling, not a styling choice.
  it("renders both views, with Complete buffer active by default", () => {
    setTerm(fakeTerm(["both tabs"]));
    render(<CopyModeDialog open onClose={() => {}} />);

    const complete = screen.getByRole("tab", { name: "Complete buffer" });
    const filtered = screen.getByRole("tab", { name: "Filtered LLM buffer" });
    expect(complete).toHaveAttribute("aria-selected", "true");
    expect(filtered).toHaveAttribute("aria-selected", "false");
    // Tab 1 by default means tab 1's content is what is on screen.
    expect(completeArea().value).toBe("both tabs");
    expect(screen.queryByLabelText("Filtered LLM buffer (copy mode)")).not.toBeInTheDocument();
  });

  // Mutation check: computing the filtered view eagerly on open (instead of on
  // first switch) makes filterText have been called here. An unused view must not
  // pay for a backend round trip — and, more importantly, must not run the
  // redaction pipeline over a buffer the user never asked to inspect.
  it("does not touch the backend until the filtered view is opened", async () => {
    setTerm(fakeTerm(["quiet"]));
    render(<CopyModeDialog open onClose={() => {}} />);
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    expect(filterText).not.toHaveBeenCalled();
  });

  // Mutation check: rendering the raw buffer in the filtered panel, or dropping
  // the FilterText call, fails the value assertion below.
  it("switching to the filtered view shows what FilterText returned", async () => {
    filterText.mockResolvedValue("REDACTED: ghp_[redacted]");
    setTerm(fakeTerm(["secret ghp_abc", "more"]));
    render(<CopyModeDialog open onClose={() => {}} />);

    await openFilteredView();

    expect(filterText).toHaveBeenCalledWith("secret ghp_abc\nmore");
    expect(
      (screen.getByLabelText("Filtered LLM buffer (copy mode)") as HTMLTextAreaElement).value
    ).toBe("REDACTED: ghp_[redacted]");
  });

  // The filtered view claims to be "exactly what the LLM would receive", so it
  // must be capped to the SAME window the context builder uses (ContextLines from
  // settings, ChatPane's default 200 when unset).
  //
  // Mutation check: reading the whole buffer for the filtered view (dropping the
  // maxLines argument) sends all 500 lines and the call assertion fails.
  it("caps the filtered view to ContextLines from settings", async () => {
    getSettings.mockResolvedValue({ ContextLines: 50 });
    const many = Array.from({ length: 500 }, (_, i) => `row ${i}`);
    setTerm(fakeTerm(many));
    render(<CopyModeDialog open onClose={() => {}} />);
    // Let the settings read land before switching.
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    await openFilteredView();

    const sent = filterText.mock.calls[0][0] as string;
    expect(sent.startsWith("row 450")).toBe(true);
    expect(sent.split("\n")).toHaveLength(50);
    expect(sent).not.toContain("row 449");
  });

  // Mutation check: copying `snapshot` unconditionally (instead of the ACTIVE
  // view's text) writes the unfiltered buffer here and fails.
  it("Copy All copies the filtered text while the filtered view is active", async () => {
    filterText.mockResolvedValue("only this");
    setTerm(fakeTerm(["raw buffer"]));
    render(<CopyModeDialog open onClose={() => {}} />);

    await openFilteredView();
    // userEvent.setup() (used above to switch tabs) installs its own
    // navigator.clipboard stub, so re-install ours before the copy click —
    // otherwise the write lands on userEvent's clipboard, not on the spy.
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
      writable: true,
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Copy All" }));
    });

    expect(writeText).toHaveBeenCalledWith("only this");
    expect(writeText).not.toHaveBeenCalledWith("raw buffer");
  });

  // Mutation check: dropping the disabled={copyDisabled} guard lets this click
  // through, and the "Copied!" claim would be a lie about an empty buffer.
  it("disables Copy All until the filtered text has arrived", async () => {
    let resolveFilter: (value: string) => void = () => {};
    filterText.mockReturnValue(
      new Promise<string>((resolve) => {
        resolveFilter = resolve;
      })
    );
    setTerm(fakeTerm(["pending"]));
    render(<CopyModeDialog open onClose={() => {}} />);

    const user = userEvent.setup();
    await user.click(screen.getByRole("tab", { name: "Filtered LLM buffer" }));

    expect(screen.getByRole("button", { name: "Copy All" })).toBeDisabled();
    expect(screen.getByText("Filtering…")).toBeInTheDocument();

    await act(async () => {
      resolveFilter("arrived");
      await new Promise((r) => setTimeout(r, 0));
    });

    expect(screen.getByRole("button", { name: "Copy All" })).toBeEnabled();
    expect(
      (screen.getByLabelText("Filtered LLM buffer (copy mode)") as HTMLTextAreaElement).value
    ).toBe("arrived");
  });

  // Mutation check: swallowing the rejection (or rendering an empty textarea
  // instead of the error) leaves the user looking at an empty box with no idea
  // the redaction never ran — the one failure mode where silence is dangerous.
  it("reports a filter failure instead of showing an empty buffer", async () => {
    filterText.mockRejectedValue("llm: filter unavailable");
    setTerm(fakeTerm(["boom"]));
    render(<CopyModeDialog open onClose={() => {}} />);

    await openFilteredView();

    expect(screen.getByRole("alert")).toHaveTextContent("llm: filter unavailable");
    expect(screen.queryByLabelText("Filtered LLM buffer (copy mode)")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Copy All" })).toBeDisabled();
  });

  // The other two rejection shapes the message helper distinguishes (an Error,
  // and a value carrying no message at all). Both must still put SOMETHING in
  // the alert: an alert with empty text is the same silence the test above
  // exists to prevent, just harder to notice.
  it.each([
    [new Error("llm: bridge error"), "llm: bridge error"],
    [{ code: 7 }, "Filtering failed"],
  ])("renders a %s rejection as alert text", async (rejection, expected) => {
    filterText.mockRejectedValue(rejection);
    setTerm(fakeTerm(["boom"]));
    render(<CopyModeDialog open onClose={() => {}} />);

    await openFilteredView();

    expect(screen.getByRole("alert")).toHaveTextContent(expected);
  });

  // Mutation check: making Refresh re-read the raw snapshot while the filtered
  // view is showing (instead of re-running the filter) leaves stale filtered text
  // on screen after the terminal changed.
  it("Refresh re-runs the filter while the filtered view is showing", async () => {
    filterText.mockResolvedValueOnce("first pass").mockResolvedValueOnce("second pass");
    setTerm(fakeTerm(["one"]));
    render(<CopyModeDialog open onClose={() => {}} />);

    const user = await openFilteredView();
    expect(
      (screen.getByLabelText("Filtered LLM buffer (copy mode)") as HTMLTextAreaElement).value
    ).toBe("first pass");

    await user.click(screen.getByRole("button", { name: "Refresh" }));
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    expect(filterText).toHaveBeenCalledTimes(2);
    expect(
      (screen.getByLabelText("Filtered LLM buffer (copy mode)") as HTMLTextAreaElement).value
    ).toBe("second pass");
  });

  // Mutation check: dropping the request-id guard in runFilter shows "older"
  // here — the slow first request landing last would overwrite the newer result.
  it("ignores a superseded filter result", async () => {
    const resolvers: Array<(value: string) => void> = [];
    filterText.mockImplementation(
      () =>
        new Promise<string>((resolve) => {
          resolvers.push(resolve);
        })
    );
    setTerm(fakeTerm(["race"]));
    render(<CopyModeDialog open onClose={() => {}} />);

    const user = userEvent.setup();
    await user.click(screen.getByRole("tab", { name: "Filtered LLM buffer" }));
    // Refresh starts a second request while the first is still in flight.
    await user.click(screen.getByRole("button", { name: "Refresh" }));
    expect(resolvers).toHaveLength(2);

    // The NEWER request lands first, then the stale one.
    await act(async () => {
      resolvers[1]("newer");
      await new Promise((r) => setTimeout(r, 0));
    });
    await act(async () => {
      resolvers[0]("older");
      await new Promise((r) => setTimeout(r, 0));
    });

    expect(
      (screen.getByLabelText("Filtered LLM buffer (copy mode)") as HTMLTextAreaElement).value
    ).toBe("newer");
  });

  // Mutation check: not resetting the view on open (or leaving the filtered text
  // cached across a close) shows the previous session's filtered buffer here.
  it("reopens on the Complete buffer view with no cached filtered text", async () => {
    filterText.mockResolvedValue("previous session");
    setTerm(fakeTerm(["raw"]));
    const { rerender } = render(<CopyModeDialog open onClose={() => {}} />);

    await openFilteredView();
    expect(screen.getByLabelText("Filtered LLM buffer (copy mode)")).toBeInTheDocument();

    rerender(<CopyModeDialog open={false} onClose={() => {}} />);
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    rerender(<CopyModeDialog open onClose={() => {}} />);
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    expect(screen.getByRole("tab", { name: "Complete buffer" })).toHaveAttribute(
      "aria-selected",
      "true"
    );
    expect(completeArea().value).toBe("raw");
    expect(screen.queryByLabelText("Filtered LLM buffer (copy mode)")).not.toBeInTheDocument();
  });
});
