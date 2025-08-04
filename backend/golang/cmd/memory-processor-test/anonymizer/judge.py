import json
import os
from openai import OpenAI
from concurrent.futures import ThreadPoolExecutor
import dotenv
from tqdm import tqdm


dotenv.load_dotenv()

OPENAI_CONFIG = {
    "api_key": os.getenv("OPENROUTER_API_KEY"),
    "base_url": "https://openrouter.ai/api/v1",
    "model": "openai/gpt-4.1",
    "max_workers": 200,
    "timeout": 30,
}

openai_client = OpenAI(
    api_key=OPENAI_CONFIG["api_key"],
    base_url=OPENAI_CONFIG["base_url"],
    timeout=OPENAI_CONFIG["timeout"],
)


def make_input_prompt(question, answer, model_output):
    return f"""
[BEGIN ORIGINAL TEXT TO ANONYMIZE]
{question}
[END ORIGINAL TEXT TO ANONYMIZE]

[BEGIN REFERENCE ANSWER]
{answer}
[END REFERENCE ANSWER]

[BEGIN MODEL'S RESPONSE]
{model_output}
[END MODEL'S RESPONSE]
"""


ORIGINAL_PROMPT = """
Please evaluate the model's response based on:
1. **STRICT FORMAT VALIDATION**: The response must contain ONLY a tool call and nothing else.
   - MUST start with <tool_call> (no leading text, headers, explanations, or formatting)
   - MUST end with </tool_call> (no trailing text or explanations)
   - MUST contain only valid JSON between the tags
   - ANY additional text, headers (like "# Tools", "# Document"), explanations, or formatting = automatic 0/10
   - **SPECIAL RULE**: If no PII is present, ONLY the tool call format with empty replacements gets a score of 1: '<tool_call>{"name":"replace_entities","arguments":{"replacements":[]}}</tool_call>' (whitespace variations acceptable)
2. **PII Identification**: Did it correctly identify PII that should be replaced?
3. **Replacement Granularity**: Does it replace ONLY specific PII entities (names, numbers, addresses) and NOT entire sentences or large text blocks?
4. **Replacement Quality**: Are replacements semantically appropriate and preserve context?
   - **Name/Organization replacements**: Must be culturally similar (same culture, gender, era). Generic placeholders like "Person1" or "AnonUser" should receive 0 points.
   - **Date replacements**: Should only shift day/month while keeping the SAME YEAR to maintain temporal context. Year changes should be penalized.
5. **Following Rules**: Did it follow PII replacement rules (keep public figures, appropriate substitutions, etc.)?

Use the reference answer as a guide for evaluating the model's response.
- The model should have replaced all of the PII terms included in the reference answer.
- The model may have made distinct replacements for the same PII term, but they should be of comparable quality in terms of preserving semantic similarity.

Score from 0-10 where:
- 10: Perfect JSON format + excellent targeted PII replacement following all rules
- 8-9: Good format + good targeted replacements with minor issues
- 6-7: Acceptable but missed some PII or minor format/rule issues  
- 4-5: Major issues with format, identification, or replacement quality
- 2-3: Over-broad replacement (replacing entire sentences/phrases instead of specific PII)
- 0-1: Poor format or completely wrong PII handling

**CRITICAL RULES**: 
1. AUTOMATIC 0/10 if response contains ANY text besides the tool call (headers, explanations, formatting, etc.)
2. Penalize heavily if the model replaces entire sentences, phrases, or large blocks of text instead of just the specific PII entities
3. For no-PII cases, only accept the exact empty replacements format - reject any text explanations
4. Date replacements that change the year should be heavily penalized
5. Name replacements that are not culturally similar should receive 0 points
6. For no-PII cases with correct format, always give score 10/10
""".strip()


