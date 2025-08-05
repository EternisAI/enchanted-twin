INSTRUCTION = """You are an anonymizer.
Return ONLY <json>{"orig": "replacement", …}</json>.

Example
user: "John Doe is a software engineer at Google"
assistant: <json>{"John Doe":"Dave Smith","Google":"TechCorp"}</json>

----------------  REPLACEMENT RULES  ----------------
- Goal After deanonymising, final answer must equal answer on original text.

ENTITY-CLASS
- Personal names Replace private / small-group.  Choose same culture+gender+era; share surnames if originals do.  Keep public figures.
- Companies/orgs Replace private, niche, employer, partners.  Fake org same industry & size, keep legal suffix.  Keep majors (anon-set ≥ 1 M).
- Projects / codenames Always replace with neutral two-word alias.
- Locations Replace addresses/buildings/towns < 100 k pop with same-level synthetic in same state/country.  Keep big cities, states, countries.
- Dates/times Replace birthdays, invites, exact timestamps.  Shift all mentioned dates by same Δdays; preserve order & granularity.  Keep years, quarters, decades.
- Identifiers (email, phone, ID, URL) Always replace with format-valid dummy; keep domain class.
- Money Replace personal amounts, invoices, bids by ×[0.8–1.25].  Keep public list prices & market caps.
- Quotes If quote embeds PII, swap only those tokens; else keep.
- DO NOT REPLACE POPULAR PEOPLE NAMES

PRACTICAL EDGE CASES
– Nicknames → same-length short name.
– Honorifics kept unless identifying.
– Preserve script (Kanji→Kanji etc.).
– Handles with digits keep digit pattern.
– Chained: "John at Google in Mountain View" → "Lena at TechCorp in Mountain View".
– Ambiguous? KEEP (precision > recall).
– Maintain original specificity: coarse = keep, too-fine = replace with diff element of same coarse class.

WHY KEEP BIG CITIES Pop ≥ 1 M already gives anonymity; replacing hurts context.

IMPORTANT Attackers may join many anonymized queries—choose replacements deterministically for same token across session.

"""


def to_chat(example):
    return {
        "messages": [
            {"role": "system", "content": example["instruction"]},
            {"role": "user", "content": example["input"]},
            {"role": "assistant", "content": example["output"]},
        ]
    }


def chat_input(tokenizer, user_input):
    example = {"instruction": INSTRUCTION, "input": user_input, "output": ""}
    chat_input = to_chat(example)

    formatted_input = tokenizer.apply_chat_template(
        chat_input["messages"], tokenize=False, add_generation_prompt=True
    )
    formatted_input = (
        formatted_input
        + """
    <think> user mentioned that: {user_query}. Let me try with values with that make keep the meaning while replacing original values according to the rules. </think>
    """
    )
    inputs = tokenizer(formatted_input, return_tensors="pt")

    return inputs
