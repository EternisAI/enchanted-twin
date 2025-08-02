import json
import random
from typing import Dict, List, Tuple


def load_dataset(file_path: str) -> List[Dict]:
    """Load the JSONL dataset."""
    data = []
    with open(file_path, "r", encoding="utf-8") as f:
        for line in f:
            data.append(json.loads(line.strip()))
    return data


def make_conversation(dataset: List[Dict], turns: int) -> Tuple[str, Dict]:
    """
    Create a conversation with specified number of turns.

    Returns:
        - conversation: string with alternating "me:" and "you:" turns
        - answer_json: union of all answer_json dictionaries from sampled turns
    """
    # Sample turns number of questions
    sampled_data = random.sample(dataset, turns)

    # Create conversation string
    conversation_lines = []
    for i, data_point in enumerate(sampled_data):
        speaker = "me" if i % 2 == 0 else "you"
        conversation_lines.append(f"{speaker}: {data_point['input']}")
    conversation = "\n".join(conversation_lines)

    joint_answer_json = {}
    for data_point in sampled_data:
        joint_answer_json.update(data_point["answer_json"])

    return conversation, joint_answer_json


def generate_dataset(dataset: List[Dict], output_file: str):
    """Generate the new conversation dataset."""
    new_dataset = []

    for turns in [1, 2, 3, 4, 5]:
        print(f"Generating {turns}-turn conversations...")
        for i in range(100):
            conversation, answer_json = make_conversation(dataset, turns)

            new_data_point = {
                "input": conversation,
                "answer_json": answer_json,
                "answer_str": json.dumps(answer_json),
                "turns": turns,
                "id": f"{turns}turn_{i}",
            }
            new_dataset.append(new_data_point)

    with open(output_file, "w", encoding="utf-8") as f:
        for data_point in new_dataset:
            f.write(json.dumps(data_point, ensure_ascii=False) + "\n")

    print(f"Generated {len(new_dataset)} conversation examples")
    print(f"Saved to {output_file}")


def main(dataset_path, output_path):
    dataset = load_dataset(dataset_path)
    generate_dataset(dataset, output_path)


if __name__ == "__main__":
    main(
        dataset_path="pii_test_new.jsonl",
        output_path="conversation_dataset.jsonl",
    )
