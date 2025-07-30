from fastapi import FastAPI, File, UploadFile, BackgroundTasks, status
from dotenv import load_dotenv
import httpx
import whisper
import os

from adapters.mongo import get_db_collection, insert
from helpers.transcriber import transcribe

load_dotenv()

app = FastAPI()

USE_DATABASE = os.getenv("USE_DB", "false").lower() == "true"
DATABASE_URL = os.getenv("DB_URL", "")
DATABASE = os.getenv("DATABASE")
WEBHOOK_URL = os.getenv("WEBHOOK_URL", "")

model = whisper.load_model("medium")

async def process_request(file):
    """Process Transcription Request in the Background"""
    result = await transcribe(file, model)
    
    # Return the text transcription
    response = {"transcription": result["text"]}

    async with httpx.AsyncClient() as client:
        try:
            response = await client.post(WEBHOOK_URL, json=response)
            response.raise_for_status()
        except httpx.HTTPError as e:
            print(f"Webhook POST failed: {e}")


@app.get("/health")
async def health_check():
    """Health check endpoint"""
    return {"status": "healthy", "service": "audio-upload-service"}

@app.post("/transcribe/", status_code=status.HTTP_201_CREATED)
async def transcribe_audio(background_tasks: BackgroundTasks, file: UploadFile = File(...)):
    """Transcribe Audio Endpoint"""
    
    background_tasks.add_task(process_request, file)
    return {"detail":"File received"}

def run():
    import uvicorn
    uvicorn.run("journal_receiver.main:app", host="0.0.0.0", port=8000, reload=True)
