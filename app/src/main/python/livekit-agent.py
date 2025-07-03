import asyncio
import logging
import os
import sys
import json
import httpx
import requests
import aiohttp
from dataclasses import dataclass


from livekit.agents import (
    Agent,
    AgentSession,
    JobContext,
    WorkerOptions,
    cli,
)
import openai as oai
from livekit.plugins import openai, silero
from livekit.plugins.openai.utils import to_chat_ctx
from livekit.agents import APIConnectionError, llm
from livekit.agents import utils as agent_utils

# Configure logging with more detailed output
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)


# Global mute state management
class MuteManager:
    def __init__(self):
        self.is_muted = False

    def set_muted(self, muted):
        self.is_muted = muted
        logger.info(f"Agent {'muted' if muted else 'unmuted'}")

    def get_muted(self):
        return self.is_muted


# Global mute manager instance
mute_manager = MuteManager()


def report_state(state):
    """Report state changes to TypeScript"""
    import time

    state_update = {"state": state, "timestamp": int(time.time() * 1000)}
    print(f"STATE:{json.dumps(state_update)}", flush=True)


def handle_command(command):
    """Handle commands from TypeScript"""
    try:
        command_type = command.get("type")
        if command_type == "mute":
            mute_manager.set_muted(True)
        elif command_type == "unmute":
            mute_manager.set_muted(False)
        else:
            logger.warning(f"Unknown command type: {command_type}")
    except Exception as e:
        logger.error(f"Error handling command: {e}")


async def start_command_listener():
    """Start async command listener integrated with event loop"""
    import asyncio
    import sys

    async def async_command_listener():
        """Async command listener that doesn't block the main loop"""
        try:
            loop = asyncio.get_event_loop()
            reader = asyncio.StreamReader()
            protocol = asyncio.StreamReaderProtocol(reader)

            # Create async stdin reader
            await loop.connect_read_pipe(lambda: protocol, sys.stdin)

            logger.info("Async command listener started")

            while True:
                try:
                    # Read line with timeout to prevent blocking
                    line = await asyncio.wait_for(reader.readline(), timeout=0.1)
                    if not line:
                        break

                    line = line.decode().strip()
                    if line:
                        try:
                            command = json.loads(line)
                            handle_command(command)
                        except json.JSONDecodeError as e:
                            logger.error(
                                f"Invalid command JSON: {e} - Line: {repr(line)}"
                            )

                except asyncio.TimeoutError:
                    # No command received, continue loop
                    continue
                except Exception as e:
                    logger.error(f"Command listener error: {e}")
                    await asyncio.sleep(0.1)

        except Exception as e:
            logger.error(f"Async command listener setup error: {e}")

    # Start the async task
    asyncio.create_task(async_command_listener())


# Patch termios for non-TTY environments before importing livekit
if os.getenv("LIVEKIT_DISABLE_TERMIOS"):

    logger.info("Starting termios patching")
    import termios

    original_tcgetattr = termios.tcgetattr
    original_tcsetattr = termios.tcsetattr

    def patched_tcgetattr(fd):
        # Check if fd is actually a TTY before attempting termios operations
        if not os.isatty(fd):
            # Return properly formatted dummy terminal attributes for non-TTY
            # Structure: [iflag, oflag, cflag, lflag, ispeed, ospeed, cc]
            # cc needs to be a list with at least 32 elements for VMIN/VTIME access
            cc = [0] * 32  # Create list with 32 zero bytes
            return [0, 0, 0, 0, 0, 0, cc]
        try:
            return original_tcgetattr(fd)
        except OSError:
            # Fallback for edge cases - also needs proper structure
            cc = [0] * 32
            return [0, 0, 0, 0, 0, 0, cc]

    def patched_tcsetattr(fd, when, attrs):
        # Check if fd is actually a TTY before attempting termios operations
        if not os.isatty(fd):
            # Silently ignore for non-TTY
            return None
        try:
            return original_tcsetattr(fd, when, attrs)
        except OSError:
            # Silently ignore for non-TTY
            return None

    termios.tcgetattr = patched_tcgetattr
    termios.tcsetattr = patched_tcsetattr

    logger.info("Termios patching enabled for non-TTY environment")


