import re
import torch
import json
import random
from tqdm import tqdm
from transformers import AutoTokenizer, AutoModelForCausalLM

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


def build_model_and_tokenizer(model_name, device: str = "cuda"):
    """Load model and tokenizer with optional adapter"""
    tokenizer = AutoTokenizer.from_pretrained(model_name, trust_remote_code=True)

    model = AutoModelForCausalLM.from_pretrained(model_name, trust_remote_code=True).to(
        device
    )
    model.eval()

    if torch.backends.cuda.is_built():
        try:
            model = torch.compile(model)
        except Exception:
            pass

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
    messages = [
        {"role": "system", "content": TASK_INSTRUCTION},
        {"role": "user", "content": input + "\n/no_think"},
    ]

    prompt = tokenizer.apply_chat_template(
        messages,
        tools=TOOLS_SCHEMA,
        tokenize=False,
        add_generation_prompt=True,
    )

    input = tokenizer(prompt, return_tensors="pt", padding=True, truncation=True).to(
        "cuda"
    )

    with torch.no_grad():
        gen_cfg = dict(
            max_new_tokens=max_new_tokens,
            temperature=0.3,
            do_sample=True,
            top_p=0.9,
            eos_token_id=tokenizer.convert_tokens_to_ids("<|im_end|>"),
            pad_token_id=tokenizer.eos_token_id,
            num_return_sequences=1,
            use_cache=True,
        )
        out_ids = model.generate(
            input_ids=input["input_ids"],
            attention_mask=input["attention_mask"],
            **gen_cfg,
        )

    decoded = tokenizer.batch_decode(out_ids, skip_special_tokens=True)
    return clean_model_response(decoded[0])


def load_jsonl(file_path):
    """Load a JSONL file and return a list of dictionaries"""
    data = []
    with open(file_path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line:  # Skip empty lines
                data.append(json.loads(line))
    return data


def run_on_dataset(model_id, dataset_path, output_path, sample_size=None):
    model, tokenizer = build_model_and_tokenizer(model_id)
    data = load_jsonl(dataset_path)
    if sample_size is not None:
        random.seed(42)
        data = random.sample(data, sample_size)

    with open(output_path, "w", encoding="utf-8") as f:
        for item in tqdm(data, desc="Processing questions"):
            model_output = call_model(model, tokenizer, input=item["input"])
            item["model_output"] = model_output
            f.write(json.dumps(item) + "\n")
            f.flush()


if __name__ == "__main__":
    run_on_dataset(
        model_id=MODEL_ID,
        dataset_path="datasets/pii_test.jsonl",
        output_path="results/test.jsonl",
        sample_size=100,
    )
