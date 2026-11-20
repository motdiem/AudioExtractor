// Dev-server-only screen previews, for checking the layout of every screen in
// a plain browser (no Go side): `npm run dev`, then open
//   http://127.0.0.1:9245/?screen=summary&lang=fr
// screen: idle | working | done | error | summary | stopped. main.ts only
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
    percent: -1,
    name: "",
    output: "",
    message: "",
};

export function preview(params: URLSearchParams): Preview | null {
    const screen = params.get("screen");
    if (!screen) return null;
    const lang = params.get("lang") === "fr" ? "fr" : "en";
    const messages: Record<string, string> = lang === "fr" ? fr : en;
    const failures = [
        { name: "2026-11-20 10.00.00 Point hebdomadaire avec toute l’équipe produit/video1234567890.mp4", message: messages["error.unreadable"] },
        { name: "Rapport.pdf", message: messages["error.not-media"] },
        { name: "Écran partagé sans micro.mov", message: messages["error.no-audio"] },
    ];
    const states: Record<string, Partial<StateEvent>> = {
        idle: {},
        working: { phase: "working", index: 2, total: 5, percent: 42 },
        done: { phase: "done", name: "2026-11-20 10.00.00 Point hebdomadaire avec toute l’équipe produit.m4a" },
        error: { phase: "error", message: messages["error.unreadable"] },
        summary: { phase: "summary", index: 12, total: 12, done: 9, failed: 3, failures },
        stopped: { phase: "summary", index: 4, total: 12, done: 2, failed: 2, stopped: true, failures: failures.slice(0, 2) },
    };
    const state = { ...base, ...(states[screen] ?? {}) } as StateEvent;
    return { messages, lang, version: params.get("version") ?? "v1.0.0-12-g29403c8-dirty", state };
}