# Verify required environment variables
required_env_vars = [
    "CHAT_ID",
    "TTS_API_KEY",
    "TTS_URL",
    "TTS_MODEL",
    "STT_API_KEY",
    "STT_URL",
    "STT_MODEL",
    "SEND_MESSAGE_URL"
]

missing_vars = [var for var in required_env_vars if not os.getenv(var)]
if missing_vars:
    logger.error(f"Missing required environment variables: {', '.join(missing_vars)}")
    logger.error("Please create a .env file with the required API keys")
    sys.exit(1)

TTS_API_KEY = os.getenv("TTS_API_KEY")
TTS_URL = os.getenv("TTS_URL")
TTS_MODEL = os.getenv("TTS_MODEL")

STT_API_KEY = os.getenv("STT_API_KEY")
STT_URL = os.getenv("STT_URL")
STT_MODEL = os.getenv("STT_MODEL")
CHAT_ID = os.getenv("CHAT_ID")
SEND_MESSAGE_URL = os.getenv("SEND_MESSAGE_URL")
GREETING = open(os.path.join(os.path.dirname(__file__), "greeting.txt")).read()


def get_onboarding_state():
    """Read the current onboarding state from file"""
    try:
        with open(
            os.path.join(os.path.dirname(__file__), "onboarding_state.txt"), "r"
        ) as f:
            return f.read().strip().lower() == "true"
    except FileNotFoundError:
        return False


def get_chat_history(chat_id: str):
    url = SEND_MESSAGE_URL

    query = """
    query getChatHistory($id: ID!) {
        getChat(id: $id) {
            messages {
                role
                text
            }
        }
    }
    """

    variables = {"id": chat_id}

    try:
        resp = requests.post(url, json={"query": query, "variables": variables})

        if resp.status_code == 200:
            body = resp.json()
            history = body["data"]["getChat"]["messages"]
            history = [item for item in history if item["role"].lower() != "system"]
            return history
        else:
            logger.warning(f"HTTP request failed with status code: {resp.status_code}")
            return []

    except requests.exceptions.RequestException as e:
        logger.error(f"Failed to fetch chat history: {e}")
        return []
    except (KeyError, TypeError, ValueError) as e:
        logger.error(f"Failed to parse chat history response: {e}")
        return []


async def send_message_stream(context, chat_id: str):
    """Stream messages from the GraphQL subscription"""
    import websockets
    import json

    ws_url = SEND_MESSAGE_URL.replace("http://", "ws://")
    is_onboarding = get_onboarding_state()

    subscription = """
    subscription streamMsg($chatId: ID!, $context: [MessageInput!]!, $isOnboarding: Boolean!) {
        processMessageHistoryStream(
            chatId: $chatId
            messages: $context
            isOnboarding: $isOnboarding
        ) {
            messageId
            chunk
            role
            isComplete
            createdAt
            imageUrls
        }
    }
    """

    variables = {"chatId": chat_id, "context": context, "isOnboarding": is_onboarding}

    try:
        async with websockets.connect(ws_url, subprotocols=["graphql-ws"]) as websocket:
            # Send connection init
            await websocket.send(json.dumps({"type": "connection_init"}))

            # Wait for connection ack
            response = await websocket.recv()
            ack = json.loads(response)
            if ack.get("type") != "connection_ack":
                logger.error(f"Expected connection_ack, got: {ack}")
                return

            # Send subscription
            start_message = {
                "id": "1",
                "type": "start",
                "payload": {"query": subscription, "variables": variables},
            }
            await websocket.send(json.dumps(start_message))

            # Stream chunks as they arrive
            async for message in websocket:
                data = json.loads(message)

                if data.get("type") == "data":
                    payload = (
                        data.get("payload", {})
                        .get("data", {})
                        .get("processMessageHistoryStream", {})
                    )
                    chunk = payload.get("chunk", "")
                    is_complete = payload.get("isComplete", False)

                    if chunk:
                        yield chunk  # Yield each chunk for streaming

                    if is_complete:
                        return  # End the generator
                elif data.get("type") == "error":
                    logger.error(f"GraphQL subscription error: {data}")
                    return  # End the generator on error
                elif data.get("type") == "complete":
                    return  # End the generator when complete

    except Exception as e:
        logger.error(f"WebSocket streaming error: {e}")
        # Fallback to HTTP mutation if WebSocket fails
        fallback_message = await send_message_fallback(context, chat_id)
        if fallback_message:
            yield fallback_message


