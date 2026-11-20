import { Events } from "@wailsio/runtime";
import * as Extract from "../bindings/github.com/motdiem/AudioExtractor/internal/extract/service";
import * as Notices from "../bindings/github.com/motdiem/AudioExtractor/internal/notices/service";

// All the logic lives in the Go service. This file only shows its state:
// the service emits "extract:state" and we swap the matching screen.

export type Phase = "idle" | "working" | "done" | "error" | "summary";
export interface StateEvent {
    phase: Phase;
    index: number; // 1-based item position in a batch
    total: number;
    done: number; // summary only
    failed: number;
    stopped: boolean;
    failures: { name: string; message: string }[] | null;
    title: string; // summary only: Go resolves the plurals
    counts: string;
    percent: number; // 0-100, or -1 when unknown
    name: string;
    output: string;
    message: string;
}

const screens = ["idle", "working", "done", "error", "summary"] as const;
let output = "";
let messages: Record<string, string> = {};

const $ = <T extends HTMLElement>(id: string) => document.getElementById(id) as T;

function show(phase: Phase) {
    for (const s of screens) $(s).hidden = s !== phase;
}

function t(key: string, args: Record<string, string> = {}): string {
    let s = messages[key] ?? key;
    for (const [k, v] of Object.entries(args)) s = s.replaceAll(`{${k}}`, v);
    return s;
}

// Quick jobs (an M4A copy takes a fraction of a second) must not flash the
// progress screen, so "working" only becomes visible after this delay.
const WORKING_DELAY_MS = 500;
let workingTimer: number | undefined;
let workingShown = false;

function showProgress(percent: number) {
    const bar = $("bar");
    const known = percent >= 0;
    bar.classList.toggle("known", known);
    if (known) {
        bar.style.setProperty("--p", String(percent / 100));
        bar.setAttribute("aria-valuenow", String(percent));
    } else {
        bar.removeAttribute("aria-valuenow");
    }
    $("progress-text").textContent = known ? t("working.progress", { percent: String(percent) }) : "";
}

function showCount(e: StateEvent) {
    const el = $("working-count");
    el.hidden = e.total < 2;
    el.textContent = e.total < 2 ? "" : t("working.count", { index: String(e.index), total: String(e.total) });
}

function showSummary(e: StateEvent) {
    output = e.output;
    $("summary-title").textContent = e.title;
    // Set for a stopped batch only: otherwise the title already says it.
    $("summary-counts").textContent = e.counts;
    const list = $("failures");
    list.replaceChildren(
        ...(e.failures ?? []).map((f) => {
            const li = document.createElement("li");
            const name = document.createElement("b");
            name.textContent = f.name;
            const msg = document.createElement("span");
            msg.textContent = f.message;
            li.append(name, msg);
            return li;
        }),
    );
    $("summary-reveal").hidden = e.done === 0;
    $("summary-copy").hidden = e.failed === 0;
    $("summary-copy").textContent = t("copy.details");
}

function onState(e: StateEvent) {
    if (e.phase === "working") {
        showCount(e);
        showProgress(e.percent);
        if (!workingShown && workingTimer === undefined) {
            workingTimer = window.setTimeout(() => {
                workingTimer = undefined;
                workingShown = true;
                show("working");
            }, WORKING_DELAY_MS);
        }
        return; // stay on the current screen until the delay has passed
    }
    window.clearTimeout(workingTimer);
    workingTimer = undefined;
    workingShown = false;
    switch (e.phase) {
        case "done":
            output = e.output;
            $("done-hint").textContent = t("done.hint", { name: e.name });
            break;
        case "summary":
            showSummary(e);
            break;
        case "error":
            $("error-message").textContent = e.message;
            $("copy-details").textContent = t("copy.details");
            break;
    }
    show(e.phase);
}

// The subtitle translates the app name; it is only worth showing when the
// translation differs from the name itself (French: "Extracteur audio").
const sameName = (a: string, b: string) => a.replace(/\s/g, "").toLowerCase() === b.replace(/\s/g, "").toLowerCase();

