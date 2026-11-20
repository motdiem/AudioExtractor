AudioExtractor for Mac - first launch
=====================================

AudioExtractor pulls the audio out of a Zoom recording. Drop the video on the
window, choose where to save, done. Everything happens on your Mac: nothing is
uploaded anywhere. Works on Apple Silicon and Intel Macs.

1. INSTALL
   - Double-click AudioExtractor-mac.zip if it did not unzip by itself.
   - Drag AudioExtractor into your Applications folder (optional: it runs
     from anywhere, for example Documents or the Desktop).

2. OPEN IT THE FIRST TIME
   The app is not signed by an Apple-registered developer, so your Mac will
   stop you the first time. It is safe, and you only do this once.

   a) Double-click AudioExtractor. macOS says it cannot verify the app.
      Click "Done" (do NOT click "Move to Trash").
   b) Open the Apple menu > System Settings > Privacy & Security.
   c) Scroll down to the "Security" section. You will see a message about
      "AudioExtractor" being blocked. Click "Open Anyway".
   d) Confirm with your password or Touch ID, then click "Open Anyway" again.

   (The exact wording of the messages changes a little between macOS
   versions. On older Macs - macOS 14 or earlier - it is simpler: right-click
   the app, choose "Open", then click "Open".)

   If the button does not appear, open the Terminal app and paste:
     xattr -dr com.apple.quarantine /Applications/AudioExtractor.app
   (adjust the path if you put the app somewhere else), then open it again.

3. USE IT
   Drop an MP4 (or a whole Zoom folder) on the window, or on the app's icon.
   Pick M4A (instant, best quality) or MP3 (smaller, works everywhere).

WHERE THINGS ARE KEPT
   - Your audio files: only where you choose to save them.
   - A small helper program (ffmpeg), unpacked on first use:
       ~/Library/Caches/AudioExtractor
   - Your settings (last format, window position):
       ~/Library/Application Support/AudioExtractor
   (~ is your home folder. In Finder: Go > Go to Folder..., paste the path.)

TO REMOVE EVERYTHING
   Drag AudioExtractor to the Trash, then delete the two folders above.

LICENCES
   AudioExtractor includes the open-source programs FFmpeg and LAME. Their
   licences are in THIRD_PARTY_NOTICES.txt (next to this file, inside the app
   under Contents/Resources, and behind the "Licences" link in the window).

Problem? Tell the person who sent you the app. The version number is at the
bottom right of the window.
