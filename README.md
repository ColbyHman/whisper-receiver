# Whisper Based Transcriber

## What is this?
This is a microservice that is meant to take in audio files, transcribe them, and either return the data as a response or dump the response into MongoDB.

The initial version of this microservice was only meant to return the transcription, but the addition of sending the output to a database is intended to prevent request timeouts.

## What is the purpose of this?
This microservice is currently used in conjunction with my [journal-receiver](https://github.com/ColbyHman/journal-receiver) to transcribe and summarize audio-based journal entries I create.