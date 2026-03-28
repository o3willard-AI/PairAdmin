import { describe, it, expect, beforeEach } from "vitest";
import { useChatStore } from "@/stores/chatStore";

describe("chatStore", () => {
  beforeEach(() => {
    useChatStore.setState({ messagesByTab: {} });
  });

  it("addUserMessage adds a user message to the specified tab", () => {
    const id = useChatStore.getState().addUserMessage("tab-1", "hello");
    const messages = useChatStore.getState().messagesByTab["tab-1"];
    expect(messages).toHaveLength(1);
    expect(messages[0].role).toBe("user");
    expect(messages[0].content).toBe("hello");
    expect(typeof id).toBe("string");
    expect(id.length).toBeGreaterThan(0);
  });

  it("addAssistantMessage adds an assistant message to the specified tab", () => {
    const id = useChatStore.getState().addAssistantMessage("tab-1", "echo");
    const messages = useChatStore.getState().messagesByTab["tab-1"];
    expect(messages).toHaveLength(1);
    expect(messages[0].role).toBe("assistant");
    expect(messages[0].content).toBe("echo");
    expect(typeof id).toBe("string");
  });

  it("messages are isolated per tab", () => {
    useChatStore.getState().addUserMessage("tab-1", "msg for tab-1");
    const tab2Messages = useChatStore.getState().messagesByTab["tab-2"];
    expect(tab2Messages).toBeUndefined();
  });

  it("clearTab empties only the specified tab", () => {
    useChatStore.getState().addUserMessage("tab-1", "msg1");
    useChatStore.getState().addUserMessage("tab-2", "msg2");
    useChatStore.getState().clearTab("tab-1");
    expect(useChatStore.getState().messagesByTab["tab-1"]).toHaveLength(0);
    expect(useChatStore.getState().messagesByTab["tab-2"]).toHaveLength(1);
  });

  it("each message has id, role, content, isStreaming fields", () => {
    useChatStore.getState().addUserMessage("tab-1", "test");
    const msg = useChatStore.getState().messagesByTab["tab-1"][0];
    expect(msg).toHaveProperty("id");
    expect(msg).toHaveProperty("role");
    expect(msg).toHaveProperty("content");
    expect(msg).toHaveProperty("isStreaming");
  });

  // Streaming actions tests (RED — implementations not yet added)
  describe("streaming actions", () => {
    it("startStreamingMessage creates an assistant message with isStreaming=true and returns its id", () => {
      const id = useChatStore.getState().startStreamingMessage("tab-1");
      const messages = useChatStore.getState().messagesByTab["tab-1"];
      expect(messages).toHaveLength(1);
      expect(messages[0].id).toBe(id);
      expect(messages[0].role).toBe("assistant");
      expect(messages[0].isStreaming).toBe(true);
      expect(messages[0].content).toBe("");
      expect(typeof id).toBe("string");
      expect(id.length).toBeGreaterThan(0);
    });

    it("appendChunk appends text to message content and calling twice concatenates both texts", () => {
      const id = useChatStore.getState().startStreamingMessage("tab-1");
      useChatStore.getState().appendChunk("tab-1", id, "hello ");
      useChatStore.getState().appendChunk("tab-1", id, "world");
      const msg = useChatStore.getState().messagesByTab["tab-1"][0];
      // Content should contain both texts (with cursor logic)
      expect(msg.content).toContain("hello ");
      expect(msg.content).toContain("world");
    });

    it("appendChunk appends ▋ cursor to end of content on each call", () => {
      const id = useChatStore.getState().startStreamingMessage("tab-1");
      useChatStore.getState().appendChunk("tab-1", id, "hello");
      const msg = useChatStore.getState().messagesByTab["tab-1"][0];
      expect(msg.content.endsWith("▋")).toBe(true);
    });

    it("finalizeMessage sets isStreaming=false and strips trailing ▋ from content", () => {
      const id = useChatStore.getState().startStreamingMessage("tab-1");
      useChatStore.getState().appendChunk("tab-1", id, "hello");
      useChatStore.getState().finalizeMessage("tab-1", id);
      const msg = useChatStore.getState().messagesByTab["tab-1"][0];
      expect(msg.isStreaming).toBe(false);
      expect(msg.content).toBe("hello");
      expect(msg.content.endsWith("▋")).toBe(false);
    });

    it("finalizeMessage sets tokenCount if provided", () => {
      const id = useChatStore.getState().startStreamingMessage("tab-1");
      useChatStore.getState().appendChunk("tab-1", id, "response");
      useChatStore.getState().finalizeMessage("tab-1", id, 42);
      const msg = useChatStore.getState().messagesByTab["tab-1"][0];
      expect(msg.tokenCount).toBe(42);
    });

    it("setStreamError with msgId=null creates a new error message with isError=true, isStreaming=false, content=errorText", () => {
      useChatStore.getState().setStreamError("tab-1", null, "Connection timeout");
      const messages = useChatStore.getState().messagesByTab["tab-1"];
      expect(messages).toHaveLength(1);
      expect(messages[0].role).toBe("assistant");
      expect(messages[0].isError).toBe(true);
      expect(messages[0].isStreaming).toBe(false);
      expect(messages[0].content).toBe("Connection timeout");
    });

    it("setStreamError with existing msgId preserves partial content, appends (stream interrupted), sets isError=true", () => {
      const id = useChatStore.getState().startStreamingMessage("tab-1");
      useChatStore.getState().appendChunk("tab-1", id, "partial response");
      useChatStore.getState().setStreamError("tab-1", id, "Stream failed");
      const msg = useChatStore.getState().messagesByTab["tab-1"][0];
      expect(msg.isError).toBe(true);
      expect(msg.isStreaming).toBe(false);
      expect(msg.content).toContain("partial response");
      expect(msg.content).toContain("(stream interrupted)");
    });
  });
});
