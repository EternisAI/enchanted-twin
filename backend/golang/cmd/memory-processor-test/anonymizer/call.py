import re
import json
import random
from tqdm import tqdm
from transformers import AutoTokenizer
from vllm import LLM, SamplingParams

MODEL_ID = "eternis/qwen1.7b-anonymizer-merged"

TASK_INSTRUCTION = """
You are an anonymizer. Your task is to identify and replace personally identifiable information (PII) in the given text.
Replace PII entities with semantically equivalent alternatives that preserve the context needed for a good response.
If no PII is found or replacement is not needed, return an empty replacements list.

REPLACEMENT RULES:
• Personal names: Replace private or small-group individuals. Pick same culture + gender + era; keep surnames aligned across family members. DO NOT replace globally recognised public figures (heads of state, Nobel laureates, A-list entertainers, Fortune-500 CEOs, etc.).
• Companies / organisations: Replace private, niche, employer & partner orgs. Invent a fictitious org in the same industry & size tier; keep legal suffix. Keep major public companies (anonymity set ≥ 1,000,000).
• Projects / codenames / internal tools: Always replace with a neutral two-word alias of similar length.
• Locations: Replace street addresses, buildings, villages & towns < 100k pop with a same-level synthetic location inside the same state/country. Keep big cities (≥ 1M), states, provinces, countries, iconic landmarks.
• Dates & times: Replace birthdays, meeting invites, exact timestamps. Shift day/month by small amounts while KEEPING THE SAME YEAR to maintain temporal context. DO NOT shift public holidays or famous historic dates ("July 4 1776", "Christmas Day", "9/11/2001", etc.). Keep years, fiscal quarters, decade references unchanged.
• Identifiers: (emails, phone #s, IDs, URLs, account #s) Always replace with format-valid dummies; keep domain class (.com big-tech, .edu, .gov).
• Monetary values: Replace personal income, invoices, bids by × [0.8 – 1.25] to keep order-of-magnitude. Keep public list prices & market caps.
• Quotes / text snippets: If the quote contains PII, swap only the embedded tokens; keep the rest verbatim.
/no_think
""".strip()


TOOLS_SCHEMA = [
    {
        "name": "replace_entities",
        "description": "Replace PII entities in the text with semantically equivalent alternatives that preserve context.",
        "parameters": {
            "replacements": {
                "description": "List of replacements to make. Each replacement has an 'original' field with the PII text and a 'replacement' field with the anonymized version.",
                "type": "array",
                "items": {
                    "type": "object",
                    "properties": {
                        "original": {
                            "description": "The original PII text to replace",
                            "type": "string",
                        },
                        "replacement": {
                            "description": "The anonymized replacement text",
                            "type": "string",
                        },
                    },
                    "required": ["original", "replacement"],
                },
            }
        },
    }
]


def build_model_and_tokenizer(model_name):
    """Load vLLM model and tokenizer"""
    tokenizer = AutoTokenizer.from_pretrained(model_name, trust_remote_code=True)
    model = LLM(
        model=model_name,
        trust_remote_code=True,
        tensor_parallel_size=1,
        gpu_memory_utilization=0.8,  # Reduced from 0.9 to leave more room for large batches
    )

    return model, tokenizer


def clean_model_response(completion):
    output = completion.split("assistant\n")[-1].strip()
    pattern = r"<tool_call>(.*?)</tool_call>"
    match = re.search(pattern, output, re.DOTALL)
    if match:
        return match.group(1).strip()
    else:
        return None


def call_model(model, tokenizer, input: str, max_new_tokens: int = 250):
    """Single input inference"""
    return call_model_batch(model, tokenizer, [input], max_new_tokens)[0]


def call_model_batch(model, tokenizer, inputs: list[str], max_new_tokens: int = 250):
    """Batched inference - the main improvement"""
    prompts = []
    for input_text in inputs:
        messages = [
            {"role": "system", "content": TASK_INSTRUCTION},
            {"role": "user", "content": input_text + "\n/no_think"},
        ]

        prompt = tokenizer.apply_chat_template(
            messages,
            tools=TOOLS_SCHEMA,
            tokenize=False,
            add_generation_prompt=True,
        )
        prompts.append(prompt)

    # vLLM sampling params - preserve your exact generation config
    sampling_params = SamplingParams(
        temperature=0.3,
        top_p=0.9,
        max_tokens=max_new_tokens,
        stop=["<|im_end|>"],  # equivalent to eos_token_id
    )

    # This is where the magic happens - true batching
    outputs = model.generate(prompts, sampling_params)
    return [clean_model_response(output.outputs[0].text) for output in outputs]


def load_jsonl(file_path):
    """Load a JSONL file and return a list of dictionaries"""
    data = []
    with open(file_path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line:  # Skip empty lines
                data.append(json.loads(line))
    return data


def run_on_dataset(model_id, dataset_path, output_path, sample_size=None, batch_size=8):
    model, tokenizer = build_model_and_tokenizer(model_id)
    data = load_jsonl(dataset_path)
    if sample_size is not None:
        random.seed(42)
        data = random.sample(data, sample_size)

    with open(output_path, "w", encoding="utf-8") as f:
        # Process in batches for true GPU parallelism
        for i in tqdm(range(0, len(data), batch_size), desc="Processing batches"):
            batch = data[i : i + batch_size]
            inputs = [item["input"] for item in batch]

            # Batched inference - much faster!
            model_outputs = call_model_batch(model, tokenizer, inputs)

            # Write results
            for item, output in zip(batch, model_outputs):
                item["model_output"] = output
                f.write(json.dumps(item) + "\n")
            f.flush()


if __name__ == "__main__":
    run_on_dataset(
        model_id=MODEL_ID,
        dataset_path="datasets/pii_test.jsonl",
        output_path="results/test.jsonl",
        sample_size=100,
        batch_size=100,
    )