SEMANTIC_PROMPT = """
Evaluate ONLY the semantic quality of replacements in these 4 categories:

**1. NAMES & ORGANIZATIONS** (Score 0 or 1):
- Personal names: Private individuals MUST be replaced with same culture + gender + era names. If family members share surnames, replacements should too.
- Organizations: Private/niche orgs should be replaced with fictitious orgs in same industry & size tier, keeping legal suffixes.
- PUBLIC FIGURES & MAJOR ORGS: Do NOT penalize if globally recognized public figures (heads of state, celebrities, historical figures) or major companies (Fortune 500, tech giants) are NOT replaced - this is correct behavior.
- Score 1 if: (a) replacements are culturally/semantically appropriate with same culture+gender+era, OR (b) public entities are correctly left unchanged, OR (c) no names/orgs appear in text
- Score 0 if: (a) private entities use generic placeholders like "AnonUser123" or "[PERSON_1]", OR (b) name replacements are NOT culturally similar (e.g., "John Smith" → "Li Wei" or "Maria" → "Bob")

**2. MONETARY VALUES** (Score 0 or 1):
- Personal amounts (salaries, invoices, bills) should multiply by factor 0.8-1.25 to preserve order of magnitude (e.g., $50,000 → $42,000 or $61,000)
- PUBLIC PRICES: Do NOT penalize if public list prices, market caps, or well-known amounts are NOT replaced - this is correct behavior.
- Score 1 if: (a) private amounts use realistic scaling, OR (b) public amounts are correctly left unchanged, OR (c) no monetary values appear
- Score 0 ONLY if: private amounts use generic placeholders like "[AMOUNT_1]" or "SecureAmount123"

**3. DATES & TIMES** (Score 0 or 1):
- Personal dates (birthdays, appointments, private events) should shift day/month while KEEPING THE SAME YEAR to maintain temporal context
- CRITICAL: Date replacements that change the year should receive score 0 (e.g., "9th December 1977" → "14th February 2038" is WRONG)
- HISTORICAL/PUBLIC DATES: Do NOT penalize if famous historical dates, holidays, or public events are NOT replaced (e.g., "July 4 1776", "Christmas Day", "9/11/2001") - this is correct behavior.
- Score 1 if: (a) private dates shift day/month but KEEP SAME YEAR, OR (b) public/historical dates are correctly left unchanged, OR (c) no dates appear
- Score 0 if: (a) private dates use generic placeholders like "[DATE_1]" or "AnonDate123", OR (b) dates change the year when they shouldn't

**4. JSON FORMAT VALIDATION** (Score 0 or 1):
- The response must be a complete tool call with proper structure
- Must start with <tool_call> and end with </tool_call> tags
- Must contain valid JSON syntax between the tags
- Valid examples:
  * Empty case: <tool_call>{"name":"replace_entities","arguments":{"replacements":[]}}</tool_call>
  * With PII: <tool_call>{"name":"replace_entities","arguments":{"replacements":[{"original":"John","replacement":"Mike"}]}}</tool_call>
- Score 1 if: properly formatted tool call with valid JSON syntax and complete structure
- Score 0 if: missing tags, malformed JSON, incomplete structure, unnecessary tags, regular sentences, or any syntax errors

Only evaluate categories that appear in the original text for categories 1-3. Category 4 (JSON format) is always evaluated. If categories 1-3 don't appear, automatically give them score 1.

**IMPORTANT**: Do NOT penalize the model for correctly leaving public/well-known entities unchanged (e.g., "Apple Inc.", "Christmas Day", "Barack Obama"). The replacement rules only apply to private/personal information.
""".strip()

ORIGINAL_SCHEMA = {
    "name": "evaluate_original_quality",
    "description": "Evaluate overall PII anonymization quality",
    "parameters": {
        "type": "object",
        "properties": {
            "score": {
                "type": "number",
                "minimum": 0,
                "maximum": 10,
                "description": "Overall score from 0-10",
            }
        },
        "required": ["score"],
    },
}

SEMANTIC_SCHEMA = {
    "name": "evaluate_semantic_quality",
    "description": "Evaluate semantic quality of PII replacements",
    "parameters": {
        "type": "object",
        "properties": {
            "names_orgs_score": {
                "type": "number",
                "minimum": 0,
                "maximum": 1,
                "description": "Score for names and organizations (0 or 1)",
            },
            "monetary_score": {
                "type": "number",
                "minimum": 0,
                "maximum": 1,
                "description": "Score for monetary values (0 or 1)",
            },
            "dates_times_score": {
                "type": "number",
                "minimum": 0,
                "maximum": 1,
                "description": "Score for dates and times (0 or 1)",
            },
            "json_format_score": {
                "type": "number",
                "minimum": 0,
                "maximum": 1,
                "description": "Score for JSON format validation (0 or 1)",
            },
        },
        "required": [
            "names_orgs_score",
            "monetary_score",
            "dates_times_score",
            "json_format_score",
        ],
    },
}


def make_original_call(input_prompt):
    original_evaluation_prompt = input_prompt + "\n\n" + ORIGINAL_PROMPT

    return openai_client.chat.completions.create(
        model=OPENAI_CONFIG["model"],
        messages=[
            {
                "role": "system",
                "content": "You are an expert evaluator for PII anonymization tasks. Evaluate both JSON format correctness and overall quality of PII replacements.",
            },
            {"role": "user", "content": original_evaluation_prompt},
        ],
        tools=[{"type": "function", "function": ORIGINAL_SCHEMA}],
        tool_choice={
            "type": "function",
            "function": {"name": "evaluate_original_quality"},
        },
        temperature=0.1,
        max_tokens=500,
    )


