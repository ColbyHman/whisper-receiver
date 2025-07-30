from fastapi import FastAPI, File, UploadFile
import whisper
import tempfile

app = FastAPI()

# Load the Whisper model once (this can take some time, so do it globally)
model = whisper.load_model("medium")  # You can use tiny, base, small, medium, large

@app.get("/health")
async def health_check():
    """Health check endpoint"""
    return {"status": "healthy", "service": "audio-upload-service"}

@app.post("/transcribe/")
async def transcribe_audio(file: UploadFile = File(...)):
    # Save the uploaded file to a temp file
    suffix = ".m4a"
    with tempfile.NamedTemporaryFile(suffix=suffix, delete=True) as temp_audio:
        content = await file.read()
        temp_audio.write(content)
        temp_audio.flush()

        # Run whisper transcription
        result = model.transcribe(temp_audio.name)
        
    # Return the text transcription
    return {"transcription": result["text"]}

def run():
    import uvicorn
    uvicorn.run("journal_receiver.main:app", host="0.0.0.0", port=8000, reload=True)
