import tempfile

async def transcribe(file, model) -> dict:

    suffix = ".m4a"
    with tempfile.NamedTemporaryFile(suffix=suffix, delete=True) as temp_audio:
        content = await file.read()
        temp_audio.write(content)
        temp_audio.flush()

        # Run whisper transcription
        result = model.transcribe(temp_audio.name)

        return result