async def send_message_fallback(context, chat_id: str):
    """Fallback to HTTP mutation if streaming fails"""
    url = SEND_MESSAGE_URL
    is_onboarding = get_onboarding_state()

    query = """
   mutation sendmsg($chatId: ID!, $context: [MessageInput!]!, $isOnboarding: Boolean!) {
    processMessageHistory(
      chatId: $chatId
      messages: $context
      isOnboarding: $isOnboarding
    ) {
        id
        text
        createdAt
      }
    }
    """

    variables = {"chatId": chat_id, "context": context, "isOnboarding": is_onboarding}

    async with aiohttp.ClientSession() as session:
        async with session.post(
            url, json={"query": query, "variables": variables}
        ) as resp:
            if resp.status == 200:
                body = await resp.json()
                return body["data"]["processMessageHistory"]["text"]
    return ""


@dataclass
class APIConnectOptions:
    max_retry: int = 3
    retry_interval: float = 2.0
    timeout: float = 10.0

    def __post_init__(self) -> None:
        if self.max_retry < 0:
            raise ValueError("max_retry must be greater than or equal to 0")

        if self.retry_interval < 0:
            raise ValueError("retry_interval must be greater than or equal to 0")

        if self.timeout < 0:
            raise ValueError("timeout must be greater than or equal to 0")

    def _interval_for_retry(self, num_retries: int) -> float:
        if num_retries == 0:
            return 0.1
        return self.retry_interval


class LLMStream(llm.LLMStream):
    def __init__(
        self, llm, chat_ctx: llm.ChatContext, chat_id: str, chat_history: list
    ) -> None:
        super().__init__(llm, chat_ctx=chat_ctx, tools=None, conn_options=None)
        self._chat_id = chat_id
        self._llm = llm
        self._conn_options = APIConnectOptions()
        self._chat_history = chat_history

    async def _run(self) -> None:
        # current function call that we're waiting for full completion (args are streamed)
        # (defined inside the _run method to make sure the state is reset for each run/attempt)
        self._oai_stream = None
        self._tool_call_id: str | None = None
        self._fnc_name: str | None = None
        self._fnc_raw_arguments: str | None = None
        self._tool_index: int | None = None
        retryable = True

        try:
            context = to_chat_ctx(self._chat_ctx, "1")
            context = [
                item
                for item in context
                if (item["role"] != "system" and item["content"] != "")
            ]
            context = [
                {"role": item["role"].upper(), "text": item["content"]}
                for item in context
            ]
            context = self._chat_history + context

            # Stream the response
            async for chunk_text in send_message_stream(context, self._chat_id):
                if chunk_text:
                    chunk = llm.ChatChunk(
                        id="1",
                        delta=llm.ChoiceDelta(content=chunk_text, role="assistant"),
                    )
                    self._event_ch.send_nowait(chunk)

        except Exception as e:
            raise APIConnectionError(retryable=retryable) from e


class LLM(llm.LLM):
    def __init__(self, chat_id: str) -> None:
        super().__init__()
        self._chat_id = chat_id
        self._chat_history = get_chat_history(chat_id)

    def chat(
        self,
        *,
        chat_ctx: llm.ChatContext,
        tools=None,
        conn_options=None,
        parallel_tool_calls=None,
        tool_choice=False,
        response_format=False,
        extra_kwargs=False,
    ):
        return LLMStream(
            llm=self,
            chat_ctx=chat_ctx,
            chat_id=self._chat_id,
            chat_history=self._chat_history,
        )