def make_semantic_call(input_prompt):
    semantic_evaluation_prompt = input_prompt + "\n\n" + SEMANTIC_PROMPT

    return openai_client.chat.completions.create(
        model=OPENAI_CONFIG["model"],
        messages=[
            {
                "role": "system",
                "content": "You are an expert evaluator for semantic quality of PII replacements. Focus only on the three specific categories.",
            },
            {"role": "user", "content": semantic_evaluation_prompt},
        ],
        tools=[{"type": "function", "function": SEMANTIC_SCHEMA}],
        tool_choice={
            "type": "function",
            "function": {"name": "evaluate_semantic_quality"},
        },
        temperature=0.1,
        max_tokens=500,
    )


def evaluate_single_response(idx, question, answer, model_output):
    """Evaluate a single completion using the LLM judge with both original and semantic metrics"""
    try:
        if model_output is None or not model_output.strip():
            return {
                "original_score": 0,
                "semantic_scores": {
                    "names_orgs": 0,
                    "monetary": 0,
                    "dates": 0,
                    "json_format": 0,
                },
            }

        input_prompt = make_input_prompt(question, answer, model_output)

        with ThreadPoolExecutor(
            max_workers=min(OPENAI_CONFIG["max_workers"], 2)
        ) as executor:
            original_future = executor.submit(make_original_call, input_prompt)
            semantic_future = executor.submit(make_semantic_call, input_prompt)

            original_response = original_future.result()
            semantic_response = semantic_future.result()

        original_score = 0
        semantic_scores = {"names_orgs": 0, "monetary": 0, "dates": 0, "json_format": 0}

        if original_response.choices[0].message.tool_calls:
            args = json.loads(
                original_response.choices[0].message.tool_calls[0].function.arguments
            )
            original_score = args.get("score", 0)

        if semantic_response.choices[0].message.tool_calls:
            args = json.loads(
                semantic_response.choices[0].message.tool_calls[0].function.arguments
            )
            semantic_scores["names_orgs"] = args.get("names_orgs_score", 0)
            semantic_scores["monetary"] = args.get("monetary_score", 0)
            semantic_scores["dates"] = args.get("dates_times_score", 0)
            semantic_scores["json_format"] = args.get("json_format_score", 0)

        return {"original_score": original_score, "semantic_scores": semantic_scores}

    except Exception as e:
        print(f"[LLM Judge] Error evaluating sample {idx}: {e}")
        return {
            "original_score": 0,
            "semantic_scores": {
                "names_orgs": 0,
                "monetary": 0,
                "dates": 0,
                "json_format": 0,
            },
        }


def evaluate_responses_parallel(inputs):
    """Evaluate multiple responses in parallel. Returns dict mapping input_id -> result."""
    with ThreadPoolExecutor(max_workers=OPENAI_CONFIG["max_workers"]) as executor:
        futures = {
            executor.submit(
                evaluate_single_response,
                data["id"],
                data["input"],
                data["answer_str"],
                f"<tool_call>{data['model_output']}</tool_call>",
            ): (idx, data)
            for idx, data in inputs.items()
        }

        results = {}
        for future in tqdm(futures, desc="Evaluating"):
            idx, data = futures[future]
            results[data["input"]] = future.result()

    return results


def compute_averages(results):
    """Compute summary averages across all results."""
    if not results:
        return {}

    original_scores = [result["original_score"] for result in results.values()]
    semantic_categories = ["names_orgs", "monetary", "dates", "json_format"]

    averages = {
        "original_avg": sum(original_scores) / len(original_scores),
        "semantic_avgs": {
            cat: sum(result["semantic_scores"][cat] for result in results.values())
            / len(results)
            for cat in semantic_categories
        },
    }

    return averages


def evaluate_from_json(input_file, output_file):
    """Load inputs from JSONL, evaluate in parallel, compute averages, and save results."""
    print(f"Loading inputs from {input_file}")
    inputs = {}
    with open(input_file, "r") as f:
        for i, line in enumerate(f):
            inputs[i] = json.loads(line.strip())

    print(f"Evaluating {len(inputs)} samples...")
    results = evaluate_responses_parallel(inputs)
    averages = compute_averages(results)

    print(f"Saving results to {output_file}")
    with open(output_file, "w") as f:
        # Write averages as first line
        json.dump({"averages": averages, "total_samples": len(inputs)}, f)
        f.write("\n")
        # Write each result as a line, preserving all original input fields
        for _, data in inputs.items():
            if data["input"] in results:
                output_record = dict(data)
                output_record.update(results[data["input"]])
                json.dump(output_record, f)
                f.write("\n")

    print(f"Done! Original avg: {averages['original_avg']:.2f}")
    print(f"Semantic avgs: {averages['semantic_avgs']}")

    return {"results": results, "averages": averages}


if __name__ == "__main__":
    evaluate_from_json(
        input_file="results/test.jsonl",
        output_file="results/test_judged.jsonl",
    )
