// Dev-server-only screen previews, for checking the layout of every screen in
// a plain browser (no Go side): `npm run dev`, then open
//   http://127.0.0.1:9245/?screen=summary&lang=fr
// screen: idle | working | done | error | summary | stopped; lang: en | fr;
// os: mac | windows (for the per-OS texts, "Show in Finder"). main.ts only
// imports this file behind import.meta.env.DEV, so builds leave it out.
// The samples use the longest real messages, to catch overflow.
import en from "../../internal/i18n/locales/en.json";
import fr from "../../internal/i18n/locales/fr.json";
import type { StateEvent } from "./main";

export interface Preview {
    messages: Record<string, string>;
    lang: string;
    version: string;
    state: StateEvent;
}

const base: StateEvent = {
    phase: "idle",
    index: 0,
    total: 0,
    done: 0,
    failed: 0,
    stopped: false,
    failures: null,
    title: "",
    counts: "",
    percent: -1,
    name: "",
    output: "",
    message: "",
};

export function preview(params: URLSearchParams): Preview | null {
    const screen = params.get("screen");
    if (!screen) return null;
    const lang = params.get("lang") === "fr" ? "fr" : "en";
    const messages = forOS(lang === "fr" ? fr : en, params.get("os") === "windows" ? "windows" : "mac");
    const names =
        lang === "fr"
            ? {
                  meeting: "2026-11-20 10.00.00 Point hebdomadaire avec toute l’équipe produit",
                  report: "Rapport.pdf",
                  silent: "Écran partagé sans micro.mov",
              }
            : {
                  meeting: "2026-11-20 10.00.00 Weekly sync with the whole product team",
                  report: "Report.pdf",
                  silent: "Shared screen without a mic.mov",
              };
    const failures = [
        { name: `${names.meeting}/video1234567890.mp4`, message: messages["error.unreadable"] },
        { name: names.report, message: messages["error.not-media"] },
        { name: names.silent, message: messages["error.no-audio"] },
    ];
    // Go builds the summary text; this mirrors it for the samples (counts > 1).
    const fill = (key: string, args: Record<string, string>) =>
        Object.entries(args).reduce((s, [k, v]) => s.replaceAll(`{${k}}`, v), messages[key] ?? key);
    const counts = (done: number, failed: number) =>
        fill("summary.counts", {
            done: fill("count.done.other", { n: String(done) }),
            failed: fill("count.failed.other", { n: String(failed) }),
        });
    const states: Record<string, Partial<StateEvent>> = {
        idle: {},
        working: { phase: "working", index: 2, total: 5, percent: 42 },
        done: { phase: "done", name: `${names.meeting}.m4a` },
        error: { phase: "error", message: messages["error.unreadable"] },
        summary: { phase: "summary", index: 12, total: 12, done: 9, failed: 3, failures, title: counts(9, 3) },
        stopped: {
            phase: "summary",
            index: 4,
            total: 12,
            done: 2,
            failed: 2,
            stopped: true,
            failures: failures.slice(0, 2),
            title: fill("summary.title.stopped", { n: "4", total: "12" }),
            counts: counts(2, 2),
        },
    };
    const state = { ...base, ...(states[screen] ?? {}) } as StateEvent;
    return { messages, lang, version: params.get("version") ?? "v1.0.0-12-g29403c8-dirty", state };
}

// forOS mirrors i18n.forOS in Go: "key@mac" / "key@windows" become "key" for
// the chosen OS, and the other variants are dropped.
function forOS(catalog: Record<string, string>, os: "mac" | "windows"): Record<string, string> {
    const out: Record<string, string> = {};
    for (const [key, value] of Object.entries(catalog)) {
        const at = key.indexOf("@");
        if (at < 0) out[key] = value;
        else if (key.slice(at + 1) === os) out[key.slice(0, at)] = value;
    }
    return out;
}