class MyAgentSession(AgentSession):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)

    @agent_utils.log_exceptions(logger=logger)
    async def _forward_audio_task(self) -> None:
        audio_input = self.input.audio
        if audio_input is None:
            return

        frame_count = 0
        async for frame in audio_input:
            if mute_manager.get_muted():
                # Yield control every 10 frames to prevent event loop blocking
                frame_count += 1
                if frame_count % 10 == 0:
                    await asyncio.sleep(0)
                continue  # Skip forwarding audio frames if muted

            frame_count = 0  # Reset when not muted
            if self._activity is not None:
                self._activity.push_audio(frame)


async def entrypoint(ctx: JobContext):
    """Main entry point for the LiveKit agent"""

    logger.info("Starting LiveKit agent entrypoint")

    # Start command listener for TypeScript communication
    await start_command_listener()

    # Connect to the room
    await ctx.connect()
    logger.info(f"Connected to LiveKit room: {ctx.room.name}")

    if os.getenv("FAKE_INIT") == "true":
        logger.info("Fake init enabled, sleeping for 1 second")
        await asyncio.sleep(1)
        logger.info("Fake init done, exiting")
        return

    # Create the agent with instructions
    agent = Agent(instructions="")
    logger.info("Agent created with instructions")

    # Create the agent session with voice components
    logger.info("Initializing voice components...")

    # Initialize components with better configuration
    client = oai.AsyncClient(
        max_retries=0,
        api_key=STT_API_KEY,
        base_url=STT_URL,
        http_client=httpx.AsyncClient(
            timeout=httpx.Timeout(connect=15.0, read=5.0, write=5.0, pool=5.0),
            follow_redirects=True,
            limits=httpx.Limits(
                max_connections=50,
                max_keepalive_connections=50,
                keepalive_expiry=120,
            ),
            headers={"X-BASE-URL": "https://audio-processing.model.tinfoil.sh/v1"},
        ),
    )
    vad = silero.VAD.load()
    stt = openai.STT(
        base_url=STT_URL, model=STT_MODEL, api_key=STT_API_KEY, client=client
    )
    llm = LLM(chat_id=os.getenv("CHAT_ID"))
    tts = openai.TTS(
        base_url=TTS_URL,
        model=TTS_MODEL,
        api_key=TTS_API_KEY,
        voice="af_v0bella",
        client=client,
    )

    logger.info("Voice components initialized")

    session = MyAgentSession(vad=vad, stt=stt, llm=llm, tts=tts)

    # State change handler - map and report LiveKit states to TypeScript
    def on_agent_state_changed(event):
        """Handle agent state changes and report them to TypeScript"""
        # Extract the new state from the event object
        new_state = (
            str(event.new_state).lower()
            if hasattr(event, "new_state")
            else str(event).lower()
        )

        # Map internal states to our defined states
        state_mapping = {
            "initializing": "initializing",
            "idle": "idle",
            "listening": "listening",
            "thinking": "thinking",
            "speaking": "speaking",
        }

        # Try to map the state, default to the original if not found
        mapped_state = state_mapping.get(new_state, new_state)
        if mapped_state in [
            "initializing",
            "idle",
            "listening",
            "thinking",
            "speaking",
        ]:
            report_state(mapped_state)

        logger.info(f"Agent state changed to: {event}")

    # Connect the state change handler
    session.on("agent_state_changed", on_agent_state_changed)

    # Start the session
    logger.info("Starting agent session...")
    await session.start(agent=agent, room=ctx.room)
    logger.info("Agent session started successfully")

    # Wait a moment for the session to fully initialize
    if GREETING:
        await session.say(GREETING)

    logger.info("Agent is now active and ready for conversation")


if __name__ == "__main__":
    logger.info("Starting LiveKit agent in console mode")
    cli.run_app(WorkerOptions(entrypoint_fnc=entrypoint))
