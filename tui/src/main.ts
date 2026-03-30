import type { CliRenderer, KeyEvent } from "@opentui/core";
import {
  BoxRenderable,
  CliRenderEvents,
  createCliRenderer,
  TextRenderable,
} from "@opentui/core";
import { fetchRelayStatus } from "./lib/status.ts";
import {
  buildDashboardViewModel,
  type DashboardState,
} from "./lib/view-model.ts";

const DEFAULT_RELAY_URL = "http://127.0.0.1:8080";
const DEFAULT_REFRESH_MS = 2_000;
const DEFAULT_TIMEOUT_MS = 5_000;
const MIN_REFRESH_MS = 1_000;
const MAX_REFRESH_MS = 30_000;

interface CliOptions {
  relayURL: string;
  refreshMs: number;
  timeoutMs: number;
}

function printHelp(): void {
  console.log(`bore-tui -- live relay operator console

Usage:
  bun run start --relay http://127.0.0.1:8080
  bun run start --relay http://127.0.0.1:8080 --refresh 2s --timeout 5s

Flags:
  --relay URL      relay base URL (default: ${DEFAULT_RELAY_URL})
  --refresh D      refresh interval in ms or simple duration (default: 2s)
  --timeout D      request timeout in ms or simple duration (default: 5s)
  -h, --help       show this help

Keys:
  r  refresh now
  +  slow refresh down
  -  speed refresh up
  q  quit
`);
}

function parseDurationFlag(value: string, label: string): number {
  const trimmed = value.trim();
  if (trimmed.length === 0) {
    throw new Error(`${label} must not be empty`);
  }

  const match = trimmed.match(/^(\d+)(ms|s|m)?$/i);
  if (!match) {
    throw new Error(`${label} must be a number, or end in ms, s, or m`);
  }

  const numeric = Number(match[1]);
  const unit = (match[2] ?? "ms").toLowerCase();
  switch (unit) {
    case "ms":
      return numeric;
    case "s":
      return numeric * 1_000;
    case "m":
      return numeric * 60_000;
    default:
      throw new Error(`${label} unit ${unit} is not supported`);
  }
}

export function parseArgs(args: string[]): CliOptions {
  const options: CliOptions = {
    relayURL: DEFAULT_RELAY_URL,
    refreshMs: DEFAULT_REFRESH_MS,
    timeoutMs: DEFAULT_TIMEOUT_MS,
  };

  for (let index = 0; index < args.length; index += 1) {
    const arg = args[index];
    switch (arg) {
      case "--relay":
        options.relayURL = args[++index] ?? "";
        if (!options.relayURL) {
          throw new Error("--relay requires a value");
        }
        break;
      case "--refresh":
        options.refreshMs = parseDurationFlag(args[++index] ?? "", "--refresh");
        break;
      case "--timeout":
        options.timeoutMs = parseDurationFlag(args[++index] ?? "", "--timeout");
        break;
      case "-h":
      case "--help":
        printHelp();
        process.exit(0);
        return options;
      default:
        throw new Error(`unknown argument ${arg}`);
    }
  }

  return options;
}

class BoreRelayDashboard {
  private readonly state: DashboardState;
  private readonly root: BoxRenderable;
  private readonly headerText: TextRenderable;
  private readonly alertBox: BoxRenderable;
  private readonly alertText: TextRenderable;
  private readonly overviewText: TextRenderable;
  private readonly roomsText: TextRenderable;
  private readonly transportText: TextRenderable;
  private readonly limitsText: TextRenderable;
  private readonly footerText: TextRenderable;
  private refreshTimer: Timer | null = null;
  private stopped = false;

