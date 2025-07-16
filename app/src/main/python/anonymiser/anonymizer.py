import os
import json
import torch
from to_chat import chat_input
from pydantic import BaseModel
from typing import Any, Optional, Tuple
from fastapi import FastAPI, HTTPException
from transformers import AutoModelForCausalLM, AutoTokenizer
import uvicorn

print("Starting anonymizer service...")

app = FastAPI(title="Enchanted Anonymizer")

device = torch.device("mps" if torch.backends.mps.is_available() else "cpu")
print(f"Using device: {device}")

model_name = os.environ.get(
    "MODEL_PATH", "eternis/eternis_anonymizer_merge_Qwen3-0.6B_9jul_30k"
)
print(f"Loading model from: {model_name}")

try:
    tokenizer = AutoTokenizer.from_pretrained(model_name)
    print("Tokenizer loaded successfully")

    model = AutoModelForCausalLM.from_pretrained(model_name).to(device).eval()
    model.config.use_cache = True
    print("Model loaded successfully")
except Exception as e:
    print(f"Error loading model: {e}")
    import traceback

    traceback.print_exc()
    exit(1)


class GenerateRequest(BaseModel):
    prompt: str
    max_new_tokens: int | None = 1000


@app.post("/generate")
async def generate(req: GenerateRequest):
    try:
        tensor_inputs = {
            k: v.to(device) for k, v in chat_input(tokenizer, req.prompt).items()
        }

        max_attempts = 10
        text = ""

        for attempt in range(max_attempts):
            with torch.no_grad():
                outputs = model.generate(
                    **tensor_inputs,
                    max_new_tokens=req.max_new_tokens,
                )

            text = tokenizer.decode(
                outputs[0, tensor_inputs["input_ids"].shape[1] :],
                skip_special_tokens=True,
            )

            if not text:
                print(f"Attempt {attempt + 1}: not found")
                text = tokenizer.decode(
                    outputs[0],
                    skip_special_tokens=True,
                )
            else:
                print(f"Attempt {attempt + 1}: found")
                break

        extracted_json, parsed_json = extract_first_json(text)
        return {
            "genetated": text,
            "extracted": extracted_json,
            "parsed": parsed_json,
        }

    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc


def extract_first_json(raw: str) -> Tuple[Optional[str], Optional[Any]]:
    decoder = json.JSONDecoder()
    for i, ch in enumerate(raw):
        if ch in "{[":
            try:
                obj, end = decoder.raw_decode(raw, i)
                return raw[i:end], obj
            except json.JSONDecodeError:
                pass
    return None, None


if __name__ == "__main__":
    print("Starting FastAPI server on port 8000...")
    try:
        uvicorn.run(app, host="0.0.0.0", port=8000)
    except Exception as e:
        print(f"Error starting server: {e}")
        import traceback

        traceback.print_exc()
        exit(1)
