# 7. Batches, folder drops and Zoom names

Goal of this chapter: drop or pick several recordings, or a whole folder, and get one audio file per recording in a folder of your choice, with "file 2 of 5" progress and a summary of anything that failed. Zoom's generic file names (`video1234567890.mp4`) become the meeting's name. A single file keeps exactly the flow from chapter 4.

## The flow

```
paths (drop, picker, launch args)
  -> expandInputs: folders -> media files, dedupe
  -> 0 items: "empty folder" error     1 item: Save dialog (as before)     2+ items: folder dialog
  -> items run one after another, "file N of M"
  -> summary screen (batches only)
```

Everything lives in `internal/extract`; the window only shows states. New pieces:

- `inputs.go`: folder expansion, Zoom naming, collision suffixes (all pure and tested).
- `Service.runBatch`: the sequential loop. `runOne` (single file) and `runBatch` share `extractItem`, which runs ffmpeg for one item and returns an `Outcome`.
- `Port.ChooseFolder(FolderRequest{Title, Dir})`: the native folder dialog. The fake port in tests returns a temp folder.
- A new `summary` phase in `StateEvent`, with `Done`, `Failed`, `Stopped` and `Failures`.

## Expanding folders

`expandInputs` walks the inputs in order:

- A **file** is always kept, whatever its extension. If someone drops `notes.txt`, the error classification from chapter 6 explains it ("not a recording") instead of the app silently ignoring it.
- A **folder** becomes its **top-level** media files, sorted by name case-insensitively. Media means `mp4 mov m4v mkv webm m4a mp3 wav aac`. Hidden files (a leading dot, which also covers macOS `._*` companions and our own `.audioextractor-*.part` temp files) and everything else are ignored. Sub-folders are not entered.
- **Duplicates** (same cleaned path) are dropped, so a file dropped together with its own folder runs once.
- A folder with nothing usable, with nothing else dropped, gives the `empty-folder` error. In a mixed drop an empty folder is just ignored.

A folder holding one MP4 therefore becomes a single item and opens the Save dialog, like dropping the file.

Windows "hidden" file attributes are not checked, only the dot prefix; Zoom folders do not use them.

## Several inputs: the folder dialog and collisions

With two or more items the Save dialog would mean N dialogs, so we ask for one folder (`SetDirectory` starts it in the first source's folder). Outputs use the suggested names. There is no replace prompt in batch mode, so we never overwrite: `uniqueName` appends ` (2)`, ` (3)`... when the name exists in the folder or was already used earlier in the same batch (compared case-insensitively, since macOS and Windows file systems are).

The picker is multi-select (`PromptForMultipleSelection`). On macOS it can also pick folders; the Windows dialog cannot mix files and folders, so on Windows folders arrive by drop (and, since chapter 8, "Open with").

## Zoom naming

`SuggestName(src, format)` is used by both flows (the Save dialog's pre-filled name and batch outputs). Zoom's local recordings are named generically, so a folder of meetings becomes `video1.mp4`, `video2.mp4`... in a Save dialog. The rule:

1. The file name must match, case-insensitively, one of: `video\d+\.mp4`, `audio\d+\.m4a`, `zoom_\d+\.mp4`, `GMT\d{8}-\d{6}_Recording.*\.mp4`.
2. **and** the parent folder must look like a Zoom meeting folder: its name starts with `YYYY-MM-DD HH.MM.SS` (Zoom's own folder naming, e.g. `2026-11-20 10.00.00 Weekly sync`), or it contains another Zoom generic file or `audio_only.m4a` next to this one.

Then the suggested name is the folder's name. Anything else keeps its own base name. So a lone `video123.mp4` in Downloads is not renamed to "Downloads". The folder-derived name has Windows-invalid characters (`<>:"/\|?*` and control characters) stripped, plus trailing dots and spaces, on every OS, so the file moves safely between a Mac and a PC. Two files from the same meeting (`video1.mp4` and `audio1.m4a`) get the same name; the collision rule makes the second `... (2)`.

## Running, failures and Cancel

Items run strictly one after another, each emitting `working` with its `Index` and `Total`; the window shows "File 2 of 5" above the progress bar. A failed item never stops the batch: its `Outcome` carries the kind and the one-sentence message from chapter 6.

At the end a `summary` event arrives: the title ("3 files done" or "3 done, 2 failed"), a list of the failed files each with its message (scrolls if long), **Show in Finder/Explorer**, **Copy details** and **Extract another**. Show reveals the first output file, which opens the destination folder with it selected (so no new port method). **Copy details** now covers every failed item: one block per file (header plus its ffmpeg log), joined by a separator line.

**Cancel** kills the current ffmpeg (the temp file is removed, chapter 5), starts no more items and leaves finished outputs in place. We chose to *report* this: the summary reads "Stopped after 2 of 5" with the done/failed counts. If nothing had finished yet, the window silently returns to idle like a single-file cancel. A single item keeps its old behaviour, including the full error screen.

## Tests

All with real ffmpeg and a fake port: folder expansion (top level, hidden and non-media ignored, sorted, explicit files kept, dedupe), an empty folder, a folder with one MP4 using the Save dialog, Zoom names (positive and negative cases, the sibling rule, sanitising, the Save dialog), collision suffixes against existing files and within the batch, a mixed batch (good, silent, text file, good) with the right outcomes, summary and details, "file N of M" events, cancel mid-batch (finished output kept, no `.part` file, the next item never started) and cancelling the folder dialog.

Fixtures need `lavfi` and a video encoder, which the slim bundled ffmpeg deliberately lacks. So tests take two binaries: `AUDIOEXTRACTOR_FIXTURE_FFMPEG` (default: `ffmpeg` on PATH) generates and inspects clips, and `AUDIOEXTRACTOR_FFMPEG` (default: PATH) is what the service runs. To test what ships:

```sh
AUDIOEXTRACTOR_FFMPEG=$PWD/internal/ffmpeg/bin/darwin/arm64/ffmpeg env -u GOPATH go test ./...
```

## Pitfalls

- A zero-byte file is `unreadable`, not `not-media`; give a "text file" test fixture some text.
- The Windows open dialog cannot select files and folders together; do not set both flags there.
- Do not pre-check the Zoom folder rule by file name alone: `video1.mp4` in Downloads is common and must keep its name.
- Cancel is observed between items too (`ctx.Err()` before starting the next), not only inside ffmpeg.
