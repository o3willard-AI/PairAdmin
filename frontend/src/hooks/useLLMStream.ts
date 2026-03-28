import { useEffect, useRef } from "react";
import { useChatStore } from "@/stores/chatStore";

export function useLLMStream(tabId: string) {
  const msgIdRef = useRef<string | null>(null);
  const nextSeqRef = useRef(0);
  const pendingRef = useRef(new Map<number, string>());

  useEffect(() => {
    // Reset on tabId change
    msgIdRef.current = null;
    nextSeqRef.current = 0;
    pendingRef.current.clear();

    const { startStreamingMessage, appendChunk, finalizeMessage, setStreamError } =
      useChatStore.getState();

    const flushPending = () => {
      while (pendingRef.current.has(nextSeqRef.current)) {
        const text = pendingRef.current.get(nextSeqRef.current)!;
        pendingRef.current.delete(nextSeqRef.current);
        if (!msgIdRef.current) msgIdRef.current = startStreamingMessage(tabId);
        appendChunk(tabId, msgIdRef.current, text);
        nextSeqRef.current++;
      }
    };

    const handleChunk = (event: { seq: number; text: string }) => {
      if (event.seq === nextSeqRef.current) {
        if (!msgIdRef.current) msgIdRef.current = startStreamingMessage(tabId);
        appendChunk(tabId, msgIdRef.current, event.text);
        nextSeqRef.current++;
        flushPending();
      } else {
        pendingRef.current.set(event.seq, event.text);
      }
    };

    const handleDone = () => {
      flushPending();
      if (msgIdRef.current) finalizeMessage(tabId, msgIdRef.current);
      msgIdRef.current = null;
      nextSeqRef.current = 0;
      pendingRef.current.clear();
    };

    const handleError = (event: { error: string }) => {
      setStreamError(tabId, msgIdRef.current, event.error);
      msgIdRef.current = null;
      nextSeqRef.current = 0;
      pendingRef.current.clear();
    };

    let unsubChunk: (() => void) | null = null;
    let unsubDone: (() => void) | null = null;
    let unsubError: (() => void) | null = null;

    import(/* @vite-ignore */ "../../wailsjs/runtime/runtime").then((rt) => {
      // Cast handlers — Wails EventsOn accepts (...args: unknown[]) but delivers typed payloads at runtime
      unsubChunk = rt.EventsOn("llm:chunk", handleChunk as (...args: unknown[]) => void);
      unsubDone = rt.EventsOn("llm:done", handleDone as (...args: unknown[]) => void);
      unsubError = rt.EventsOn("llm:error", handleError as (...args: unknown[]) => void);
    });

    return () => {
      unsubChunk?.();
      unsubDone?.();
      unsubError?.();
    };
  }, [tabId]);
}
