import tempfile

async def transcribe(file_path, model) -> dict:

    with open(file_path, "rb") as f:
        audio_bytes = f.read()

        # Run whisper transcription
        result = model.transcribe(audio_bytes)

        return result