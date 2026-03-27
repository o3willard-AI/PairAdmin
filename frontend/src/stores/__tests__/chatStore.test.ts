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
});
