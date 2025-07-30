async def process_request(file):

    result = await transcribe(file, model)
    
    # Return the text transcription
    response = {"transcription": result["text"]}

    async with httpx.AsyncClient() as client:
        try:
            response.raise_for_status()
            response = await client.post(WEBHOOK_URL, json=response)
        except httpx.HTTPError as e:
            print(f"Webhook POST failed: {e}")