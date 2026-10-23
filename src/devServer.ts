import { Socket } from "node:net";
import contract from "./testingContract.json";

// devServerURL answers the address the project's development server listens on,
// from its .env: APP_URL when it is an http address, the HTTP_ADDR the server
// binds otherwise, and the skeleton's default when neither is written.
export function devServerURL(env: string): string {
  const values = new Map<string, string>();
  for (const raw of env.split(/\r?\n/)) {
    const match = /^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*?)\s*$/.exec(raw);
    if (match === null) {
      continue;
    }
    let value = match[2];
    const quoted = /^(["'])(.*?)\1(?:\s+#.*)?$/.exec(value);
    if (quoted !== null) {
      value = quoted[2];
    } else {
      value = value.replace(/\s+#.*$/, "");
    }
    values.set(match[1], value);
  }
  const appURL = values.get("APP_URL") ?? "";
  if (/^https?:\/\/[^\s/]+/.test(appURL)) {
    return appURL.replace(/\/+$/, "");
  }
  const addr = values.get("HTTP_ADDR") ?? "";
  const bound = /^(\[[^\]]*\]|[^:]*):(\d{1,5})$/.exec(addr);
  if (bound !== null) {
    const host = bound[1] === "" || bound[1] === "0.0.0.0" || bound[1] === "[::]" ? "localhost" : bound[1];
    return `http://${host}:${bound[2]}`;
  }
  return contract.devServerDefaultUrl;
}

// recordTarget answers what aru e2e:record starts from, given what the person
// typed and the address of a development server that is answering, if any. An
// address is used as typed, a path is joined to the running server's address,
// a route name is handed to aru to resolve, and nothing at all starts from the
// running server -- or from the application aru serves itself when none runs.
export function recordTarget(input: string, devURL: string | undefined): string | undefined {
  const value = input.trim();
  if (value === "") {
    return devURL;
  }
  if (/^https?:\/\//i.test(value)) {
    return value;
  }
  if (value.startsWith("/") && devURL !== undefined) {
    return `${devURL.replace(/\/+$/, "")}${value}`;
  }
  return value;
}

// isListening reports whether something accepts a TCP connection at the URL's
// host and port within the timeout: a development server started in another
// terminal is reused just like the one this editor started.
export function isListening(url: string, timeoutMs: number = contract.devServerProbeMilliseconds): Promise<boolean> {
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    return Promise.resolve(false);
  }
  const port = Number(parsed.port || (parsed.protocol === "https:" ? 443 : 80));
  const host = parsed.hostname.replace(/^\[|\]$/g, "");
  return new Promise((resolve) => {
    const socket = new Socket();
    const done = (answer: boolean): void => {
      socket.destroy();
      resolve(answer);
    };
    socket.setTimeout(timeoutMs);
    socket.once("connect", () => done(true));
    socket.once("timeout", () => done(false));
    socket.once("error", () => done(false));
    socket.connect(port, host);
  });
}
