// Wails v2 rejects a failed RPC call with the Go error's plain string (see
// calls.js's `callbackData.reject(message.error)`, fed by the Go dispatcher's
// `callbackMessage.Err = err.Error()`) — never a JS Error instance. Code that
// checks `err instanceof Error` before reading `.message` will therefore
// always miss the real error text for genuine backend failures and fall
// through to whatever generic fallback string it was given.
export function wailsErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof Error) return err.message;
  if (typeof err === "string" && err) return err;
  return fallback;
}
