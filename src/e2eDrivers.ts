import contract from "./testingContract.json";
import type { DriverChoice } from "./testLayout";

// requiresModule reports whether go.mod requires the module, on a require line
// of its own or inside a require block. A commented-out line, a replace or an
// exclude does not count: only a requirement puts the driver in the build.
export function requiresModule(goMod: string, modulePath: string): boolean {
  let inBlock = false;
  for (const raw of goMod.split(/\r?\n/)) {
    const line = raw.replace(/\/\/.*$/, "").trim();
    if (line === "") {
      continue;
    }
    if (inBlock) {
      if (line === ")") {
        inBlock = false;
        continue;
      }
      if (line.split(/\s+/)[0] === modulePath) {
        return true;
      }
      continue;
    }
    if (/^require\s*\($/.test(line)) {
      inBlock = true;
      continue;
    }
    const single = /^require\s+(\S+)\s+\S+/.exec(line);
    if (single !== null && single[1] === modulePath) {
      return true;
    }
  }
  return false;
}

// driverChoices answers every driver and browser pair the project can run its
// journeys with: the native driver always, and a module's driver only when
// go.mod requires that module, because a test case cannot register a driver
// its build does not contain.
export function driverChoices(goMod: string): DriverChoice[] {
  const choices: DriverChoice[] = [];
  for (const entry of contract.drivers) {
    if (entry.module !== "" && !requiresModule(goMod, entry.module)) {
      continue;
    }
    for (const browser of entry.browsers) {
      choices.push({ driver: entry.driver, browser });
    }
  }
  return choices;
}

// resolveChoice answers the remembered choice while the project can still run
// it, and undefined otherwise -- after the driver's module left go.mod, the
// run goes back to what the test case picks by itself rather than asking for a
// driver it no longer has.
export function resolveChoice(remembered: unknown, goMod: string): DriverChoice | undefined {
  if (typeof remembered !== "object" || remembered === null) {
    return undefined;
  }
  const { driver, browser } = remembered as Record<string, unknown>;
  if (typeof driver !== "string" || typeof browser !== "string") {
    return undefined;
  }
  return driverChoices(goMod).find((choice) => choice.driver === driver && choice.browser === browser);
}

// describeChoice is how a choice reads in a picker and in the Testing view.
export function describeChoice(choice: DriverChoice | undefined): string {
  return choice === undefined ? "project default" : `${choice.driver} · ${choice.browser}`;
}

// e2eEnvironment answers the variables an end-to-end test reads, for a run or a
// debug session: the driver and browser chosen, the window shown when headed,
// and a trace written after every test into traceDir, so the last trace can be
// opened after a pass as well as after a failure.
export function e2eEnvironment(
  choice: DriverChoice | undefined,
  options: { readonly headed: boolean; readonly traceDir: string | undefined },
): Record<string, string> {
  const env: Record<string, string> = { [contract.env.trace]: "1" };
  if (choice !== undefined) {
    env[contract.env.driver] = choice.driver;
    env[contract.env.browser] = choice.browser;
  }
  if (options.headed) {
    env[contract.env.headed] = "1";
  }
  if (options.traceDir !== undefined) {
    env[contract.env.traceDir] = options.traceDir;
  }
  return env;
}
