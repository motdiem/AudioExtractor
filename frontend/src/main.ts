import { Events } from "@wailsio/runtime";
import * as Extract from "../bindings/github.com/motdiem/AudioExtractor/internal/extract/service";

// All the logic lives in the Go service. This file only shows its state:
// the service emits "extract:state" and we swap the matching screen.

type Phase = "idle" | "working" | "done" | "error";
interface StateEvent {
    phase: Phase;
    name: string;
    output: string;
    message: string;
}

const screens = ["idle", "working", "done", "error"] as const;
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

function onState(e: StateEvent) {
    switch (e.phase) {
        case "done":
            output = e.output;
            $("done-hint").textContent = t("done.hint", { name: e.name });
            break;
        case "error":
            $("error-message").textContent = e.message;
            break;
    }
    show(e.phase);
}

async function main() {
    messages = (await Extract.Messages()) as Record<string, string>;
    document.documentElement.lang = await Extract.Lang();
    document.title = t("app.title");
    document.querySelectorAll<HTMLElement>("[data-i18n]").forEach((el) => {
        el.textContent = t(el.dataset.i18n!);
    });

    // Wails raises this one while a file hovers over the drop zone, so we can
    // swap the label too (the highlight itself is CSS on .file-drop-target-active).
    const drop = $("drop");
    const title = drop.querySelector<HTMLElement>(".title")!;
    const idleTitle = t("drop.title");
    new MutationObserver(() => {
        title.textContent = drop.classList.contains("file-drop-target-active") ? t("drop.release") : idleTitle;
    }).observe(drop, { attributes: true, attributeFilter: ["class"] });

    Events.On("extract:state", (ev: { data: StateEvent }) => onState(ev.data));

    // Clicking the zone: Go opens the picker, then the Save dialog, and emits
    // the states as it goes. We only need to start it.
    const pick = () => void Extract.Pick();
    drop.addEventListener("click", pick);
    $("retry").addEventListener("click", pick);
    $("again").addEventListener("click", () => show("idle"));
    $("reveal").addEventListener("click", async () => {
        await Extract.Reveal(output);
        show("idle");
    });
}

main();
