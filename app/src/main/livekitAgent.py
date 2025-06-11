import asyncio
import logging
import os
import sys
from dotenv import load_dotenv
from livekit.agents import (
    Agent,
    AgentSession,
    JobContext,
    RunContext,
    WorkerOptions,
    cli,
    function_tool,
)
from livekit.plugins import deepgram, openai, silero

# Load environment variables
load_dotenv()

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Verify required environment variables
required_env_vars = [
    "OPENAI_API_KEY"
]

missing_vars = [var for var in required_env_vars if not os.getenv(var)]
if missing_vars:
    logger.error(f"Missing required environment variables: {', '.join(missing_vars)}")
    logger.error("Please create a .env file with the required API keys")
    sys.exit(1)

# Default prompt - can be overridden by meta_prompt2.txt
PROMPT = """You are a helpful AI assistant with voice capabilities. 
You can have natural conversations and help users with various tasks.
Be friendly, concise, and engaging in your responses."""

# Try to load custom prompt if available
try:
    with open("meta_prompt2.txt", "r") as f:
        PROMPT = f.read()
        logger.info("Loaded custom prompt from meta_prompt2.txt")
except FileNotFoundError:
    logger.info("Using default prompt (meta_prompt2.txt not found)")

async def entrypoint(ctx: JobContext):
    """Main entry point for the LiveKit agent"""
    
    # Connect to the room
    await ctx.connect()
    logger.info("Connected to LiveKit room")
    
    # Create the agent with instructions and tools
    agent = Agent(instructions=PROMPT)
    
    # Create the agent session with voice components
    session = AgentSession(
        vad=silero.VAD.load(),                    # Voice Activity Detection
        stt=openai.STT(),         # Speech-to-Text
        llm=openai.LLM(model="gpt-4o-mini"),     # Large Language Model
        tts=openai.TTS(),           # Text-to-Speech
    )
    
    # Start the session
    await session.start(agent=agent, room=ctx.room)
    logger.info("Agent session started")
    
    # Generate an initial greeting
    await session.generate_reply(
        instructions="Greet the user warmly and let them know you're ready to help. Keep it brief and friendly."
    )

if __name__ == "__main__":
    # Run the agent with CLI support
    cli.run_app(WorkerOptions(entrypoint_fnc=entrypoint)) 