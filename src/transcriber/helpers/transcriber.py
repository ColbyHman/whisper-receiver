import tempfile

async def transcribe(file_path, model) -> dict:

    # Run whisper transcription
    result = model.transcribe(file_path)

    return result