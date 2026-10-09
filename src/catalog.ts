import catalogContract from "./catalogContract.json";
import fallback from "./catalogFallback.json";

// KyseDirective is one directive the view compiler knows. Kind is block for
// one that opens a block, end for the one that closes it, and inline for the
// rest; a kind this extension has not heard of is kept as it was written.
export interface KyseDirective {
  readonly name: string;
  readonly kind: string;
  readonly closedBy?: string;
  readonly closes?: string;
}

// AruCommand is one entry of the aru command line, as its help lists it.
export interface AruCommand {
  readonly name: string;
  readonly usage: string;
  readonly description: string;
  readonly flags: readonly string[];
}

// AruCatalog is the vocabulary the extension shows: the directives and the
// commands. Source says whether the running aru answered it or the extension
// fell back to its own list, which happens only with an aru that does not
// answer arandu/catalog.
export interface AruCatalog {
  readonly source: "aru" | "fallback";
  readonly directives: readonly KyseDirective[];
  readonly commands: readonly AruCommand[];
}

export class CatalogContractError extends Error {
  public constructor(message: string) {
    super(message);
    this.name = "CatalogContractError";
  }
}

// fallbackCatalog is the list kept in this repository, for an aru that does
// not answer arandu/catalog. It is the only place that list is read. It
// carries no commands: the generators are offered from the catalogue alone,
// because a list of their flags kept here would describe some other release
// of aru than the one installed.
export function fallbackCatalog(): AruCatalog {
  return {
    source: "fallback",
    directives: fallback.directives.map((entry, index) => parseDirective(entry, index)),
    commands: fallback.commands.map((entry, index) => parseCommand(entry, index)),
  };
}

// parseCatalog reads the answer to arandu/catalog.
export function parseCatalog(value: unknown): AruCatalog {
  const catalog = expectRecord(value, "catalog");
  return {
    source: "aru",
    directives: expectArray(catalog.directives, "catalog directives").map(parseDirective),
    commands: expectArray(catalog.commands, "catalog commands").map(parseCommand),
  };
}

// directiveAt finds the directive written at a character of a line: the
// at-sign and the letters after it, not preceded by a letter or a digit, so
// the at-sign of an address in text is not read as one.
export function directiveAt(
  catalog: AruCatalog,
  line: string,
  character: number,
): { readonly directive: KyseDirective; readonly start: number; readonly end: number } | undefined {
  const pattern = /@([a-z]+)/g;
  for (let match = pattern.exec(line); match !== null; match = pattern.exec(line)) {
    const start = match.index;
    const end = start + match[0].length;
    if (character < start || character > end) {
      continue;
    }
    if (start > 0 && /[A-Za-z0-9_.]/.test(line[start - 1] ?? "")) {
      return undefined;
    }
    const directive = catalog.directives.find((candidate) => candidate.name === match?.[1]);
    return directive === undefined ? undefined : { directive, start, end };
  }
  return undefined;
}

// describeDirective is the hover text of a directive, in Markdown.
export function describeDirective(directive: KyseDirective): string {
  const name = `\`@${directive.name}\``;
  if (directive.kind === "block" && directive.closedBy !== undefined) {
    return `${name} opens a Kyse block that \`@${directive.closedBy}\` closes.`;
  }
  if (directive.kind === "end" && directive.closes !== undefined) {
    return `${name} closes the Kyse block \`@${directive.closes}\` opened.`;
  }
  if (directive.kind === "inline") {
    return `${name} is an inline Kyse directive.`;
  }
  return `${name} is a Kyse directive (${directive.kind}).`;
}

// generatorCommands are the commands of the catalogue the extension may run
// for someone who asks: the generators, and nothing else. A migration, a
// seeder and anything that edits the wiring are never among them, because
// none of them is a make: command.
export function generatorCommands(catalog: AruCatalog): AruCommand[] {
  return catalog.commands.filter((command) => isGeneratorCommand(command.name));
}

export function isGeneratorCommand(name: string): boolean {
  return name.startsWith(catalogContract.generatorPrefix) && /^[a-z]+:[a-z][a-z0-9-]*$/.test(name);
}

// GeneratorFlag is one flag of a generator as its usage line writes it.
export interface GeneratorFlag {
  readonly flag: string;
  readonly takesValue: boolean;
  readonly required: boolean;
  readonly example?: string;
}

