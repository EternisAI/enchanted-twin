import json
import sys
import os
import re
import argparse
from pathlib import Path
from tqdm import tqdm
from call import call_model, build_model_and_tokenizer, MODEL_ID


def load_chunks(chunks_file):
    """Load X_1 chunks from JSONL file"""
    chunks = []
    with open(chunks_file, "r", encoding="utf-8") as f:
        for line_num, line in enumerate(f, 1):
            line = line.strip()
            if line:
                try:
                    chunk = json.loads(line)
                    chunks.append(chunk)
                except json.JSONDecodeError as e:
                    print(f"⚠️  Failed to parse line {line_num}: {e}")
                    print(f"   Line content: {line[:100]}...")
                    continue
    return chunks


def create_virtual_shards(chunk, max_chars=500):
    """Create virtual shards from a chunk for better anonymization quality"""
    messages = chunk.get("conversation", [])

    if not messages:
        return []

    virtual_shards = []
    current_shard_messages = []
    current_char_count = 0

    for msg in messages:
        msg_content = msg.get("content", "").strip()
        if not msg_content:
            continue

        msg_char_count = len(msg_content)

        # If this single message exceeds max_chars, split it by words
        if msg_char_count > max_chars:
            # First, finish current shard if it has content
            if current_shard_messages:
                shard_content = " | ".join(
                    [
                        m.get("content", "")
                        for m in current_shard_messages
                        if m.get("content", "").strip()
                    ]
                )
                if shard_content.strip():
                    virtual_shards.append(shard_content)
                current_shard_messages = []
                current_char_count = 0

            # Split the long message by words, never breaking words
            words = msg_content.split()
            current_chunk = []
            current_chunk_chars = 0

            for word in words:
                word_len = len(word) + (1 if current_chunk else 0)  # +1 for space
                if current_chunk_chars + word_len > max_chars and current_chunk:
                    # Finalize current chunk
                    virtual_shards.append(" ".join(current_chunk))
                    current_chunk = [word]
                    current_chunk_chars = len(word)
                else:
                    current_chunk.append(word)
                    current_chunk_chars += word_len

            # Add final chunk
            if current_chunk:
                virtual_shards.append(" ".join(current_chunk))

        else:
            # Normal message - check if adding it would exceed limit
            if (
                current_char_count > 0
                and current_char_count + msg_char_count + 3 > max_chars
            ):  # +3 for " | "
                # Finalize current shard
                if current_shard_messages:
                    shard_content = " | ".join(
                        [
                            m.get("content", "")
                            for m in current_shard_messages
                            if m.get("content", "").strip()
                        ]
                    )
                    if shard_content.strip():
                        virtual_shards.append(shard_content)

                # Start new shard with this message
                current_shard_messages = [msg]
                current_char_count = msg_char_count
            else:
                # Add to current shard
                separator_chars = (
                    3 if current_shard_messages else 0
                )  # +3 for " | " if not first message
                current_shard_messages.append(msg)
                current_char_count += msg_char_count + separator_chars

    # Don't forget the last shard
    if current_shard_messages:
        shard_content = " | ".join(
            [
                m.get("content", "")
                for m in current_shard_messages
                if m.get("content", "").strip()
            ]
        )
        if shard_content.strip():
            virtual_shards.append(shard_content)

    return virtual_shards