function showStatic(version: string) {
    document.title = t("app.title");
    document.querySelectorAll<HTMLElement>("[data-i18n]").forEach((el) => {
        el.textContent = t(el.dataset.i18n!);
    });
    document.querySelectorAll<HTMLElement>("[data-i18n-title]").forEach((el) => {
        el.title = t(el.dataset.i18nTitle!);
    });
    document.querySelectorAll<HTMLElement>("[data-i18n-label]").forEach((el) => {
        el.setAttribute("aria-label", t(el.dataset.i18nLabel!));
    });
    const subtitle = $("subtitle");
    subtitle.textContent = t("app.subtitle");
    subtitle.hidden = sameName(t("app.subtitle"), t("app.title"));
    const v = $("version");
    v.textContent = version;
    v.title = t("version.label", { version });
}

async function main() {
    // Dev server only (stripped from builds): ?screen=error&lang=fr renders a
    // screen with sample data, without the Go side, for layout checks.
    if (import.meta.env.DEV) {
        const { preview } = await import("./preview");
        const p = preview(new URLSearchParams(location.search));
        if (p) {
            messages = p.messages;
            document.documentElement.lang = p.lang;
            showStatic(p.version);
            onState(p.state);
            if (p.state.phase === "working") show("working"); // skip the 0.5 s delay
            return;
        }
    }

    messages = (await Extract.Messages()) as Record<string, string>;
    document.documentElement.lang = await Extract.Lang();
    showStatic(await Extract.Version());

    // Wails raises this one while a file hovers over the drop zone, so we can
    // swap the label too (the highlight itself is CSS on .file-drop-target-active).
    const drop = $("drop");
    const title = drop.querySelector<HTMLElement>(".title")!;
    const idleTitle = t("drop.title");
    new MutationObserver(() => {
        title.textContent = drop.classList.contains("file-drop-target-active") ? t("drop.release") : idleTitle;
    }).observe(drop, { attributes: true, attributeFilter: ["class"] });

    // Format toggle: Go owns the value (and remembers it); we just mirror it.
    const buttons = document.querySelectorAll<HTMLButtonElement>("#format button");
    const markFormat = (f: string) =>
        buttons.forEach((b) => b.setAttribute("aria-checked", String(b.dataset.format === f)));
    markFormat(await Extract.Format());
    buttons.forEach((b) =>
        b.addEventListener("click", async () => {
            await Extract.SetFormat(b.dataset.format!);
            markFormat(await Extract.Format());
        }),
    );

    Events.On("extract:state", (ev: { data: StateEvent }) => onState(ev.data));

    // Clicking the zone: Go opens the picker, then the Save or folder dialog, and emits
    // the states as it goes. We only need to start it.
    const pick = () => void Extract.Pick();
    drop.addEventListener("click", pick);
    $("retry").addEventListener("click", pick);
    $("cancel").addEventListener("click", () => void Extract.Cancel());
    // Copy details: Go owns the text (header + raw ffmpeg log) and the native
    // clipboard; we only confirm with "Copied" for a moment.
    for (const copy of [$<HTMLButtonElement>("copy-details"), $<HTMLButtonElement>("summary-copy")]) {
        let copiedTimer: number | undefined;
        copy.addEventListener("click", async () => {
            try {
                await Extract.CopyDetails();
            } catch {
                return; // nothing sensible to tell a non-technical user
            }
            copy.textContent = t("copy.done");
            window.clearTimeout(copiedTimer);
            copiedTimer = window.setTimeout(() => (copy.textContent = t("copy.details")), 2000);
        });
    }
    // Licences: Go writes the notice to a temporary file and opens it.
    $("licences").addEventListener("click", () => void Notices.Open().catch(() => {}));
    $("again").addEventListener("click", () => show("idle"));
    $("summary-again").addEventListener("click", () => show("idle"));
    // The first output is selected, which opens the destination folder.
    for (const id of ["reveal", "summary-reveal"]) {
        $(id).addEventListener("click", async () => {
            await Extract.Reveal(output);
            show("idle");
        });
    }

    // Everything is wired: Go may now deliver files the OS opened us with.
    Events.Emit("frontend:ready");
}

main();
