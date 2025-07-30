async def transcribe(file_path, model) -> dict:

    print("Beginning transcription", flush=True)
    # Run whisper transcription
    result = model.transcribe(file_path)
    print("Finished transcription!", flush=True)
    return result