def parse_model_output(model_output):
    """Parse model output JSON string into replacement dict"""
    if not model_output or not model_output.strip():
        return {}

    try:
        # Ensure proper encoding - decode if needed
        if isinstance(model_output, bytes):
            model_output = model_output.decode("utf-8", errors="replace")

        # Parse as JSON first
        data = json.loads(model_output)

        # Handle {"name": "replace_entities", "arguments": {"replacements": [...]}} format
        if isinstance(data, dict) and data.get("name") == "replace_entities":
            arguments = data.get("arguments", {})
            replacements = arguments.get("replacements", [])
            result = {}
            for item in replacements:
                original = item.get("original", "")
                replacement = item.get("replacement", "")
                if original and replacement:
                    result[original] = replacement
            return result

        # Handle direct {"replacements": [...]} format
        if isinstance(data, dict) and "replacements" in data:
            replacements = data["replacements"]
            result = {}
            for item in replacements:
                original = item.get("original", "")
                replacement = item.get("replacement", "")
                if original and replacement:
                    result[original] = replacement
            return result

    except (json.JSONDecodeError, KeyError, TypeError, UnicodeDecodeError) as e:
        print(f"⚠️  Failed to parse model output: {e}")
        print(f"Raw output: {repr(model_output[:200])}...")

    # Fallback: try to extract from replace_entities(...) format
    try:
        if "replace_entities" in model_output:
            match = re.search(
                r"replace_entities\s*\(\s*({.*})\s*\)", model_output, re.DOTALL
            )
            if match:
                json_str = match.group(1)
                data = json.loads(json_str)
                replacements = data.get("replacements", [])
                result = {}
                for item in replacements:
                    original = item.get("original", "")
                    replacement = item.get("replacement", "")
                    if original and replacement:
                        result[original] = replacement
                return result
    except (json.JSONDecodeError, KeyError, TypeError, UnicodeDecodeError):
        pass

    return {}


def union_replacement_dicts(dict_list):
    """Union replacement dicts with first-wins conflict resolution"""
    result = {}
    for d in dict_list:
        for key, value in d.items():
            if key not in result:  # First wins
                result[key] = value
    return result


def apply_replacements_to_chunk(chunk, replacement_lookup):
    """Apply replacement lookup to all message content in a chunk"""
    if not replacement_lookup:
        return chunk

    # Create a copy of the chunk
    anonymized_chunk = chunk.copy()

    # Apply replacements to each message content
    if "conversation" in anonymized_chunk:
        anonymized_conversation = []
        for msg in anonymized_chunk["conversation"]:
            anonymized_msg = msg.copy()
            content = anonymized_msg.get("content", "")

            # Apply all replacements - sort by length (longest first) to avoid partial replacements
            sorted_replacements = sorted(
                replacement_lookup.items(), key=lambda x: len(x[0]), reverse=True
            )
            for original, replacement in sorted_replacements:
                if original and replacement:
                    # For single characters, use word boundaries to avoid destroying words
                    if len(original) == 1 and original.isalpha():
                        # Only replace if it's a standalone word (surrounded by word boundaries)
                        pattern = r"\b" + re.escape(original) + r"\b"
                        content = re.sub(pattern, replacement, content)
                    else:
                        # For multi-character strings, use normal replacement
                        content = content.replace(original, replacement)

            anonymized_msg["content"] = content
            anonymized_conversation.append(anonymized_msg)

        anonymized_chunk["conversation"] = anonymized_conversation

    return anonymized_chunk


def anonymize_chunks(chunks_file, output_file, model_id=MODEL_ID, shard_length=500):
    """Anonymize chunks using virtual sharding and apply to full chunks"""
    print(f"🔥 Loading model: {model_id}")
    print(f"📏 Using shard length: {shard_length} characters")
    model, tokenizer = build_model_and_tokenizer(model_id)

    print(f"📄 Loading chunks from: {chunks_file}")
    chunks = load_chunks(chunks_file)

    print(f"🚀 Processing {len(chunks)} chunks...")

    anonymized_chunks = []

    chunk_progress = tqdm(chunks, desc="Processing chunks", position=0)
    for chunk_idx, chunk in enumerate(chunk_progress):
        chunk_id = chunk.get("id", f"chunk_{chunk_idx}")
        chunk_progress.set_description(f"Processing chunk {chunk_id}")

        # Create virtual shards for better anonymization quality
        virtual_shards = create_virtual_shards(chunk, max_chars=shard_length)

        if not virtual_shards:
            # Empty chunk, just add replacement_lookup field
            anonymized_chunk = chunk.copy()
            anonymized_chunk["replacement_lookup"] = {}
            anonymized_chunks.append(anonymized_chunk)
            continue

        # Run anonymization on each virtual shard
        shard_replacement_dicts = []
        for shard_idx, shard_content in enumerate(virtual_shards):
            if shard_content.strip():
                chunk_progress.set_description(
                    f"Processing chunk {chunk_id} - shard {shard_idx + 1}/{len(virtual_shards)}"
                )
                model_output = call_model(model, tokenizer, input=shard_content)
                replacement_dict = parse_model_output(model_output)
                if replacement_dict:
                    shard_replacement_dicts.append(replacement_dict)

        # Union all replacement dicts with first-wins
        consolidated_replacements = union_replacement_dicts(shard_replacement_dicts)

        # Apply replacements to the entire original chunk
        anonymized_chunk = apply_replacements_to_chunk(chunk, consolidated_replacements)

        # Add the replacement_lookup field
        anonymized_chunk["replacement_lookup"] = consolidated_replacements

        anonymized_chunks.append(anonymized_chunk)

    chunk_progress.close()

    # Save anonymized chunks with explicit UTF-8 encoding
    with open(output_file, "w", encoding="utf-8") as f:
        for chunk in anonymized_chunks:
            f.write(json.dumps(chunk, ensure_ascii=False) + "\n")

    print(f"✅ Anonymization complete! Results saved to: {output_file}")
    print(
        f"📊 Processed {len(chunks)} chunks with {sum(len(c.get('replacement_lookup', {})) for c in anonymized_chunks)} total replacements"
    )