// generatorFlags reads, from the usage line, how each flag of a generator is
// written: whether a value follows it, whether it sits outside the brackets
// that mark an optional part, and the example value the usage gives. The
// short spelling of a flag that also has a long one is left out, and so is
// the preview flag, which the extension offers as its own step.
export function generatorFlags(command: AruCommand): GeneratorFlag[] {
  const flags: GeneratorFlag[] = [];
  for (const flag of command.flags) {
    if (flag === catalogContract.previewFlag) {
      continue;
    }
    const at = flagPosition(command.usage, flag);
    if (at < 0) {
      flags.push({ flag, takesValue: false, required: false });
      continue;
    }
    const after = command.usage.slice(at + flag.length);
    if (/^\|--?[A-Za-z]/.test(after)) {
      continue;
    }
    const value = /^(?:=|\s+)("[^"]*"|<[^>]*>|[A-Za-z0-9][^\s\]|]*)/.exec(after);
    const entry: { flag: string; takesValue: boolean; required: boolean; example?: string } = {
      flag,
      takesValue: value !== null,
      required: bracketDepth(command.usage, at) === 0,
    };
    if (value?.[1] !== undefined) {
      entry.example = value[1].replace(/^"(.*)"$/, "$1");
    }
    flags.push(entry);
  }
  return flags;
}

// generatorName is the positional argument a generator's usage line asks
// for first, such as <Name> or <module>, or undefined when it asks for none.
export function generatorName(command: AruCommand): string | undefined {
  const rest = command.usage.split(/\s+/).slice(2).join(" ");
  const name = /^<([^>]+)>/.exec(rest);
  return name?.[1];
}

export function supportsPreview(command: AruCommand): boolean {
  return command.flags.includes(catalogContract.previewFlag);
}

// generatorArguments is the argument vector of one generator run. A value
// is attached to its flag with "=", which every aru flag reads, so nothing
// here depends on a shell splitting words.
export function generatorArguments(
  command: AruCommand,
  name: string | undefined,
  chosen: ReadonlyArray<{ readonly flag: string; readonly value?: string }>,
  preview: boolean,
): string[] {
  if (!isGeneratorCommand(command.name)) {
    throw new CatalogContractError(`${command.name} is not a generator.`);
  }
  const args = [command.name];
  if (name !== undefined) {
    args.push(name);
  }
  for (const entry of chosen) {
    if (!command.flags.includes(entry.flag)) {
      throw new CatalogContractError(`${command.name} has no flag ${entry.flag}.`);
    }
    args.push(entry.value === undefined ? entry.flag : `${entry.flag}=${entry.value}`);
  }
  if (preview) {
    args.push(catalogContract.previewFlag);
  }
  return args;
}

function flagPosition(usage: string, flag: string): number {
  const escaped = flag.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = new RegExp(`(^|[\\s\\[|])${escaped}(?![A-Za-z0-9-])`).exec(usage);
  return match === null ? -1 : match.index + match[1].length;
}

function bracketDepth(usage: string, at: number): number {
  let depth = 0;
  for (const character of usage.slice(0, at)) {
    if (character === "[") {
      depth += 1;
    } else if (character === "]") {
      depth = Math.max(0, depth - 1);
    }
  }
  return depth;
}

function parseDirective(raw: unknown, index: number): KyseDirective {
  const entry = expectRecord(raw, `catalog directive ${index}`);
  const parsed: { name: string; kind: string; closedBy?: string; closes?: string } = {
    name: expectString(entry.name, `catalog directive ${index} name`),
    kind: expectString(entry.kind, `catalog directive ${index} kind`),
  };
  if (entry.closedBy !== undefined && entry.closedBy !== "") {
    parsed.closedBy = expectString(entry.closedBy, `catalog directive ${parsed.name} closedBy`);
  }
  if (entry.closes !== undefined && entry.closes !== "") {
    parsed.closes = expectString(entry.closes, `catalog directive ${parsed.name} closes`);
  }
  return parsed;
}

function parseCommand(raw: unknown, index: number): AruCommand {
  const entry = expectRecord(raw, `catalog command ${index}`);
  const name = expectString(entry.name, `catalog command ${index} name`);
  const description = entry.description === undefined || entry.description === ""
    ? ""
    : expectString(entry.description, `catalog command ${name} description`);
  const flags = entry.flags === undefined || entry.flags === null
    ? []
    : expectArray(entry.flags, `catalog command ${name} flags`).map((flag) =>
      expectString(flag, `catalog command ${name} flag`),
    );
  return {
    name,
    usage: expectString(entry.usage, `catalog command ${name} usage`),
    description,
    flags,
  };
}

function expectRecord(value: unknown, label: string): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new CatalogContractError(`${label} must be an object.`);
  }
  return value as Record<string, unknown>;
}

function expectArray(value: unknown, label: string): readonly unknown[] {
  if (!Array.isArray(value)) {
    throw new CatalogContractError(`${label} must be an array.`);
  }
  return value;
}

function expectString(value: unknown, label: string): string {
  if (typeof value !== "string" || value.length === 0) {
    throw new CatalogContractError(`${label} must be a non-empty string.`);
  }
  return value;
}
