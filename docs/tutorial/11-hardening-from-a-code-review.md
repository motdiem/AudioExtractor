# 11. Hardening from a code review

Goal of this chapter: fix what a careful code review of chapters 4 to 8 found. None of these bugs showed up in the happy path, and every test was green. Each one hides in an edge the design had not looked at: quitting at the wrong moment, an unlucky folder name, a name typed without its extension. For each: what went wrong, why, and the fix.

## 1. Quitting mid-extraction left ffmpeg and a `.part` file behind

**Symptom.** Quit the app (Cmd+Q, close the window) during a long MP3 encode: ffmpeg keeps running, and `.audioextractor-1234.part` stays in the user's folder.

**Why.** Cancel was wired to the Cancel button only. Every job runs under a context, and cancelling it kills ffmpeg and removes the temp file (chapter 5), but nothing cancelled it on quit.

**Fix.** Wails v3 calls `ServiceShutdown() error` on every bound service that has one, when the app quits. The extraction service now implements it: it marks the service as closing (no new job may start), cancels the context, and waits up to 2 seconds for the job to finish cleaning up. A job stuck in a native dialog cannot be waited for, so the wait is bounded.

That covers a normal quit. A hard kill (Task Manager, a crash) runs no Go code at all. On macOS and Linux there is no portable "kill my children with me" switch, but on Windows there is: a **Job Object** with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`. We create one job, never close its handle, and assign each ffmpeg to it right after `Start`. When our process dies, however it dies, Windows closes the handle and kills everything in the job. It is pure Go with `golang.org/x/sys/windows`, in a build-tagged `proc_windows.go`.

While there: the temp file's leading dot hides it on macOS, but not in Explorer. On Windows it now gets `FILE_ATTRIBUTE_HIDDEN`, cleared just before the rename (the rename keeps attributes, and a hidden output would be a strange gift). One trap: on Windows, opening a hidden file with "create and truncate" fails with *access denied*, and that is exactly how ffmpeg opens its output. The file is empty anyway, so we pass `-truncate 0`, an option of ffmpeg's `file:` protocol that skips the truncation.

**Test.** A long encode started with `TryRun`, then `ServiceShutdown`: it returns within 2 s, the folder is empty, and a new `TryRun` is refused without the busy dialog.

## 2. "Damaged" on a perfectly good file

**Symptom.** A recording in a folder called *Partial file reviews* extracts fine, then the app says it is damaged.

**Why.** Chapter 6 catches a subtle ffmpeg behaviour: on a cut-off "faststart" MP4, ffmpeg exits 0 after writing half the audio, and only its log says `partial file`. We looked for that phrase **anywhere** in the log. But ffmpeg's log echoes the input path (`Input #0, mov,... from '/Users/.../Partial file reviews/call.mp4':`) and every metadata field, titles included.

**Fix.** Match what ffmpeg actually emits, not words. The three partial-file diagnostics are lines of their own, starting with the `[component @ 0x...]` prefix:

```
[mov,mp4,m4a,3gp,3g2,mj2 @ 0x...] stream 1, offset 0x27f4: partial file
[in#0/mov,mp4,m4a,3gp,3g2,mj2 @ 0x...] corrupt input packet in stream 1
[mov,mp4,m4a,3gp,3g2,mj2 @ 0x...] Packet corrupt (stream = 1, dts = 36864).
```

A regexp anchored on that prefix does the job. The vaguer phrases ("permission denied", "no space left on device") cannot be pinned down that way, so they are looked for in `diagnostics(log, paths...)`: the log without the `Input #`/`Output #` headers, without the indented lines under them (metadata, streams), and with our own paths blanked out.

**Test.** A real clip in *Partial file reviews*, named `partial file, corrupt input packet.mp4`, with a title that quotes a partial-file diagnostic: the extraction succeeds.

## 3. A silent overwrite after appending the extension

**Symptom.** The user types `notes` in the Save dialog; `notes.m4a` already exists; it is replaced without a word.

**Why.** The OS's "Replace?" prompt is about the name the user typed, `notes`. We then appended `.m4a`, and nobody ever confirmed that path.

**Fix.** If the extension had to be added or changed **and** the resulting file exists, use the same `" (2)"` naming as batches. When the user typed the full name, the OS has already asked, and replacing stays the right thing to do.

## 4. Writing over the source

**Symptom.** MP3 mode on `talk.mp3`, or Zoom's `audio_only.m4a` in M4A mode: the Save dialog suggests the source's own name, the user presses Enter and confirms "Replace", and ffmpeg truncates the file it is reading. The recording is gone.

**Fix,** in two layers. The suggestion never equals the source: it becomes `talk (2).mp3`. And if the user picks the source anyway, we refuse with a new `same-as-source` message: "Choose a different name: this is the original file." "The same file" is `os.SameFile` when both paths exist (hard links, `..`, a different case on a case-insensitive disk all resolve), else a case-insensitive path comparison on macOS and Windows.

## 5. Valid files called "damaged"

**Symptom.** A FLAC or an AVI gets "This file is damaged or not finished yet. If it comes from Zoom, wait…".