  constructor(
    private readonly renderer: CliRenderer,
    options: CliOptions,
  ) {
    this.state = {
      relayURL: options.relayURL,
      refreshIntervalMs: options.refreshMs,
      timeoutMs: options.timeoutMs,
      loading: true,
      lastError: null,
      lastAttemptAt: null,
      lastSuccessAt: null,
      status: null,
    };

    this.root = new BoxRenderable(this.renderer, {
      width: "100%",
      height: "100%",
      padding: 1,
      gap: 1,
      flexDirection: "column",
      focusable: true,
      onKeyDown: (key) => this.handleKey(key),
    });

    const headerBox = new BoxRenderable(this.renderer, {
      border: true,
      title: "relay header",
      height: 4,
      padding: 1,
    });
    this.headerText = new TextRenderable(this.renderer, {
      content: "loading relay operator console...",
    });
    headerBox.add(this.headerText);

    this.alertBox = new BoxRenderable(this.renderer, {
      border: true,
      title: "failure state",
      height: 5,
      padding: 1,
      visible: false,
    });
    this.alertText = new TextRenderable(this.renderer, { content: "" });
    this.alertBox.add(this.alertText);

    const body = new BoxRenderable(this.renderer, {
      flexGrow: 1,
      flexDirection: "row",
      gap: 1,
    });

    const leftColumn = new BoxRenderable(this.renderer, {
      width: "40%",
      flexDirection: "column",
      gap: 1,
    });
    const rightColumn = new BoxRenderable(this.renderer, {
      flexGrow: 1,
      flexDirection: "column",
      gap: 1,
    });

    const overviewBox = new BoxRenderable(this.renderer, {
      border: true,
      title: "overview",
      padding: 1,
      flexGrow: 1,
    });
    this.overviewText = new TextRenderable(this.renderer, { content: "" });
    overviewBox.add(this.overviewText);

    const limitsBox = new BoxRenderable(this.renderer, {
      border: true,
      title: "limits",
      padding: 1,
      flexGrow: 1,
    });
    this.limitsText = new TextRenderable(this.renderer, { content: "" });
    limitsBox.add(this.limitsText);

    const roomsBox = new BoxRenderable(this.renderer, {
      border: true,
      title: "room gauges",
      padding: 1,
      flexGrow: 1,
    });
    this.roomsText = new TextRenderable(this.renderer, { content: "" });
    roomsBox.add(this.roomsText);

    const transportBox = new BoxRenderable(this.renderer, {
      border: true,
      title: "transport mix",
      padding: 1,
      flexGrow: 1,
    });
    this.transportText = new TextRenderable(this.renderer, { content: "" });
    transportBox.add(this.transportText);

    leftColumn.add(overviewBox);
    leftColumn.add(limitsBox);
    rightColumn.add(roomsBox);
    rightColumn.add(transportBox);
    body.add(leftColumn);
    body.add(rightColumn);

    const footerBox = new BoxRenderable(this.renderer, {
      border: true,
      title: "controls",
      height: 4,
      padding: 1,
    });
    this.footerText = new TextRenderable(this.renderer, { content: "" });
    footerBox.add(this.footerText);

    this.root.add(headerBox);
    this.root.add(this.alertBox);
    this.root.add(body);
    this.root.add(footerBox);
    this.renderer.root.add(this.root);
  }

  async start(): Promise<void> {
    this.render();
    this.renderer.start();
    this.root.focus();
    void this.refresh();
    this.installRefreshTimer();

    await new Promise<void>((resolve) => {
      this.renderer.on(CliRenderEvents.DESTROY, () => resolve());
    });
  }

  private installRefreshTimer(): void {
    if (this.refreshTimer) {
      clearInterval(this.refreshTimer);
    }
    this.refreshTimer = setInterval(() => {
      void this.refresh();
    }, this.state.refreshIntervalMs);
  }

  private async refresh(): Promise<void> {
    if (this.stopped) {
      return;
    }

    this.state.loading = true;
    this.state.lastAttemptAt = Date.now();
    this.render();

    try {
      const status = await fetchRelayStatus(
        this.state.relayURL,
        this.state.timeoutMs,
      );
      this.state.status = status;
      this.state.lastError = null;
      this.state.lastSuccessAt = Date.now();
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      this.state.lastError = `refresh failed: ${message}`;
    } finally {
      this.state.loading = false;
      this.render();
    }
  }

  private adjustRefreshInterval(deltaMs: number): void {
    const next = Math.min(
      MAX_REFRESH_MS,
      Math.max(MIN_REFRESH_MS, this.state.refreshIntervalMs + deltaMs),
    );
    this.state.refreshIntervalMs = next;
    this.installRefreshTimer();
    this.render();
  }

  private handleKey(key: KeyEvent): void {
    if (key.name === "q") {
      key.preventDefault();
      this.stop();
      return;
    }

    if (key.name === "r") {
      key.preventDefault();
      void this.refresh();
      return;
    }

    if (key.sequence === "+" || key.sequence === "=") {
      key.preventDefault();
      this.adjustRefreshInterval(1_000);
      return;
    }

    if (key.sequence === "-") {
      key.preventDefault();
      this.adjustRefreshInterval(-1_000);
    }
  }

  private render(): void {
    const view = buildDashboardViewModel(this.state);
    this.headerText.content = view.header;
    this.alertBox.title = view.alertTitle;
    this.alertBox.visible = view.alertVisible;
    this.alertText.content = view.alertBody;
    this.overviewText.content = view.overview;
    this.roomsText.content = view.rooms;
    this.transportText.content = view.transport;
    this.limitsText.content = view.limits;
    this.footerText.content = view.footer;
    this.root.requestRender();
  }

  private stop(): void {
    if (this.stopped) {
      return;
    }
    this.stopped = true;
    if (this.refreshTimer) {
      clearInterval(this.refreshTimer);
      this.refreshTimer = null;
    }
    this.renderer.destroy();
  }
}

async function run(): Promise<void> {
  const options = parseArgs(process.argv.slice(2));
  const renderer = await createCliRenderer({
    screenMode: "alternate-screen",
    consoleMode: "disabled",
    exitOnCtrlC: true,
  });

  const dashboard = new BoreRelayDashboard(renderer, options);
  await dashboard.start();
}

if (import.meta.main) {
  run().catch((error) => {
    const message = error instanceof Error ? error.message : String(error);
    console.error(`bore-tui: ${message}`);
    process.exit(1);
  });
}
