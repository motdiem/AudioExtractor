AudioExtractor for Windows - first launch
=========================================

AudioExtractor pulls the audio out of a Zoom recording. Drop the video on the
window, choose where to save, done. Everything happens on your PC: nothing is
uploaded anywhere.

1. NO INSTALLATION
   AudioExtractor.exe is the whole app. No installer, no administrator rights.
   Unzip it and keep it anywhere you like, for example in Documents. Double-click
   to start it. (Dropping a video on the .exe icon also works.)

2. THE FIRST TIME: "WINDOWS PROTECTED YOUR PC"
   The app is not signed with a paid certificate, so Windows SmartScreen shows
   a blue window. It is safe, and this only happens until Windows has seen the
   file a few times.

   a) Click "More info" (the small link in the blue window).
   b) Click "Run anyway".

   If your company blocks this, ask your IT team, or right-click the .exe >
   Properties > tick "Unblock" at the bottom > OK, then open it again.

3. USE IT
   Drop an MP4 (or a whole Zoom folder) on the window. Pick M4A (instant, best
   quality) or MP3 (smaller, works everywhere).

   If a message says WebView2 is missing, follow the link in the message
   (Microsoft's free component, already on most Windows 10/11 PCs).

WHERE THINGS ARE KEPT
   - Your audio files: only where you choose to save them.
   - A small helper program (ffmpeg), unpacked on first use:
       %LocalAppData%\AudioExtractor
   - Your settings (last format, window position):
       %AppData%\AudioExtractor
   (Paste these into the File Explorer address bar.)

TO REMOVE EVERYTHING
   Delete AudioExtractor.exe, then delete the two folders above.

LICENCES
   AudioExtractor includes the open-source programs FFmpeg and LAME. Their
   licences are in THIRD_PARTY_NOTICES.txt (next to the .exe, and behind the
   "Licences" link in the window).

Problem? Tell the person who sent you the app. The version number is at the
bottom right of the window.