**Why.** The slim ffmpeg (chapter 2) only has the demuxers we need: MOV/MP4, Matroska, MP3, AAC, WAV. To it, a valid FLAC is just bytes it cannot parse, and it says *Invalid data found when processing input*, the same words it uses for a damaged MP4.

**Fix.** A new `unsupported-format` kind: when ffmpeg rejects the input as invalid data and the extension is a known media format outside our set (avi, flac, ogg, oga, wma, wmv, asf, ts, mts, m2ts, aiff), we say so and list what works. Text and empty files keep their own messages. Note that `.3gp` is not on the list even though it seemed it should be: the MOV demuxer reads 3GP, so a 3GP that fails to open really is damaged.

Upstream of that, `Info.plist` declared `public.movie` and `public.audio`, so Finder offered AudioExtractor in "Open with" for every AVI and FLAC on the Mac. The document types now list only what we read: `public.mpeg-4`, `com.apple.m4v-video`, `com.apple.quicktime-movie`, `public.mpeg-4-audio`, `com.apple.m4a-audio`, `public.mp3`, `com.microsoft.waveform-audio` and `public.aac-audio`, plus a second entry with `CFBundleTypeExtensions` for `mkv` and `webm`, which have no system type identifier.

**Test.** A real FLAC and AVI made with Homebrew's ffmpeg. The test asks the ffmpeg under test for its demuxer list: with the slim build it expects `unsupported-format`, with a full build a plain success. Run it both ways (see "Testing" below).

## 6. An ffmpeg crash blamed on the file

**Why.** Probing runs `ffmpeg -i file` with no output, which always "fails". The code accepted any `*exec.ExitError` as "the probe ran" and classified the file from the log. A SIGSEGV or a kill is also an `ExitError`, and the half-written log then decided the verdict.

**Fix.** `probeRan(err)` accepts only a normal exit with a code from 1 to 255: 1 is "the input is fine, but you gave me no output", and other codes are the low byte of an ffmpeg error (183 for invalid data, 254 for a missing file). A signal, or a Windows exception code like `0xC0000005`, is a plain failure, with the log kept for Copy details. The review suggested accepting exit code 1 only, but a damaged file exits with 183, so that would have broken chapter 6's classification.

**Test.** Real `ExitError`s from `/bin/sh -c 'exit 183'` and `kill -SEGV $$`, and a fake "ffmpeg" script that prints *Invalid data* and then crashes: the result is `failed`, not `unreadable`.

## 7. A race at launch, and a refusal nobody saw

**Why.** For files the OS opened us with, `main.go` did `if svc.Busy() { show busy } else { go svc.Run(...) }`. Check-then-act: a window drop could start a job in between, and the launch batch was then refused silently by `Run`'s own lock. Likewise, the user clicking the drop zone in the first 500 ms (before "working" shows) got nothing at all.

**Fix.** The service owns a single check-and-claim, `begin()`, a `TryLock` that also creates the job's context. `Run`, `Pick` and a new `TryRun` (claim synchronously, run in the background) all go through it, and every refusal shows the same busy message through a new `Port.ShowInfo`. `Pick` claims the service *before* opening the picker, so a launch batch arriving while the picker is open is the one refused, out loud. `main.go` shrank to `svc.TryRun(paths, "")` for drops and launches.

**Test.** With the picker held open by the fake port, ten concurrent `TryRun`s and one `Run` are all refused, and the busy message is shown eleven times.

## 8. French typography and plurals

French puts a narrow no-break space (U+202F) before `:`, `;`, `?`, `!` and `%`, and inside `« »`. The catalog had plain spaces, which can wrap a lone colon onto the next line. A test now rejects a plain or a regular no-break space in those spots.

"1 réussis" was a plural bug, and the English "1 files done" too. Counted messages are now pairs, `count.done.one` and `count.done.other`, picked by `i18n.Plural(lang, n)`. French counts 0 as singular ("0 réussi"), English does not ("0 files done"). The Go service builds the summary's title and counts line, so the rule lives in one tested place, and the catalog test checks that every `.one` has its `.other`.

Last, the Windows Save dialog showed no title: Wails' `SetMessage` is what macOS displays, but Windows displays `Title`, and the v3 Save builder has no `SetTitle`. `SetOptions` sets both.

## Testing

The suite runs twice: once with the default (Homebrew) ffmpeg, and once with the slim build that ships.

```sh
env -u GOPATH go test -race ./...
AUDIOEXTRACTOR_FFMPEG=$PWD/internal/ffmpeg/bin/darwin/arm64/ffmpeg env -u GOPATH go test -race ./...
GOOS=windows env -u GOPATH go vet ./...
```

## Pitfalls

- **Words in a log are not diagnostics.** If you grep a tool's log, anchor on the shape of its messages, not on the words: the log quotes user data.
- **The OS confirmed a path, not yours.** Any change you make to a path after a Save dialog voids its "Replace?" prompt.
- **`ExitError` is not "exited".** Check `Exited()` and the code before trusting the output of a process that "failed as expected".
- **Check-then-act on a mutex is a race.** `Busy()` followed by `Run()` is two steps; `TryLock` and claim is one.
- **Hidden files on Windows refuse `CREATE_ALWAYS`.** If another program will open your file with truncation, hide it only if it can open without truncating.
