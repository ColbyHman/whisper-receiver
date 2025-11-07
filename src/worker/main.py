import os
from redis import Redis 
import time
import whisper

from src.common.adapters.redis import get_redis_client

REDIS_HOST = os.getenv("REDIS_HOST", "localhost")
REDIS_PORT = int(os.getenv("REDIS_PORT", 6379))
REDIS_DB = int(os.getenv("REDIS_DB", 0))

r : Redis = get_redis_client(host=REDIS_HOST, port=REDIS_PORT,db=REDIS_DB)
model = whisper.load_model("small")

while True:
    job = r.brpop("whisper_queue", timeout=5)
    if not job:
        continue
    _, data = job
    job_id, idx, path = data.decode().split(":")
    print(f"Processing chunk {idx} of job {job_id}")

    result = model.transcribe(path)
    text = result["text"]

    r.rpush(f"whisper_result:{job_id}", f"{idx}:{text}")