def find_chunks_file_by_type(type_filter=None):
    """Find X_1 chunks file with optional type filtering"""
    pipeline_dir = Path("../pipeline_output")
    if not pipeline_dir.exists():
        return None

    chunk_files = list(pipeline_dir.glob("X_1_*.jsonl"))
    if not chunk_files:
        return None

    if type_filter:
        # Filter by type (fuzzy substring matching)
        type_filter = type_filter.lower()
        matching_files = []
        for file in chunk_files:
            filename = file.name.lower()
            if type_filter in filename:
                matching_files.append(file)

        if not matching_files:
            print(f"⚠️  No X_1 chunks files found matching type: {type_filter}")
            print(f"Available files: {[f.name for f in chunk_files]}")
            return None

        if len(matching_files) > 1:
            print(
                f"⚠️  Multiple files match type filter, using first: {matching_files[0].name}"
            )

        return str(matching_files[0])

    # Return the most recent one if no type filter
    return str(sorted(chunk_files)[-1])


def generate_output_filename(input_file):
    """Generate X_2_{source}_anon.jsonl filename from X_1 input"""
    input_path = Path(input_file)
    base_name = input_path.stem

    # Split on underscore and reconstruct
    parts = base_name.split("_")

    if (
        len(parts) >= 4
        and parts[0] == "X"
        and parts[1] == "1"
        and parts[-1] == "chunks"
    ):
        # X_1_{source}_chunks -> extract middle parts
        source = "_".join(parts[2:-1])
    elif len(parts) >= 3 and parts[0] == "X" and parts[1] == "1":
        # X_1_{source} -> take everything after X_1
        source = "_".join(parts[2:])
    else:
        # Fallback
        source = "unknown"

    output_filename = f"X_2_{source}_anon.jsonl"
    output_path = input_path.parent / output_filename
    return str(output_path)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="Anonymize chunks using virtual sharding"
    )
    parser.add_argument("chunks_file", nargs="?", help="Path to X_1 chunks file")
    parser.add_argument(
        "--type", help="Type filter for auto-detection (e.g., whats, gmail, tel)"
    )
    parser.add_argument(
        "--shard-length",
        type=int,
        default=500,
        help="Maximum characters per virtual shard (default: 500)",
    )

    args = parser.parse_args()

    # Determine input file
    if args.chunks_file:
        chunks_file = args.chunks_file
        if not os.path.exists(chunks_file):
            print(f"❌ Chunks file not found: {chunks_file}")
            sys.exit(1)
    else:
        # Auto-detect chunks file
        chunks_file = find_chunks_file_by_type(args.type)
        if not chunks_file:
            print("❌ No X_1 chunks file found in ../pipeline_output/")
            print(
                "💡 Usage: python anonymize.py [path_to_chunks_file.jsonl] [--type TYPE]"
            )
            sys.exit(1)
        print(f"🔍 Auto-detected chunks file: {chunks_file}")

    # Generate output filename
    output_file = generate_output_filename(chunks_file)

    print(f"🎯 Input: {chunks_file}")
    print(f"🎯 Output: {output_file}")

    anonymize_chunks(chunks_file, output_file, shard_length=args.shard_length)
