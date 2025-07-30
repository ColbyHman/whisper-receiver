async def transcribe(file_path, model) -> dict:

    print("Beginning transcription")
    # Run whisper transcription
    result = model.transcribe(file_path)
    print("Finished transcription!")
    return result