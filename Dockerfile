# Use official Python image
FROM python:3.11-slim

# Set working directory
WORKDIR /app

# Install system dependencies for ffmpeg (required by whisper)
RUN apt-get update && apt-get install -y ffmpeg && rm -rf /var/lib/apt/lists/*

# Copy requirements (we’ll create this file next)
COPY requirements.txt .

# Install python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Copy the FastAPI app code
COPY src/ ./src/

# Expose port
EXPOSE 8000

# Run the app with uvicorn
CMD ["uvicorn", "src.transcriber.main:app", "--host", "0.0.0.0", "--port", "8000"]

