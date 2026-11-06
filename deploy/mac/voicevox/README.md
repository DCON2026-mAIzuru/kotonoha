# VOICEVOX on the Mac

This dedicated Compose project runs official VOICEVOX Engine 0.26.1 on the existing OrbStack VM. The image is pinned to the Linux ARM64 manifest digest, so Apple Silicon does not need x86 emulation. It only publishes `127.0.0.1:50021`; do not add a public proxy route to the engine. Web clients use the authenticated Go endpoint `/api/v1/tts/voicevox`.

The engine uses two CPU threads and loads voice models on demand. Dictionary and settings mutation APIs are disabled. No Supabase services or volumes are managed here, and no paid service or new account is required.

```sh
python3 deploy/mac/voicevox/manage.py pull
python3 deploy/mac/voicevox/manage.py start
python3 deploy/mac/voicevox/manage.py status
# Save a short WAV and an accompanying credit file to a new path.
python3 deploy/mac/voicevox/manage.py sample --output /absolute/path/to/sample.wav
python3 deploy/mac/voicevox/manage.py stop
```

`start` checks the engine version and configured speaker before reporting readiness. `sample` uses the same query/synthesis sequence and speed as the backend and validates the WAV format, duration, nonzero samples, and clipping count. It reports timings without playing sound automatically. `stop` affects only this project's engine container. The container restarts with Docker unless explicitly stopped; start OrbStack first after a Mac restart.

The existing backend defaults already match:

```dotenv
VOICEVOX_ENGINE_URL=http://127.0.0.1:50021
VOICEVOX_SPEAKER_ID=13
VOICEVOX_SPEAKER_NAME=青山龍星
VOICEVOX_STYLE_NAME=ノーマル
VOICEVOX_SPEED_SCALE=1.12
VOICEVOX_TIMEOUT_SECONDS=30
```

The frontend's voice toggle requests audio from the authenticated backend and plays it in the browser. It does not need the engine URL. The backend first resolves the speaker name/style from `/speakers`, with ID 13 as its fallback. Initial synthesis may be slower while the model loads.

Verified on 2026-11-06: engine version 0.26.1, ARM64 image, and 青山龍星/ノーマル ID 13. The sample produced a 4.213-second, 24 kHz, 16-bit mono WAV with nonzero audio and no clipped samples; audio query took 1.119 seconds and synthesis took 2.438 seconds. Container memory after synthesis was approximately 320 MiB, excluding the OrbStack VM. These measurements do not establish browser playback or subjective audio quality; the public authenticated API and browser playback are checked separately.

## Credits and use conditions

Audio credit: **VOICEVOX:青山龍星**. Keep this credit with distributed samples and show it where users access the app's voice feature. VOICEVOX and the voice library have separate terms. The 青山龍星 terms require prior permission for use by companies, sole proprietors, or individuals contracted by companies, regardless of revenue; consult the linked terms for the applicable conditions. This deployment does not obtain that permission or publish audio on the user's behalf.

Official sources checked on 2026-11-06:

- [VOICEVOX Engine 0.26.1 release](https://github.com/VOICEVOX/voicevox_engine/releases/tag/0.26.1)
- [Official Docker image](https://hub.docker.com/r/voicevox/voicevox_engine/tags?name=cpu-arm64-ubuntu24.04-0.26.1)
- [Versioned engine instructions and API examples](https://github.com/VOICEVOX/voicevox_engine/blob/0.26.1/README.md)
- [VOICEVOX software terms](https://voicevox.hiroshiba.jp/term/)
- [青山龍星 voice-library terms](https://www.virvoxproject.com/voicevox%E3%81%AE%E5%88%A9%E7%94%A8%E8%A6%8F%E7%B4%84)
